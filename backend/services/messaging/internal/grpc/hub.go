package grpc

import (
	"context"
	"log/slog"
	"net/http"
	"sync"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/gorilla/websocket"
	"github.com/redis/go-redis/v9"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"

	pb "ngac-platform/proto/messaging"
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
}

// Client represents a connected WebSocket user.
type Client struct {
	conn          *websocket.Conn
	userID        string
	username      string
	ngacNodeID    string
	hub           *Hub
	send          chan []byte
	authenticated bool
	jwtSecret     string
}

// NewHub creates a Hub with optional Redis pub/sub for horizontal scaling.
func NewHub(rdb *redis.Client) *Hub {
	ctx, cancel := context.WithCancel(context.Background())
	h := &Hub{
		channels: make(map[string]map[*Client]bool),
		users:    make(map[string]map[*Client]bool),
		rdb:      rdb,
		ctx:      ctx,
		cancel:   cancel,
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

// Subscribe adds a client to a local channel group.
func (h *Hub) Subscribe(channelID string, client *Client) {
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

// subscribeRedis listens on channel:* pattern and delivers to local clients.
func (h *Hub) subscribeRedis() {
	pubsub := h.rdb.PSubscribe(h.ctx, "channel:*", "user:*")
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

			if len(msg.Channel) > 5 && msg.Channel[:5] == "user:" {
				userID := msg.Channel[5:]
				h.sendToUser(userID, payload)
			} else if len(msg.Channel) > 8 && msg.Channel[:8] == "channel:" {
				channelID := msg.Channel[8:]
				h.broadcastLocal(channelID, payload)
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

func redisChanKey(channelID string) string {
	return "channel:" + channelID
}

// WSClaims are JWT claims for WebSocket authentication.
type WSClaims struct {
	UserID     string `json:"user_id"`
	Username   string `json:"username"`
	NGACNodeID string `json:"ngac_node_id"`
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
	c.authenticated = true

	// Track client by userID
	c.hub.mu.Lock()
	if c.hub.users[claims.UserID] == nil {
		c.hub.users[claims.UserID] = make(map[*Client]bool)
	}
	c.hub.users[claims.UserID][c] = true
	c.hub.mu.Unlock()

	c.sendAuthResponse(true, claims.UserID, "")
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
		c.hub.Subscribe(payload.Subscribe.ChannelId, c)

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

// Helper to create a timestamppb from time.Time — used by notification consumer.
func TimestampProto(t time.Time) *timestamppb.Timestamp {
	return timestamppb.New(t)
}
