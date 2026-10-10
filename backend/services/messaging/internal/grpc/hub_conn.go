package grpc

import (
	"log/slog"
	"net/http"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/gorilla/websocket"
	"google.golang.org/protobuf/proto"

	"ngac-platform/pkg/grpcauth"
	pb "ngac-platform/proto/messaging"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool { return true },
}

const authTimeout = 5 * time.Second

// Client represents a connected WebSocket user.
type Client struct {
	conn       *websocket.Conn
	userID     string
	username   string
	ngacNodeID string
	// tenantID is the tenant the session's JWT was issued for. Presence and
	// approval events are confined to it; empty means the session belongs to
	// no tenant and receives no tenant-scoped events.
	tenantID string
	hub      *Hub
	// expiry closes the session when the token it authenticated with expires.
	expiry        *time.Timer
	send          chan []byte
	authenticated bool
	jwtSecret     string
}

// WSClaims are JWT claims for WebSocket authentication.
type WSClaims struct {
	UserID     string `json:"user_id"`
	Username   string `json:"username"`
	NGACNodeID string `json:"ngac_node_id"`
	TenantID   string `json:"tenant_id,omitempty"`
	jwt.RegisteredClaims
}

// HandleWebSocket returns an HTTP handler that upgrades to WebSocket.
// Auth is done via first message (ClientEnvelope{auth}) instead of URL query param.
func (h *Hub) HandleWebSocket(jwtSecret string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			slog.Warn("websocket upgrade failed", "error", err)
			return
		}

		client := &Client{
			conn:      conn,
			hub:       h,
			send:      make(chan []byte, 256),
			jwtSecret: jwtSecret,
		}

		go client.writePump()
		go client.readPump()
	}
}

// sendError sends a protobuf ErrorEvent to the client.
func (c *Client) sendError(code int32, message string) {
	env := &pb.ServerEnvelope{
		Payload: &pb.ServerEnvelope_Error{
			Error: &pb.ErrorEvent{Code: code, Message: message},
		},
	}
	data := marshalEnvelope(env)
	if data != nil {
		select {
		case c.send <- data:
		default:
		}
	}
}

// handleAuth validates JWT from the first client message and authenticates.
func (c *Client) handleAuth(req *pb.AuthRequest) bool {
	token, err := jwt.ParseWithClaims(req.Token, &WSClaims{},
		func(t *jwt.Token) (interface{}, error) { return []byte(c.jwtSecret), nil },
		// Same rules as httputil.JWTMiddleware: pin HS256 so the HMAC secret
		// the keyfunc returns is never applied under another algorithm, and
		// require exp so a token cannot be a permanent credential.
		jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Alg()}),
		jwt.WithExpirationRequired(),
	)
	if err != nil {
		c.sendAuthResponse(false, "", "invalid token")
		return false
	}
	claims := token.Claims.(*WSClaims)

	c.userID = claims.UserID
	c.username = claims.Username
	c.ngacNodeID = claims.NGACNodeID
	c.tenantID = claims.TenantID
	c.authenticated = true
	// A session is good for as long as the token it proved itself with; the
	// client reconnects with its refreshed one.
	if claims.ExpiresAt != nil {
		c.expiry = time.AfterFunc(time.Until(claims.ExpiresAt.Time), func() {
			c.sendError(401, "token expired")
			c.conn.Close()
		})
	}

	// Track client by userID
	c.hub.mu.Lock()
	if c.hub.users[claims.UserID] == nil {
		c.hub.users[claims.UserID] = make(map[*Client]bool)
	}
	c.hub.users[claims.UserID][c] = true
	c.hub.mu.Unlock()

	c.sendAuthResponse(true, claims.UserID, "")
	c.hub.userCameOnline(c)
	return true
}

