package domain_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"ngac-platform/ngac"
	"ngac-platform/pkg/grpcauth"
	"ngac-platform/services/messaging/internal/domain"
	"ngac-platform/testutil"
)

// A direct message belongs to exactly one workspace. These tests keep the
// caller's DM in the workspace they are signed in to and hold every list and
// every door to it: the same DM is invisible from another workspace, and a DM
// with no workspace at all is visible from none.

// dmScope is a caller with a DM in each of two workspaces and one with none.
type dmScope struct {
	f          *fixture
	wsA, wsB   string // the fixture's workspace, and another
	inA, inB   string // the DM channels
	inNone     string // a DM with no workspace
	oaA, oaB   string
	oaNone     string
	peerA      string
	peerUser   string // a user who sends the messages that mention the caller
	msgA, msgB string
}

func (f *fixture) dmRow(t *testing.T, name, workspaceID string) (chID, oaID, peer string) {
	t.Helper()
	ctx := context.Background()
	chID, oaID = f.insertChannel(t, name)
	_, err := f.pool.Exec(ctx, `UPDATE channels SET channel_type = 'dm', workspace_id = NULLIF($2, '') WHERE id = $1`, chID, workspaceID)
	require.NoError(t, err)
	_, peer = testutil.CreateUser(t, f.pool)
	addDMMember(t, f, chID, userNode)
	addDMMember(t, f, chID, peer)
	f.read.allow[grant{userNode, oaID, ngac.OpRead}] = true
	f.read.allow[grant{userNode, oaID, ngac.OpWrite}] = true
	t.Cleanup(func() {
		f.pool.Exec(ctx, `DELETE FROM starred_conversations WHERE channel_id = $1`, chID)
		f.pool.Exec(ctx, `DELETE FROM messages WHERE channel_id = $1`, chID)
		f.pool.Exec(ctx, `DELETE FROM channel_members WHERE channel_id = $1`, chID)
	})
	return chID, oaID, peer
}

func (f *fixture) dmMessage(t *testing.T, chID, content string, mentioning ...string) string {
	t.Helper()
	id := "dm-msg-" + chID
	_, err := f.pool.Exec(context.Background(),
		`INSERT INTO messages (id, channel_id, sender_id, content, message_type, mentions, created_at)
		 VALUES ($1, $2, $3, $4, 'user', $5, now())`, id, chID, f.userID, content, mentioning)
	require.NoError(t, err)
	return id
}

func (f *fixture) inWorkspace(workspaceID string) context.Context {
	return grpcauth.WithCaller(context.Background(), grpcauth.Caller{UserID: f.userID, NGACNodeID: userNode, TenantID: workspaceID})
}

func newDMScope(t *testing.T) *dmScope {
	t.Helper()
	f := newFixture(t)
	wsB, _ := testutil.CreateWorkspace(t, f.pool, f.userID)
	w := &dmScope{f: f, wsA: f.wsID, wsB: wsB}
	w.peerUser, _ = testutil.CreateUser(t, f.pool) // before the DMs, so it goes after their messages
	w.inA, w.oaA, w.peerA = f.dmRow(t, "dm-a", w.wsA)
	f.read.addMember(w.wsA, userNode, w.peerA)
	w.inB, w.oaB, _ = f.dmRow(t, "dm-b", w.wsB)
	w.inNone, w.oaNone, _ = f.dmRow(t, "dm-none", "")
	w.msgA = f.dmMessage(t, w.inA, "alpha secretword", f.userID)
	w.msgB = f.dmMessage(t, w.inB, "beta secretword", f.userID)
	f.dmMessage(t, w.inNone, "orphan secretword", f.userID)
	return w
}

func channelIDsOf[T interface{ GetId() string }](list []T) []string {
	out := make([]string, 0, len(list))
	for _, c := range list {
		out = append(out, c.GetId())
	}
	return out
}

func TestListDMs_ShowsOnlyTheDMsOfTheWorkspace(t *testing.T) {
	w := newDMScope(t)
	ctx := context.Background()

	inA, err := w.f.svc.ListDMs(ctx, w.wsA, userNode)
	require.NoError(t, err)
	assert.Equal(t, []string{w.inA}, channelIDsOf(inA), "allow: the workspace's own DM")

	inB, err := w.f.svc.ListDMs(ctx, w.wsB, userNode)
	require.NoError(t, err)
	assert.Equal(t, []string{w.inB}, channelIDsOf(inB), "the same person, signed in to the other workspace")

	none, err := w.f.svc.ListDMs(ctx, "", userNode)
	require.NoError(t, err)
	assert.Empty(t, none, "no workspace, no DM — the one with no workspace is in nobody's list")

	other, err := w.f.svc.ListDMs(ctx, w.wsA, "ngac-someone-else")
	require.NoError(t, err)
	assert.Empty(t, other, "deny: a DM the caller is not in")
}

