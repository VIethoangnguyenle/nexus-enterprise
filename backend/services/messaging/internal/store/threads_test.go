package store_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"ngac-platform/services/messaging/internal/store"
	"ngac-platform/testutil"
)

// threadWorld is one channel with a few users who post into it.
type threadWorld struct {
	pool  *pgxpool.Pool
	st    *store.Store
	chID  string
	users []string
	clock time.Time
}

func newThreadWorld(t *testing.T, users int) *threadWorld {
	t.Helper()
	pool := testutil.SetupTestDB(t)
	ctx := context.Background()
	w := &threadWorld{pool: pool, st: store.NewStore(pool), clock: time.Now().Add(-time.Hour).Truncate(time.Millisecond)}
	for range users {
		id, _ := testutil.CreateUser(t, pool)
		w.users = append(w.users, id)
	}
	oa, ua := "oa-"+uuid.NewString(), "ua-"+uuid.NewString()
	for _, n := range [][2]string{{oa, "OA"}, {ua, "UA"}} {
		_, err := pool.Exec(ctx, `INSERT INTO ngac_nodes (id, name, node_type) VALUES ($1, $1, $2)`, n[0], n[1])
		require.NoError(t, err)
	}
	ch := &store.Channel{ID: "ch-" + uuid.NewString(), Name: "threads", ChannelType: "workspace", NGACOaID: oa, NGACUaID: ua, CreatedBy: w.users[0]}
	require.NoError(t, w.st.InsertChannelWithMembers(ctx, ch))
	w.chID = ch.ID
	t.Cleanup(func() {
		pool.Exec(ctx, `DELETE FROM thread_follows WHERE root_message_id IN (SELECT id FROM messages WHERE channel_id = $1)`, ch.ID)
		pool.Exec(ctx, `DELETE FROM messages WHERE channel_id = $1 AND parent_message_id IS NOT NULL`, ch.ID)
		pool.Exec(ctx, `DELETE FROM channels WHERE id = $1`, ch.ID)
		pool.Exec(ctx, `DELETE FROM ngac_nodes WHERE id IN ($1, $2)`, oa, ua)
	})
	return w
}

// post stores a message one second after the previous one, so order is certain.
func (w *threadWorld) post(t *testing.T, sender, parent string, mentions ...string) *store.Message {
	t.Helper()
	w.clock = w.clock.Add(time.Second)
	m := &store.Message{ID: "m-" + uuid.NewString(), ChannelID: w.chID, SenderID: sender, Content: "text",
		MessageType: "user", ParentMessageID: parent, Mentions: mentions, CreatedAt: w.clock}
	require.NoError(t, w.st.InsertMessage(context.Background(), m))
	return m
}

func (w *threadWorld) state(t *testing.T, user, root string) store.ThreadState {
	t.Helper()
	states, err := w.st.ThreadStates(context.Background(), user, []string{root})
	require.NoError(t, err)
	return states[root]
}

// After a reply, everyone who follows the thread — with or without a follow
// row — is listed with their NGAC node and their own unread count; whoever
// unfollowed, or never followed, is not.
func TestThreadFollowers_ListsEveryFollowerWithTheirUnread(t *testing.T) {
	w := newThreadWorld(t, 5)
	starter, mentioned, replier, quitter, stranger := w.users[0], w.users[1], w.users[2], w.users[3], w.users[4]
	ctx := context.Background()
	root := w.post(t, starter, "", mentioned)
	w.post(t, quitter, root.ID)
	require.NoError(t, w.st.SetThreadFollow(ctx, quitter, root.ID, false))
	w.post(t, replier, root.ID)

	followers, err := w.st.ThreadFollowers(ctx, root.ID)
	require.NoError(t, err)
	got := map[string]store.ThreadFollower{}
	for _, f := range followers {
		got[f.UserID] = f
	}
	assert.Len(t, got, 3)
	assert.Equal(t, "u-"+starter, got[starter].NodeID)
	assert.EqualValues(t, 2, got[starter].Unread)
	assert.EqualValues(t, 2, got[mentioned].Unread)
	assert.EqualValues(t, 0, got[replier].Unread, "the replier has read up to their own reply")
	assert.NotContains(t, got, quitter)
	assert.NotContains(t, got, stranger)

	summary, err := w.st.ThreadSummary(ctx, root.ID)
	require.NoError(t, err)
	assert.Equal(t, w.chID, summary.ChannelID)
	assert.EqualValues(t, 2, summary.ReplyCount)
	require.NotNil(t, summary.LastReplyAt)
	assert.Equal(t, []string{replier, quitter}, summary.Participants)
}

