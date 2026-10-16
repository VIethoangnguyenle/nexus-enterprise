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
	"ngac-platform/pkg/realtime"
	pb "ngac-platform/proto/messaging"
	"ngac-platform/services/messaging/internal/domain"
	"ngac-platform/testutil"
)

func channelIDs(list []*pb.Channel) []string {
	out := make([]string, 0, len(list))
	for _, c := range list {
		out = append(out, c.Id)
	}
	return out
}

func TestChatState_AllowReturnsEveryPart(t *testing.T) {
	w := newSectionWorld(t)
	ctx := context.Background()
	svc := w.f.svc

	sec, err := svc.CreateSection(ctx, w.f.wsID, w.me, domain.NewSection{Name: "Công việc"})
	require.NoError(t, err)
	require.NoError(t, svc.PlaceChannel(ctx, w.f.wsID, w.me, sec.ID, w.chA))

	got, err := svc.ChatState(ctx, w.f.wsID, w.me)
	require.NoError(t, err)

	assert.ElementsMatch(t, []string{w.chA, w.chB}, channelIDs(got.Channels))
	var unreadFor []string
	for _, u := range got.Unread {
		unreadFor = append(unreadFor, u.ChannelId)
	}
	assert.Contains(t, unreadFor, w.chA)
	var activityFor []string
	for _, a := range got.Activity {
		activityFor = append(activityFor, a.ChannelId)
	}
	assert.Contains(t, activityFor, w.chA, "chA has a message, so it has a preview")
	require.Len(t, got.Sections, 1)
	assert.Equal(t, []string{w.chA}, got.Sections[0].ChannelIDs)
	assert.Zero(t, got.UnreadMentions)
	assert.Empty(t, got.UnreadThreads)

	// The part for each endpoint equals what the endpoint itself answers.
	chans, err := svc.ListChannels(ctx, w.f.wsID, userNode)
	require.NoError(t, err)
	assert.ElementsMatch(t, channelIDs(chans), channelIDs(got.Channels))
}

func TestChatState_DenyHidesWhatTheCallerCannotRead(t *testing.T) {
	w := newSectionWorld(t)
	ctx := context.Background()

	// A third channel the caller holds no grant on, with a message in it.
	hidden, hiddenOA := w.f.insertChannel(t, "hidden")
	sender, senderNode := testutil.CreateUser(t, w.f.pool)
	w.f.read.allow[grant{senderNode, hiddenOA, ngac.OpWrite}] = true
	_, err := w.f.svc.SendMessage(ctx, domain.SendMessageInput{
		ChannelID: hidden, SenderID: sender, SenderNodeID: senderNode, Content: "secret plans",
	})
	require.NoError(t, err)
	t.Cleanup(func() { w.f.pool.Exec(ctx, `DELETE FROM messages WHERE channel_id = $1`, hidden) })

	got, err := w.f.svc.ChatState(ctx, w.f.wsID, w.me)
	require.NoError(t, err)

	assert.NotContains(t, channelIDs(got.Channels), hidden, "channels without read are absent")
	for _, u := range got.Unread {
		assert.NotEqual(t, hidden, u.ChannelId, "no unread count for a channel the caller cannot read")
	}
	for _, a := range got.Activity {
		assert.NotEqual(t, hidden, a.ChannelId, "no preview of a channel the caller cannot read")
		if a.Latest != nil {
			assert.NotContains(t, a.Latest.Preview, "secret")
		}
	}
}

func TestChatState_RefusesAnotherWorkspaceAndAnonymousCallers(t *testing.T) {
	w := newSectionWorld(t)
	ctx := context.Background()

	_, err := w.f.svc.ChatState(ctx, "some-other-workspace", w.me)
	require.ErrorIs(t, err, domain.ErrAccessDenied)

	foreign := w.me
	foreign.TenantID = "some-other-tenant"
	_, err = w.f.svc.ChatState(ctx, w.f.wsID, foreign)
	require.ErrorIs(t, err, domain.ErrAccessDenied, "the workspace must be the caller's own tenant")

	_, err = w.f.svc.ChatState(ctx, w.f.wsID, domain.Caller{TenantID: w.f.wsID})
	require.ErrorIs(t, err, domain.ErrInvalidInput)
}