// Every list that carries DM content is built from the workspace's DMs alone.
func TestDMFeeds_AreScopedToTheWorkspace(t *testing.T) {
	w := newDMScope(t)
	ctx := context.Background()
	svc, f := w.f.svc, w.f
	require.NoError(t, svc.StarMessage(f.inWorkspace(w.wsA), w.msgA, f.userID, userNode))

	t.Run("unread counts", func(t *testing.T) {
		counts, err := svc.GetUnreadCounts(ctx, f.userID, userNode, w.wsA)
		require.NoError(t, err)
		got := map[string]bool{}
		for _, c := range counts {
			got[c.ChannelId] = true
		}
		assert.True(t, got[w.inA])
		assert.False(t, got[w.inB], "the other workspace's DM")
		assert.False(t, got[w.inNone], "a DM with no workspace")
	})

	t.Run("activity previews", func(t *testing.T) {
		list, err := svc.ConversationActivity(ctx, f.userID, userNode, w.wsA)
		require.NoError(t, err)
		var ids []string
		for _, a := range list {
			ids = append(ids, a.ChannelId)
		}
		assert.Equal(t, []string{w.inA}, ids)
	})

	t.Run("search", func(t *testing.T) {
		list, err := svc.SearchAll(ctx, f.userID, userNode, w.wsA, domain.SearchQuery{Text: "secretword"})
		require.NoError(t, err)
		assert.Equal(t, []string{w.msgA}, channelIDsOf(list.Messages))
		list, err = svc.SearchAll(ctx, f.userID, userNode, w.wsB, domain.SearchQuery{Text: "secretword"})
		require.NoError(t, err)
		assert.Equal(t, []string{w.msgB}, channelIDsOf(list.Messages))
	})

	t.Run("mentions", func(t *testing.T) {
		// Mentions of the caller are by others; make the senders someone else.
		_, err := f.pool.Exec(ctx, `UPDATE messages SET sender_id = $1 WHERE channel_id IN ($2, $3, $4)`,
			w.peerUser, w.inA, w.inB, w.inNone)
		require.NoError(t, err)
		list, err := svc.Mentions(ctx, f.userID, userNode, w.wsA, "", "", 10)
		require.NoError(t, err)
		assert.Equal(t, []string{w.msgA}, channelIDsOf(list.Messages))
		n, err := svc.UnreadMentionCount(ctx, f.userID, userNode, w.wsA)
		require.NoError(t, err)
		assert.Equal(t, 1, n)
		n, err = svc.UnreadMentionCount(ctx, f.userID, userNode, w.wsB)
		require.NoError(t, err)
		assert.Equal(t, 1, n, "the other workspace counts its own")
	})

	t.Run("starred messages", func(t *testing.T) {
		require.NoError(t, svc.StarMessage(f.inWorkspace(w.wsB), w.msgB, f.userID, userNode))
		a, err := svc.StarredMessages(ctx, f.userID, userNode, w.wsA, "", "", 10)
		require.NoError(t, err)
		assert.Equal(t, []string{w.msgA}, channelIDsOf(a.Messages))
		b, err := svc.StarredMessages(ctx, f.userID, userNode, w.wsB, "", "", 10)
		require.NoError(t, err)
		assert.Equal(t, []string{w.msgB}, channelIDsOf(b.Messages))
	})

	t.Run("chat state", func(t *testing.T) {
		state, err := svc.ChatState(ctx, w.wsA, domain.Caller{UserID: f.userID, NodeID: userNode, TenantID: w.wsA})
		require.NoError(t, err)
		assert.Equal(t, []string{w.inA}, channelIDsOf(state.DMs))
	})
}

