package domain_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"ngac-platform/ngac"
	"ngac-platform/services/messaging/internal/domain"
	"ngac-platform/testutil"
)

// threadTalk is a talk whose first message has one reply by a second user.
type threadTalk struct {
	*talk
	rootID   string
	replier  string // users.id of the replier
	replyID  string
}

const replierNode = "ngac-user-replier"

func newThreadTalk(t *testing.T) *threadTalk {
	t.Helper()
	k := newTalk(t, "threads")
	ctx := context.Background()
	replier, _ := testutil.CreateUser(t, k.f.pool)
	k.f.read.allow[grant{replierNode, k.oaID, ngac.OpRead}] = true
	k.f.read.allow[grant{replierNode, k.oaID, ngac.OpWrite}] = true
	reply, err := k.f.svc.SendMessage(ctx, domain.SendMessageInput{
		ChannelID: k.chID, SenderID: replier, SenderNodeID: replierNode, Content: "a reply", ParentMessageID: k.msgID,
	})
	require.NoError(t, err)
	t.Cleanup(func() {
		k.f.pool.Exec(ctx, `DELETE FROM thread_follows WHERE root_message_id = $1`, k.msgID)
		k.f.pool.Exec(ctx, `DELETE FROM messages WHERE parent_message_id = $1`, k.msgID)
	})
	return &threadTalk{talk: k, rootID: k.msgID, replier: replier, replyID: reply.Id}
}

func TestGetMessages_CarriesTheCallersThreadSummary(t *testing.T) {
	k := newThreadTalk(t)
	list, err := k.f.svc.GetMessages(context.Background(), k.chID, k.f.userID, userNode, "", 20)
	require.NoError(t, err)
	require.Len(t, list.Messages, 1, "replies stay out of the main stream")
	root := list.Messages[0]
	assert.EqualValues(t, 1, root.ReplyCount)
	assert.NotNil(t, root.LastReplyAt)
	assert.True(t, root.Following, "the starter follows the thread")
	assert.EqualValues(t, 1, root.ThreadUnreadCount)
	assert.Equal(t, []string{k.replier}, root.ReplyParticipantIds)

	thread, err := k.f.svc.GetThread(context.Background(), k.rootID, k.f.userID, userNode)
	require.NoError(t, err)
	require.Len(t, thread.Messages, 2)
	assert.True(t, thread.Messages[0].Following)
	assert.EqualValues(t, 1, thread.Messages[0].ThreadUnreadCount)
}

func TestFollowUnfollowAndReadThread_AllowedWithRead(t *testing.T) {
	k := newThreadTalk(t)
	ctx := context.Background()
	svc := k.f.svc
	// Read alone is enough: following and reading change only the caller's own state.
	k.f.read.allow[grant{"ngac-reader", k.oaID, ngac.OpRead}] = true
	reader, _ := testutil.CreateUser(t, k.f.pool)

	require.NoError(t, svc.FollowThread(ctx, k.rootID, reader, "ngac-reader"))
	threads, err := svc.ListThreads(ctx, k.chID, reader, "ngac-reader", "following", "", "", 20)
	require.NoError(t, err)
	require.Len(t, threads.Messages, 1)
	assert.Equal(t, k.rootID, threads.Messages[0].Id)

	require.NoError(t, svc.MarkThreadRead(ctx, k.rootID, k.f.userID, userNode))
	threads, err = svc.ListThreads(ctx, k.chID, k.f.userID, userNode, "unread", "", "", 20)
	require.NoError(t, err)
	assert.Empty(t, threads.Messages)

	require.NoError(t, svc.UnfollowThread(ctx, k.rootID, k.f.userID, userNode))
	threads, err = svc.ListThreads(ctx, k.chID, k.f.userID, userNode, "following", "", "", 20)
	require.NoError(t, err)
	assert.Empty(t, threads.Messages)
}

func TestThreadEndpoints_DeniedWithoutRead(t *testing.T) {
	k := newThreadTalk(t)
	ctx := context.Background()
	svc := k.f.svc
	stranger, _ := testutil.CreateUser(t, k.f.pool)

	require.ErrorIs(t, svc.FollowThread(ctx, k.rootID, stranger, outsider), domain.ErrAccessDenied)
	require.ErrorIs(t, svc.UnfollowThread(ctx, k.rootID, stranger, outsider), domain.ErrAccessDenied)
	require.ErrorIs(t, svc.MarkThreadRead(ctx, k.rootID, stranger, outsider), domain.ErrAccessDenied)
	_, err := svc.ListThreads(ctx, k.chID, stranger, outsider, "all", "", "", 20)
	require.ErrorIs(t, err, domain.ErrAccessDenied)

	// A denied follow leaves no state behind.
	var rows int
	require.NoError(t, k.f.pool.QueryRow(ctx,
		`SELECT count(*) FROM thread_follows WHERE user_id = $1`, stranger).Scan(&rows))
	assert.Zero(t, rows)
	assert.Contains(t, k.f.read.asked(), grant{outsider, k.oaID, ngac.OpRead})
}