func TestInsertMessage_AReplyMovesTheThreadsLastReplyTime(t *testing.T) {
	w := newThreadWorld(t, 2)
	root := w.post(t, w.users[0], "")
	reply := w.post(t, w.users[1], root.ID)

	msgs, _, err := w.st.ListMessages(context.Background(), w.chID, nil, 10)
	require.NoError(t, err)
	require.Len(t, msgs, 1)
	require.NotNil(t, msgs[0].LastReplyAt)
	assert.True(t, msgs[0].LastReplyAt.Equal(reply.CreatedAt))
	assert.EqualValues(t, 1, msgs[0].ReplyCount)
}

func TestThreadStates_SummarisesTheThreadForTheCaller(t *testing.T) {
	w := newThreadWorld(t, 5)
	a, b, c, d, e := w.users[0], w.users[1], w.users[2], w.users[3], w.users[4]
	root := w.post(t, a, "")
	w.post(t, b, root.ID)
	w.post(t, c, root.ID)
	w.post(t, b, root.ID)
	w.post(t, d, root.ID)
	w.post(t, e, root.ID)

	s := w.state(t, a, root.ID)
	assert.True(t, s.Following, "the starter follows the thread")
	assert.EqualValues(t, 5, s.Unread, "every reply by someone else is unread for the starter")
	assert.Equal(t, []string{e, d, b}, s.Participants, "the three most recent distinct repliers, newest first")

	s = w.state(t, b, root.ID)
	assert.True(t, s.Following, "a replier follows the thread")
	assert.EqualValues(t, 2, s.Unread, "only replies after b's own last reply are unread for b")
}

func TestThreadStates_UnreadCountsOnlyForAFollowedThread(t *testing.T) {
	w := newThreadWorld(t, 3)
	a, b, outsider := w.users[0], w.users[1], w.users[2]
	root := w.post(t, a, "")
	w.post(t, b, root.ID)

	s := w.state(t, outsider, root.ID)
	assert.False(t, s.Following)
	assert.EqualValues(t, 0, s.Unread, "a thread the user does not follow raises no unread")

	require.NoError(t, w.st.SetThreadFollow(context.Background(), a, root.ID, false))
	s = w.state(t, a, root.ID)
	assert.False(t, s.Following)
	assert.EqualValues(t, 0, s.Unread, "unfollowing silences the thread")
}

func TestInsertMessage_AMentionMakesTheMentionedUserFollow(t *testing.T) {
	w := newThreadWorld(t, 3)
	a, b, c := w.users[0], w.users[1], w.users[2]

	rootMention := w.post(t, a, "", b)
	assert.True(t, w.state(t, b, rootMention.ID).Following, "mentioned in the root message")

	root := w.post(t, a, "")
	w.post(t, b, root.ID, c)
	s := w.state(t, c, root.ID)
	assert.True(t, s.Following, "mentioned in a reply")
	assert.EqualValues(t, 1, s.Unread, "the reply that mentioned c is unread for c")

	// A mention of an id that is not a user is ignored, not a failed send.
	w.post(t, a, root.ID, "not-a-user")
}

