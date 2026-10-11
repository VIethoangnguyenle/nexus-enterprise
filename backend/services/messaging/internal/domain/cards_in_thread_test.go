package domain_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"ngac-platform/ngac"
	"ngac-platform/services/messaging/internal/domain"
)

// cardRows counts what a poll or task creation writes in the channel.
type cardRows struct{ messages, polls, tasks int }

func (w *cardWorld) rows(t *testing.T) cardRows {
	t.Helper()
	var r cardRows
	require.NoError(t, w.f.pool.QueryRow(context.Background(),
		`SELECT (SELECT count(*) FROM messages WHERE channel_id = $1),
		        (SELECT count(*) FROM polls WHERE channel_id = $1),
		        (SELECT count(*) FROM chat_tasks WHERE channel_id = $1)`, w.chID).
		Scan(&r.messages, &r.polls, &r.tasks))
	return r
}

func (w *cardWorld) replyCount(t *testing.T, rootID string) int {
	t.Helper()
	var n int
	require.NoError(t, w.f.pool.QueryRow(context.Background(),
		`SELECT reply_count FROM messages WHERE id = $1`, rootID).Scan(&n))
	return n
}

func (w *cardWorld) pollIn(parent string) domain.CreatePollInput {
	return domain.CreatePollInput{
		ChannelID: w.chID, UserID: w.f.userID, UserNodeID: userNode, Question: "Ăn trưa ở đâu?",
		Options: []string{"Phở", "Cơm"}, ParentMessageID: parent,
	}
}

func (w *cardWorld) taskIn(parent string) domain.CreateTaskInput {
	return domain.CreateTaskInput{
		ChannelID: w.chID, UserID: w.f.userID, UserNodeID: userNode, Title: "Đặt phòng", ParentMessageID: parent,
	}
}

// A poll or task made from a thread lands in the thread as a reply, counted and
// followed like any reply, and stays out of the main stream.
func TestCreateCard_WithParentIsAReplyInTheThread(t *testing.T) {
	w := newCardWorld(t)
	ctx := context.Background()
	svc := w.f.svc

	poll, pollMsg, err := svc.CreatePollMessage(ctx, w.pollIn(w.msgID))
	require.NoError(t, err)
	task, taskMsg, err := svc.CreateTaskMessage(ctx, w.taskIn(w.msgID))
	require.NoError(t, err)

	for _, m := range []struct {
		id, kind, linked string
	}{{poll.MessageId, "poll", poll.Id}, {task.MessageId, "task", task.Id}} {
		var parent, kind, linked string
		require.NoError(t, w.f.pool.QueryRow(ctx,
			`SELECT parent_message_id, linked_entity_type, linked_entity_id FROM messages WHERE id = $1`, m.id).
			Scan(&parent, &kind, &linked))
		assert.Equal(t, w.msgID, parent)
		assert.Equal(t, m.kind, kind)
		assert.Equal(t, m.linked, linked)
	}
	assert.Equal(t, poll.MessageId, pollMsg.Id)
	assert.Equal(t, w.msgID, pollMsg.ParentMessageId, "the announcement is the reply that goes out live")
	assert.Equal(t, task.MessageId, taskMsg.Id)
	assert.Equal(t, w.msgID, taskMsg.ParentMessageId)
	assert.Equal(t, "system", pollMsg.MessageType)

	assert.Equal(t, 2, w.replyCount(t, w.msgID))
	var participants int
	require.NoError(t, w.f.pool.QueryRow(ctx,
		`SELECT count(*) FROM thread_participants WHERE message_id = $1 AND user_id = $2`, w.msgID, w.f.userID).Scan(&participants))
	assert.Equal(t, 1, participants)

	list, err := svc.GetMessages(ctx, w.chID, w.f.userID, userNode, "", 20)
	require.NoError(t, err)
	require.Len(t, list.Messages, 1, "replies stay out of the main stream")
	root := list.Messages[0]
	assert.EqualValues(t, 2, root.ReplyCount)
	assert.True(t, root.Following, "creating the card follows the thread")
	assert.NotNil(t, root.LastReplyAt)
	assert.Equal(t, []string{w.f.userID}, root.ReplyParticipantIds)

	thread, err := svc.GetThread(ctx, w.msgID, w.f.userID, userNode)
	require.NoError(t, err)
	require.Len(t, thread.Messages, 3)
	t.Cleanup(func() {
		w.f.pool.Exec(ctx, `DELETE FROM thread_follows WHERE root_message_id = $1`, w.msgID)
	})
}

