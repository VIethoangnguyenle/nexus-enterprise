package domain_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"ngac-platform/ngac"
	"ngac-platform/services/messaging/internal/domain"
)

const outsider = "ngac-outsider"

// talk is a channel the caller may read and write, with one message in it.
type talk struct {
	f     *fixture
	chID  string
	oaID  string
	msgID string
}

func newTalk(t *testing.T, name string) *talk {
	t.Helper()
	f := newFixture(t)
	chID, oaID := f.insertChannel(t, name)
	f.read.allow[grant{userNode, oaID, ngac.OpRead}] = true
	f.read.allow[grant{userNode, oaID, ngac.OpWrite}] = true
	msg, err := f.svc.SendMessage(context.Background(), domain.SendMessageInput{
		ChannelID: chID, SenderID: f.userID, SenderNodeID: userNode, Content: "hello needle",
	})
	require.NoError(t, err)
	t.Cleanup(func() {
		ctx := context.Background()
		f.pool.Exec(ctx, `DELETE FROM message_pins WHERE channel_id = $1`, chID)
		f.pool.Exec(ctx, `DELETE FROM polls WHERE channel_id = $1`, chID)
		f.pool.Exec(ctx, `DELETE FROM chat_tasks WHERE channel_id = $1`, chID)
		f.pool.Exec(ctx, `DELETE FROM read_receipts WHERE channel_id = $1`, chID)
		f.pool.Exec(ctx, `DELETE FROM messages WHERE channel_id = $1`, chID)
	})
	return &talk{f: f, chID: chID, oaID: oaID, msgID: msg.Id}
}

func TestReactions_AddListRemoveAndDeny(t *testing.T) {
	k := newTalk(t, "react")
	ctx := context.Background()
	svc := k.f.svc

	require.NoError(t, svc.AddReaction(ctx, k.msgID, userNode, k.f.userID, "👍"))
	require.NoError(t, svc.AddReaction(ctx, k.msgID, userNode, k.f.userID, "👍"), "adding twice is idempotent")
	groups, err := svc.ListReactions(ctx, k.msgID, userNode)
	require.NoError(t, err)
	require.Len(t, groups, 1)
	assert.EqualValues(t, 1, groups[0].Count)

	// Deny: someone with no grant on the channel neither reads nor writes reactions.
	require.ErrorIs(t, svc.AddReaction(ctx, k.msgID, outsider, "someone", "🎉"), domain.ErrAccessDenied)
	_, err = svc.ListReactions(ctx, k.msgID, outsider)
	require.ErrorIs(t, err, domain.ErrAccessDenied)
	require.ErrorIs(t, svc.RemoveReaction(ctx, k.msgID, outsider, k.f.userID, "👍"), domain.ErrAccessDenied)
	groups, _ = svc.ListReactions(ctx, k.msgID, userNode)
	require.Len(t, groups, 1, "a denied write changes nothing")

	// Read alone is not enough to react.
	k.f.read.allow[grant{"ngac-reader", k.oaID, ngac.OpRead}] = true
	require.ErrorIs(t, svc.AddReaction(ctx, k.msgID, "ngac-reader", "someone", "🎉"), domain.ErrAccessDenied)

	require.NoError(t, svc.RemoveReaction(ctx, k.msgID, userNode, k.f.userID, "👍"))
	groups, err = svc.ListReactions(ctx, k.msgID, userNode)
	require.NoError(t, err)
	assert.Empty(t, groups)

	require.ErrorIs(t, svc.AddReaction(ctx, "", userNode, k.f.userID, "👍"), domain.ErrInvalidInput)
	require.ErrorIs(t, svc.AddReaction(ctx, k.msgID, userNode, k.f.userID, ""), domain.ErrInvalidInput)
	require.ErrorIs(t, svc.AddReaction(ctx, "no-such-message", userNode, k.f.userID, "👍"), domain.ErrNotFound)
}

