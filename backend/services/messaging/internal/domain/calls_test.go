package domain_test

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"ngac-platform/ngac"
	"ngac-platform/pkg/grpcauth"
	"ngac-platform/services/messaging/internal/domain"
	"ngac-platform/testutil"
)

// fakeMedia stands in for the LiveKit server and records what it was asked.
type fakeMedia struct {
	mu      sync.Mutex
	rooms   map[string]bool
	removed []string // room/identity
	present map[string][]string
	tokens  []string // room/identity
}

func newFakeMedia() *fakeMedia {
	return &fakeMedia{rooms: map[string]bool{}, present: map[string][]string{}}
}

func (m *fakeMedia) PublicURL() string { return "wss://calls.test/livekit" }
func (m *fakeMedia) CreateRoom(_ context.Context, name string, _ uint32, _ time.Duration) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.rooms[name] = true
	return nil
}
func (m *fakeMedia) DeleteRoom(_ context.Context, name string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.rooms, name)
	return nil
}
func (m *fakeMedia) RemoveParticipant(_ context.Context, room, identity string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.removed = append(m.removed, room+"/"+identity)
	return nil
}
func (m *fakeMedia) Participants(_ context.Context, room string) ([]string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.present[room], nil
}
func (m *fakeMedia) JoinToken(room, identity, _ string, _ time.Duration) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.tokens = append(m.tokens, room+"/"+identity)
	return "token-for-" + identity, nil
}

// recordingCalls keeps every call change the service announced.
type recordingCalls struct {
	mu      sync.Mutex
	changes []domain.CallChange
}

func (r *recordingCalls) CallChanged(_ context.Context, c domain.CallChange) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.changes = append(r.changes, c)
}

func (r *recordingCalls) types() []string {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]string, 0, len(r.changes))
	for _, c := range r.changes {
		out = append(out, c.Type)
	}
	return out
}

// callWorld is a conversation where the caller writes, a peer writes, and a
// reader only reads.
type callWorld struct {
	*talk
	media              *fakeMedia
	calls              *recordingCalls
	peer, peerNode     string
	reader, readerNode string
	nodes              map[string]string // user id → the node they act as here
}

func newCallWorld(t *testing.T) *callWorld {
	t.Helper()
	k := newTalk(t, "calls")
	w := &callWorld{talk: k, media: newFakeMedia(), calls: &recordingCalls{}, nodes: map[string]string{}}
	k.f.svc.SetCallMedia(w.media)
	k.f.svc.SetCallObserver(w.calls)
	w.peer, w.peerNode = testutil.CreateUser(t, k.f.pool)
	w.reader, w.readerNode = testutil.CreateUser(t, k.f.pool)
	w.writer(w.peerNode)
	k.f.read.allow[grant{w.readerNode, k.oaID, ngac.OpRead}] = true
	w.nodes[k.f.userID], w.nodes[w.peer], w.nodes[w.reader] = userNode, w.peerNode, w.readerNode
	t.Cleanup(func() {
		k.f.pool.Exec(context.Background(), `DELETE FROM calls WHERE channel_id = $1`, k.chID)
	})
	return w
}

// writer lets a node read and write in the conversation.
func (w *callWorld) writer(node string) {
	w.f.read.mu.Lock()
	defer w.f.read.mu.Unlock()
	w.f.read.allow[grant{node, w.oaID, ngac.OpRead}] = true
	w.f.read.allow[grant{node, w.oaID, ngac.OpWrite}] = true
}

// member is a new person who reads and writes in the conversation.
func (w *callWorld) member(t *testing.T) string {
	t.Helper()
	u, node := testutil.CreateUser(t, w.f.pool)
	w.writer(node)
	w.nodes[u] = node
	return u
}