func TestSetThreadFollow_ReplyingAfterUnfollowFollowsAgain(t *testing.T) {
	w := newThreadWorld(t, 2)
	a, b := w.users[0], w.users[1]
	root := w.post(t, a, "")
	w.post(t, b, root.ID)
	ctx := context.Background()

	require.NoError(t, w.st.SetThreadFollow(ctx, b, root.ID, false))
	w.post(t, a, root.ID)
	assert.False(t, w.state(t, b, root.ID).Following, "someone else's reply does not undo an unfollow")

	w.post(t, b, root.ID)
	s := w.state(t, b, root.ID)
	assert.True(t, s.Following, "replying follows the thread again")
	assert.EqualValues(t, 0, s.Unread)

	// The starter's unfollow also survives later replies by others.
	require.NoError(t, w.st.SetThreadFollow(ctx, a, root.ID, false))
	w.post(t, b, root.ID)
	assert.False(t, w.state(t, a, root.ID).Following)

	require.NoError(t, w.st.SetThreadFollow(ctx, a, root.ID, true))
	assert.True(t, w.state(t, a, root.ID).Following)
}

func TestMarkThreadRead_ClearsTheThreadsUnread(t *testing.T) {
	w := newThreadWorld(t, 2)
	a, b := w.users[0], w.users[1]
	root := w.post(t, a, "")
	w.post(t, b, root.ID)
	w.post(t, b, root.ID)
	require.EqualValues(t, 2, w.state(t, a, root.ID).Unread)

	// Replies are stored with past timestamps; reading now covers them all.
	require.NoError(t, w.st.MarkThreadRead(context.Background(), a, root.ID))
	s := w.state(t, a, root.ID)
	assert.EqualValues(t, 0, s.Unread)
	assert.True(t, s.Following, "reading a thread keeps the starter following it")

	// Reading a thread one does not follow records the position and follows nothing.
	other := w.users[1]
	root2 := w.post(t, a, "")
	w.post(t, a, root2.ID)
	require.NoError(t, w.st.MarkThreadRead(context.Background(), other, root2.ID))
	assert.False(t, w.state(t, other, root2.ID).Following)
}

func TestThreadStates_IsOneQueryForAPage(t *testing.T) {
	w := newThreadWorld(t, 2)
	a, b := w.users[0], w.users[1]
	var roots []string
	for range 3 {
		r := w.post(t, a, "")
		w.post(t, b, r.ID)
		roots = append(roots, r.ID)
	}
	noReplies := w.post(t, a, "")
	roots = append(roots, noReplies.ID)

	states, err := w.st.ThreadStates(context.Background(), a, roots)
	require.NoError(t, err)
	for _, r := range roots[:3] {
		assert.EqualValues(t, 1, states[r].Unread)
		assert.Equal(t, []string{b}, states[r].Participants)
	}
	assert.True(t, states[noReplies.ID].Following, "the sender of a message without replies follows it")
	assert.EqualValues(t, 0, states[noReplies.ID].Unread)
	assert.Empty(t, states[noReplies.ID].Participants)
}

func TestListThreads_OrdersByActivityAndFilters(t *testing.T) {
	w := newThreadWorld(t, 3)
	a, b, c := w.users[0], w.users[1], w.users[2]
	ctx := context.Background()
	quiet := w.post(t, a, "")        // no replies: not a thread
	older := w.post(t, b, "")        // a does not follow
	followed := w.post(t, a, "")     // a follows, read
	w.post(t, c, older.ID)           // older moves first
	w.post(t, c, followed.ID)        // then followed
	require.NoError(t, w.st.MarkThreadRead(ctx, a, followed.ID))
	unread := w.post(t, a, "")
	w.post(t, b, unread.ID) // newest activity, unread for a

	ids := func(filter string) []string {
		t.Helper()
		msgs, _, err := w.st.ListThreads(ctx, w.chID, a, filter, nil, "", 20)
		require.NoError(t, err)
		var out []string
		for _, m := range msgs {
			out = append(out, m.ID)
		}
		return out
	}
	assert.Equal(t, []string{unread.ID, followed.ID, older.ID}, ids("all"))
	assert.Equal(t, []string{unread.ID, followed.ID}, ids("following"))
	assert.Equal(t, []string{unread.ID}, ids("unread"))
	assert.NotContains(t, ids("all"), quiet.ID)

	page, more, err := w.st.ListThreads(ctx, w.chID, a, "all", nil, "", 2)
	require.NoError(t, err)
	assert.True(t, more)
	require.Len(t, page, 2)
	rest, more, err := w.st.ListThreads(ctx, w.chID, a, "all", page[1].LastReplyAt, page[1].ID, 2)
	require.NoError(t, err)
	assert.False(t, more)
	require.Len(t, rest, 1)
	assert.Equal(t, older.ID, rest[0].ID)
}