func TestChatState_WithoutReadOnTheWorkspaceStillListsConversationsButNoSections(t *testing.T) {
	k := newTalk(t, "no-ws-read") // no read on the workspace attribute
	me := domain.Caller{UserID: k.f.userID, NodeID: userNode, TenantID: k.f.wsID}

	got, err := k.f.svc.ChatState(context.Background(), k.f.wsID, me)
	require.NoError(t, err)
	assert.Contains(t, channelIDs(got.Channels), k.chID)
	assert.Empty(t, got.Sections)
}

// memVersions is an in-memory Versions.
type memVersions struct {
	mu   sync.Mutex
	vals map[string]int64
}

func (m *memVersions) Get(_ context.Context, keys ...string) ([]int64, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make([]int64, len(keys))
	for i, k := range keys {
		out[i] = m.vals[k]
	}
	return out, nil
}

func (m *memVersions) Bump(_ context.Context, keys ...string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.vals == nil {
		m.vals = map[string]int64{}
	}
	for _, k := range keys {
		m.vals[k]++
	}
	return nil
}

func TestChatState_AnnouncedChangeMovesTheTenantsVersionOnly(t *testing.T) {
	f := newFixture(t)
	channelsOA := f.withChannelsOA()
	f.read.allow[grant{userNode, channelsOA, ngac.OpCreateChannel}] = true
	v := &memVersions{}
	f.svc.SetVersions(v)
	f.svc.SetEmitter(&realtime.Recorder{})

	ours := domain.ChatVersionKeys(f.wsID, f.userID)
	theirs := domain.ChatVersionKeys("another-tenant", f.userID)
	before, _ := v.Get(context.Background(), ours...)
	otherBefore, _ := v.Get(context.Background(), theirs...)

	_, err := f.svc.CreateChannel(f.callerCtx(), f.createInput(fmt.Sprintf("etag-%d", time.Now().UnixNano())))
	require.NoError(t, err)

	after, _ := v.Get(context.Background(), ours...)
	assert.Greater(t, after[0], before[0], "an announced channel change moves the tenant's version")
	otherAfter, _ := v.Get(context.Background(), theirs...)
	assert.Equal(t, otherBefore, otherAfter, "another tenant's version does not move")
}

func TestChatState_RefusedChangeMovesNothing(t *testing.T) {
	f := newFixture(t)
	f.withChannelsOA() // no create_channel grant
	v := &memVersions{}
	f.svc.SetVersions(v)

	_, err := f.svc.CreateChannel(f.callerCtx(), f.createInput(fmt.Sprintf("etag-deny-%d", time.Now().UnixNano())))
	require.ErrorIs(t, err, domain.ErrAccessDenied)

	got, _ := v.Get(context.Background(), domain.ChatVersionKeys(f.wsID, f.userID)...)
	assert.Equal(t, []int64{0, 0}, got)
}

func TestBumpChat_NamesTheUsersWhoseStateChanged(t *testing.T) {
	f := newFixture(t)
	v := &memVersions{}
	f.svc.SetVersions(v)

	f.svc.BumpChat("t1", "alice")
	alice, _ := v.Get(context.Background(), domain.ChatVersionKeys("t1", "alice")...)
	bob, _ := v.Get(context.Background(), domain.ChatVersionKeys("t1", "bob")...)
	assert.Equal(t, []int64{1, 1}, alice)
	assert.Equal(t, []int64{1, 0}, bob, "bob shares the tenant's version, not alice's own")

	f.svc.SetVersions(nil)
	f.svc.BumpChat("t1", "alice") // no Redis: a no-op, not a panic
}
