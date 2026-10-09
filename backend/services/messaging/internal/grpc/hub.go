package grpc

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/gorilla/websocket"
	"github.com/redis/go-redis/v9"
	"google.golang.org/protobuf/proto"

	"ngac-platform/ngac"
	pb "ngac-platform/proto/messaging"
	"ngac-platform/services/messaging/internal/events"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool { return true },
}

const authTimeout = 5 * time.Second

// Hub manages WebSocket clients and uses Redis pub/sub for cross-instance messaging.
type Hub struct {
	mu       sync.RWMutex
	channels map[string]map[*Client]bool
	users    map[string]map[*Client]bool // userID → connected clients
	rdb      *redis.Client
	ctx      context.Context
	cancel   context.CancelFunc

	// access authorizes joining a channel's live stream. A nil checker denies
	// every subscription.
	access ChannelAccessChecker
}

// subscribeCheckTimeout bounds the policy round-trip a Subscribe waits on.
const subscribeCheckTimeout = 5 * time.Second

// Client represents a connected WebSocket user.
type Client struct {
	conn       *websocket.Conn
	userID     string
	username   string
	ngacNodeID string
	// tenantID is the tenant the session's JWT was issued for. Presence and
	// approval events are confined to it; empty means the session belongs to
	// no tenant and receives no tenant-scoped events.
	tenantID      string
	hub           *Hub
	send          chan []byte
	authenticated bool
	jwtSecret     string
}

// ChannelAccessChecker answers whether a user may perform an operation on a
// channel. domain.Service satisfies it.
type ChannelAccessChecker interface {
	AuthorizeChannelAccess(ctx context.Context, channelID, userNodeID, operation string) error
}

// NewHub creates a Hub with optional Redis pub/sub for horizontal scaling.
//
// access authorizes every channel subscription; pass nil only where no client
// should ever be allowed to subscribe.
func NewHub(rdb *redis.Client, access ChannelAccessChecker) *Hub {
	ctx, cancel := context.WithCancel(context.Background())
	h := &Hub{
		channels: make(map[string]map[*Client]bool),
		users:    make(map[string]map[*Client]bool),
		rdb:      rdb,
		ctx:      ctx,
		cancel:   cancel,
		access:   access,
	}
	if rdb != nil {
		go h.subscribeRedis()
	}
	return h
}

// Close shuts down the Hub and its Redis subscription.
func (h *Hub) Close() {
	h.cancel()
}

// authorizeSubscribe decides whether a user may join a channel's live stream.
//
// A subscription delivers every message posted to the channel, so it is a read
// of the channel and takes the same check as fetching its history: read on the
// channel's content OA. Knowing a channel ID is not a capability. Any failure —
// no checker, no identity, a policy error — is a refusal.
func (h *Hub) authorizeSubscribe(channelID, userNodeID string) error {
	if h.access == nil {
		return errors.New("no channel access checker configured")
	}
	if channelID == "" || userNodeID == "" {
		return errors.New("channel and user identity required")
	}
	ctx, cancel := context.WithTimeout(h.ctx, subscribeCheckTimeout)
	defer cancel()
	return h.access.AuthorizeChannelAccess(ctx, channelID, userNodeID, ngac.OpRead)
}

// subscribe adds a client to a local channel group. Callers must have passed
// authorizeSubscribe first.
func (h *Hub) subscribe(channelID string, client *Client) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.channels[channelID] == nil {
		h.channels[channelID] = make(map[*Client]bool)
	}
	h.channels[channelID][client] = true
}

// Unsubscribe removes a client from a channel group.
func (h *Hub) Unsubscribe(channelID string, client *Client) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if clients, ok := h.channels[channelID]; ok {
		delete(clients, client)
	}
}

// UnsubscribeAll removes a client from all channels and user tracking.
func (h *Hub) UnsubscribeAll(client *Client) {
	h.mu.Lock()
	defer h.mu.Unlock()
	for _, clients := range h.channels {
		delete(clients, client)
	}
	if userClients, ok := h.users[client.userID]; ok {
		delete(userClients, client)
		if len(userClients) == 0 {
			delete(h.users, client.userID)
		}
	}
}