// asDM turns the conversation into a direct message between caller and peer.
// The peer is a member by their workspace node (tenant_users), as people are
// in a workspace; the caller by their own.
func (w *callWorld) asDM(t *testing.T) {
	t.Helper()
	ctx := context.Background()
	_, err := w.f.pool.Exec(ctx, `UPDATE channels SET channel_type = 'dm', workspace_id = NULL WHERE id = $1`, w.chID)
	require.NoError(t, err)
	var callerNode string
	require.NoError(t, w.f.pool.QueryRow(ctx, `SELECT ngac_node FROM users WHERE id = $1`, w.f.userID).Scan(&callerNode))
	peerTenantNode := "tn-" + w.peer
	_, err = w.f.pool.Exec(ctx, `INSERT INTO ngac_nodes (id, name, node_type, properties) VALUES ($1, $1, 'U', '{}')`, peerTenantNode)
	require.NoError(t, err)
	_, err = w.f.pool.Exec(ctx, `INSERT INTO tenant_users (tenant_id, user_id, ngac_node_id) VALUES ($1, $2, $3)`, w.f.wsID, w.peer, peerTenantNode)
	require.NoError(t, err)
	for _, node := range []string{callerNode, peerTenantNode} {
		_, err := w.f.pool.Exec(ctx, `INSERT INTO channel_members (channel_id, ngac_node_id) VALUES ($1, $2) ON CONFLICT DO NOTHING`, w.chID, node)
		require.NoError(t, err)
	}
	t.Cleanup(func() {
		w.f.pool.Exec(ctx, `DELETE FROM channel_members WHERE channel_id = $1`, w.chID)
		w.f.pool.Exec(ctx, `DELETE FROM tenant_users WHERE user_id = $1 AND ngac_node_id = $2`, w.peer, peerTenantNode)
		w.f.pool.Exec(ctx, `DELETE FROM ngac_nodes WHERE id = $1`, peerTenantNode)
	})
}

func (w *callWorld) start(t *testing.T, kind string) string {
	t.Helper()
	join, _, err := w.f.svc.StartCall(context.Background(), domain.StartCallInput{
		ChannelID: w.chID, UserID: w.f.userID, UserNodeID: userNode, Kind: kind,
	})
	require.NoError(t, err)
	return join.Call.Id
}

// joined is a person entering the room as LiveKit reports it: with the ticket
// they were handed, when they could get one.
func (w *callWorld) joined(t *testing.T, callID, userID string) {
	t.Helper()
	ctx := context.Background()
	_, _ = w.f.svc.CallToken(ctx, callID, userID, w.nodes[userID])
	require.NoError(t, w.f.svc.HandleCallEvent(ctx, domain.CallEvent{
		Kind: domain.CallEventJoined, Room: callID, Identity: userID,
	}))
}

func TestStartCall_GroupIsActiveWithCardRoomAndTicket(t *testing.T) {
	w := newCallWorld(t)
	ctx := context.Background()
	join, card, err := w.f.svc.StartCall(ctx, domain.StartCallInput{
		ChannelID: w.chID, UserID: w.f.userID, UserNodeID: userNode, Kind: "video",
	})
	require.NoError(t, err)
	assert.Equal(t, "active", join.Call.Status, "a group call does not ring")
	assert.Equal(t, "wss://calls.test/livekit", join.Url)
	assert.Equal(t, "token-for-"+w.f.userID, join.Token)
	assert.True(t, w.media.rooms[join.Call.Id], "the room is opened ahead of the first person")
	require.NotNil(t, card)
	assert.Equal(t, "call", card.LinkedEntityType)
	assert.Equal(t, join.Call.Id, card.LinkedEntityId)
	assert.Equal(t, join.Call.MessageId, card.Id)
	assert.Equal(t, []string{domain.CallChangeState}, w.calls.types())
}

func TestStartCall_DirectMessageRingsThePeer(t *testing.T) {
	w := newCallWorld(t)
	w.asDM(t)
	join, _, err := w.f.svc.StartCall(context.Background(), domain.StartCallInput{
		ChannelID: w.chID, UserID: w.f.userID, UserNodeID: userNode, Kind: "audio",
	})
	require.NoError(t, err)
	assert.Equal(t, "ringing", join.Call.Status)
	require.NotNil(t, join.Call.RingExpiresAt)
	left := join.Call.RingExpiresAt.AsTime().Sub(time.Now())
	assert.InDelta(t, 45, left.Seconds(), 3, "a direct call rings for 45 seconds")
	require.Len(t, w.calls.changes, 1)
	ring := w.calls.changes[0]
	assert.Equal(t, domain.CallChangeRing, ring.Type)
	assert.Equal(t, []string{w.peer}, ring.Users, "the peer's devices ring, not the caller's")
}

