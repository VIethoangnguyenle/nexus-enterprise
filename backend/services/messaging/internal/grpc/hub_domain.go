package grpc

import (
	"context"
	"errors"
	"log/slog"
	"strings"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"

	"ngac-platform/ngac"
	"ngac-platform/pkg/grpcauth"
	"ngac-platform/pkg/realtime"
	pb "ngac-platform/proto/messaging"
)

// WorkspaceAccessChecker answers whether a session may follow a workspace's
// live changes. domain.Service satisfies it.
type WorkspaceAccessChecker interface {
	AuthorizeWorkspaceAccess(ctx context.Context, workspaceID, userNodeID, tenantID, operation string) error
}

// Redis channels that carry a DomainEvent between hub instances. The tenant is
// part of every name so a receiving instance can enforce the tenant boundary
// without trusting the payload; the second segment is the workspace, channel or
// user node the event is addressed to.
const (
	workspaceEventPrefix = "wsev:"
	channelEventPrefix   = "chev:"
	userEventPrefix      = "usev:"
	// permEventPrefix is the user channel for permission events, which also make
	// every receiving instance re-check the addressed users' subscriptions.
	permEventPrefix       = "permev:"
	workspaceRevokePrefix = "wsrevoke:"
	workspaceSeqPrefix    = "wsseq:"
)

// defaultPresenceGrace is how long a user's last session may stay gone before
// the tenant is told they went offline. A page reload or a brief network drop
// reconnects well inside it, so colleagues do not see the user flicker.
const defaultPresenceGrace = 3 * time.Second

// domainState is the hub's workspace-subscription and sequencing state.
type domainState struct {
	workspaces map[string]map[*Client]bool // workspaceID → subscribed sessions
	access     WorkspaceAccessChecker

	// seqMu serialises "take a number, publish it" so one instance never hands
	// out 6 before 5 reaches the wire. Across instances the numbers still come
	// from Redis; an unlucky interleaving costs a client one resync.
	seqMu sync.Mutex
	seqs  map[string]uint64 // local counters, used when there is no Redis

	presenceMu    sync.Mutex
	presenceGrace time.Duration
	pendingAbsent map[string]*time.Timer // tenant|user → offline announcement not yet sent
}

func newDomainState() domainState {
	return domainState{
		workspaces:    make(map[string]map[*Client]bool),
		seqs:          make(map[string]uint64),
		presenceGrace: defaultPresenceGrace,
		pendingAbsent: make(map[string]*time.Timer),
	}
}

// SetWorkspaceAccess installs the check every workspace subscription goes
// through. Without one, every workspace subscription is refused.
func (h *Hub) SetWorkspaceAccess(a WorkspaceAccessChecker) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.dom.access = a
}

// SetPresenceGrace changes how long an absence is held back before it is
// announced. Zero announces immediately.
func (h *Hub) SetPresenceGrace(d time.Duration) {
	h.dom.presenceMu.Lock()
	defer h.dom.presenceMu.Unlock()
	h.dom.presenceGrace = d
}

// authorizeWorkspaceSubscribe decides whether a session may follow a workspace.
// It is a read of the workspace, asked of the policy service as the session's
// own user; any failure — no checker, no identity, a policy error — refuses.
func (h *Hub) authorizeWorkspaceSubscribe(workspaceID string, who grpcauth.Caller) error {
	h.mu.RLock()
	access := h.dom.access
	h.mu.RUnlock()
	if access == nil {
		return errors.New("no workspace access checker configured")
	}
	if workspaceID == "" || who.NGACNodeID == "" || who.TenantID == "" {
		return errors.New("workspace, user and tenant identity required")
	}
	ctx, cancel := context.WithTimeout(grpcauth.WithCaller(h.ctx, who), subscribeCheckTimeout)
	defer cancel()
	return access.AuthorizeWorkspaceAccess(ctx, workspaceID, who.NGACNodeID, who.TenantID, ngac.OpRead)
}

// subscribeWorkspace adds a session to a workspace's stream and returns the
// last sequence number issued so far. Callers must have passed
// authorizeWorkspaceSubscribe first.
func (h *Hub) subscribeWorkspace(workspaceID string, c *Client) uint64 {
	h.mu.Lock()
	if h.dom.workspaces[workspaceID] == nil {
		h.dom.workspaces[workspaceID] = make(map[*Client]bool)
	}
	h.dom.workspaces[workspaceID][c] = true
	h.mu.Unlock()
	// Read after registering: anything numbered above this reaches the session.
	return h.currentSeq(workspaceID)
}

