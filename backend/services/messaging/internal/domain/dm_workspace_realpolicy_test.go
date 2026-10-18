//go:build realpolicy

package domain_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"ngac-platform/ngac"
	"ngac-platform/pkg/grpcauth"
	"ngac-platform/pkg/policyclient"
	"ngac-platform/pkg/realtime"
	policypb "ngac-platform/proto/policy"
	"ngac-platform/services/messaging/internal/domain"
	"ngac-platform/testutil"
)

// A direct message belongs to the workspace it was started in. These run the
// service against the real policy: the same two people have one DM for each
// workspace they share, and a DM is started only inside a workspace both are in.

func (w *dmWorld) in(p rpPerson, workspaceID string) context.Context {
	return grpcauth.WithCaller(context.Background(), grpcauth.Caller{UserID: p.id, NGACNodeID: p.node, TenantID: workspaceID})
}

// secondWorkspace puts the given people into another workspace's Members UA.
func (w *dmWorld) secondWorkspace(people ...rpPerson) testutil.WorkspaceGraph {
	w.t.Helper()
	g2 := testutil.SeedWorkspaceGraph(w.t, w.write)
	for _, p := range people {
		testutil.Assign(w.t, w.write, p.node, g2.Members)
	}
	_, err := w.pool.Exec(context.Background(),
		`INSERT INTO workspaces (id, name, owner_id, ngac_pc_id, type) VALUES ($1, 'rp2', $2, $3, 'organization')`, g2.WorkspaceID, w.owner.id, g2.PC)
	require.NoError(w.t, err)
	w.t.Cleanup(func() {
		w.pool.Exec(context.Background(), `DELETE FROM channels WHERE workspace_id = $1`, g2.WorkspaceID)
		w.pool.Exec(context.Background(), `DELETE FROM workspaces WHERE id = $1`, g2.WorkspaceID)
		for _, n := range []string{g2.Members, g2.Owners, g2.PC} {
			w.write.DeleteNode(context.Background(), &policypb.DeleteNodeRequest{NodeId: n})
		}
	})
	return g2
}

func (w *dmWorld) openDMIn(workspaceID string, a, b rpPerson) (string, error) {
	w.t.Helper()
	ch, err := w.svc.FindOrCreateDM(w.in(a, workspaceID), a.id, a.node, b.id, b.node)
	if err != nil {
		return "", err
	}
	w.cleanChannel(ch)
	assert.Equal(w.t, workspaceID, ch.WorkspaceId, "the DM is stamped with the workspace it was started in")
	return ch.Id, nil
}

func (w *dmWorld) storedWorkspace(chID string) string {
	w.t.Helper()
	var ws *string
	require.NoError(w.t, w.pool.QueryRow(context.Background(), `SELECT workspace_id FROM channels WHERE id = $1`, chID).Scan(&ws))
	if ws == nil {
		return ""
	}
	return *ws
}

func TestRealPolicy_TheSamePairHasOneDMPerSharedWorkspace(t *testing.T) {
	w := newDMWorld(t)
	g2 := w.secondWorkspace(w.creator, w.member)

	inA, err := w.openDMIn(w.g.WorkspaceID, w.creator, w.member)
	require.NoError(t, err)
	inB, err := w.openDMIn(g2.WorkspaceID, w.creator, w.member)
	require.NoError(t, err)
	assert.NotEqual(t, inA, inB, "the DM started in one workspace is not reused in the other")
	assert.Equal(t, w.g.WorkspaceID, w.storedWorkspace(inA))
	assert.Equal(t, g2.WorkspaceID, w.storedWorkspace(inB))

	again, err := w.openDMIn(w.g.WorkspaceID, w.member, w.creator)
	require.NoError(t, err)
	assert.Equal(t, inA, again, "either person finds the workspace's DM again")
	again, err = w.openDMIn(g2.WorkspaceID, w.creator, w.member)
	require.NoError(t, err)
	assert.Equal(t, inB, again)

	// Each workspace lists its own and nothing of the other's.
	listA, err := w.svc.ListDMs(w.in(w.creator, w.g.WorkspaceID), w.g.WorkspaceID, w.creator.node)
	require.NoError(t, err)
	assert.Equal(t, []string{inA}, channelIDsOf(listA))
	listB, err := w.svc.ListDMs(w.in(w.creator, g2.WorkspaceID), g2.WorkspaceID, w.creator.node)
	require.NoError(t, err)
	assert.Equal(t, []string{inB}, channelIDsOf(listB))

	// And a DM is read from its own workspace only.
	_, err = w.svc.GetMessages(w.in(w.creator, w.g.WorkspaceID), inA, w.creator.id, w.creator.node, "", 20)
	require.NoError(t, err, "allow: signed in to its workspace")
	_, err = w.svc.GetMessages(w.in(w.creator, g2.WorkspaceID), inA, w.creator.id, w.creator.node, "", 20)
	assert.ErrorIs(t, err, domain.ErrNotFound, "deny: signed in to the other workspace")
}