func TestStartCall_SecondStartJoinsTheOpenCall(t *testing.T) {
	w := newCallWorld(t)
	first := w.start(t, "video")
	join, card, err := w.f.svc.StartCall(context.Background(), domain.StartCallInput{
		ChannelID: w.chID, UserID: w.peer, UserNodeID: w.peerNode, Kind: "audio",
	})
	require.NoError(t, err)
	assert.Equal(t, first, join.Call.Id, "one open call per conversation")
	assert.Nil(t, card, "joining posts no second card")
	assert.Equal(t, "token-for-"+w.peer, join.Token)
}

func TestStartCall_Deny(t *testing.T) {
	w := newCallWorld(t)
	ctx := context.Background()
	in := func(user, node, kind string) domain.StartCallInput {
		return domain.StartCallInput{ChannelID: w.chID, UserID: user, UserNodeID: node, Kind: kind}
	}
	// Deny: someone who cannot write in the conversation starts nothing.
	_, _, err := w.f.svc.StartCall(ctx, in(w.reader, w.readerNode, "video"))
	require.ErrorIs(t, err, domain.ErrAccessDenied, "a reader")
	_, _, err = w.f.svc.StartCall(ctx, in("someone", outsider, "video"))
	require.ErrorIs(t, err, domain.ErrAccessDenied, "an outsider")
	_, _, err = w.f.svc.StartCall(ctx, in(w.f.userID, userNode, "hologram"))
	require.ErrorIs(t, err, domain.ErrInvalidInput)
	assert.Empty(t, w.media.rooms, "a refused start opens no room")
	assert.Empty(t, w.media.tokens, "and signs no ticket")
}

func TestStartCall_WithoutMediaIsUnavailable(t *testing.T) {
	k := newTalk(t, "no-calls")
	_, _, err := k.f.svc.StartCall(context.Background(), domain.StartCallInput{
		ChannelID: k.chID, UserID: k.f.userID, UserNodeID: userNode, Kind: "video",
	})
	require.ErrorIs(t, err, domain.ErrCallsUnavailable)
}

func TestCallToken_DenyAndClosedAndFull(t *testing.T) {
	w := newCallWorld(t)
	ctx := context.Background()
	svc := w.f.svc
	id := w.start(t, "video")

	join, err := svc.CallToken(ctx, id, w.peer, w.peerNode)
	require.NoError(t, err)
	assert.Equal(t, "token-for-"+w.peer, join.Token)

	// Deny: a reader, an outsider, and a member removed since the call began.
	_, err = svc.CallToken(ctx, id, w.reader, w.readerNode)
	require.ErrorIs(t, err, domain.ErrAccessDenied)
	_, err = svc.CallToken(ctx, id, "someone", outsider)
	require.ErrorIs(t, err, domain.ErrAccessDenied)
	delete(w.f.read.allow, grant{w.peerNode, w.oaID, ngac.OpWrite})
	_, err = svc.CallToken(ctx, id, w.peer, w.peerNode)
	require.ErrorIs(t, err, domain.ErrAccessDenied, "removed from the conversation, no more tickets")

	_, err = svc.CallToken(ctx, "no-such-call", w.f.userID, userNode)
	require.ErrorIs(t, err, domain.ErrNotFound)

	// Full: twenty people are in.
	for i := 0; i < domain.MaxCallParticipants; i++ {
		w.joined(t, id, w.member(t))
	}
	_, err = svc.CallToken(ctx, id, w.f.userID, userNode)
	require.ErrorIs(t, err, domain.ErrCallFull)

	// Ended: the room closed.
	require.NoError(t, svc.HandleCallEvent(ctx, domain.CallEvent{Kind: domain.CallEventRoomFinished, Room: id}))
	_, err = svc.CallToken(ctx, id, w.f.userID, userNode)
	require.ErrorIs(t, err, domain.ErrCallEnded)
}

