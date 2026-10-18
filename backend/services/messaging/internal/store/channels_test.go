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

func TestInsertChannelWithMembers_WritesTheRowAndItsMembersTogether(t *testing.T) {
	pool := testutil.SetupTestDB(t)
	st := store.NewStore(pool)
	ctx := context.Background()
	user, _ := testutil.CreateUser(t, pool)
	oa, ua := "oa-"+uuid.NewString(), "ua-"+uuid.NewString()
	for _, n := range [][3]string{{oa, "OA"}, {ua, "UA"}} {
		_, err := pool.Exec(ctx, `INSERT INTO ngac_nodes (id, name, node_type) VALUES ($1, $1, $2)`, n[0], n[1])
		require.NoError(t, err)
	}
	ws, _ := testutil.CreateWorkspace(t, pool, user)
	ch := &store.Channel{ID: "ch-" + uuid.NewString(), Name: "dm", ChannelType: "dm", WorkspaceID: ws, NGACOaID: oa, NGACUaID: ua, CreatedBy: user}
	t.Cleanup(func() {
		pool.Exec(ctx, `DELETE FROM channels WHERE id = $1`, ch.ID)
		pool.Exec(ctx, `DELETE FROM ngac_nodes WHERE id IN ($1, $2)`, oa, ua)
	})

	require.NoError(t, st.InsertChannelWithMembers(ctx, ch, "node-a", "node-b"))

	var members int
	require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM channel_members WHERE channel_id = $1`, ch.ID).Scan(&members))
	assert.Equal(t, 2, members)

	// The same channel again fails on the row, and adds no member of a second attempt.
	err := st.InsertChannelWithMembers(ctx, ch, "node-c")
	require.Error(t, err)
	require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM channel_members WHERE channel_id = $1`, ch.ID).Scan(&members))
	assert.Equal(t, 2, members, "a failed insert leaves no member behind")
}

func TestInsertMessage_AReplyIsCountedAndSubscribesItsSender(t *testing.T) {
	pool := testutil.SetupTestDB(t)
	st := store.NewStore(pool)
	ctx := context.Background()
	user, _ := testutil.CreateUser(t, pool)
	oa, ua := "oa-"+uuid.NewString(), "ua-"+uuid.NewString()
	for _, n := range [][3]string{{oa, "OA"}, {ua, "UA"}} {
		_, err := pool.Exec(ctx, `INSERT INTO ngac_nodes (id, name, node_type) VALUES ($1, $1, $2)`, n[0], n[1])
		require.NoError(t, err)
	}
	ws, _ := testutil.CreateWorkspace(t, pool, user)
	ch := &store.Channel{ID: "ch-" + uuid.NewString(), Name: "general", ChannelType: "workspace", WorkspaceID: ws, NGACOaID: oa, NGACUaID: ua, CreatedBy: user}
	require.NoError(t, st.InsertChannelWithMembers(ctx, ch, "n"))
	t.Cleanup(func() {
		pool.Exec(ctx, `DELETE FROM channels WHERE id = $1`, ch.ID)
		pool.Exec(ctx, `DELETE FROM ngac_nodes WHERE id IN ($1, $2)`, oa, ua)
	})

	parent := &store.Message{ID: "m-" + uuid.NewString(), ChannelID: ch.ID, SenderID: user, Content: "top", MessageType: "user", CreatedAt: time.Now()}
	require.NoError(t, st.InsertMessage(ctx, parent))
	reply := &store.Message{ID: "m-" + uuid.NewString(), ChannelID: ch.ID, SenderID: user, Content: "re", MessageType: "user", ParentMessageID: parent.ID, CreatedAt: time.Now()}
	require.NoError(t, st.InsertMessage(ctx, reply))

	var replies, participants int
	require.NoError(t, pool.QueryRow(ctx, `SELECT reply_count FROM messages WHERE id = $1`, parent.ID).Scan(&replies))
	require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM thread_participants WHERE message_id = $1`, parent.ID).Scan(&participants))
	assert.Equal(t, 1, replies)
	assert.Equal(t, 1, participants)

	// A reply to a message that does not exist stores nothing at all.
	orphan := &store.Message{ID: "m-" + uuid.NewString(), ChannelID: ch.ID, SenderID: user, Content: "lost", MessageType: "user", ParentMessageID: "no-such-message", CreatedAt: time.Now()}
	require.Error(t, st.InsertMessage(ctx, orphan))
	var stored int
	require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM messages WHERE id = $1`, orphan.ID).Scan(&stored))
	assert.Equal(t, 0, stored)
}