func TestThreadEndpoints_RejectBadInput(t *testing.T) {
	k := newThreadTalk(t)
	ctx := context.Background()
	svc := k.f.svc

	require.ErrorIs(t, svc.FollowThread(ctx, "", k.f.userID, userNode), domain.ErrInvalidInput)
	require.ErrorIs(t, svc.FollowThread(ctx, "no-such-message", k.f.userID, userNode), domain.ErrNotFound)
	require.ErrorIs(t, svc.FollowThread(ctx, k.replyID, k.f.userID, userNode), domain.ErrInvalidInput,
		"a thread is named by its root message, not by a reply")
	require.ErrorIs(t, svc.MarkThreadRead(ctx, k.replyID, k.f.userID, userNode), domain.ErrInvalidInput)
	_, err := svc.ListThreads(ctx, k.chID, k.f.userID, userNode, "everything", "", "", 20)
	require.ErrorIs(t, err, domain.ErrInvalidInput)
	_, err = svc.ListThreads(ctx, k.chID, k.f.userID, userNode, "all", "yesterday", "", 20)
	require.ErrorIs(t, err, domain.ErrInvalidInput)
}

// A reply names its parent; the parent must be a root message of the same
// channel. Otherwise a writer of one channel could attach replies to a thread
// in a channel they cannot write, and raise its counts.
func TestSendMessage_ReplyMustTargetARootMessageOfTheSameChannel(t *testing.T) {
	k := newThreadTalk(t)
	ctx := context.Background()
	otherCh, otherOA := k.f.insertChannel(t, "threads-other")
	k.f.read.allow[grant{userNode, otherOA, ngac.OpWrite}] = true
	foreign, err := k.f.svc.SendMessage(ctx, domain.SendMessageInput{
		ChannelID: otherCh, SenderID: k.f.userID, SenderNodeID: userNode, Content: "elsewhere",
	})
	require.NoError(t, err)
	t.Cleanup(func() { k.f.pool.Exec(ctx, `DELETE FROM messages WHERE channel_id = $1`, otherCh) })

	send := func(parent string) error {
		_, err := k.f.svc.SendMessage(ctx, domain.SendMessageInput{
			ChannelID: k.chID, SenderID: k.f.userID, SenderNodeID: userNode, Content: "re", ParentMessageID: parent,
		})
		return err
	}
	require.ErrorIs(t, send(foreign.Id), domain.ErrInvalidInput, "parent in another channel")
	require.ErrorIs(t, send("no-such-message"), domain.ErrInvalidInput, "parent that does not exist")
	require.ErrorIs(t, send(k.replyID), domain.ErrInvalidInput, "a reply to a reply")

	var replies int
	require.NoError(t, k.f.pool.QueryRow(ctx, `SELECT reply_count FROM messages WHERE id = $1`, foreign.Id).Scan(&replies))
	assert.Zero(t, replies, "the other channel's message gained no reply")
	require.NoError(t, send(k.rootID))
}

func TestGetUnreadCounts_IncludesThreadUnread(t *testing.T) {
	k := newThreadTalk(t)
	ctx := context.Background()

	_, err := k.f.svc.GetUnreadCounts(ctx, k.f.userID, "", k.f.wsID)
	require.ErrorIs(t, err, domain.ErrInvalidInput, "a caller without a node has no channels to count")
	counts, err := k.f.svc.GetUnreadCounts(ctx, k.f.userID, userNode, k.f.wsID)
	require.NoError(t, err)
	var found bool
	for _, c := range counts {
		if c.ChannelId == k.chID {
			found = true
			assert.EqualValues(t, 1, c.ThreadUnreadCount)
		}
	}
	assert.True(t, found)
}

// Unread counts cover every channel of the workspace the caller may read —
// joined or not, as the sidebar lists them — and no channel they may not read.
func TestGetUnreadCounts_CoversExactlyTheChannelsTheCallerMayRead(t *testing.T) {
	k := newThreadTalk(t)
	ctx := context.Background()
	hidden, _ := k.f.insertChannel(t, "unread-hidden")
	notJoined, notJoinedOA := k.f.insertChannel(t, "unread-not-joined")
	k.f.read.allow[grant{userNode, notJoinedOA, ngac.OpRead}] = true

	counts, err := k.f.svc.GetUnreadCounts(ctx, k.f.userID, userNode, k.f.wsID)
	require.NoError(t, err)
	listed := map[string]bool{}
	for _, c := range counts {
		listed[c.ChannelId] = true
	}
	assert.True(t, listed[k.chID])
	assert.True(t, listed[notJoined], "a readable channel the caller never joined still has its count")
	assert.False(t, listed[hidden], "a channel without read is never named")

	// The policy service unreachable: no channel is named at all.
	k.f.read.err = assert.AnError
	counts, err = k.f.svc.GetUnreadCounts(ctx, k.f.userID, userNode, k.f.wsID)
	require.NoError(t, err)
	assert.Empty(t, counts)
}