func TestCallToken_SomeoneAlreadyInsideMayRejoinAFullCall(t *testing.T) {
	w := newCallWorld(t)
	ctx := context.Background()
	id := w.start(t, "video")
	w.joined(t, id, w.f.userID)
	for i := 0; i < domain.MaxCallParticipants-1; i++ {
		w.joined(t, id, w.member(t))
	}
	_, err := w.f.svc.CallToken(ctx, id, w.f.userID, userNode)
	require.NoError(t, err, "a person already in takes no new place (another device, a reconnect)")
}

func TestGetCall_ReadersSeeWhoIsIn(t *testing.T) {
	w := newCallWorld(t)
	ctx := context.Background()
	id := w.start(t, "video")
	w.joined(t, id, w.f.userID)
	w.joined(t, id, w.peer)

	c, err := w.f.svc.GetCall(ctx, id, w.readerNode)
	require.NoError(t, err, "reading the conversation is enough to see its call")
	require.Len(t, c.Participants, 2)
	assert.NotEmpty(t, c.Participants[0].DisplayName)
	assert.NotEmpty(t, c.StartedByName)

	// Deny: an outsider learns nothing.
	_, err = w.f.svc.GetCall(ctx, id, outsider)
	require.ErrorIs(t, err, domain.ErrAccessDenied)
}

func TestDirectCall_AnsweredWhenThePeerJoins(t *testing.T) {
	w := newCallWorld(t)
	w.asDM(t)
	ctx := context.Background()
	id := w.start(t, "video")
	w.joined(t, id, w.f.userID)
	c, err := w.f.svc.GetCall(ctx, id, userNode)
	require.NoError(t, err)
	assert.Equal(t, "ringing", c.Status, "the caller alone does not answer their own call")

	w.joined(t, id, w.peer)
	c, err = w.f.svc.GetCall(ctx, id, userNode)
	require.NoError(t, err)
	assert.Equal(t, "active", c.Status)
	assert.NotNil(t, c.StartedAt)
	assert.Contains(t, w.calls.types(), domain.CallChangeAnswered)
}

func TestDeclineAndCancel(t *testing.T) {
	w := newCallWorld(t)
	w.asDM(t)
	ctx := context.Background()
	svc := w.f.svc

	id := w.start(t, "video")
	// Deny: the caller does not decline their own call, the peer does not cancel it.
	_, err := svc.DeclineCall(ctx, id, w.f.userID, userNode)
	require.ErrorIs(t, err, domain.ErrAccessDenied)
	_, err = svc.CancelCall(ctx, id, w.peer, w.peerNode)
	require.ErrorIs(t, err, domain.ErrAccessDenied)
	// Deny: an outsider does neither.
	_, err = svc.DeclineCall(ctx, id, "someone", outsider)
	require.ErrorIs(t, err, domain.ErrAccessDenied)

	c, err := svc.DeclineCall(ctx, id, w.peer, w.peerNode)
	require.NoError(t, err)
	assert.Equal(t, "declined", c.Status)
	assert.False(t, w.media.rooms[id], "the room closes")
	_, err = svc.CancelCall(ctx, id, w.f.userID, userNode)
	require.ErrorIs(t, err, domain.ErrCallEnded, "a declined call cannot be cancelled")

	id = w.start(t, "audio")
	c, err = svc.CancelCall(ctx, id, w.f.userID, userNode)
	require.NoError(t, err)
	assert.Equal(t, "cancelled", c.Status)
	assert.Contains(t, w.calls.types(), domain.CallChangeCancel)
}

func TestExpireRinging_MissedOnceAndCardSaysSo(t *testing.T) {
	w := newCallWorld(t)
	w.asDM(t)
	ctx := context.Background()
	id := w.start(t, "video")

	missed, err := w.f.svc.ExpireRingingCalls(ctx, time.Now().Add(time.Minute))
	require.NoError(t, err)
	require.Len(t, missed, 1)
	assert.Equal(t, id, missed[0])
	again, err := w.f.svc.ExpireRingingCalls(ctx, time.Now().Add(time.Minute))
	require.NoError(t, err)
	assert.Empty(t, again, "a missed call is missed once")

	c, err := w.f.svc.GetCall(ctx, id, userNode)
	require.NoError(t, err)
	assert.Equal(t, "missed", c.Status)
	var text string
	require.NoError(t, w.f.pool.QueryRow(ctx, `SELECT content FROM messages WHERE id = $1`, c.MessageId).Scan(&text))
	assert.Contains(t, text, "nhỡ")
	assert.Contains(t, w.calls.types(), domain.CallChangeMissed)
}

