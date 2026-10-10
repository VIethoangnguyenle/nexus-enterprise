package store_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
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
	ch := &store.Channel{ID: "ch-" + uuid.NewString(), Name: "dm", ChannelType: "dm", NGACOaID: oa, NGACUaID: ua, CreatedBy: user}
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
	ch := &store.Channel{ID: "ch-" + uuid.NewString(), Name: "general", ChannelType: "workspace", NGACOaID: oa, NGACUaID: ua, CreatedBy: user}
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
	ch := &store.Channel{ID: "ch-" + uuid.NewString(), Name: "general", ChannelType: "workspace", NGACOaID: oa, NGACUaID: ua, CreatedBy: user}
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
