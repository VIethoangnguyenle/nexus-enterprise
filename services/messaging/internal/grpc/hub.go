package grpc

import (
	"encoding/json"
	"log"
	"net/http"
	"sync"

	"github.com/golang-jwt/jwt/v5"
	"github.com/gorilla/websocket"

	pb "ngac-platform/proto/messaging"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool { return true },
}

type Hub struct {
	mu       sync.RWMutex
	channels map[string]map[*Client]bool // channelID -> set of clients
}

type Client struct {
	conn       *websocket.Conn
	userID     string
	username   string
	ngacNodeID string
	hub        *Hub
	send       chan []byte
}

type WSMessage struct {
	Type      string `json:"type"`
	ChannelID string `json:"channel_id,omitempty"`
	Content   string `json:"content,omitempty"`
	// Embedded message for broadcasts
	Message *pb.Message `json:"message,omitempty"`
}

func NewHub() *Hub {
	return &Hub{channels: make(map[string]map[*Client]bool)}
}

func (h *Hub) Subscribe(channelID string, client *Client) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.channels[channelID] == nil {
		h.channels[channelID] = make(map[*Client]bool)
	}
	h.channels[channelID][client] = true
}

func (h *Hub) Unsubscribe(channelID string, client *Client) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if clients, ok := h.channels[channelID]; ok {
		delete(clients, client)
	}
}

func (h *Hub) UnsubscribeAll(client *Client) {
	h.mu.Lock()
	defer h.mu.Unlock()
	for _, clients := range h.channels {
		delete(clients, client)
	}
}

func (h *Hub) BroadcastToChannel(channelID string, msg *pb.Message) {
	h.mu.RLock()
	defer h.mu.RUnlock()

	data, _ := json.Marshal(WSMessage{
		Type:      "message",
		ChannelID: channelID,
		Message:   msg,
	})

	if clients, ok := h.channels[channelID]; ok {
		for client := range clients {
			select {
			case client.send <- data:
			default:
				close(client.send)
				delete(clients, client)
			}
		}
	}
}

type WSClaims struct {
	UserID     string `json:"user_id"`
	Username   string `json:"username"`
	NGACNodeID string `json:"ngac_node_id"`
	jwt.RegisteredClaims
}

func (h *Hub) HandleWebSocket(jwtSecret string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Auth via query param
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
			log.Printf("WebSocket upgrade error: %v", err)
			return
		}

		client := &Client{
			conn: conn, userID: claims.UserID, username: claims.Username,
			ngacNodeID: claims.NGACNodeID, hub: h, send: make(chan []byte, 256),
		}

		go client.writePump()
		go client.readPump()
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
			// Broadcast typing indicator
			data, _ := json.Marshal(WSMessage{
				Type:      "typing",
				ChannelID: msg.ChannelID,
				Content:   c.username,
			})
			c.hub.mu.RLock()
			if clients, ok := c.hub.channels[msg.ChannelID]; ok {
				for other := range clients {
					if other != c {
						select {
						case other.send <- data:
						default:
						}
					}
				}
			}
			c.hub.mu.RUnlock()
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