// A DM opened by its id from another workspace is answered as a conversation
// that is not there, whichever way it is reached.
func TestDM_ByIdIsNotFoundFromAnotherWorkspace(t *testing.T) {
	w := newDMScope(t)
	svc, f := w.f.svc, w.f
	here, away := f.inWorkspace(w.wsA), f.inWorkspace(w.wsB)

	_, err := svc.GetChannel(here, w.inA, userNode)
	require.NoError(t, err, "allow: from its own workspace")
	_, err = svc.GetMessages(here, w.inA, f.userID, userNode, "", 20)
	require.NoError(t, err)

	doors := map[string]func(ctx context.Context) error{
		"GetChannel": func(ctx context.Context) error { _, err := svc.GetChannel(ctx, w.inA, userNode); return err },
		"GetMessages": func(ctx context.Context) error {
			_, err := svc.GetMessages(ctx, w.inA, f.userID, userNode, "", 20)
			return err
		},
		"SendMessage": func(ctx context.Context) error {
			_, err := svc.SendMessage(ctx, domain.SendMessageInput{ChannelID: w.inA, SenderID: f.userID, SenderNodeID: userNode, Content: "hi"})
			return err
		},
		"ListPins":     func(ctx context.Context) error { _, err := svc.ListPins(ctx, w.inA, userNode); return err },
		"ListMembers":  func(ctx context.Context) error { _, err := svc.ListMembers(ctx, w.inA, userNode); return err },
		"MemberRoster": func(ctx context.Context) error { _, err := svc.MemberRoster(ctx, w.inA, userNode); return err },
		"star":         func(ctx context.Context) error { return svc.StarConversation(ctx, w.inA, f.userID, userNode) },
		"search": func(ctx context.Context) error {
			_, err := svc.SearchMessages(ctx, w.inA, userNode, "secretword", 10)
			return err
		},
		"notification": func(ctx context.Context) error {
			_, err := svc.GetNotificationLevel(ctx, w.inA, f.userID, userNode)
			return err
		},
		"react":     func(ctx context.Context) error { return svc.AddReaction(ctx, w.msgA, userNode, f.userID, "👍") },
		"mark read": func(ctx context.Context) error { return svc.MarkChannelRead(ctx, f.userID, userNode, w.inA, w.msgA) },
		"join the live stream": func(ctx context.Context) error {
			return svc.AuthorizeChannelAccess(ctx, w.inA, userNode, ngac.OpRead)
		},
		"pin": func(ctx context.Context) error { return svc.PinMessage(ctx, w.inA, w.msgA, userNode, f.userID) },
		"reply in a thread": func(ctx context.Context) error {
			_, err := svc.SendMessage(ctx, domain.SendMessageInput{
				ChannelID: w.inA, SenderID: f.userID, SenderNodeID: userNode, Content: "re", ParentMessageID: w.msgA,
			})
			return err
		},
		"remove member": func(ctx context.Context) error { return svc.RemoveMember(ctx, w.inA, userNode, w.peerA) },
	}
	for name, door := range doors {
		assert.ErrorIs(t, door(away), domain.ErrNotFound, name+": from another workspace")
		assert.ErrorIs(t, door(context.Background()), domain.ErrNotFound, name+": with no workspace on the caller")
	}

	// A DM with no workspace is in none, its own people's included.
	for _, ws := range []string{w.wsA, w.wsB} {
		_, err = svc.GetMessages(f.inWorkspace(ws), w.inNone, f.userID, userNode, "", 20)
		assert.ErrorIs(t, err, domain.ErrNotFound, "a DM with no workspace, from "+ws)
	}

	// A group is not held to it: it is workspace-scoped by the graph already.
	group, groupOA := f.insertChannel(t, "group-not-dm")
	f.read.allow[grant{userNode, groupOA, ngac.OpRead}] = true
	_, err = svc.GetMessages(away, group, f.userID, userNode, "", 20)
	assert.NoError(t, err)
}

// A DM sits in the navigator of the workspace it belongs to and no other.
func TestSections_ADMOfAnotherWorkspaceCannotBePlaced(t *testing.T) {
	w := newDMScope(t)
	f := w.f
	docs := f.pool
	_ = docs
	ctx := f.inWorkspace(w.wsA)
	me := domain.Caller{UserID: f.userID, NodeID: userNode, TenantID: w.wsA}
	// "Being in the workspace" is read on the workspace's Documents OA.
	oa := "test-docs-oa-" + w.wsA
	_, err := f.pool.Exec(ctx, `INSERT INTO ngac_nodes (id, name, node_type) VALUES ($1, $1, $2)`, oa, ngac.TypeOA)
	require.NoError(t, err)
	_, err = f.pool.Exec(ctx, `UPDATE workspaces SET documents_oa_id = $2 WHERE id = $1`, w.wsA, oa)
	require.NoError(t, err)
	t.Cleanup(func() {
		f.pool.Exec(context.Background(), `DELETE FROM chat_sections WHERE workspace_id = $1`, w.wsA)
		f.pool.Exec(context.Background(), `UPDATE workspaces SET documents_oa_id = NULL WHERE id = $1`, w.wsA)
		f.pool.Exec(context.Background(), `DELETE FROM ngac_nodes WHERE id = $1`, oa)
	})
	f.read.allow[grant{userNode, oa, ngac.OpRead}] = true

	sec, err := f.svc.CreateSection(ctx, w.wsA, me, domain.NewSection{Name: "DM"})
	require.NoError(t, err)
	require.NoError(t, f.svc.PlaceChannel(ctx, w.wsA, me, sec.ID, w.inA), "allow: the workspace's own DM")
	assert.Error(t, f.svc.PlaceChannel(f.inWorkspace(w.wsA), w.wsA, me, sec.ID, w.inB), "deny: a DM of the other workspace")
}
