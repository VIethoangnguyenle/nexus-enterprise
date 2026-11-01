package grpc

import (
	"context"
	"log/slog"
	"net/http"
	"sync"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/gorilla/websocket"
	"golang.org/x/time/rate"
	"google.golang.org/protobuf/proto"

	"ngac-platform/pkg/grpcauth"
	"ngac-platform/pkg/policyclient"
	pb "ngac-platform/proto/messaging"
)

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

	// ip is the client address the connection holds a slot for; preAuth says
	// it still holds the unauthenticated one too. Touched by readPump only.
	ip      string
	preAuth bool
	// limits are the hub's limits when the connection opened. limiter is the
	// frame budget: the connection's own until it authenticates, then the one
	// its user's sessions share. Touched by readPump only.
	limits  ConnLimits
	limiter *rate.Limiter
	// viewing is the conversation this session has on screen, "" for none.
	// Guarded by hub.mu.
	viewing string

	// While a workspace replay runs, live frames of that workspace are set
	// aside in held (hub_replay.go) so the replay reaches the session first.
	holdMu sync.Mutex
	holdWS string
	held   [][]byte
	// replayedTo is, per workspace, the highest number a finished replay
	// covered. A frame is numbered before it is delivered, so one the replay
	// already sent can still arrive live afterwards; it is dropped here.
	replayedTo map[string]uint64
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
//
// Before the upgrade the request's Origin is checked (OriginPolicy) and the
// client address must have a free slot (ConnLimits); a refusal is answered 403
// or 429 and no socket is opened.
func (h *Hub) HandleWebSocket(jwtSecret string) http.HandlerFunc {
	upgrader := websocket.Upgrader{CheckOrigin: h.checkOrigin}
	return func(w http.ResponseWriter, r *http.Request) {
		h.mu.RLock()
		limits, clientIP := h.limits, h.clientIP
		h.mu.RUnlock()

		if !h.checkOrigin(r) {
			slog.Warn("websocket refused: origin not allowed", "origin", r.Header.Get("Origin"), "host", r.Host)
			http.Error(w, "origin not allowed", http.StatusForbidden)
			return
		}
		ip := clientIP(r)
		if !h.slots.take(ip, limits) {
			slog.Warn("websocket refused: too many connections from one address", "ip", ip)
			http.Error(w, "too many connections", http.StatusTooManyRequests)
			return
		}
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			h.slots.closed(ip, true)
			slog.Warn("websocket upgrade failed", "error", err)
			return
		}
		conn.SetReadLimit(limits.MaxFrameBytes)

		client := &Client{
			conn:      conn,
			hub:       h,
			send:      make(chan []byte, 256),
			jwtSecret: jwtSecret,
			ip:        ip,
			preAuth:   true,
			limits:    limits,
			limiter:   rate.NewLimiter(rate.Limit(limits.FramesPerSecond), limits.FrameBurst),
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
	c.preAuth = false
	c.hub.slots.authenticated(c.ip)
	// A session is good for as long as the token it proved itself with; the
	// client reconnects with its refreshed one.
	if claims.ExpiresAt != nil {
		c.expiry = time.AfterFunc(time.Until(claims.ExpiresAt.Time), func() {
			c.sendError(401, "token expired")
			c.conn.Close()
		})
	}

	// Track client by userID; from now on its frames draw on the budget its
	// user's sessions share.
	c.hub.mu.Lock()
	if c.hub.users[claims.UserID] == nil {
		c.hub.users[claims.UserID] = make(map[*Client]bool)
	}
	c.hub.users[claims.UserID][c] = true
	c.limiter = c.hub.userLimiterLocked(claims.UserID)
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
		c.hub.slots.closed(c.ip, c.preAuth)
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

	// A peer that sends nothing, not even a pong to the pings writePump sends,
	// for PongWait is gone; its socket is not kept open on its behalf.
	_ = c.conn.SetReadDeadline(time.Now().Add(c.limits.PongWait))
	c.conn.SetPongHandler(func(string) error {
		return c.conn.SetReadDeadline(time.Now().Add(c.limits.PongWait))
	})

	for {
		msgType, message, err := c.conn.ReadMessage()
		if err != nil {
			// Over the read limit gorilla has already sent 1009; any other error
			// is the peer gone or the deadline passed.
			break
		}
		// Frames over the budget wait their turn; while they wait nothing more
		// is read, so a flood backs up into the sender's own TCP window.
		if err := c.limiter.Wait(c.hub.ctx); err != nil {
			break
		}
		_ = c.conn.SetReadDeadline(time.Now().Add(c.limits.PongWait))

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
		// A socket has one identity for its whole life. Taking a second one
		// would carry the first identity's subscriptions over to it, out of
		// reach of the sweeps that end them; a client that needs another
		// identity opens another socket.
		if c.authenticated {
			slog.Warn("websocket closed: second auth frame on an authenticated socket",
				"user_id", c.userID, "tenant_id", c.tenantID)
			c.sendError(400, "already authenticated")
			c.closeWith(websocket.ClosePolicyViolation, "already authenticated")
			return
		}
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
		workspaceID, err := c.hub.authorizeSubscribe(channelID, c.caller())
		if err != nil {
			slog.Warn("websocket subscribe denied",
				"user_id", c.userID, "channel_id", channelID, "error", err)
			c.sendError(403, "not allowed to subscribe to this channel")
			return
		}
		c.hub.subscribe(channelID, workspaceID, c)

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

// caller is the session's verified identity, for checks made on its behalf.
func (c *Client) caller() grpcauth.Caller {
	return grpcauth.Caller{UserID: c.userID, NGACNodeID: c.ngacNodeID, TenantID: c.tenantID}
}

// writePump is the connection's only data writer. Between frames it pings the
// peer, whose pong keeps readPump's deadline moving.
func (c *Client) writePump() {
	ping := time.NewTicker(c.limits.PongWait * 9 / 10)
	defer func() {
		ping.Stop()
		c.conn.Close()
	}()
	for {
		select {
		case msg, ok := <-c.send:
			if !ok {
				return
			}
			kind := websocket.BinaryMessage
			if isControl(msg) {
				kind = websocket.TextMessage
			}
			_ = c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if err := c.conn.WriteMessage(kind, msg); err != nil {
				return
			}
		case <-ping.C:
			if err := c.conn.WriteControl(websocket.PingMessage, nil, time.Now().Add(writeWait)); err != nil {
				return
			}
		}
	}
}