// Two threads that moved at the same instant straddle a page edge: the second
// page starts with the one the first page left out, not after both.
func TestListThreads_PagesThroughThreadsThatMovedTogether(t *testing.T) {
	w := newThreadWorld(t, 2)
	a, b := w.users[0], w.users[1]
	ctx := context.Background()
	var roots []string
	for range 3 {
		r := w.post(t, a, "")
		w.post(t, b, r.ID)
		roots = append(roots, r.ID)
	}
	_, err := w.pool.Exec(ctx, `UPDATE messages SET last_reply_at = $2 WHERE id = ANY($1)`, roots, w.clock)
	require.NoError(t, err)

	var seen []string
	var before *time.Time
	beforeID := ""
	for range 3 {
		page, _, err := w.st.ListThreads(ctx, w.chID, a, "all", before, beforeID, 1)
		require.NoError(t, err)
		require.Len(t, page, 1)
		seen = append(seen, page[0].ID)
		before, beforeID = page[0].LastReplyAt, page[0].ID
	}
	assert.ElementsMatch(t, roots, seen, "every thread appears exactly once")
}

func TestGetUnreadCounts_AddsUnreadRepliesOfFollowedThreads(t *testing.T) {
	w := newThreadWorld(t, 5)
	a, b, outsider, mentioned, quitter := w.users[0], w.users[1], w.users[2], w.users[3], w.users[4]
	ctx := context.Background()
	root := w.post(t, a, "", mentioned)
	w.post(t, quitter, root.ID)
	require.NoError(t, w.st.SetThreadFollow(ctx, quitter, root.ID, false))
	w.post(t, b, root.ID)
	w.post(t, b, root.ID)

	threadUnread := func(user string) int32 {
		counts, err := w.st.GetUnreadCounts(ctx, user, []string{w.chID})
		require.NoError(t, err)
		for _, c := range counts {
			if c.ChannelID == w.chID {
				return c.ThreadUnread
			}
		}
		t.Fatalf("channel missing from %s's unread counts", user)
		return 0
	}
	assert.EqualValues(t, 3, threadUnread(a), "the starter follows without a row: three replies by others")
	assert.EqualValues(t, 3, threadUnread(mentioned), "mentioned in the root message: follows without a row")
	assert.EqualValues(t, 0, threadUnread(quitter), "an unfollow silences the thread")
	assert.EqualValues(t, 0, threadUnread(outsider), "replies in a thread one does not follow are not counted")
	// The channel-level count and the per-thread summary agree.
	assert.EqualValues(t, threadUnread(a), w.state(t, a, root.ID).Unread)
}

// Unread counts cover exactly the channels asked for — the domain decides
// which the user may read — never every channel in the table.
func TestGetUnreadCounts_CountsExactlyTheChannelsAskedFor(t *testing.T) {
	w := newThreadWorld(t, 1)
	user := w.users[0]
	ctx := context.Background()
	w.post(t, user, "")

	counts, err := w.st.GetUnreadCounts(ctx, user, nil)
	require.NoError(t, err)
	assert.Empty(t, counts, "no channel asked for, none counted")

	counts, err = w.st.GetUnreadCounts(ctx, user, []string{"some-other-channel"})
	require.NoError(t, err)
	for _, c := range counts {
		assert.NotEqual(t, w.chID, c.ChannelID, "a channel not asked for is never named")
	}

	counts, err = w.st.GetUnreadCounts(ctx, user, []string{w.chID, w.chID})
	require.NoError(t, err)
	require.Len(t, counts, 1, "a channel named twice is counted once")
	assert.EqualValues(t, 1, counts[0].UnreadCount)
}