// messageToChatMessage converts a gRPC Message to a WebSocket ChatMessage.
func messageToChatMessage(m *pb.Message) *pb.ChatMessage {
	return &pb.ChatMessage{
		Id:              m.Id,
		ChannelId:       m.ChannelId,
		SenderId:        m.SenderId,
		SenderName:      m.SenderName,
		Content:         m.Content,
		CreatedAt:       m.CreatedAt,
		MessageType:     m.MessageType,
		ParentMessageId: m.ParentMessageId,
		ReplyCount:      m.ReplyCount,
	}
}

// marshalEnvelope serializes a ServerEnvelope to protobuf bytes.
func marshalEnvelope(env *pb.ServerEnvelope) []byte {
	data, err := proto.Marshal(env)
	if err != nil {
		slog.Error("failed to marshal server envelope", "error", err)
		return nil
	}
	return data
}

// BroadcastToChannel publishes to Redis (cross-instance) or local broadcast.
func (h *Hub) BroadcastToChannel(channelID string, msg *pb.Message) {
	chatMsg := messageToChatMessage(msg)
	env := &pb.ServerEnvelope{
		Payload: &pb.ServerEnvelope_ChatMessage{ChatMessage: chatMsg},
	}
	data := marshalEnvelope(env)
	if data == nil {
		return
	}

	if h.rdb != nil {
		if err := h.rdb.Publish(h.ctx, redisChanKey(channelID), data).Err(); err != nil {
			slog.Warn("redis publish failed, falling back to local", "error", err)
			h.broadcastLocal(channelID, data)
		}
		return
	}
	h.broadcastLocal(channelID, data)
}

// broadcastLocal sends data to local WebSocket clients in a channel.
func (h *Hub) broadcastLocal(channelID string, data []byte) {
	h.mu.RLock()
	defer h.mu.RUnlock()

	clients, ok := h.channels[channelID]
	if !ok {
		return
	}
	for client := range clients {
		select {
		case client.send <- data:
		default:
			close(client.send)
			delete(clients, client)
		}
	}
}

// broadcastTyping sends typing indicator via Redis or local.
func (h *Hub) broadcastTyping(channelID, username string, sender *Client) {
	env := &pb.ServerEnvelope{
		Payload: &pb.ServerEnvelope_TypingEvent{
			TypingEvent: &pb.TypingEvent{
				ChannelId: channelID,
				Username:  username,
			},
		},
	}
	data := marshalEnvelope(env)
	if data == nil {
		return
	}

	if h.rdb != nil {
		if err := h.rdb.Publish(h.ctx, redisChanKey(channelID), data).Err(); err != nil {
			slog.Warn("redis typing publish failed", "error", err)
		}
		return
	}

	h.mu.RLock()
	defer h.mu.RUnlock()
	if clients, ok := h.channels[channelID]; ok {
		for other := range clients {
			if other != sender {
				select {
				case other.send <- data:
				default:
				}
			}
		}
	}
}

// subscribeRedis listens for cross-instance traffic and delivers it to local
// clients: channel:<id> to that channel's subscribers, user:<id> to that user's
// sessions, presence:<tenant> to sessions in that tenant.
func (h *Hub) subscribeRedis() {
	pubsub := h.rdb.PSubscribe(h.ctx, "channel:*", "user:*", presenceKeyPrefix+"*")
	defer pubsub.Close()

	slog.Info("redis pub/sub subscriber started")

	ch := pubsub.Channel()
	for {
		select {
		case <-h.ctx.Done():
			slog.Info("redis subscriber shutting down")
			return
		case msg, ok := <-ch:
			if !ok {
				return
			}
			payload := []byte(msg.Payload)

			switch {
			case strings.HasPrefix(msg.Channel, presenceKeyPrefix):
				h.broadcastToTenant(strings.TrimPrefix(msg.Channel, presenceKeyPrefix), payload)
			case strings.HasPrefix(msg.Channel, "user:"):
				h.sendToUser(strings.TrimPrefix(msg.Channel, "user:"), payload)
			case strings.HasPrefix(msg.Channel, "channel:"):
				h.broadcastLocal(strings.TrimPrefix(msg.Channel, "channel:"), payload)
			}
		}
	}
}

// sendToUser delivers data to all WebSocket clients for a given user.
func (h *Hub) sendToUser(userID string, data []byte) {
	h.mu.RLock()
	defer h.mu.RUnlock()
	if clients, ok := h.users[userID]; ok {
		for client := range clients {
			select {
			case client.send <- data:
			default:
			}
		}
	}
}

