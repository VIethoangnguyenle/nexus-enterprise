package domain_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"ngac-platform/ngac"
	"ngac-platform/services/messaging/internal/domain"
	"ngac-platform/testutil"
)

// cardWorld is a channel with a reader (read only) besides the caller.
type cardWorld struct {
	*talk
	reader, readerNode string
}

func newCardWorld(t *testing.T) *cardWorld {
	t.Helper()
	k := newTalk(t, "cards")
	reader, _ := testutil.CreateUser(t, k.f.pool)
	k.f.read.allow[grant{"ngac-card-reader", k.oaID, ngac.OpRead}] = true
	return &cardWorld{talk: k, reader: reader, readerNode: "ngac-card-reader"}
}

func (w *cardWorld) poll(t *testing.T, multi, anonymous bool, options ...string) string {
	t.Helper()
	p, err := w.f.svc.CreatePoll(context.Background(), domain.CreatePollInput{
		ChannelID: w.chID, UserID: w.f.userID, UserNodeID: userNode, Question: "Ăn trưa ở đâu?",
		Options: options, IsMulti: multi, IsAnonymous: anonymous,
	})
	require.NoError(t, err)
	return p.Id
}

func TestCreatePoll_TwoToTenOptions(t *testing.T) {
	w := newCardWorld(t)
	ctx := context.Background()
	in := func(opts ...string) domain.CreatePollInput {
		return domain.CreatePollInput{ChannelID: w.chID, UserID: w.f.userID, UserNodeID: userNode, Question: "?", Options: opts}
	}
	_, err := w.f.svc.CreatePoll(ctx, in("một"))
	require.ErrorIs(t, err, domain.ErrInvalidInput)
	_, err = w.f.svc.CreatePoll(ctx, in("1", "2", "3", "4", "5", "6", "7", "8", "9", "10", "11"))
	require.ErrorIs(t, err, domain.ErrInvalidInput)
	_, err = w.f.svc.CreatePoll(ctx, in("có", "  "))
	require.ErrorIs(t, err, domain.ErrInvalidInput, "an empty option")
	past := time.Now().Add(-time.Hour)
	bad := in("có", "không")
	bad.EndsAt = &past
	_, err = w.f.svc.CreatePoll(ctx, bad)
	require.ErrorIs(t, err, domain.ErrInvalidInput, "an end time in the past")
	// Deny: a reader cannot create.
	r := in("có", "không")
	r.UserID, r.UserNodeID = w.reader, w.readerNode
	_, err = w.f.svc.CreatePoll(ctx, r)
	require.ErrorIs(t, err, domain.ErrAccessDenied)
}

// Results stay hidden from a member until they vote (or the poll closes); a
// single-choice vote replaces the previous one; reading is enough to vote.
func TestVotePoll_ReadToVoteSingleChoiceAndHiddenResults(t *testing.T) {
	w := newCardWorld(t)
	ctx := context.Background()
	svc := w.f.svc
	id := w.poll(t, false, false, "Phở", "Cơm")
	p, err := svc.GetPoll(ctx, id, w.f.userID, userNode)
	require.NoError(t, err)
	pho, com := p.Options[0].Id, p.Options[1].Id

	require.NoError(t, svc.VotePoll(ctx, id, pho, w.readerNode, w.reader), "read is enough to vote")
	asReader, err := svc.GetPoll(ctx, id, w.reader, w.readerNode)
	require.NoError(t, err)
	assert.True(t, asReader.ResultsVisible)
	assert.Equal(t, []string{pho}, asReader.MyOptionIds)
	assert.EqualValues(t, 1, asReader.Options[0].VoteCount)

	asCreator, err := svc.GetPoll(ctx, id, w.f.userID, userNode)
	require.NoError(t, err)
	assert.False(t, asCreator.ResultsVisible, "not voted yet: no results")
	for _, o := range asCreator.Options {
		assert.Zero(t, o.VoteCount)
		assert.Empty(t, o.VoterIds)
	}
	assert.Zero(t, asCreator.TotalVotes)

	require.NoError(t, svc.VotePoll(ctx, id, com, w.readerNode, w.reader))
	asReader, _ = svc.GetPoll(ctx, id, w.reader, w.readerNode)
	assert.Equal(t, []string{com}, asReader.MyOptionIds, "a single-choice vote replaces the previous one")
	assert.EqualValues(t, 1, asReader.TotalVotes)

	// Deny: no read on the channel, no vote and no poll.
	require.ErrorIs(t, svc.VotePoll(ctx, id, pho, outsider, "x"), domain.ErrAccessDenied)
	_, err = svc.GetPoll(ctx, id, "x", outsider)
	require.ErrorIs(t, err, domain.ErrAccessDenied)
}