func TestPins_PinListUnpinAndDeny(t *testing.T) {
	k := newTalk(t, "pins")
	ctx := context.Background()
	svc := k.f.svc

	require.NoError(t, svc.PinMessage(ctx, k.chID, k.msgID, userNode, k.f.userID))
	pins, err := svc.ListPins(ctx, k.chID, userNode)
	require.NoError(t, err)
	require.Len(t, pins, 1)
	assert.Equal(t, k.msgID, pins[0].Message.Id)

	require.ErrorIs(t, svc.PinMessage(ctx, k.chID, k.msgID, outsider, "x"), domain.ErrAccessDenied)
	_, err = svc.ListPins(ctx, k.chID, outsider)
	require.ErrorIs(t, err, domain.ErrAccessDenied)
	require.ErrorIs(t, svc.UnpinMessage(ctx, k.chID, k.msgID, outsider), domain.ErrAccessDenied)
	pins, _ = svc.ListPins(ctx, k.chID, userNode)
	require.Len(t, pins, 1, "a denied unpin leaves the pin")

	require.ErrorIs(t, svc.PinMessage(ctx, "", k.msgID, userNode, k.f.userID), domain.ErrInvalidInput)
	require.NoError(t, svc.UnpinMessage(ctx, k.chID, k.msgID, userNode))
	pins, err = svc.ListPins(ctx, k.chID, userNode)
	require.NoError(t, err)
	assert.Empty(t, pins)
}

// A pin names a message of the channel it is pinned in. Pinning another
// channel's message would hand its text to everyone who can read this one.
func TestPins_RefuseAMessageFromAnotherChannel(t *testing.T) {
	here := newTalk(t, "pins-here")
	ctx := context.Background()
	otherCh, otherOA := here.f.insertChannel(t, "pins-other")
	here.f.read.allow[grant{userNode, otherOA, ngac.OpWrite}] = true
	foreign, err := here.f.svc.SendMessage(ctx, domain.SendMessageInput{
		ChannelID: otherCh, SenderID: here.f.userID, SenderNodeID: userNode, Content: "secret of the other channel",
	})
	require.NoError(t, err)
	t.Cleanup(func() { here.f.pool.Exec(ctx, `DELETE FROM messages WHERE channel_id = $1`, otherCh) })

	err = here.f.svc.PinMessage(ctx, here.chID, foreign.Id, userNode, here.f.userID)
	require.ErrorIs(t, err, domain.ErrInvalidInput)
	err = here.f.svc.PinMessage(ctx, here.chID, "no-such-message", userNode, here.f.userID)
	require.ErrorIs(t, err, domain.ErrInvalidInput)

	pins, err := here.f.svc.ListPins(ctx, here.chID, userNode)
	require.NoError(t, err)
	assert.Empty(t, pins)
}

// A pin row that already points at another channel's message (written before
// the check existed) is never shown.
func TestListPins_SkipsAPinThatPointsOutsideTheChannel(t *testing.T) {
	here := newTalk(t, "pins-legacy")
	ctx := context.Background()
	otherCh, _ := here.f.insertChannel(t, "pins-legacy-other")
	foreign, err := here.f.svc.SendMessage(ctx, domain.SendMessageInput{
		ChannelID: otherCh, SenderID: here.f.userID, SenderNodeID: userNode, Content: "elsewhere", MessageType: "system",
	})
	require.NoError(t, err)
	t.Cleanup(func() { here.f.pool.Exec(ctx, `DELETE FROM messages WHERE channel_id = $1`, otherCh) })
	_, err = here.f.pool.Exec(ctx, `INSERT INTO message_pins (channel_id, message_id, pinned_by) VALUES ($1, $2, $3)`,
		here.chID, foreign.Id, here.f.userID)
	require.NoError(t, err)

	pins, err := here.f.svc.ListPins(ctx, here.chID, userNode)
	require.NoError(t, err)
	assert.Empty(t, pins)
}

func TestReadReceipts_MarkAndCountAndDeny(t *testing.T) {
	k := newTalk(t, "receipts")
	ctx := context.Background()
	svc := k.f.svc

	require.NoError(t, svc.MarkChannelRead(ctx, k.f.userID, userNode, k.chID, k.msgID))
	counts, err := svc.GetUnreadCounts(ctx, k.f.userID)
	require.NoError(t, err)
	for _, c := range counts {
		if c.ChannelId == k.chID {
			assert.Equal(t, k.msgID, c.LastReadMessageId)
		}
	}

	require.ErrorIs(t, svc.MarkChannelRead(ctx, k.f.userID, outsider, k.chID, k.msgID), domain.ErrAccessDenied)
	require.ErrorIs(t, svc.MarkChannelRead(ctx, k.f.userID, userNode, "", k.msgID), domain.ErrInvalidInput)
	require.ErrorIs(t, svc.MarkChannelRead(ctx, k.f.userID, userNode, k.chID, "invented-id"), domain.ErrInvalidInput)

	otherCh, _ := k.f.insertChannel(t, "receipts-other")
	err = svc.MarkChannelRead(ctx, k.f.userID, userNode, otherCh, k.msgID)
	require.Error(t, err, "a receipt cannot reference another channel's message")
}