func TestRealPolicy_ADMNeedsBothPeopleInTheWorkspaceItIsStartedIn(t *testing.T) {
	w := newDMWorld(t)
	// creator is in both workspaces; member only in the first; the second has
	// someone else, so the two people share a workspace (the first) but not it.
	g2 := w.secondWorkspace(w.creator)

	_, err := w.openDMIn(w.g.WorkspaceID, w.creator, w.member)
	require.NoError(t, err, "allow: both are in the first")

	_, err = w.openDMIn(g2.WorkspaceID, w.creator, w.member)
	assert.ErrorIs(t, err, domain.ErrAccessDenied, "they share a workspace, but not this one")
	assert.Equal(t, 1, w.dmsOf(w.member), "nothing was made for the second workspace")

	// A caller cannot start a DM in a workspace they are not in themselves.
	_, err = w.openDMIn(g2.WorkspaceID, w.member, w.creator)
	assert.ErrorIs(t, err, domain.ErrAccessDenied, "the caller is not in the workspace named by the session")

	// A session with no workspace starts nothing.
	_, err = w.svc.FindOrCreateDM(w.in(w.creator, ""), w.creator.id, w.creator.node, w.member.id, w.member.node)
	assert.ErrorIs(t, err, domain.ErrAccessDenied, "no workspace on the caller")
	assert.Equal(t, 1, w.dmsOf(w.member))
}

// The pair's existing DM is found only through the workspace both are in now:
// once the other person is gone from it, asking for the DM answers nothing.
func TestRealPolicy_ADMInOneWorkspaceIsNotReachedThroughAnother(t *testing.T) {
	w := newDMWorld(t)
	g2 := w.secondWorkspace(w.creator, w.member)
	inA, err := w.openDMIn(w.g.WorkspaceID, w.creator, w.member)
	require.NoError(t, err)

	w.leaveWorkspace(w.member) // out of the first workspace, still in the second
	_, err = w.openDMIn(w.g.WorkspaceID, w.creator, w.member)
	assert.ErrorIs(t, err, domain.ErrAccessDenied, "the first workspace no longer holds them both")

	inB, err := w.openDMIn(g2.WorkspaceID, w.creator, w.member)
	require.NoError(t, err, "the second still does")
	assert.NotEqual(t, inA, inB, "and gets its own DM, not the first workspace's")

	// Writing to the first workspace's DM needs the two to be in that workspace.
	_, err = w.send(w.creator, inA, "still there?")
	assert.ErrorIs(t, err, domain.ErrAccessDenied, "a DM is written to while both are in its workspace")
}

// --- The regression matrix -------------------------------------------------
//
// A person in two workspaces (A, B) with a colleague who is in both and one who is
// in A only: the DMs they started in A are not seen, read, written to, searched,
// counted or announced from B.

type dmMatrix struct {
	*dmWorld
	wsA, wsB      string
	both, onlyA   rpPerson // the colleagues; creator is the person in both workspaces
	dmBoth, dmOnA string   // creator's DMs in A, with each colleague
}

func newDMMatrix(t *testing.T) *dmMatrix {
	t.Helper()
	w := newDMWorld(t)
	g2 := w.secondWorkspace(w.creator, w.member) // member is in both; latecomer in A only
	m := &dmMatrix{dmWorld: w, wsA: w.g.WorkspaceID, wsB: g2.WorkspaceID, both: w.member, onlyA: w.latecomer}
	m.dmBoth = w.mustDM(w.creator, w.member).Id
	m.dmOnA = w.mustDM(w.creator, w.latecomer).Id
	return m
}

func (m *dmMatrix) inA(p rpPerson) context.Context { return m.in(p, m.wsA) }
func (m *dmMatrix) inB(p rpPerson) context.Context { return m.in(p, m.wsB) }

func TestRealPolicy_ADMStartedInOneWorkspaceCannotBeStartedOrReusedFromTheOther(t *testing.T) {
	m := newDMMatrix(t)

	// Same pair, two workspaces, two DMs.
	inB, err := m.openDMIn(m.wsB, m.creator, m.both)
	require.NoError(t, err)
	assert.NotEqual(t, m.dmBoth, inB)
	again, err := m.openDMIn(m.wsA, m.creator, m.both)
	require.NoError(t, err)
	assert.Equal(t, m.dmBoth, again, "A still finds its own")

	// A colleague who is only in A is not reachable from B.
	_, err = m.openDMIn(m.wsB, m.creator, m.onlyA)
	assert.ErrorIs(t, err, domain.ErrAccessDenied)
	_, err = m.openDMIn(m.wsB, m.onlyA, m.creator)
	assert.ErrorIs(t, err, domain.ErrAccessDenied, "and cannot start one from B either")
	assert.Equal(t, 1, m.dmsOf(m.onlyA), "only the DM made in A exists")
}

