package grpc

import (
	"context"
	"log/slog"
	"net/http"
	"sync"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/gorilla/websocket"
	"google.golang.org/protobuf/proto"

	"ngac-platform/pkg/grpcauth"
	"ngac-platform/pkg/policyclient"
	pb "ngac-platform/proto/messaging"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool { return true },
}

const authTimeout = 5 * time.Second

// CloseNotMember is the WebSocket close code for "you no longer belong to the
// workspace this connection is for". The client must not reconnect with the same
// token (the browser's reconnect loop would otherwise hammer the hub with a token
// that is still valid for another 15 minutes); it asks for a fresh one once.
const CloseNotMember = 4403

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
	// viewing is the conversation this session has on screen, "" for none.
	// Guarded by hub.mu.
	viewing string

	// While a workspace replay runs, live frames of that workspace are set
	// aside in held (hub_replay.go) so the replay reaches the session first.
	holdMu sync.Mutex
	holdWS string
	held   [][]byte
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

	// A token names a tenant and outlives the membership it was issued for, so
	// a person removed from the workspace could reconnect with it. Ask the graph.
	if claims.TenantID != "" {
		if code, reason := c.hub.checkMember(claims); code != 0 {
			c.sendAuthResponse(false, "", reason)
			c.closeWith(code, reason)
			return false
		}
	}

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

// checkMember asks whether the token's person still belongs to its tenant. It
// returns 0 when they do, else the close code to refuse them with: CloseNotMember
// when the graph says they do not, and "try again later" when it could not say, so
// an outage neither lets anybody in nor bans anybody.
func (h *Hub) checkMember(claims *WSClaims) (int, string) {
	h.mu.RLock()
	members := h.members
	h.mu.RUnlock()
	if members == nil {
		slog.Error("websocket refused: no membership check is configured", "tenant_id", claims.TenantID)
		return websocket.CloseTryAgainLater, "membership cannot be checked"
	}
	who := grpcauth.Caller{UserID: claims.UserID, NGACNodeID: claims.NGACNodeID, TenantID: claims.TenantID}
	ctx, cancel := context.WithTimeout(grpcauth.WithCaller(h.ctx, who), subscribeCheckTimeout)
	defer cancel()
	ok, err := members.IsWorkspaceMember(ctx, claims.NGACNodeID, claims.TenantID)
	if err != nil && (policyclient.IsNotFound(err) || policyclient.IsNotMembersUA(err)) {
		// The workspace has no Members UA to be under (it is gone): nobody belongs.
		return CloseNotMember, "not a member of this workspace"
	}
	if err != nil {
		slog.Warn("websocket refused: membership could not be checked",
			"user_id", claims.UserID, "tenant_id", claims.TenantID, "error", err)
		return websocket.CloseTryAgainLater, "membership cannot be checked"
	}
	if !ok {
		slog.Info("websocket refused: not a member of the workspace",
			"user_id", claims.UserID, "tenant_id", claims.TenantID)
		return CloseNotMember, "not a member of this workspace"
	}
	return 0, ""
}

// closeWith ends the connection with a close frame carrying code and reason, so
// the client can tell why from the code. The frame is a control write, which may
// be made while writePump is writing.
func (c *Client) closeWith(code int, reason string) {
	_ = c.conn.WriteControl(websocket.CloseMessage, websocket.FormatCloseMessage(code, reason), time.Now().Add(time.Second))
	_ = c.conn.Close()
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

		// Text frames carry JSON control messages (sync cursors); every
		// domain frame stays binary protobuf.
		if msgType == websocket.TextMessage {
			c.handleTextMessage(message)
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

	case *pb.ClientEnvelope_View:
		if !c.authenticated {
			return
		}
		// Viewing holds back push for the conversation, so it is only accepted
		// for one the session is subscribed to — which already required read.
		channelID := payload.View.ChannelId
		if channelID != "" && !c.hub.isSubscribed(channelID, c) {
			c.sendError(403, "not subscribed to this channel")
			return
		}
		c.hub.setViewing(c, channelID)
	}
}

func (c *Client) writePump() {
	defer c.conn.Close()
	for msg := range c.send {
		kind := websocket.BinaryMessage
		if isControl(msg) {
			kind = websocket.TextMessage
		}
		if err := c.conn.WriteMessage(kind, msg); err != nil {
			return
		}
	}
}