func TestSearchMessages_FindsOnlyInAChannelTheCallerMayRead(t *testing.T) {
	k := newTalk(t, "search")
	ctx := context.Background()
	svc := k.f.svc

	got, err := svc.SearchMessages(ctx, k.chID, userNode, "needle", 10)
	require.NoError(t, err)
	require.Len(t, got.Messages, 1)
	assert.Equal(t, k.msgID, got.Messages[0].Id)

	got, err = svc.SearchMessages(ctx, k.chID, userNode, "absent-word", 10)
	require.NoError(t, err)
	assert.Empty(t, got.Messages)

	_, err = svc.SearchMessages(ctx, k.chID, outsider, "needle", 10)
	require.ErrorIs(t, err, domain.ErrAccessDenied)

	empty, err := svc.SearchMessages(ctx, k.chID, outsider, "", 10)
	require.NoError(t, err, "an empty query is answered before anything is read")
	assert.Empty(t, empty.Messages)
}

func TestPolls_CreateVoteRemoveAndDeny(t *testing.T) {
	k := newTalk(t, "polls")
	ctx := context.Background()
	svc := k.f.svc

	_, err := svc.CreatePoll(ctx, domain.CreatePollInput{ChannelID: k.chID, UserID: k.f.userID, UserNodeID: outsider, Question: "q?", Options: []string{"a", "b"}})
	require.ErrorIs(t, err, domain.ErrAccessDenied)
	_, err = svc.CreatePoll(ctx, domain.CreatePollInput{ChannelID: k.chID, UserID: k.f.userID, UserNodeID: userNode, Question: "", Options: []string{"a", "b"}})
	require.ErrorIs(t, err, domain.ErrInvalidInput)
	_, err = svc.CreatePoll(ctx, domain.CreatePollInput{ChannelID: k.chID, UserID: k.f.userID, UserNodeID: userNode, Question: "q?", Options: []string{"only"}})
	require.ErrorIs(t, err, domain.ErrInvalidInput, "a poll needs two options")

	poll, err := svc.CreatePoll(ctx, domain.CreatePollInput{ChannelID: k.chID, UserID: k.f.userID, UserNodeID: userNode, Question: "lunch?", Options: []string{"pho", "bun"}})
	require.NoError(t, err)
	require.Len(t, poll.Options, 2)
	opt := poll.Options[0].Id

	require.ErrorIs(t, svc.VotePoll(ctx, poll.Id, opt, outsider, k.f.userID), domain.ErrAccessDenied)
	require.NoError(t, svc.VotePoll(ctx, poll.Id, opt, userNode, k.f.userID))
	require.NoError(t, svc.VotePoll(ctx, poll.Id, opt, userNode, k.f.userID), "voting twice is idempotent")
	got, err := svc.GetPoll(ctx, poll.Id, userNode)
	require.NoError(t, err)
	assert.EqualValues(t, 1, got.Options[0].VoteCount)

	_, err = svc.GetPoll(ctx, poll.Id, outsider)
	require.ErrorIs(t, err, domain.ErrAccessDenied)
	require.ErrorIs(t, svc.RemoveVote(ctx, poll.Id, opt, outsider, k.f.userID), domain.ErrAccessDenied)
	got, _ = svc.GetPoll(ctx, poll.Id, userNode)
	assert.EqualValues(t, 1, got.Options[0].VoteCount, "a denied removal keeps the vote")

	require.NoError(t, svc.RemoveVote(ctx, poll.Id, opt, userNode, k.f.userID))
	got, _ = svc.GetPoll(ctx, poll.Id, userNode)
	assert.EqualValues(t, 0, got.Options[0].VoteCount)

	require.ErrorIs(t, svc.VotePoll(ctx, "", opt, userNode, k.f.userID), domain.ErrInvalidInput)
	require.ErrorIs(t, svc.VotePoll(ctx, "no-such-poll", opt, userNode, k.f.userID), domain.ErrNotFound)
}