func TestVotePoll_AnonymousNeverNamesVoters(t *testing.T) {
	w := newCardWorld(t)
	ctx := context.Background()
	id := w.poll(t, true, true, "A", "B")
	p, _ := w.f.svc.GetPoll(ctx, id, w.f.userID, userNode)
	require.NoError(t, w.f.svc.VotePoll(ctx, id, p.Options[0].Id, userNode, w.f.userID))
	require.NoError(t, w.f.svc.VotePoll(ctx, id, p.Options[1].Id, userNode, w.f.userID))
	p, err := w.f.svc.GetPoll(ctx, id, w.f.userID, userNode)
	require.NoError(t, err)
	assert.Len(t, p.MyOptionIds, 2, "multi-choice keeps both")
	for _, o := range p.Options {
		assert.EqualValues(t, 1, o.VoteCount)
		assert.Empty(t, o.VoterIds, "an anonymous poll names nobody, not even to its creator")
	}
}

func TestClosePoll_CreatorOrManagerOnly(t *testing.T) {
	w := newCardWorld(t)
	ctx := context.Background()
	svc := w.f.svc
	id := w.poll(t, false, false, "A", "B")
	p, _ := svc.GetPoll(ctx, id, w.f.userID, userNode)
	a := p.Options[0].Id

	_, err := svc.ClosePoll(ctx, id, w.reader, w.readerNode)
	require.ErrorIs(t, err, domain.ErrAccessDenied, "neither its creator nor a manager")

	closed, err := svc.ClosePoll(ctx, id, w.f.userID, userNode)
	require.NoError(t, err)
	assert.True(t, closed.Closed)
	assert.True(t, closed.ResultsVisible, "a closed poll shows its results")
	require.ErrorIs(t, svc.VotePoll(ctx, id, a, w.readerNode, w.reader), domain.ErrInvalidInput, "no votes after closing")
	require.ErrorIs(t, svc.RemoveVote(ctx, id, a, w.readerNode, w.reader), domain.ErrInvalidInput)

	// A manager may close someone else's poll.
	other := w.poll(t, false, false, "A", "B")
	w.f.read.allow[grant{w.readerNode, w.oaID, ngac.OpManage}] = true
	_, err = svc.ClosePoll(ctx, other, w.reader, w.readerNode)
	require.NoError(t, err)
}

// A task is given to someone who reads the channel; anyone else would be
// handed work they cannot even see.
func TestCreateTask_AssigneeMustReadTheChannel(t *testing.T) {
	w := newCardWorld(t)
	ctx := context.Background()
	stranger, strangerNode := testutil.CreateUser(t, w.f.pool)
	_ = strangerNode // holds no grant on the channel
	_, err := w.f.svc.CreateTask(ctx, domain.CreateTaskInput{
		ChannelID: w.chID, UserID: w.f.userID, UserNodeID: userNode, Title: "x", AssigneeID: stranger,
	})
	require.ErrorIs(t, err, domain.ErrInvalidInput)

	member, memberNode := testutil.CreateUser(t, w.f.pool)
	w.f.read.allow[grant{memberNode, w.oaID, ngac.OpRead}] = true
	_, err = w.f.svc.CreateTask(ctx, domain.CreateTaskInput{
		ChannelID: w.chID, UserID: w.f.userID, UserNodeID: userNode, Title: "x", AssigneeID: member,
	})
	require.NoError(t, err)
}