func TestRoomFinished_EndsTheCallWithItsLength(t *testing.T) {
	w := newCallWorld(t)
	ctx := context.Background()
	id := w.start(t, "video")
	w.joined(t, id, w.f.userID)
	w.joined(t, id, w.peer)
	require.NoError(t, w.f.svc.HandleCallEvent(ctx, domain.CallEvent{Kind: domain.CallEventLeft, Room: id, Identity: w.peer}))
	c, err := w.f.svc.GetCall(ctx, id, userNode)
	require.NoError(t, err)
	require.Len(t, c.Participants, 1, "the peer left")

	require.NoError(t, w.f.svc.HandleCallEvent(ctx, domain.CallEvent{Kind: domain.CallEventRoomFinished, Room: id}))
	c, err = w.f.svc.GetCall(ctx, id, userNode)
	require.NoError(t, err)
	assert.Equal(t, "ended", c.Status)
	assert.Empty(t, c.Participants)
	assert.EqualValues(t, 2, c.TakerCount)
	var text string
	require.NoError(t, w.f.pool.QueryRow(ctx, `SELECT content FROM messages WHERE id = $1`, c.MessageId).Scan(&text))
	assert.Contains(t, text, "Cuộc gọi video")
	// A second room_finished (LiveKit retries) changes nothing.
	require.NoError(t, w.f.svc.HandleCallEvent(ctx, domain.CallEvent{Kind: domain.CallEventRoomFinished, Room: id}))
}

// Deny: a person who enters the room without the right to be there (a ticket
// signed before they were removed) is put out at once.
func TestParticipantJoined_WithoutWriteIsRemoved(t *testing.T) {
	w := newCallWorld(t)
	ctx := context.Background()
	id := w.start(t, "video")

	// A reader can get no ticket; in the room anyway, they are put out.
	w.joined(t, id, w.reader)
	assert.Equal(t, []string{id + "/" + w.reader}, w.media.removed)

	// A ticket taken before losing write does not keep anyone in.
	_, err := w.f.svc.CallToken(ctx, id, w.peer, w.peerNode)
	require.NoError(t, err)
	delete(w.f.read.allow, grant{w.peerNode, w.oaID, ngac.OpWrite})
	require.NoError(t, w.f.svc.HandleCallEvent(ctx, domain.CallEvent{Kind: domain.CallEventJoined, Room: id, Identity: w.peer}))
	assert.Equal(t, []string{id + "/" + w.reader, id + "/" + w.peer}, w.media.removed)

	// Someone this service never handed a ticket (a token made elsewhere) is put out.
	stranger := w.member(t)
	require.NoError(t, w.f.svc.HandleCallEvent(ctx, domain.CallEvent{Kind: domain.CallEventJoined, Room: id, Identity: stranger}))
	assert.Contains(t, w.media.removed, id+"/"+stranger)

	c, err := w.f.svc.GetCall(ctx, id, userNode)
	require.NoError(t, err)
	assert.Empty(t, c.Participants, "a refused person is not recorded as in the call")

	// An unknown room is not an error: LiveKit may outlive a deleted call.
	require.NoError(t, w.f.svc.HandleCallEvent(ctx, domain.CallEvent{Kind: domain.CallEventJoined, Room: "gone", Identity: w.peer}))
}

// The policy service answers only on behalf of a person. A webhook has none, so
// the check runs as the participant themself — the node and workspace they
// were handed their ticket as — never as a service.
func TestParticipantJoined_ChecksAsTheParticipant(t *testing.T) {
	w := newCallWorld(t)
	id := w.start(t, "video")
	ctx := grpcauth.WithCaller(context.Background(), grpcauth.Caller{UserID: w.peer, NGACNodeID: w.peerNode, TenantID: w.f.wsID})
	_, err := w.f.svc.CallToken(ctx, id, w.peer, w.peerNode)
	require.NoError(t, err)

	require.NoError(t, w.f.svc.HandleCallEvent(context.Background(), domain.CallEvent{Kind: domain.CallEventJoined, Room: id, Identity: w.peer}))
	w.f.read.mu.Lock()
	got := w.f.read.callers[w.peerNode]
	w.f.read.mu.Unlock()
	assert.Equal(t, grpcauth.Caller{UserID: w.peer, NGACNodeID: w.peerNode, TenantID: w.f.wsID}, got)
	assert.Empty(t, w.media.removed)
}