// Without a parent a card is a main-stream message exactly as before.
func TestCreateCard_WithoutParentStaysInTheMainStream(t *testing.T) {
	w := newCardWorld(t)
	ctx := context.Background()
	_, pollMsg, err := w.f.svc.CreatePollMessage(ctx, w.pollIn(""))
	require.NoError(t, err)
	assert.Empty(t, pollMsg.ParentMessageId)
	assert.Equal(t, 0, w.replyCount(t, w.msgID))
}

func TestCreateCard_ParentThatIsNotAThreadRootIsRefused(t *testing.T) {
	w := newCardWorld(t)
	ctx := context.Background()
	svc := w.f.svc

	otherCh, otherOA := w.f.insertChannel(t, "cards-other")
	w.f.read.allow[grant{userNode, otherOA, ngac.OpWrite}] = true
	other, err := svc.SendMessage(ctx, domain.SendMessageInput{
		ChannelID: otherCh, SenderID: w.f.userID, SenderNodeID: userNode, Content: "elsewhere",
	})
	require.NoError(t, err)
	t.Cleanup(func() { w.f.pool.Exec(ctx, `DELETE FROM messages WHERE channel_id = $1`, otherCh) })

	reply, err := svc.SendMessage(ctx, domain.SendMessageInput{
		ChannelID: w.chID, SenderID: w.f.userID, SenderNodeID: userNode, Content: "a reply", ParentMessageID: w.msgID,
	})
	require.NoError(t, err)

	deleted, err := svc.SendMessage(ctx, domain.SendMessageInput{
		ChannelID: w.chID, SenderID: w.f.userID, SenderNodeID: userNode, Content: "soon gone",
	})
	require.NoError(t, err)
	_, err = w.f.pool.Exec(ctx, `UPDATE messages SET deleted_at = now() WHERE id = $1`, deleted.Id)
	require.NoError(t, err)

	t.Cleanup(func() { w.f.pool.Exec(ctx, `DELETE FROM thread_follows WHERE root_message_id = $1`, w.msgID) })

	for name, parent := range map[string]string{
		"parent in another channel": other.Id,
		"parent is itself a reply":  reply.Id,
		"parent is deleted":         deleted.Id,
		"parent does not exist":     "no-such-message",
	} {
		before := w.rows(t)
		_, _, err := svc.CreatePollMessage(ctx, w.pollIn(parent))
		require.ErrorIs(t, err, domain.ErrInvalidInput, "poll, %s", name)
		_, _, err = svc.CreateTaskMessage(ctx, w.taskIn(parent))
		require.ErrorIs(t, err, domain.ErrInvalidInput, "task, %s", name)
		assert.Equal(t, before, w.rows(t), "%s: nothing is written", name)
	}
	assert.Equal(t, 1, w.replyCount(t, w.msgID), "only the plain reply is counted")
}

func TestCreateCard_InAThreadNeedsWriteOnTheChannel(t *testing.T) {
	w := newCardWorld(t)
	ctx := context.Background()
	svc := w.f.svc

	for name, who := range map[string][2]string{
		"reader":   {w.reader, w.readerNode},
		"outsider": {"x", outsider},
	} {
		before := w.rows(t)
		p := w.pollIn(w.msgID)
		p.UserID, p.UserNodeID = who[0], who[1]
		_, _, err := svc.CreatePollMessage(ctx, p)
		require.ErrorIs(t, err, domain.ErrAccessDenied, "poll, %s", name)
		tk := w.taskIn(w.msgID)
		tk.UserID, tk.UserNodeID = who[0], who[1]
		_, _, err = svc.CreateTaskMessage(ctx, tk)
		require.ErrorIs(t, err, domain.ErrAccessDenied, "task, %s", name)
		assert.Equal(t, before, w.rows(t), "%s: nothing is written", name)
	}
	assert.Equal(t, 0, w.replyCount(t, w.msgID))

	// A caller who may not write learns nothing about the parent: a bad parent
	// is refused as access, not as input.
	p := w.pollIn("no-such-message")
	p.UserID, p.UserNodeID = w.reader, w.readerNode
	_, _, err := svc.CreatePollMessage(ctx, p)
	require.ErrorIs(t, err, domain.ErrAccessDenied)
}