// channelFixture inserts a channel (and its nodes) and returns it with the
// user who created it.
func channelFixture(t *testing.T) (*store.Store, *store.Channel, string) {
	t.Helper()
	pool := testutil.SetupTestDB(t)
	st := store.NewStore(pool)
	ctx := context.Background()
	user, _ := testutil.CreateUser(t, pool)
	oa, ua := "oa-"+uuid.NewString(), "ua-"+uuid.NewString()
	for _, n := range [][3]string{{oa, "OA"}, {ua, "UA"}} {
		_, err := pool.Exec(ctx, `INSERT INTO ngac_nodes (id, name, node_type) VALUES ($1, $1, $2)`, n[0], n[1])
		require.NoError(t, err)
	}
	ws, _ := testutil.CreateWorkspace(t, pool, user)
	ch := &store.Channel{ID: "ch-" + uuid.NewString(), Name: "general", ChannelType: "workspace", WorkspaceID: ws, NGACOaID: oa, NGACUaID: ua, CreatedBy: user}
	require.NoError(t, st.InsertChannelWithMembers(ctx, ch, "n"))
	t.Cleanup(func() {
		pool.Exec(ctx, `DELETE FROM channels WHERE id = $1`, ch.ID)
		pool.Exec(ctx, `DELETE FROM ngac_nodes WHERE id IN ($1, $2)`, oa, ua)
	})
	return st, ch, user
}

func count(t *testing.T, q string, args ...any) int {
	t.Helper()
	var n int
	require.NoError(t, testutil.SetupTestDB(t).QueryRow(context.Background(), q, args...).Scan(&n))
	return n
}

func systemMessage(ch *store.Channel, user, entityType, entityID string) *store.Message {
	return &store.Message{
		ID: "m-" + uuid.NewString(), ChannelID: ch.ID, SenderID: user, Content: "announcement",
		MessageType: "system", LinkedEntityType: entityType, LinkedEntityID: entityID, CreatedAt: time.Now(),
	}
}

func TestInsertPollWithMessage_StoresTheAnnouncementThePollAndItsOptions(t *testing.T) {
	st, ch, user := channelFixture(t)
	pollID := "p-" + uuid.NewString()
	msg := systemMessage(ch, user, "poll", pollID)
	poll := &store.Poll{ID: pollID, MessageID: msg.ID, ChannelID: ch.ID, Question: "Lunch?", CreatedBy: user, CreatedAt: time.Now()}
	opts := []store.PollOptionInput{{ID: "o-" + uuid.NewString(), Text: "yes"}, {ID: "o-" + uuid.NewString(), Text: "no"}}

	require.NoError(t, st.InsertPollWithMessage(context.Background(), msg, poll, opts))

	assert.Equal(t, 1, count(t, `SELECT count(*) FROM messages WHERE id = $1`, msg.ID))
	assert.Equal(t, 1, count(t, `SELECT count(*) FROM polls WHERE id = $1`, pollID))
	assert.Equal(t, 2, count(t, `SELECT count(*) FROM poll_options WHERE poll_id = $1`, pollID))
}

// A poll that cannot be stored takes its announcement with it: no message is
// left behind for a poll that does not exist.
func TestInsertPollWithMessage_AFailureLeavesNoAnnouncement(t *testing.T) {
	st, ch, user := channelFixture(t)
	pollID := "p-" + uuid.NewString()
	msg := systemMessage(ch, user, "poll", pollID)
	// The poll names a channel that does not exist, so its insert fails after the message went in.
	poll := &store.Poll{ID: pollID, MessageID: msg.ID, ChannelID: "no-such-channel", Question: "Lunch?", CreatedBy: user, CreatedAt: time.Now()}

	err := st.InsertPollWithMessage(context.Background(), msg, poll, []store.PollOptionInput{{ID: "o-" + uuid.NewString(), Text: "yes"}})

	require.Error(t, err)
	assert.Equal(t, 0, count(t, `SELECT count(*) FROM messages WHERE id = $1`, msg.ID), "the announcement was rolled back")
	assert.Equal(t, 0, count(t, `SELECT count(*) FROM polls WHERE id = $1`, pollID))
}

