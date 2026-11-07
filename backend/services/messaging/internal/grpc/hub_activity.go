package grpc

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"google.golang.org/protobuf/proto"

	pb "ngac-platform/proto/messaging"
)

// Conversation activity (docs/specs/chat-unread-counts): every message sent to
// or deleted from a conversation is told, without its text, to every session
// of every member, so each keeps its unread counts without asking. One publish
// per message goes to the conversation's workspace; each instance hands it to
// the sessions it holds of that workspace whose person is a member.
//
// Who is a member comes from the channel_members cache, which messaging keeps
// beside the graph (as push notifications do): a frame carries ids and numbers
// only, and the conversation itself is read through its access check.

const (
	activityKeyPrefix = "activity:"
	// directoryTTL bounds how long an instance trusts a conversation's member
	// list. Removals forget it at once (revokeLocal, revokeWorkspaceLocal); a
	// person added hears of the conversation's activity within this time.
	directoryTTL = 30 * time.Second
	// activityQueue is how many events wait, to be published or delivered,
	// before new ones are dropped; a dropped event shows as a gap in the
	// numbers, which a client heals by asking for its counts.
	activityQueue = 4096
	// directoryTimeout bounds one read of a conversation's members.
	directoryTimeout = 2 * time.Second
)

// ChannelDirectory answers who belongs to a conversation: its workspace and the
// NGAC nodes of its members. *store.Store does.
type ChannelDirectory interface {
	ChannelMemberNodes(ctx context.Context, channelID string) (workspaceID string, nodeIDs []string, err error)
}

type directoryEntry struct {
	workspaceID string
	nodes       map[string]bool
	loaded      time.Time
}

type activityDelivery struct {
	workspaceID string
	data        []byte
}

type activityState struct {
	mu    sync.Mutex
	dir   ChannelDirectory
	known map[string]*directoryEntry // channelID → members; an entry is never changed in place
	// epoch moves on every removal, so a member list read before a removal is
	// not remembered after it.
	epoch   uint64
	publish chan *pb.ConversationActivityEvent
	deliver chan activityDelivery
}

func newActivityState() activityState {
	return activityState{
		known:   map[string]*directoryEntry{},
		publish: make(chan *pb.ConversationActivityEvent, activityQueue),
		deliver: make(chan activityDelivery, activityQueue),
	}
}

// SetChannelDirectory turns conversation activity on. Without a directory no
// activity is published or delivered.
func (h *Hub) SetChannelDirectory(d ChannelDirectory) {
	h.act.mu.Lock()
	defer h.act.mu.Unlock()
	h.act.dir = d
}

// channelMembers returns a conversation's workspace and members, from memory
// while fresh, else from the directory. A list read while a removal happened is
// used once but not remembered.
func (h *Hub) channelMembers(channelID string) (*directoryEntry, bool) {
	h.act.mu.Lock()
	dir, e, epoch := h.act.dir, h.act.known[channelID], h.act.epoch
	h.act.mu.Unlock()
	if dir == nil {
		return nil, false
	}
	if e != nil && time.Since(e.loaded) < directoryTTL {
		return e, true
	}
	ctx, cancel := context.WithTimeout(h.ctx, directoryTimeout)
	defer cancel()
	ws, nodes, err := dir.ChannelMemberNodes(ctx, channelID)
	if err != nil {
		slog.Warn("conversation members not loaded", "channel_id", channelID, "error", err)
		return nil, false
	}
	e = &directoryEntry{workspaceID: ws, nodes: make(map[string]bool, len(nodes)), loaded: time.Now()}
	for _, n := range nodes {
		e.nodes[n] = true
	}
	h.act.mu.Lock()
	if h.act.epoch == epoch {
		h.act.known[channelID] = e
	}
	h.act.mu.Unlock()
	return e, true
}

// forgetMember is told a person left a conversation, or a whole workspace when
// channelID is "": the conversations they were remembered in are forgotten and
// read again from the directory (which no longer lists them) when next needed,
// and a read already under way is not remembered.
func (h *Hub) forgetMember(workspaceID, channelID, nodeID string) {
	h.act.mu.Lock()
	defer h.act.mu.Unlock()
	h.act.epoch++
	for id, e := range h.act.known {
		if !e.nodes[nodeID] {
			continue
		}
		if (channelID != "" && id == channelID) || (channelID == "" && e.workspaceID == workspaceID) {
			delete(h.act.known, id)
		}
	}
}

// activityOfMessage is the activity a stored main message makes.
func activityOfMessage(m *pb.Message) *pb.ConversationActivityEvent {
	return &pb.ConversationActivityEvent{
		ChannelId: m.ChannelId, Kind: "sent", Seq: m.Seq, RootMessageId: m.ParentMessageId,
		SenderId: m.SenderId, Mentions: m.Mentions, LastSeq: m.Seq,
	}
}

// publishActivity queues a conversation's activity to be published. It never
// blocks or reads the database on the caller's path (a send request): the
// activity loop finds the workspace and publishes, in order.
func (h *Hub) publishActivity(ev *pb.ConversationActivityEvent) {
	if ev.ChannelId == "" {
		return
	}
	select {
	case h.act.publish <- ev:
	default:
		slog.Warn("conversation activity dropped: publish queue full", "channel_id", ev.ChannelId)
	}
}

// enqueueActivity hands an envelope to the activity loop for delivery; it
// never blocks the caller (the Redis subscriber).
func (h *Hub) enqueueActivity(workspaceID string, data []byte) {
	select {
	case h.act.deliver <- activityDelivery{workspaceID: workspaceID, data: data}:
	default:
		slog.Warn("conversation activity dropped: delivery queue full", "workspace_id", workspaceID)
	}
}

// runActivity publishes and delivers conversation activity until the hub closes.
func (h *Hub) runActivity() {
	for {
		select {
		case <-h.ctx.Done():
			return
		case ev := <-h.act.publish:
			e, ok := h.channelMembers(ev.ChannelId)
			if !ok || e.workspaceID == "" {
				continue
			}
			data := marshalEnvelope(&pb.ServerEnvelope{Payload: &pb.ServerEnvelope_ConversationActivity{ConversationActivity: ev}})
			if data == nil {
				continue
			}
			h.route(activityKeyPrefix+e.workspaceID, data, func() { h.enqueueActivity(e.workspaceID, data) })
		case d := <-h.act.deliver:
			h.deliverActivity(d.workspaceID, d.data)
		}
	}
}

// hasSessionsIn reports whether this instance holds any session following the
// workspace's stream.
func (h *Hub) hasSessionsIn(workspaceID string) bool {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return len(h.dom.workspaces[workspaceID]) > 0
}

// deliverActivity hands an activity envelope to every session this instance
// holds whose person belongs to the workspace and is a member of the
// conversation. An instance with no session of that workspace reads nothing.
func (h *Hub) deliverActivity(workspaceID string, data []byte) {
	if !h.hasSessionsIn(workspaceID) {
		return
	}
	var env pb.ServerEnvelope
	if err := proto.Unmarshal(data, &env); err != nil {
		return
	}
	ev := env.GetConversationActivity()
	if ev == nil {
		return
	}
	e, ok := h.channelMembers(ev.ChannelId)
	if !ok || e.workspaceID != workspaceID {
		return
	}
	// Every session of the app follows its workspace's stream, so those are the
	// sessions to look through, not every session on the instance.
	h.mu.RLock()
	defer h.mu.RUnlock()
	for c := range h.dom.workspaces[workspaceID] {
		if c.tenantID == workspaceID && e.nodes[c.ngacNodeID] {
			offer(c, data)
		}
	}
}
