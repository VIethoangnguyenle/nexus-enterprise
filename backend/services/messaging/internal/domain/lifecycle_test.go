package domain_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"ngac-platform/ngac"
	pb "ngac-platform/proto/messaging"
	"ngac-platform/services/messaging/internal/domain"
	"ngac-platform/testutil"
)

func listed(t *testing.T, k *talk, id string) *pb.Message {
	t.Helper()
	list, err := k.f.svc.GetMessages(context.Background(), k.chID, k.f.userID, userNode, "", 50)
	require.NoError(t, err)
	for _, m := range list.Messages {
		if m.Id == id {
			return m
		}
	}
	return nil
}

func TestEditMessage_OnlyTheSenderEditsTheirOwnText(t *testing.T) {
	k := newTalk(t, "edit")
	ctx := context.Background()
	svc := k.f.svc

	edited, err := svc.EditMessage(ctx, k.msgID, k.f.userID, userNode, `<p>sửa <b>lại</b><script>x()</script></p>`, "html", nil)
	require.NoError(t, err)
	assert.Equal(t, `<p>sửa <b>lại</b></p>`, edited.Content, "an edit is filtered like a send")
	assert.NotNil(t, edited.EditedAt)
	assert.NotNil(t, listed(t, k, k.msgID).EditedAt)

	_, err = svc.EditMessage(ctx, k.msgID, k.f.userID, userNode, "   ", "markdown", nil)
	require.ErrorIs(t, err, domain.ErrInvalidInput, "an edit to nothing is a delete, not an edit")

	// Deny: someone else, even with write on the channel, cannot edit it.
	other, _ := testutil.CreateUser(t, k.f.pool)
	k.f.read.allow[grant{"ngac-other", k.oaID, ngac.OpWrite}] = true
	_, err = svc.EditMessage(ctx, k.msgID, other, "ngac-other", "chiếm quyền", "markdown", nil)
	require.ErrorIs(t, err, domain.ErrAccessDenied)
	// Deny: the sender who lost write on the channel.
	_, err = svc.EditMessage(ctx, k.msgID, k.f.userID, outsider, "x", "markdown", nil)
	require.ErrorIs(t, err, domain.ErrAccessDenied)
	assert.Equal(t, `<p>sửa <b>lại</b></p>`, listed(t, k, k.msgID).Content, "a refused edit changes nothing")

	_, err = svc.EditMessage(ctx, "no-such-message", k.f.userID, userNode, "x", "markdown", nil)
	require.ErrorIs(t, err, domain.ErrNotFound)
}

func TestEditMessage_RefusesACard(t *testing.T) {
	k := newTalk(t, "edit-card")
	ctx := context.Background()
	card, err := k.f.svc.SendMessage(ctx, domain.SendMessageInput{
		ChannelID: k.chID, SenderID: k.f.userID, SenderNodeID: userNode, Content: "Bình chọn",
		LinkedEntityType: "poll", LinkedEntityID: "poll-1",
	})
	require.NoError(t, err)
	_, err = k.f.svc.EditMessage(ctx, card.Id, k.f.userID, userNode, "khác", "markdown", nil)
	require.ErrorIs(t, err, domain.ErrInvalidInput)
}

func TestDeleteMessage_SenderOrManagerAndThePlaceholderRule(t *testing.T) {
	k := newThreadTalk(t) // root k.rootID by the caller, one reply by the replier
	ctx := context.Background()
	svc := k.f.svc

	// Deny: another writer without manage cannot delete the caller's message.
	k.f.read.allow[grant{replierNode, k.oaID, ngac.OpWrite}] = true
	require.ErrorIs(t, delErr(svc.DeleteMessage(ctx, k.rootID, k.replier, replierNode)), domain.ErrAccessDenied)
	require.ErrorIs(t, delErr(svc.DeleteMessage(ctx, k.rootID, "x", outsider)), domain.ErrAccessDenied)
	require.NotNil(t, listed(t, k.talk, k.rootID))

	// The sender deletes the root of a thread: it stays as a placeholder without its text.
	require.NoError(t, delErr(svc.DeleteMessage(ctx, k.rootID, k.f.userID, userNode)))
	root := listed(t, k.talk, k.rootID)
	require.NotNil(t, root, "a root with replies keeps its place for its thread")
	assert.True(t, root.Deleted)
	assert.Empty(t, root.Content)
	assert.Empty(t, root.Attachments)

	// A manager deletes someone else's reply; it leaves the thread and its count.
	k.f.read.allow[grant{userNode, k.oaID, ngac.OpManage}] = true
	require.NoError(t, delErr(svc.DeleteMessage(ctx, k.replyID, k.f.userID, userNode)))
	thread, err := svc.GetThread(ctx, k.rootID, k.f.userID, userNode)
	require.NoError(t, err)
	for _, m := range thread.Messages[1:] {
		assert.NotEqual(t, k.replyID, m.Id)
	}
	assert.EqualValues(t, 0, thread.Messages[0].ReplyCount)
	var by string
	require.NoError(t, k.f.pool.QueryRow(ctx, `SELECT deleted_by FROM messages WHERE id = $1`, k.replyID).Scan(&by))
	assert.Equal(t, k.f.userID, by, "who removed someone else's message is recorded")

	// A message without replies simply leaves the stream.
	lone, err := svc.SendMessage(ctx, domain.SendMessageInput{
		ChannelID: k.chID, SenderID: k.f.userID, SenderNodeID: userNode, Content: "lỡ tay",
	})
	require.NoError(t, err)
	require.NoError(t, delErr(svc.DeleteMessage(ctx, lone.Id, k.f.userID, userNode)))
	assert.Nil(t, listed(t, k.talk, lone.Id))
	require.ErrorIs(t, delErr(svc.DeleteMessage(ctx, lone.Id, k.f.userID, userNode)), domain.ErrNotFound, "deleted once")
}