// An option belongs to one poll. A vote that names this channel's poll but
// another channel's option would land in the other channel's tally.
func TestPolls_RefuseAnOptionOfAnotherPoll(t *testing.T) {
	here := newTalk(t, "polls-here")
	ctx := context.Background()
	svc := here.f.svc
	mine, err := svc.CreatePoll(ctx, domain.CreatePollInput{ChannelID: here.chID, UserID: here.f.userID, UserNodeID: userNode, Question: "mine?", Options: []string{"a", "b"}})
	require.NoError(t, err)

	otherCh, otherOA := here.f.insertChannel(t, "polls-other")
	here.f.read.allow[grant{userNode, otherOA, ngac.OpRead}] = true
	here.f.read.allow[grant{userNode, otherOA, ngac.OpWrite}] = true
	theirs, err := svc.CreatePoll(ctx, domain.CreatePollInput{ChannelID: otherCh, UserID: here.f.userID, UserNodeID: userNode, Question: "theirs?", Options: []string{"x", "y"}})
	require.NoError(t, err)
	t.Cleanup(func() {
		here.f.pool.Exec(ctx, `DELETE FROM polls WHERE channel_id = $1`, otherCh)
		here.f.pool.Exec(ctx, `DELETE FROM messages WHERE channel_id = $1`, otherCh)
	})

	err = svc.VotePoll(ctx, mine.Id, theirs.Options[0].Id, userNode, here.f.userID)
	require.ErrorIs(t, err, domain.ErrInvalidInput)
	got, err := svc.GetPoll(ctx, theirs.Id, userNode)
	require.NoError(t, err)
	assert.EqualValues(t, 0, got.Options[0].VoteCount, "the other poll's tally is untouched")
}

func TestTasks_CreateUpdateListAndDeny(t *testing.T) {
	k := newTalk(t, "tasks")
	ctx := context.Background()
	svc := k.f.svc

	_, err := svc.CreateTask(ctx, domain.CreateTaskInput{ChannelID: k.chID, UserID: k.f.userID, UserNodeID: outsider, Title: "t"})
	require.ErrorIs(t, err, domain.ErrAccessDenied)
	_, err = svc.CreateTask(ctx, domain.CreateTaskInput{ChannelID: k.chID, UserID: k.f.userID, UserNodeID: userNode})
	require.ErrorIs(t, err, domain.ErrInvalidInput)

	task, err := svc.CreateTask(ctx, domain.CreateTaskInput{ChannelID: k.chID, UserID: k.f.userID, UserNodeID: userNode, Title: "ship it", DueDate: "2026-12-01"})
	require.NoError(t, err)
	assert.Equal(t, "todo", task.Status)

	_, err = svc.UpdateTask(ctx, domain.UpdateTaskInput{TaskID: task.Id, UserNodeID: outsider, Status: "done"})
	require.ErrorIs(t, err, domain.ErrAccessDenied)
	listed, err := svc.ListTasks(ctx, k.chID, userNode, "")
	require.NoError(t, err)
	require.Len(t, listed, 1)
	assert.Equal(t, "todo", listed[0].Status, "a denied update changes nothing")

	updated, err := svc.UpdateTask(ctx, domain.UpdateTaskInput{TaskID: task.Id, UserNodeID: userNode, Status: "done"})
	require.NoError(t, err)
	assert.Equal(t, "done", updated.Status)

	done, err := svc.ListTasks(ctx, k.chID, userNode, "done")
	require.NoError(t, err)
	assert.Len(t, done, 1)
	todo, err := svc.ListTasks(ctx, k.chID, userNode, "todo")
	require.NoError(t, err)
	assert.Empty(t, todo)

	_, err = svc.ListTasks(ctx, k.chID, outsider, "")
	require.ErrorIs(t, err, domain.ErrAccessDenied)
	_, err = svc.UpdateTask(ctx, domain.UpdateTaskInput{TaskID: "no-such-task", UserNodeID: userNode, Status: "done"})
	require.ErrorIs(t, err, domain.ErrNotFound)
}