// An option that cannot be stored takes the poll and its announcement with it.
func TestInsertPollWithMessage_AFailingOptionRollsBackThePoll(t *testing.T) {
	st, ch, user := channelFixture(t)
	pollID := "p-" + uuid.NewString()
	msg := systemMessage(ch, user, "poll", pollID)
	poll := &store.Poll{ID: pollID, MessageID: msg.ID, ChannelID: ch.ID, Question: "Lunch?", CreatedBy: user, CreatedAt: time.Now()}
	same := "o-" + uuid.NewString()

	err := st.InsertPollWithMessage(context.Background(), msg, poll, []store.PollOptionInput{{ID: same, Text: "yes"}, {ID: same, Text: "no"}})

	require.Error(t, err, "the second option reuses the first one's primary key")
	assert.Equal(t, 0, count(t, `SELECT count(*) FROM polls WHERE id = $1`, pollID))
	assert.Equal(t, 0, count(t, `SELECT count(*) FROM messages WHERE id = $1`, msg.ID))
	assert.Equal(t, 0, count(t, `SELECT count(*) FROM poll_options WHERE poll_id = $1`, pollID))
}

func TestInsertTaskWithMessage_StoresBothOrNeither(t *testing.T) {
	st, ch, user := channelFixture(t)
	ctx := context.Background()

	taskID := "t-" + uuid.NewString()
	msg := systemMessage(ch, user, "task", taskID)
	task := &store.ChatTask{ID: taskID, MessageID: msg.ID, ChannelID: ch.ID, Title: "Ship it", Status: "todo", CreatedBy: user}
	require.NoError(t, st.InsertTaskWithMessage(ctx, msg, task))
	assert.Equal(t, 1, count(t, `SELECT count(*) FROM messages WHERE id = $1`, msg.ID))
	assert.Equal(t, 1, count(t, `SELECT count(*) FROM chat_tasks WHERE id = $1`, taskID))

	badID := "t-" + uuid.NewString()
	badMsg := systemMessage(ch, user, "task", badID)
	bad := &store.ChatTask{ID: badID, MessageID: badMsg.ID, ChannelID: "no-such-channel", Title: "Lost", Status: "todo", CreatedBy: user}
	require.Error(t, st.InsertTaskWithMessage(ctx, badMsg, bad))
	assert.Equal(t, 0, count(t, `SELECT count(*) FROM messages WHERE id = $1`, badMsg.ID), "no announcement for a task that was not stored")
}

// dmFixture makes DM and non-DM channels for the store tests and removes them.
type dmFixture struct {
	t    *testing.T
	pool *pgxpool.Pool
	st   *store.Store
	user string
}

func newDMFixture(t *testing.T) *dmFixture {
	t.Helper()
	pool := testutil.SetupTestDB(t)
	user, _ := testutil.CreateUser(t, pool)
	return &dmFixture{t: t, pool: pool, st: store.NewStore(pool), user: user}
}

func (d *dmFixture) workspace() string {
	id, _ := testutil.CreateWorkspace(d.t, d.pool, d.user)
	return id
}

func (d *dmFixture) channel(kind, workspaceID string, members ...string) *store.Channel {
	d.t.Helper()
	ctx := context.Background()
	oa, ua := "oa-"+uuid.NewString(), "ua-"+uuid.NewString()
	for _, n := range [][2]string{{oa, "OA"}, {ua, "UA"}} {
		_, err := d.pool.Exec(ctx, `INSERT INTO ngac_nodes (id, name, node_type) VALUES ($1, $1, $2)`, n[0], n[1])
		require.NoError(d.t, err)
	}
	ch := &store.Channel{ID: "ch-" + uuid.NewString(), Name: kind, ChannelType: kind, WorkspaceID: workspaceID, NGACOaID: oa, NGACUaID: ua, CreatedBy: d.user}
	require.NoError(d.t, d.st.InsertChannelWithMembers(ctx, ch, members...))
	d.t.Cleanup(func() {
		d.pool.Exec(ctx, `DELETE FROM channels WHERE id = $1`, ch.ID)
		d.pool.Exec(ctx, `DELETE FROM ngac_nodes WHERE id IN ($1, $2)`, oa, ua)
	})
	return ch
}

func channelIDs(chs []*store.Channel) []string {
	out := make([]string, 0, len(chs))
	for _, c := range chs {
		out = append(out, c.ID)
	}
	return out
}