func TestSendMessage_QuotesAMessageOfTheSameChannel(t *testing.T) {
	k := newTalk(t, "quote")
	ctx := context.Background()
	quoting, err := k.f.svc.SendMessage(ctx, domain.SendMessageInput{
		ChannelID: k.chID, SenderID: k.f.userID, SenderNodeID: userNode, Content: "đồng ý", QuotedMessageID: k.msgID,
	})
	require.NoError(t, err)
	got := listed(t, k, quoting.Id)
	require.NotNil(t, got.Quoted)
	assert.Equal(t, k.msgID, got.Quoted.Id)
	assert.Equal(t, "hello needle", got.Quoted.Text)
	assert.Equal(t, "testuser", got.Quoted.SenderName)

	otherCh, otherOA := k.f.insertChannel(t, "quote-other")
	k.f.read.allow[grant{userNode, otherOA, ngac.OpWrite}] = true
	foreign, err := k.f.svc.SendMessage(ctx, domain.SendMessageInput{
		ChannelID: otherCh, SenderID: k.f.userID, SenderNodeID: userNode, Content: "bí mật",
	})
	require.NoError(t, err)
	t.Cleanup(func() { k.f.pool.Exec(ctx, `DELETE FROM messages WHERE channel_id = $1`, otherCh) })
	_, err = k.f.svc.SendMessage(ctx, domain.SendMessageInput{
		ChannelID: k.chID, SenderID: k.f.userID, SenderNodeID: userNode, Content: "lộ", QuotedMessageID: foreign.Id,
	})
	require.ErrorIs(t, err, domain.ErrInvalidInput, "quoting another channel would show its text here")
	require.ErrorIs(t, err, domain.ErrQuoteUnavailable, "the refusal is told apart from other bad input")

	// A quoted message deleted later shows as deleted, without its text.
	require.NoError(t, delErr(k.f.svc.DeleteMessage(ctx, k.msgID, k.f.userID, userNode)))
	got = listed(t, k, quoting.Id)
	require.NotNil(t, got.Quoted)
	assert.True(t, got.Quoted.Deleted)
	assert.Empty(t, got.Quoted.Text)
}

func TestMarkMessageUnread_MovesTheReadPositionBeforeIt(t *testing.T) {
	k := newThreadTalk(t)
	ctx := context.Background()
	svc := k.f.svc
	require.NoError(t, svc.MarkChannelRead(ctx, k.f.userID, userNode, k.chID, k.rootID))
	_, err := svc.MarkThreadRead(ctx, k.rootID, k.f.userID, userNode)
	require.NoError(t, err)

	channelUnread := func() (int32, int32) {
		counts, err := svc.GetUnreadCounts(ctx, k.f.userID, userNode, k.f.wsID)
		require.NoError(t, err)
		for _, c := range counts {
			if c.ChannelId == k.chID {
				return c.UnreadCount, c.ThreadUnreadCount
			}
		}
		t.Fatal("channel not counted")
		return 0, 0
	}
	n, tn := channelUnread()
	require.Zero(t, n)
	require.Zero(t, tn)

	require.NoError(t, svc.MarkMessageUnread(ctx, k.rootID, k.f.userID, userNode))
	n, _ = channelUnread()
	assert.EqualValues(t, 1, n, "the message and everything after it are unread again")

	require.NoError(t, svc.MarkMessageUnread(ctx, k.replyID, k.f.userID, userNode))
	_, tn = channelUnread()
	assert.EqualValues(t, 1, tn, "a reply marked unread counts in its thread")

	require.ErrorIs(t, svc.MarkMessageUnread(ctx, k.rootID, "x", outsider), domain.ErrAccessDenied)
}

// delErr drops the event a delete returns, for assertions on its error.
func delErr(_ *domain.DeleteOutcome, err error) error { return err }
