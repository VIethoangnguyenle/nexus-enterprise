package grpc

import (
	"context"
	"errors"
	"log/slog"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"

	"ngac-platform/ngac"
	"ngac-platform/pkg/grpcauth"
)

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

	// dom holds workspace subscriptions, sequence numbers and the presence
	// grace timers (hub_domain.go). Its maps are guarded by mu.
	dom domainState
}

// subscribeCheckTimeout bounds the policy round-trip a Subscribe waits on.
const subscribeCheckTimeout = 5 * time.Second

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
		dom:      newDomainState(),
	}
	if rdb != nil {
		go h.subscribeRedis()
	}
	return h
}

// Close shuts down the Hub and its Redis subscription.
func (h *Hub) Close() {
	h.cancel()
	h.dom.presenceMu.Lock()
	defer h.dom.presenceMu.Unlock()
	for key, t := range h.dom.pendingAbsent {
		t.Stop()
		delete(h.dom.pendingAbsent, key)
	}
}

// authorizeSubscribe decides whether a user may join a channel's live stream.
//
// A subscription delivers every message posted to the channel, so it is a read
// of the channel and takes the same check as fetching its history: read on the
// channel's content OA. Knowing a channel ID is not a capability. Any failure —
// no checker, no identity, a policy error — is a refusal.
//
// who is the session's verified identity (from the JWT it authenticated with);
// it rides the context so the policy check runs as that user.
func (h *Hub) authorizeSubscribe(channelID string, who grpcauth.Caller) error {
	if h.access == nil {
		return errors.New("no channel access checker configured")
	}
	if channelID == "" || who.NGACNodeID == "" {
		return errors.New("channel and user identity required")
	}
	ctx, cancel := context.WithTimeout(grpcauth.WithCaller(h.ctx, who), subscribeCheckTimeout)
	defer cancel()
	return h.access.AuthorizeChannelAccess(ctx, channelID, who.NGACNodeID, ngac.OpRead)
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

// RevokeChannelSubscriptions drops every live subscription the user (by NGAC
// node) holds on the channel, on every hub instance. Called when the user is
// removed from the channel: the subscription was authorized when it opened and
// would otherwise outlive the membership that justified it.
func (h *Hub) RevokeChannelSubscriptions(channelID, userNodeID string) {
	if channelID == "" || userNodeID == "" {
		return
	}
	if h.rdb != nil {
		// Every instance, this one included, receives this on revoke:*.
		err := h.rdb.Publish(h.ctx, revokeKeyPrefix+channelID, userNodeID).Err()
		if err == nil {
			return
		}
		slog.Warn("redis revoke publish failed, revoking locally only",
			"channel_id", channelID, "error", err)
	}
	h.revokeLocal(channelID, userNodeID)
}

// revokeLocal removes this instance's subscriptions of userNodeID to channelID.
func (h *Hub) revokeLocal(channelID, userNodeID string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	for client := range h.channels[channelID] {
		if client.ngacNodeID == userNodeID {
			delete(h.channels[channelID], client)
		}
	}
}

// isSubscribed reports whether client currently holds a subscription to channelID.
func (h *Hub) isSubscribed(channelID string, client *Client) bool {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return h.channels[channelID][client]
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
	for _, clients := range h.dom.workspaces {
		delete(clients, client)
	}
	if userClients, ok := h.users[client.userID]; ok {
		delete(userClients, client)
		if len(userClients) == 0 {
			delete(h.users, client.userID)
		}
	}
}

// presenceKeyPrefix namespaces the per-tenant Redis presence channels.
const presenceKeyPrefix = "presence:"

// revokeKeyPrefix namespaces the per-channel Redis subscription-revocation
// channels. The payload is the NGAC node ID of the user to drop.
const revokeKeyPrefix = "revoke:"

func redisChanKey(channelID string) string {
	return "channel:" + channelID
}