func (h *Hub) unsubscribeWorkspace(workspaceID string, c *Client) {
	h.mu.Lock()
	defer h.mu.Unlock()
	delete(h.dom.workspaces[workspaceID], c)
}

// PublishDomain fans a committed change out to the sessions it is addressed to
// and to no one else:
//
//   - user level: the sessions of the named users, inside the event's tenant;
//   - channel level: the subscribers of the channel, inside the event's tenant;
//   - workspace level: the sessions subscribed to the workspace, inside the
//     event's tenant, stamped with the workspace's next sequence number.
//
// An event that does not name its tenant and workspace is dropped.
func (h *Hub) PublishDomain(e realtime.Event) {
	if err := e.Validate(); err != nil {
		slog.Warn("domain event dropped", "domain", e.Domain, "kind", e.Kind, "error", err)
		return
	}
	switch e.Level() {
	case realtime.LevelUser:
		data := marshalEnvelope(domainEnvelope(e, 0))
		if data == nil {
			return
		}
		perm := e.Domain == realtime.DomainPermission
		prefix := userEventPrefix
		if perm {
			prefix = permEventPrefix
		}
		for _, node := range dedupe(e.UserNodeIDs) {
			h.route(prefix+e.TenantID+":"+node, data, func() {
				h.deliverToUserNode(e.TenantID, node, data)
				if perm {
					go h.recheckWorkspaceAccess(e.TenantID, node)
				}
			})
		}
	case realtime.LevelChannel:
		data := marshalEnvelope(domainEnvelope(e, 0))
		if data == nil {
			return
		}
		h.route(channelEventPrefix+e.TenantID+":"+e.ChannelID, data, func() { h.deliverToChannel(e.TenantID, e.ChannelID, data) })
	default:
		h.dom.seqMu.Lock()
		defer h.dom.seqMu.Unlock()
		seq := h.nextSeq(e.WorkspaceID)
		data := marshalEnvelope(domainEnvelope(e, seq))
		if data == nil {
			return
		}
		h.route(workspaceEventPrefix+e.TenantID+":"+e.WorkspaceID, data, func() { h.deliverToWorkspace(e.TenantID, e.WorkspaceID, data) })
	}
}

// RevokeWorkspaceSubscriptions ends, on every hub instance, what the user (by
// NGAC node) was following of the workspace: the workspace stream itself and
// every channel subscription held in the workspace's tenant. Called when the
// user stops belonging to the workspace; the subscription was authorized when
// it opened and would otherwise outlive the membership that justified it.
func (h *Hub) RevokeWorkspaceSubscriptions(workspaceID, userNodeID string) {
	if workspaceID == "" || userNodeID == "" {
		return
	}
	if h.rdb != nil {
		// Every instance, this one included, receives this on wsrevoke:*.
		err := h.rdb.Publish(h.ctx, workspaceRevokePrefix+workspaceID, userNodeID).Err()
		if err == nil {
			return
		}
		slog.Warn("redis workspace revoke publish failed, revoking locally only",
			"workspace_id", workspaceID, "error", err)
	}
	h.revokeWorkspaceLocal(workspaceID, userNodeID)
}

// revokeWorkspaceLocal drops this instance's subscriptions of the user to the
// workspace and to the channels of its tenant (a workspace is its own tenant).
func (h *Hub) revokeWorkspaceLocal(workspaceID, userNodeID string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	for c := range h.dom.workspaces[workspaceID] {
		if c.ngacNodeID == userNodeID {
			delete(h.dom.workspaces[workspaceID], c)
		}
	}
	for _, subscribers := range h.channels {
		for c := range subscribers {
			if c.ngacNodeID == userNodeID && c.tenantID == workspaceID {
				delete(subscribers, c)
			}
		}
	}
}

// recheckWorkspaceAccess asks the policy service again about each workspace the
// user's sessions follow in the tenant, and drops the ones that no longer pass.
// It runs when a permission event names the user: the grant that justified the
// subscription may be the very thing that changed.
func (h *Hub) recheckWorkspaceAccess(tenantID, userNodeID string) {
	type follow struct {
		c  *Client
		ws string
	}
	var follows []follow
	h.mu.RLock()
	for ws, subscribers := range h.dom.workspaces {
		for c := range subscribers {
			if c.ngacNodeID == userNodeID && c.tenantID == tenantID {
				follows = append(follows, follow{c, ws})
			}
		}
	}
	h.mu.RUnlock()
	for _, f := range follows {
		who := grpcauth.Caller{UserID: f.c.userID, NGACNodeID: f.c.ngacNodeID, TenantID: f.c.tenantID}
		if err := h.authorizeWorkspaceSubscribe(f.ws, who); err != nil {
			slog.Info("workspace subscription dropped after a permission change",
				"user_id", f.c.userID, "workspace_id", f.ws, "error", err)
			h.unsubscribeWorkspace(f.ws, f.c)
		}
	}
}