func TestRealPolicy_ADMOfAIsNotFoundWhileSignedInToB(t *testing.T) {
	m := newDMMatrix(t)
	root := m.mustSend(m.creator, m.dmBoth, "gốc")
	reply, err := m.svc.SendMessage(m.inA(m.both), domain.SendMessageInput{
		ChannelID: m.dmBoth, SenderID: m.both.id, SenderNodeID: m.both.node, Content: "trả lời", ParentMessageID: root.Id,
	})
	require.NoError(t, err, "allow: the same thread, from A")

	for _, who := range []rpPerson{m.creator, m.both} {
		doors := map[string]func(ctx context.Context) error{
			"read messages": func(ctx context.Context) error {
				_, err := m.svc.GetMessages(ctx, m.dmBoth, who.id, who.node, "", 20)
				return err
			},
			"open the channel": func(ctx context.Context) error { _, err := m.svc.GetChannel(ctx, m.dmBoth, who.node); return err },
			"send": func(ctx context.Context) error {
				_, err := m.svc.SendMessage(ctx, domain.SendMessageInput{ChannelID: m.dmBoth, SenderID: who.id, SenderNodeID: who.node, Content: "x"})
				return err
			},
			"reply in a thread": func(ctx context.Context) error {
				_, err := m.svc.SendMessage(ctx, domain.SendMessageInput{ChannelID: m.dmBoth, SenderID: who.id, SenderNodeID: who.node, Content: "x", ParentMessageID: root.Id})
				return err
			},
			"read a thread": func(ctx context.Context) error {
				_, err := m.svc.GetThread(ctx, root.Id, who.id, who.node)
				return err
			},
			"react":     func(ctx context.Context) error { return m.svc.AddReaction(ctx, reply.Id, who.node, who.id, "👍") },
			"pin":       func(ctx context.Context) error { return m.svc.PinMessage(ctx, m.dmBoth, root.Id, who.node, who.id) },
			"list pins": func(ctx context.Context) error { _, err := m.svc.ListPins(ctx, m.dmBoth, who.node); return err },
			"star":      func(ctx context.Context) error { return m.svc.StarMessage(ctx, root.Id, who.id, who.node) },
			"star chat": func(ctx context.Context) error { return m.svc.StarConversation(ctx, m.dmBoth, who.id, who.node) },
			"mark read": func(ctx context.Context) error {
				return m.svc.MarkChannelRead(ctx, who.id, who.node, m.dmBoth, root.Id)
			},
			"members": func(ctx context.Context) error { _, err := m.svc.ListMembers(ctx, m.dmBoth, who.node); return err },
			"live join": func(ctx context.Context) error {
				return m.svc.AuthorizeChannelAccess(ctx, m.dmBoth, who.node, ngac.OpRead)
			},
			"start call": func(ctx context.Context) error {
				_, _, err := m.svc.StartCall(ctx, domain.StartCallInput{ChannelID: m.dmBoth, UserID: who.id, UserNodeID: who.node, Kind: "audio"})
				return err
			},
			"search in the chat": func(ctx context.Context) error {
				_, err := m.svc.SearchMessages(ctx, m.dmBoth, who.node, "gốc", 10)
				return err
			},
		}
		for name, door := range doors {
			assert.ErrorIs(t, door(m.in(who, m.wsB)), domain.ErrNotFound, name+" from B")
		}
	}
	assert.True(t, m.readable(m.creator, m.dmBoth), "control: still readable from A")
}