func TestEvictLostCallParticipants_RemovesWhoLostWrite(t *testing.T) {
	w := newCallWorld(t)
	ctx := context.Background()
	id := w.start(t, "video")
	w.joined(t, id, w.f.userID)
	w.joined(t, id, w.peer)
	delete(w.f.read.allow, grant{w.peerNode, w.oaID, ngac.OpWrite})

	require.NoError(t, w.f.svc.EvictLostCallParticipants(ctx, w.chID, ""))
	assert.Equal(t, []string{id + "/" + w.peer}, w.media.removed, "only the person who lost the right")

	c, err := w.f.svc.GetCall(ctx, id, userNode)
	require.NoError(t, err)
	require.Len(t, c.Participants, 1, "the removed person is no longer counted in the call")

	// The same sweep by workspace reaches the call too.
	w.writer(w.peerNode)
	w.joined(t, id, w.peer)
	delete(w.f.read.allow, grant{w.peerNode, w.oaID, ngac.OpWrite})
	w.media.removed = nil
	require.NoError(t, w.f.svc.EvictLostCallParticipants(ctx, "", w.f.wsID))
	assert.Equal(t, []string{id + "/" + w.peer}, w.media.removed)
}

func TestCallCardText(t *testing.T) {
	start := time.Date(2026, 10, 13, 8, 0, 0, 0, time.UTC)
	cases := []struct {
		kind, status string
		length       time.Duration
		want         string
	}{
		{"video", "active", 0, "📹 Cuộc gọi video"},
		{"audio", "ringing", 0, "📞 Cuộc gọi thoại"},
		{"video", "ended", 32 * time.Minute, "📹 Cuộc gọi video · 32 phút"},
		{"audio", "ended", 20 * time.Second, "📞 Cuộc gọi thoại · dưới 1 phút"},
		{"video", "ended", 75 * time.Minute, "📹 Cuộc gọi video · 1 giờ 15 phút"},
		{"video", "missed", 0, "📹 Cuộc gọi nhỡ"},
		{"audio", "declined", 0, "📞 Cuộc gọi bị từ chối"},
		{"audio", "cancelled", 0, "📞 Cuộc gọi đã huỷ"},
	}
	for _, c := range cases {
		t.Run(fmt.Sprintf("%s-%s", c.kind, c.status), func(t *testing.T) {
			var started *time.Time
			var ended *time.Time
			if c.length > 0 {
				e := start.Add(c.length)
				started, ended = &start, &e
			}
			assert.Equal(t, c.want, domain.CallCardText(c.kind, c.status, started, ended))
		})
	}
}

func TestConversationActivity_ACallCardPreviewsAsACall(t *testing.T) {
	w := newCallWorld(t)
	id := w.start(t, "video")
	got := activityOf(t, w.talk, userNode)[w.chID]
	assert.Equal(t, "call", got.Latest.Kind)
	assert.Equal(t, "📹 Cuộc gọi video", got.Latest.Preview)

	require.NoError(t, w.f.svc.HandleCallEvent(context.Background(), domain.CallEvent{Kind: domain.CallEventRoomFinished, Room: id}))
	got = activityOf(t, w.talk, userNode)[w.chID]
	assert.Equal(t, "📹 Cuộc gọi video · dưới 1 phút", got.Latest.Preview, "the preview follows the card")
}

// A person cannot post a call card by hand: the card is the call's own.
func TestSendMessage_RefusesAHandMadeCallCard(t *testing.T) {
	w := newCallWorld(t)
	_, err := w.f.svc.SendMessage(context.Background(), domain.SendMessageInput{
		ChannelID: w.chID, SenderID: w.f.userID, SenderNodeID: userNode,
		Content: "📹 Cuộc gọi video", LinkedEntityType: "call", LinkedEntityID: "fake",
	})
	require.ErrorIs(t, err, domain.ErrInvalidInput)
}
