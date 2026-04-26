package grpc

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"sync"

	"github.com/golang-jwt/jwt/v5"
	"github.com/gorilla/websocket"
	"github.com/redis/go-redis/v9"

	pb "ngac-platform/proto/messaging"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool { return true },
}

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
	conn       *websocket.Conn
	userID     string
	username   string
	ngacNodeID string
	hub        *Hub
	send       chan []byte
}

// WSMessage is the wire format for WebSocket frames.
type WSMessage struct {
	Type      string      `json:"type"`
	ChannelID string      `json:"channel_id,omitempty"`
	Content   string      `json:"content,omitempty"`
	Message   *pb.Message `json:"message,omitempty"`
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

// BroadcastToChannel publishes to Redis (cross-instance) or local broadcast.
func (h *Hub) BroadcastToChannel(channelID string, msg *pb.Message) {
	data, err := json.Marshal(WSMessage{
		Type:      "message",
		ChannelID: channelID,
		Message:   msg,
	})
	if err != nil {
		slog.Error("failed to marshal broadcast message", "error", err)
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
	data, err := json.Marshal(WSMessage{
		Type:      "typing",
		ChannelID: channelID,
		Content:   username,
	})
	if err != nil {
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
	pubsub := h.rdb.PSubscribe(h.ctx, "channel:*")
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
			channelID := msg.Channel[len("channel:"):]
			h.broadcastLocal(channelID, []byte(msg.Payload))
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
func (h *Hub) HandleWebSocket(jwtSecret string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		tokenStr := r.URL.Query().Get("token")
		if tokenStr == "" {
			http.Error(w, "missing token", http.StatusUnauthorized)
			return
		}

		token, err := jwt.ParseWithClaims(tokenStr, &WSClaims{}, func(t *jwt.Token) (interface{}, error) {
			return []byte(jwtSecret), nil
		})
		if err != nil {
			http.Error(w, "invalid token", http.StatusUnauthorized)
			return
		}
		claims := token.Claims.(*WSClaims)

		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			slog.Warn("websocket upgrade failed", "error", err)
			return
		}

		client := &Client{
			conn:       conn,
			userID:     claims.UserID,
			username:   claims.Username,
			ngacNodeID: claims.NGACNodeID,
			hub:        h,
			send:       make(chan []byte, 256),
		}

		// Track client by userID for notifications
		h.mu.Lock()
		if h.users[claims.UserID] == nil {
			h.users[claims.UserID] = make(map[*Client]bool)
		}
		h.users[claims.UserID][client] = true
		h.mu.Unlock()

		go client.writePump()
		go client.readPump()
	}
}

// SendNotification pushes a notification to all connected WebSocket clients for a user.
func (h *Hub) SendNotification(userID string, notif *pb.Notification) {
	data, err := json.Marshal(map[string]interface{}{
		"type":         "notification",
		"notification": notif,
	})
	if err != nil {
		slog.Error("failed to marshal notification", "error", err)
		return
	}

	if h.rdb != nil {
		// Publish on Redis for cross-instance delivery
		if err := h.rdb.Publish(h.ctx, "user:"+userID, data).Err(); err != nil {
			slog.Warn("redis notification publish failed", "error", err)
		}
		return
	}

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

func (c *Client) readPump() {
	defer func() {
		c.hub.UnsubscribeAll(c)
		c.conn.Close()
	}()

	for {
		_, message, err := c.conn.ReadMessage()
		if err != nil {
			break
		}

		var msg WSMessage
		if err := json.Unmarshal(message, &msg); err != nil {
			continue
		}

		switch msg.Type {
		case "subscribe":
			c.hub.Subscribe(msg.ChannelID, c)
		case "unsubscribe":
			c.hub.Unsubscribe(msg.ChannelID, c)
		case "typing":
			c.hub.broadcastTyping(msg.ChannelID, c.username, c)
		}
	}
}

func (c *Client) writePump() {
	defer c.conn.Close()
	for msg := range c.send {
		if err := c.conn.WriteMessage(websocket.TextMessage, msg); err != nil {
			return
		}
	}
}