func TestRealPolicy_NoListOfBShowsADMOfA(t *testing.T) {
	m := newDMMatrix(t)
	svc := m.svc
	root := m.mustSend(m.creator, m.dmBoth, "chào needle")
	_, err := svc.SendMessage(m.inA(m.both), domain.SendMessageInput{
		ChannelID: m.dmBoth, SenderID: m.both.id, SenderNodeID: m.both.node, Content: "needle trả lời", ParentMessageID: root.Id,
	})
	require.NoError(t, err)
	mention, err := svc.SendMessage(m.inA(m.creator), domain.SendMessageInput{
		ChannelID: m.dmBoth, SenderID: m.creator.id, SenderNodeID: m.creator.node, Content: "needle @member", Mentions: []string{m.both.id},
	})
	require.NoError(t, err)
	m.mustSend(m.creator, m.dmOnA, "needle riêng")
	require.NoError(t, svc.StarMessage(m.inA(m.both), mention.Id, m.both.id, m.both.node))
	require.NoError(t, svc.StarConversation(m.inA(m.both), m.dmBoth, m.both.id, m.both.node))

	type view struct {
		dms, unread, activity []string
		search, mentions      []string
		starred, threads      []string
		followed, chatDMs     []string
		mentionCount          int
	}
	look := func(p rpPerson, ws string) view {
		t.Helper()
		ctx := m.in(p, ws)
		var v view
		dms, err := svc.ListDMs(ctx, ws, p.node)
		require.NoError(t, err)
		v.dms = channelIDsOf(dms)
		counts, err := svc.GetUnreadCounts(ctx, p.id, p.node, ws)
		require.NoError(t, err)
		for _, c := range counts {
			v.unread = append(v.unread, c.ChannelId)
		}
		acts, err := svc.ConversationActivity(ctx, p.id, p.node, ws)
		require.NoError(t, err)
		for _, a := range acts {
			v.activity = append(v.activity, a.ChannelId)
		}
		found, err := svc.SearchAll(ctx, p.id, p.node, ws, domain.SearchQuery{Text: "needle"})
		require.NoError(t, err)
		v.search = channelIDsOf(found.Messages)
		ment, err := svc.Mentions(ctx, p.id, p.node, ws, "", "", 20)
		require.NoError(t, err)
		v.mentions = channelIDsOf(ment.Messages)
		v.mentionCount, err = svc.UnreadMentionCount(ctx, p.id, p.node, ws)
		require.NoError(t, err)
		st, err := svc.StarredMessages(ctx, p.id, p.node, ws, "", "", 20)
		require.NoError(t, err)
		v.starred = channelIDsOf(st.Messages)
		ut, err := svc.UnreadThreads(ctx, p.id, p.node, ws, 20)
		require.NoError(t, err)
		v.threads = channelIDsOf(ut.Messages)
		ft, err := svc.FollowedThreads(ctx, p.id, p.node, ws, "", "", 20)
		require.NoError(t, err)
		v.followed = channelIDsOf(ft.Messages)
		state, err := svc.ChatState(ctx, ws, domain.Caller{UserID: p.id, NodeID: p.node, TenantID: ws})
		require.NoError(t, err)
		v.chatDMs = channelIDsOf(state.DMs)
		return v
	}

	// Control: from A the person sees the DM in the lists that carry it.
	a := look(m.both, m.wsA)
	assert.Equal(t, []string{m.dmBoth}, a.dms)
	assert.Contains(t, a.search, root.Id)
	assert.Equal(t, 1, a.mentionCount)
	assert.NotEmpty(t, a.mentions)
	assert.NotEmpty(t, a.starred)
	assert.Equal(t, []string{m.dmBoth}, a.chatDMs)

	// From B: nothing of A's DMs, for either person, in any list.
	for _, who := range []rpPerson{m.creator, m.both} {
		b := look(who, m.wsB)
		assert.Empty(t, b.dms, "GET /api/dms")
		assert.Empty(t, b.unread, "unread counts")
		assert.Empty(t, b.activity, "conversation previews")
		assert.Empty(t, b.search, "search")
		assert.Empty(t, b.mentions, "mentions")
		assert.Zero(t, b.mentionCount, "the mention badge")
		assert.Empty(t, b.starred, "starred messages")
		assert.Empty(t, b.threads, "unread threads")
		assert.Empty(t, b.followed, "followed threads")
		assert.Empty(t, b.chatDMs, "the chat navigator")
	}
}

// A new DM is announced in its own workspace and a DM's two people are both
// people of it, so a name list of that workspace names them both.
func TestRealPolicy_ADMIsAnnouncedInItsWorkspaceAndItsPeopleBelongToIt(t *testing.T) {
	m := newDMMatrix(t)
	rec := &realtime.Recorder{}
	m.svc.SetEmitter(rec)

	id, err := m.openDMIn(m.wsB, m.creator, m.both)
	require.NoError(t, err)

	var created []realtime.Event
	for _, e := range rec.Events() {
		if e.Domain == realtime.DomainChannel && e.Kind == realtime.KindCreated {
			created = append(created, e)
		}
	}
	require.Len(t, created, 1)
	assert.Equal(t, []string{id}, created[0].IDs)
	assert.Equal(t, m.wsB, created[0].WorkspaceID, "announced in the workspace it was started in")
	assert.Equal(t, m.wsB, created[0].TenantID)
	assert.ElementsMatch(t, []string{m.creator.node, m.both.node}, created[0].UserNodeIDs)

	for _, p := range []rpPerson{m.creator, m.both} {
		ok, err := policyclient.IsWorkspaceMember(m.in(p, m.wsB), m.write, p.node, ngac.WorkspaceID(m.wsB))
		require.NoError(t, err)
		assert.True(t, ok, "a person of the DM is a member of its workspace, so its name list resolves them")
	}
}