// route publishes to Redis when there is one (every instance, this one
// included, delivers from its subscription) and delivers locally otherwise or
// when the publish fails.
func (h *Hub) route(redisChannel string, data []byte, local func()) {
	if h.rdb != nil {
		if err := h.rdb.Publish(h.ctx, redisChannel, data).Err(); err == nil {
			return
		} else {
			slog.Warn("redis domain publish failed, delivering locally only", "error", err)
		}
	}
	local()
}

// nextSeq issues the workspace's next sequence number. Callers hold seqMu.
func (h *Hub) nextSeq(workspaceID string) uint64 {
	if h.rdb != nil {
		n, err := h.rdb.Incr(h.ctx, workspaceSeqPrefix+workspaceID).Result()
		if err == nil {
			return uint64(n)
		}
		slog.Warn("redis seq failed, using the local counter", "error", err)
	}
	h.dom.seqs[workspaceID]++
	return h.dom.seqs[workspaceID]
}

// currentSeq reports the last number issued for the workspace.
func (h *Hub) currentSeq(workspaceID string) uint64 {
	if h.rdb != nil {
		n, err := h.rdb.Get(h.ctx, workspaceSeqPrefix+workspaceID).Uint64()
		if err == nil {
			return n
		}
		if !errors.Is(err, redis.Nil) {
			slog.Warn("redis seq read failed, using the local counter", "error", err)
		}
	}
	h.dom.seqMu.Lock()
	defer h.dom.seqMu.Unlock()
	return h.dom.seqs[workspaceID]
}

func domainEnvelope(e realtime.Event, seq uint64) *pb.ServerEnvelope {
	return &pb.ServerEnvelope{Payload: &pb.ServerEnvelope_DomainEvent{DomainEvent: &pb.DomainEvent{
		Domain: e.Domain, Kind: e.Kind, TenantId: e.TenantID, WorkspaceId: e.WorkspaceID,
		Ids: e.IDs, ParentId: e.ParentID, OldParentId: e.OldParentID,
		ActorUserId: e.ActorUserID, Seq: seq, ChannelId: e.ChannelID,
	}}}
}

// offer hands data to a session without ever blocking the publisher. A session
// whose buffer is full misses the event; for workspace events the next one
// shows it a sequence hole and it resynchronises.
func offer(c *Client, data []byte) {
	select {
	case c.send <- data:
	default:
	}
}

func (h *Hub) deliverToWorkspace(tenantID, workspaceID string, data []byte) {
	if tenantID == "" {
		return
	}
	h.mu.RLock()
	defer h.mu.RUnlock()
	for c := range h.dom.workspaces[workspaceID] {
		if c.tenantID == tenantID {
			offer(c, data)
		}
	}
}

func (h *Hub) deliverToChannel(tenantID, channelID string, data []byte) {
	if tenantID == "" {
		return
	}
	h.mu.RLock()
	defer h.mu.RUnlock()
	for c := range h.channels[channelID] {
		if c.tenantID == tenantID {
			offer(c, data)
		}
	}
}

func (h *Hub) deliverToUserNode(tenantID, nodeID string, data []byte) {
	if tenantID == "" || nodeID == "" {
		return
	}
	h.mu.RLock()
	defer h.mu.RUnlock()
	for _, sessions := range h.users {
		for c := range sessions {
			if c.ngacNodeID == nodeID && c.tenantID == tenantID {
				offer(c, data)
			}
		}
	}
}