// broadcastToTenant delivers data to every local session of the given tenant.
// An empty tenant matches no one: a session without a tenant has no
// colleagues, and treating "" as a tenant would pool every such session.
func (h *Hub) broadcastToTenant(tenantID string, data []byte) {
	if tenantID == "" {
		return
	}
	h.mu.RLock()
	defer h.mu.RUnlock()
	for _, clients := range h.users {
		for client := range clients {
			if client.tenantID != tenantID {
				continue
			}
			select {
			case client.send <- data:
			default:
			}
		}
	}
}

// presenceKeyPrefix namespaces the per-tenant Redis presence channels.
const presenceKeyPrefix = "presence:"

func redisChanKey(channelID string) string {
	return "channel:" + channelID
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

// SendNotification pushes a notification to all connected WebSocket clients for a user.
func (h *Hub) SendNotification(userID string, notif *pb.Notification) {
	env := &pb.ServerEnvelope{
		Payload: &pb.ServerEnvelope_Notification{
			Notification: &pb.NotificationEvent{
				Id:         notif.Id,
				Type:       notif.Type,
				Title:      notif.Title,
				Body:       notif.Body,
				EntityType: notif.EntityType,
				EntityId:   notif.EntityId,
			},
		},
	}
	data := marshalEnvelope(env)
	if data == nil {
		return
	}

	if h.rdb != nil {
		if err := h.rdb.Publish(h.ctx, "user:"+userID, data).Err(); err != nil {
			slog.Warn("redis notification publish failed", "error", err)
		}
		return
	}

	h.sendToUser(userID, data)
}

// SendUnreadCount pushes an unread count update to a user.
func (h *Hub) SendUnreadCount(userID string, count int32) {
	env := &pb.ServerEnvelope{
		Payload: &pb.ServerEnvelope_UnreadCount{
			UnreadCount: &pb.UnreadCountEvent{Count: count},
		},
	}
	data := marshalEnvelope(env)
	if data == nil {
		return
	}

	if h.rdb != nil {
		if err := h.rdb.Publish(h.ctx, "user:"+userID, data).Err(); err != nil {
			slog.Warn("redis unread count publish failed", "error", err)
		}
		return
	}

	h.sendToUser(userID, data)
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
	token, err := jwt.ParseWithClaims(req.Token, &WSClaims{}, func(t *jwt.Token) (interface{}, error) {
		return []byte(c.jwtSecret), nil
	})
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

	// Track client by userID
	c.hub.mu.Lock()
	if c.hub.users[claims.UserID] == nil {
		c.hub.users[claims.UserID] = make(map[*Client]bool)
	}
	c.hub.users[claims.UserID][c] = true
	c.hub.mu.Unlock()

	c.sendAuthResponse(true, claims.UserID, "")
	// Broadcast presence online event
	c.hub.BroadcastPresence(c.tenantID, claims.UserID, claims.Username, "online")
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
		// Broadcast offline if this was the user's last connection in this
		// tenant. Presence is per tenant, so a session still open in another
		// tenant does not keep the user "online" here.
		if c.authenticated {
			c.hub.mu.RLock()
			remaining := 0
			for other := range c.hub.users[c.userID] {
				if other.tenantID == c.tenantID {
					remaining++
				}
			}
			c.hub.mu.RUnlock()
			// Will be 0 after UnsubscribeAll removes this client
			if remaining <= 1 {
				c.hub.BroadcastPresence(c.tenantID, c.userID, c.username, "offline")
			}
		}
		c.hub.UnsubscribeAll(c)
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

		// Text frame = JSON legacy (dual-mode transition)
		if msgType == websocket.TextMessage {
			c.handleLegacyJSON(message)
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
		channelID := payload.Subscribe.ChannelId
		if err := c.hub.authorizeSubscribe(channelID, c.ngacNodeID); err != nil {
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
		c.hub.Unsubscribe(payload.Unsubscribe.ChannelId, c)

	case *pb.ClientEnvelope_Typing:
		if !c.authenticated {
			return
		}
		c.hub.broadcastTyping(payload.Typing.ChannelId, c.username, c)
	}
}

// handleLegacyJSON supports the old JSON text-based protocol during migration.
func (c *Client) handleLegacyJSON(data []byte) {
	// Legacy JSON support removed — this is a no-op placeholder.
	// JSON clients should upgrade to binary protocol.
	slog.Warn("received legacy JSON WebSocket message, ignoring", "userID", c.userID)
}

func (c *Client) writePump() {
	defer c.conn.Close()
	for msg := range c.send {
		if err := c.conn.WriteMessage(websocket.BinaryMessage, msg); err != nil {
			return
		}
	}
}

// BroadcastThreadReply sends a thread reply event to channel subscribers.
func (h *Hub) BroadcastThreadReply(channelID string, msg *pb.Message) {
	chatMsg := messageToChatMessage(msg)
	env := &pb.ServerEnvelope{
		Payload: &pb.ServerEnvelope_ThreadReply{
			ThreadReply: &pb.ThreadReplyEvent{
				Message:         chatMsg,
				ParentMessageId: msg.ParentMessageId,
			},
		},
	}
	data := marshalEnvelope(env)
	if data == nil {
		return
	}

	if h.rdb != nil {
		if err := h.rdb.Publish(h.ctx, redisChanKey(channelID), data).Err(); err != nil {
			slog.Warn("redis thread reply publish failed", "error", err)
			h.broadcastLocal(channelID, data)
		}
		return
	}
	h.broadcastLocal(channelID, data)
}

// BroadcastAssetUpdated sends an asset state change event to channel subscribers.
func (h *Hub) BroadcastAssetUpdated(assetID, newState string) {
	env := &pb.ServerEnvelope{
		Payload: &pb.ServerEnvelope_AssetUpdated{
			AssetUpdated: &pb.AssetUpdatedEvent{
				AssetId:  assetID,
				NewState: newState,
			},
		},
	}
	data := marshalEnvelope(env)
	if data == nil {
		return
	}

	// Broadcast to all connected users (no channel scope for asset updates)
	h.mu.RLock()
	defer h.mu.RUnlock()
	for _, clients := range h.users {
		for client := range clients {
			select {
			case client.send <- data:
			default:
			}
		}
	}
}

// BroadcastApprovalEvent sends an approval status change to the users the
// notice names, and to no one else.
//
// It used to go to every connected session of every tenant, which handed the
// request ID, template name and actor of one tenant's approvals to everyone on
// the platform. Delivery is now by recipient: a session receives the event
// only if its NGAC user node is in RecipientNodeIDs and, when the notice
// carries a tenant, only if the session belongs to that tenant. A notice that
// names nobody is dropped.
func (h *Hub) BroadcastApprovalEvent(n events.ApprovalNotice) {
	recipients := make(map[string]bool, len(n.RecipientNodeIDs))
	for _, id := range n.RecipientNodeIDs {
		if id != "" {
			recipients[id] = true
		}
	}
	if len(recipients) == 0 {
		return
	}

	env := &pb.ServerEnvelope{
		Payload: &pb.ServerEnvelope_ApprovalEvent{
			ApprovalEvent: &pb.ApprovalEvent{
				RequestId:    n.RequestID,
				Status:       n.Status,
				Action:       n.Action,
				ActorNodeId:  n.ActorNodeID,
				TemplateName: n.TemplateName,
			},
		},
	}
	data := marshalEnvelope(env)
	if data == nil {
		return
	}

	h.mu.RLock()
	defer h.mu.RUnlock()
	for _, clients := range h.users {
		for client := range clients {
			if !recipients[client.ngacNodeID] {
				continue
			}
			if n.TenantID != "" && client.tenantID != n.TenantID {
				continue
			}
			select {
			case client.send <- data:
			default:
			}
		}
	}
}

// BroadcastPresence sends a presence event (online/offline) to the sessions of
// the given tenant. Presence says who is around; outside the tenant it is a
// directory of another organisation's people. A session with no tenant
// announces itself to no one.
func (h *Hub) BroadcastPresence(tenantID, userID, username, status string) {
	if tenantID == "" {
		return
	}
	env := &pb.ServerEnvelope{
		Payload: &pb.ServerEnvelope_PresenceEvent{
			PresenceEvent: &pb.PresenceEvent{
				UserId:   userID,
				Username: username,
				Status:   status,
			},
		},
	}
	data := marshalEnvelope(env)
	if data == nil {
		return
	}

	if h.rdb != nil {
		// Publish to all instances, on the tenant's own presence channel.
		if err := h.rdb.Publish(h.ctx, presenceKeyPrefix+tenantID, data).Err(); err != nil {
			slog.Warn("redis presence publish failed", "error", err)
		}
		return
	}

	h.broadcastToTenant(tenantID, data)
}
