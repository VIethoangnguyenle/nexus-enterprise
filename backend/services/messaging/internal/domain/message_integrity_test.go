package domain_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"ngac-platform/ngac"
	"ngac-platform/services/messaging/internal/domain"
)

// An old-style file link is no back door: it is checked like an attachment, so
// another channel's file cannot be named, and a valid one becomes an attachment.
func TestSendMessage_ALinkedDriveFileIsCheckedLikeAnAttachment(t *testing.T) {
	k := newTalk(t, "legacy-link")
	ctx := context.Background()
	root := k.driveItem(t, "folder", "", "channel", k.chID, "active", "", true)
	mine := k.driveItem(t, "file", root, "channel", k.chID, "active", "image/png", false)
	otherRoot := k.driveItem(t, "folder", "", "channel", "another-channel", "active", "", true)
	foreign := k.driveItem(t, "file", otherRoot, "channel", "another-channel", "active", "image/png", false)
	t.Cleanup(func() { k.f.pool.Exec(ctx, `DELETE FROM drive_items WHERE parent_id IN ($1, $2)`, root, otherRoot) })

	_, err := k.f.svc.SendMessage(ctx, domain.SendMessageInput{
		ChannelID: k.chID, SenderID: k.f.userID, SenderNodeID: userNode, Content: "📎 x",
		LinkedEntityType: "drive_file", LinkedEntityID: foreign,
	})
	require.ErrorIs(t, err, domain.ErrInvalidInput, "another channel's file")

	msg, err := k.f.svc.SendMessage(ctx, domain.SendMessageInput{
		ChannelID: k.chID, SenderID: k.f.userID, SenderNodeID: userNode, Content: "📎 ảnh",
		LinkedEntityType: "drive_file", LinkedEntityID: mine,
	})
	require.NoError(t, err)
	require.Len(t, msg.Attachments, 1)
	assert.Equal(t, mine, msg.Attachments[0].DriveFileId)
	assert.Empty(t, msg.LinkedEntityType, "stored as an attachment, not a link")
}

// Whatever its label, a body with markup is filtered, and an unknown label is
// refused rather than failing in the database.
func TestSendMessage_MarkupIsFilteredWhateverTheLabel(t *testing.T) {
	k := newTalk(t, "label")
	ctx := context.Background()
	msg, err := k.f.svc.SendMessage(ctx, domain.SendMessageInput{
		ChannelID: k.chID, SenderID: k.f.userID, SenderNodeID: userNode, ContentFormat: "markdown",
		Content: `<p>xin chào<img src="https://track.example/p.gif"></p>`,
	})
	require.NoError(t, err)
	assert.Equal(t, "<p>xin chào</p>", msg.Content)
	assert.Equal(t, "html", msg.ContentFormat)

	plain, err := k.f.svc.SendMessage(ctx, domain.SendMessageInput{
		ChannelID: k.chID, SenderID: k.f.userID, SenderNodeID: userNode, ContentFormat: "markdown", Content: "a < b và b > c",
	})
	require.NoError(t, err)
	assert.Equal(t, "a < b và b > c", plain.Content, "a comparison is not markup")

	_, err = k.f.svc.SendMessage(ctx, domain.SendMessageInput{
		ChannelID: k.chID, SenderID: k.f.userID, SenderNodeID: userNode, ContentFormat: "rtf", Content: "x",
	})
	require.ErrorIs(t, err, domain.ErrInvalidInput)

	_, err = k.f.svc.SendMessage(ctx, domain.SendMessageInput{
		ChannelID: k.chID, SenderID: k.f.userID, SenderNodeID: userNode, ContentFormat: "html", Content: "<p><script>x()</script></p>",
	})
	require.ErrorIs(t, err, domain.ErrInvalidInput, "nothing left once filtered")

	_, err = k.f.svc.EditMessage(ctx, k.msgID, k.f.userID, userNode, `<b>sửa</b><iframe src="x"></iframe>`, "plain", nil)
	require.NoError(t, err)
	got := listed(t, k, k.msgID)
	assert.Equal(t, "<b>sửa</b>", got.Content, "an edit cannot relabel its way past the filter")
}

func TestSendMessage_NoReplyToADeletedMessage(t *testing.T) {
	k := newTalk(t, "reply-deleted")
	ctx := context.Background()
	_, err := k.f.svc.DeleteMessage(ctx, k.msgID, k.f.userID, userNode)
	require.NoError(t, err)
	_, err = k.f.svc.SendMessage(ctx, domain.SendMessageInput{
		ChannelID: k.chID, SenderID: k.f.userID, SenderNodeID: userNode, Content: "re", ParentMessageID: k.msgID,
	})
	require.ErrorIs(t, err, domain.ErrInvalidInput)
	assert.Nil(t, listed(t, k, k.msgID), "the deleted message does not come back")
}

// A manager deleting someone else's message leaves a line in the conversation
// saying who removed whose message; a sender deleting their own does not.
func TestDeleteMessage_AManagersDeletionIsRecordedInTheChannel(t *testing.T) {
	k := newThreadTalk(t)
	ctx := context.Background()
	k.f.read.allow[grant{userNode, k.oaID, ngac.OpManage}] = true
	count := func() int {
		var n int
		require.NoError(t, k.f.pool.QueryRow(ctx,
			`SELECT count(*) FROM messages WHERE channel_id = $1 AND message_type = 'system' AND linked_entity_type = 'message_deleted'`,
			k.chID).Scan(&n))
		return n
	}
	_, err := k.f.svc.DeleteMessage(ctx, k.replyID, k.f.userID, userNode)
	require.NoError(t, err)
	assert.Equal(t, 1, count())

	own, err := k.f.svc.SendMessage(ctx, domain.SendMessageInput{ChannelID: k.chID, SenderID: k.f.userID, SenderNodeID: userNode, Content: "x"})
	require.NoError(t, err)
	_, err = k.f.svc.DeleteMessage(ctx, own.Id, k.f.userID, userNode)
	require.NoError(t, err)
	assert.Equal(t, 1, count(), "deleting one's own message is not logged")
}

func TestSearchMessages_SkipsDeletedMessages(t *testing.T) {
	k := newTalk(t, "search-deleted")
	ctx := context.Background()
	found, err := k.f.svc.SearchMessages(ctx, k.chID, userNode, "needle", 10)
	require.NoError(t, err)
	require.Len(t, found.Messages, 1)
	_, err = k.f.svc.DeleteMessage(ctx, k.msgID, k.f.userID, userNode)
	require.NoError(t, err)
	found, err = k.f.svc.SearchMessages(ctx, k.chID, userNode, "needle", 10)
	require.NoError(t, err)
	assert.Empty(t, found.Messages)
}