// deliverRedisDomain routes a message received on one of the domain channels.
func (h *Hub) deliverRedisDomain(channel string, data []byte) bool {
	var prefix string
	switch {
	case strings.HasPrefix(channel, workspaceEventPrefix):
		prefix = workspaceEventPrefix
	case strings.HasPrefix(channel, channelEventPrefix):
		prefix = channelEventPrefix
	case strings.HasPrefix(channel, userEventPrefix):
		prefix = userEventPrefix
	case strings.HasPrefix(channel, permEventPrefix):
		prefix = permEventPrefix
	default:
		return false
	}
	tenantID, target, ok := strings.Cut(strings.TrimPrefix(channel, prefix), ":")
	if !ok {
		return true
	}
	switch prefix {
	case workspaceEventPrefix:
		h.deliverToWorkspace(tenantID, target, data)
	case channelEventPrefix:
		h.deliverToChannel(tenantID, target, data)
	case permEventPrefix:
		h.deliverToUserNode(tenantID, target, data)
		go h.recheckWorkspaceAccess(tenantID, target)
	default:
		h.deliverToUserNode(tenantID, target, data)
	}
	return true
}

func dedupe(in []string) []string {
	seen := make(map[string]bool, len(in))
	out := make([]string, 0, len(in))
	for _, s := range in {
		if s != "" && !seen[s] {
			seen[s] = true
			out = append(out, s)
		}
	}
	return out
}

// handleWorkspaceSubscribe processes a workspace SubscribeRequest.
func (c *Client) handleWorkspaceSubscribe(workspaceID string) {
	who := grpcauth.Caller{UserID: c.userID, NGACNodeID: c.ngacNodeID, TenantID: c.tenantID}
	if err := c.hub.authorizeWorkspaceSubscribe(workspaceID, who); err != nil {
		slog.Warn("websocket workspace subscribe denied",
			"user_id", c.userID, "workspace_id", workspaceID, "error", err)
		c.sendError(403, "not allowed to subscribe to this workspace")
		// Answer as well, so a client waiting on the subscription is released.
		if data := marshalEnvelope(&pb.ServerEnvelope{Payload: &pb.ServerEnvelope_WorkspaceSubscribed{
			WorkspaceSubscribed: &pb.WorkspaceSubscribed{WorkspaceId: workspaceID, Denied: true},
		}}); data != nil {
			offer(c, data)
		}
		return
	}
	seq := c.hub.subscribeWorkspace(workspaceID, c)
	data := marshalEnvelope(&pb.ServerEnvelope{Payload: &pb.ServerEnvelope_WorkspaceSubscribed{
		WorkspaceSubscribed: &pb.WorkspaceSubscribed{WorkspaceId: workspaceID, Seq: seq},
	}})
	if data != nil {
		offer(c, data)
	}
}

// --- presence ---------------------------------------------------------------

func presenceKey(tenantID, userID string) string { return tenantID + "|" + userID }

// sessionsOf counts the user's open sessions in the tenant.
func (h *Hub) sessionsOf(tenantID, userID string) int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	n := 0
	for c := range h.users[userID] {
		if c.tenantID == tenantID {
			n++
		}
	}
	return n
}

// userCameOnline announces a user, unless a recent absence was never announced
// (then they never looked gone) or another session of theirs already did.
func (h *Hub) userCameOnline(c *Client) {
	key := presenceKey(c.tenantID, c.userID)
	h.dom.presenceMu.Lock()
	t, pending := h.dom.pendingAbsent[key]
	if pending {
		t.Stop()
		delete(h.dom.pendingAbsent, key)
	}
	h.dom.presenceMu.Unlock()
	if pending {
		return
	}
	h.BroadcastPresence(c.tenantID, c.userID, c.username, "online")
}

// userLeft schedules the offline announcement for after the grace period and
// sends it only if the user still has no session in the tenant by then.
func (h *Hub) userLeft(c *Client) {
	tenantID, userID, username := c.tenantID, c.userID, c.username
	announce := func() {
		if h.sessionsOf(tenantID, userID) == 0 {
			h.BroadcastPresence(tenantID, userID, username, "offline")
		}
	}
	h.dom.presenceMu.Lock()
	grace := h.dom.presenceGrace
	key := presenceKey(tenantID, userID)
	if old, ok := h.dom.pendingAbsent[key]; ok {
		old.Stop()
	}
	if grace <= 0 {
		delete(h.dom.pendingAbsent, key)
		h.dom.presenceMu.Unlock()
		announce()
		return
	}
	var timer *time.Timer
	timer = time.AfterFunc(grace, func() {
		h.dom.presenceMu.Lock()
		if h.dom.pendingAbsent[key] != timer {
			h.dom.presenceMu.Unlock()
			return // superseded by a reconnect or a newer departure
		}
		delete(h.dom.pendingAbsent, key)
		h.dom.presenceMu.Unlock()
		announce()
	})
	h.dom.pendingAbsent[key] = timer
	h.dom.presenceMu.Unlock()
}