func TestListDMsOf_ListsOnlyTheDMsTheNodeIsInWithinTheWorkspace(t *testing.T) {
	d := newDMFixture(t)
	ctx := context.Background()
	wsA, wsB := d.workspace(), d.workspace()

	me := "node-" + uuid.NewString()
	other := "node-" + uuid.NewString()
	inA := d.channel("dm", wsA, me, other)
	inB := d.channel("dm", wsB, me, other)
	d.channel("dm", wsA, "node-"+uuid.NewString(), "node-"+uuid.NewString())
	d.channel("workspace", wsA, me)

	got, err := d.st.ListDMsOf(ctx, wsA, me)
	require.NoError(t, err)
	assert.Equal(t, []string{inA.ID}, channelIDs(got), "another pair's DM, a non-DM channel and the same pair's DM of another workspace are not candidates")

	got, err = d.st.ListDMsOf(ctx, wsB, me)
	require.NoError(t, err)
	assert.Equal(t, []string{inB.ID}, channelIDs(got))

	none, err := d.st.ListDMsOf(ctx, wsA, "node-"+uuid.NewString())
	require.NoError(t, err)
	assert.Empty(t, none)
}

// The schema requires a workspace for every channel: the database refuses one
// without it (or with a workspace that does not exist), whatever its type, and
// leaves no row and no member behind.
func TestInsertChannelWithMembers_TheDatabaseRefusesAChannelWithoutAWorkspace(t *testing.T) {
	d := newDMFixture(t)
	ctx := context.Background()
	for _, kind := range []string{"dm", "private", "workspace"} {
		for name, ws := range map[string]string{"no workspace": "", "unknown workspace": "ws-" + uuid.NewString()} {
			ch := &store.Channel{ID: "ch-" + uuid.NewString(), Name: kind, ChannelType: kind, WorkspaceID: ws, CreatedBy: d.user}
			require.Error(t, d.st.InsertChannelWithMembers(ctx, ch, "node-"+uuid.NewString()), "%s, %s", kind, name)
			var rows, members int
			require.NoError(t, d.pool.QueryRow(ctx, `SELECT count(*) FROM channels WHERE id = $1`, ch.ID).Scan(&rows))
			require.NoError(t, d.pool.QueryRow(ctx, `SELECT count(*) FROM channel_members WHERE channel_id = $1`, ch.ID).Scan(&members))
			assert.Zero(t, rows+members, "%s, %s: nothing is left behind", kind, name)
		}
	}
	var nullable string
	require.NoError(t, d.pool.QueryRow(ctx, `SELECT is_nullable FROM information_schema.columns WHERE table_name = 'channels' AND column_name = 'workspace_id'`).Scan(&nullable))
	assert.Equal(t, "NO", nullable)

	// A raw insert is refused the same way: the rule is the table's, not the store's.
	_, err := d.pool.Exec(ctx, `INSERT INTO channels (id, name, channel_type, workspace_id) VALUES ($1, 'dm', 'dm', NULL)`, "ch-"+uuid.NewString())
	require.Error(t, err)
}

func TestFindDMByMembers_FindsThePairsDMInThatWorkspaceOnly(t *testing.T) {
	d := newDMFixture(t)
	ctx := context.Background()
	wsA, wsB, wsC := d.workspace(), d.workspace(), d.workspace()
	a, b, c := "node-"+uuid.NewString(), "node-"+uuid.NewString(), "node-"+uuid.NewString()
	inA := d.channel("dm", wsA, a, b)
	inB := d.channel("dm", wsB, a, b)
	d.channel("dm", wsA, a, c)

	got, err := d.st.FindDMByMembers(ctx, wsA, a, b)
	require.NoError(t, err)
	require.NotNil(t, got)
	assert.Equal(t, inA.ID, got.ID)
	assert.Equal(t, wsA, got.WorkspaceID)

	got, err = d.st.FindDMByMembers(ctx, wsB, b, a)
	require.NoError(t, err)
	require.NotNil(t, got)
	assert.Equal(t, inB.ID, got.ID, "either order finds it")

	got, err = d.st.FindDMByMembers(ctx, wsC, a, b)
	require.NoError(t, err)
	assert.Nil(t, got, "the pair has no DM in a workspace they have not used")

	got, err = d.st.FindDMByMembers(ctx, wsA, a, c)
	require.NoError(t, err)
	require.NotNil(t, got)
	assert.Equal(t, wsA, got.WorkspaceID)

	got, err = d.st.FindDMByMembers(ctx, "", a, c)
	require.NoError(t, err)
	assert.Nil(t, got, "an empty workspace id names no workspace")
}