// Marking a task done from its card is for its assignee or its creator (or a
// channel manager), not for every writer.
func TestUpdateTask_StatusByAssigneeCreatorOrManager(t *testing.T) {
	w := newCardWorld(t)
	ctx := context.Background()
	svc := w.f.svc
	writer, _ := testutil.CreateUser(t, w.f.pool)
	w.f.read.allow[grant{"ngac-card-writer", w.oaID, ngac.OpWrite}] = true
	w.f.read.allow[grant{"u-" + w.reader, w.oaID, ngac.OpRead}] = true // the assignee reads the channel
	task, err := svc.CreateTask(ctx, domain.CreateTaskInput{
		ChannelID: w.chID, UserID: w.f.userID, UserNodeID: userNode, Title: "Gửi báo cáo", AssigneeID: w.reader,
	})
	require.NoError(t, err)

	_, err = svc.UpdateTask(ctx, domain.UpdateTaskInput{TaskID: task.Id, UserID: writer, UserNodeID: "ngac-card-writer", Status: "done"})
	require.ErrorIs(t, err, domain.ErrAccessDenied, "a writer who is neither assignee nor creator")

	w.f.read.allow[grant{w.readerNode, w.oaID, ngac.OpWrite}] = true
	done, err := svc.UpdateTask(ctx, domain.UpdateTaskInput{TaskID: task.Id, UserID: w.reader, UserNodeID: w.readerNode, Status: "done"})
	require.NoError(t, err, "the assignee")
	assert.Equal(t, "done", done.Status)
	_, err = svc.UpdateTask(ctx, domain.UpdateTaskInput{TaskID: task.Id, UserID: w.f.userID, UserNodeID: userNode, Status: "todo"})
	require.NoError(t, err, "the creator")
}

// The assignee marks their own task done even when they only read the
// conversation; changing anything else about it still takes write.
func TestUpdateTask_AReadOnlyAssigneeCompletesTheirTask(t *testing.T) {
	w := newCardWorld(t)
	ctx := context.Background()
	w.f.read.allow[grant{w.readerNode, w.oaID, ngac.OpRead}] = true
	w.f.read.allow[grant{"u-" + w.reader, w.oaID, ngac.OpRead}] = true
	task, err := w.f.svc.CreateTask(ctx, domain.CreateTaskInput{
		ChannelID: w.chID, UserID: w.f.userID, UserNodeID: userNode, Title: "Gửi báo cáo", AssigneeID: w.reader,
	})
	require.NoError(t, err)

	done, err := w.f.svc.UpdateTask(ctx, domain.UpdateTaskInput{TaskID: task.Id, UserID: w.reader, UserNodeID: w.readerNode, Status: "done"})
	require.NoError(t, err)
	assert.Equal(t, "done", done.Status)

	_, err = w.f.svc.UpdateTask(ctx, domain.UpdateTaskInput{TaskID: task.Id, UserID: w.reader, UserNodeID: w.readerNode, Title: "Đổi tên"})
	require.ErrorIs(t, err, domain.ErrAccessDenied, "renaming takes write")

	// Deny: a reader who is not the assignee cannot touch it at all.
	other, otherNode := testutil.CreateUser(t, w.f.pool)
	w.f.read.allow[grant{otherNode, w.oaID, ngac.OpRead}] = true
	_, err = w.f.svc.UpdateTask(ctx, domain.UpdateTaskInput{TaskID: task.Id, UserID: other, UserNodeID: otherNode, Status: "todo"})
	require.ErrorIs(t, err, domain.ErrAccessDenied)
}

// Handing a task to someone else is for its creator or a channel manager: a
// writer must not make themselves the assignee and then mark it done.
func TestUpdateTask_ReassignIsForCreatorOrManager(t *testing.T) {
	w := newCardWorld(t)
	ctx := context.Background()
	svc := w.f.svc
	writer, writerNode := testutil.CreateUser(t, w.f.pool)
	w.f.read.allow[grant{writerNode, w.oaID, ngac.OpWrite}] = true
	w.f.read.allow[grant{writerNode, w.oaID, ngac.OpRead}] = true
	task, err := svc.CreateTask(ctx, domain.CreateTaskInput{ChannelID: w.chID, UserID: w.f.userID, UserNodeID: userNode, Title: "x"})
	require.NoError(t, err)

	_, err = svc.UpdateTask(ctx, domain.UpdateTaskInput{TaskID: task.Id, UserID: writer, UserNodeID: writerNode, AssigneeID: writer})
	require.ErrorIs(t, err, domain.ErrAccessDenied, "a writer cannot take the task over")

	// Deny: even the creator cannot hand it to someone who cannot read the channel.
	stranger, _ := testutil.CreateUser(t, w.f.pool)
	_, err = svc.UpdateTask(ctx, domain.UpdateTaskInput{TaskID: task.Id, UserID: w.f.userID, UserNodeID: userNode, AssigneeID: stranger})
	require.ErrorIs(t, err, domain.ErrInvalidInput)

	got, err := svc.UpdateTask(ctx, domain.UpdateTaskInput{TaskID: task.Id, UserID: w.f.userID, UserNodeID: userNode, AssigneeID: writer})
	require.NoError(t, err, "the creator hands it to a member")
	assert.Equal(t, writer, got.AssigneeId)
}