// sendAuthResponse sends an AuthResponse to the client.
func (c *Client) sendAuthResponse(ok bool, userID, reason string) {
	env := &pb.ServerEnvelope{
		Payload: &pb.ServerEnvelope_AuthResponse{
			AuthResponse: &pb.AuthResponse{Ok: ok, UserId: userID, Reason: reason},
		},
	}
	data := marshalEnvelope(env)
	if data != nil {
		select {
		case c.send <- data:
		default:
		}
	}
}

func (c *Client) readPump() {
	defer func() {
		if c.expiry != nil {
			c.expiry.Stop()
		}
		c.hub.UnsubscribeAll(c)
		// Presence is per tenant, so a session still open in another tenant
		// does not keep the user "online" here. The absence is announced after
		// a grace period (userLeft), so a reload does not flicker.
		if c.authenticated && c.hub.sessionsOf(c.tenantID, c.userID) == 0 {
			c.hub.userLeft(c)
		}
		c.conn.Close()
	}()

	// Auth timeout: client must authenticate within 5 seconds
	authTimer := time.NewTimer(authTimeout)
	defer authTimer.Stop()

	go func() {
		<-authTimer.C
		if !c.authenticated {
			c.sendError(401, "auth timeout")
			c.conn.Close()
		}
	}()

	for {
		msgType, message, err := c.conn.ReadMessage()
		if err != nil {
			break
		}

		// Binary frame = protobuf
		if msgType == websocket.BinaryMessage {
			c.handleBinaryMessage(message, authTimer)
			continue
		}

		// Text frames were the retired JSON protocol; they are logged and dropped.
		if msgType == websocket.TextMessage {
			slog.Warn("received text WebSocket frame, ignoring (binary protobuf only)", "userID", c.userID)
		}
	}
}

// handleBinaryMessage processes a protobuf-encoded ClientEnvelope.
func (c *Client) handleBinaryMessage(data []byte, authTimer *time.Timer) {
	var env pb.ClientEnvelope
	if err := proto.Unmarshal(data, &env); err != nil {
		c.sendError(400, "invalid protobuf message")
		return
	}

	switch payload := env.Payload.(type) {
	case *pb.ClientEnvelope_Auth:
		if c.handleAuth(payload.Auth) {
			authTimer.Stop()
		}

	case *pb.ClientEnvelope_Subscribe:
		if !c.authenticated {
			c.sendError(401, "not authenticated")
			return
		}
		if ws := payload.Subscribe.WorkspaceId; ws != "" {
			c.handleWorkspaceSubscribe(ws)
			return
		}
		channelID := payload.Subscribe.ChannelId
		if err := c.hub.authorizeSubscribe(channelID, grpcauth.Caller{UserID: c.userID, NGACNodeID: c.ngacNodeID, TenantID: c.tenantID}); err != nil {
			slog.Warn("websocket subscribe denied",
				"user_id", c.userID, "channel_id", channelID, "error", err)
			c.sendError(403, "not allowed to subscribe to this channel")
			return
		}
		c.hub.subscribe(channelID, c)

	case *pb.ClientEnvelope_Unsubscribe:
		if !c.authenticated {
			c.sendError(401, "not authenticated")
			return
		}
		if ws := payload.Unsubscribe.WorkspaceId; ws != "" {
			c.hub.unsubscribeWorkspace(ws, c)
			return
		}
		c.hub.Unsubscribe(payload.Unsubscribe.ChannelId, c)

	case *pb.ClientEnvelope_Typing:
		if !c.authenticated {
			return
		}
		// Typing goes out under the sender's name to everyone in the channel,
		// so it is only accepted for a channel the sender is subscribed to —
		// which already required read on it.
		if !c.hub.isSubscribed(payload.Typing.ChannelId, c) {
			c.sendError(403, "not subscribed to this channel")
			return
		}
		c.hub.broadcastTyping(payload.Typing.ChannelId, c.username, c)
	}
}

func (c *Client) writePump() {
	defer c.conn.Close()
	for msg := range c.send {
		if err := c.conn.WriteMessage(websocket.BinaryMessage, msg); err != nil {
			return
		}
	}
}
