package domain_test

import (
	"context"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"ngac-platform/ngac"
	pb "ngac-platform/proto/messaging"
	"ngac-platform/services/messaging/internal/domain"
	"ngac-platform/testutil"
)

func activityOf(t *testing.T, k *talk, node string) map[string]*pb.ConversationActivity {
	t.Helper()
	list, err := k.f.svc.ConversationActivity(context.Background(), k.f.userID, node, k.f.wsID)
	require.NoError(t, err)
	out := map[string]*pb.ConversationActivity{}
	for _, a := range list {
		out[a.ChannelId] = a
	}
	return out
}

func TestConversationActivity_LatestMessageAndNewerReply(t *testing.T) {
	k := newThreadTalk(t) // "hello needle" by the caller, then a reply by someone else
	got := activityOf(t, k.talk, userNode)[k.chID]
	require.NotNil(t, got)
	assert.Equal(t, k.msgID, got.Latest.Id)
	assert.Equal(t, "hello needle", got.Latest.Preview)
	assert.Equal(t, "text", got.Latest.Kind)
	assert.Equal(t, k.f.userID, got.Latest.SenderId)
	assert.NotEmpty(t, got.Latest.SenderName)
	assert.NotNil(t, got.Latest.CreatedAt)
	assert.Empty(t, got.Latest.ParentMessageId)
	require.NotNil(t, got.LatestReply, "the reply is newer than the main message")
	assert.Equal(t, k.replyID, got.LatestReply.Id)
	assert.Equal(t, "a reply", got.LatestReply.Preview)
	assert.Equal(t, k.rootID, got.LatestReply.ParentMessageId)

	// A newer main message makes the reply old news.
	k.send(t, k.f.userID, userNode, "newer main", "")
	got = activityOf(t, k.talk, userNode)[k.chID]
	assert.Equal(t, "newer main", got.Latest.Preview)
	assert.Nil(t, got.LatestReply, "a reply older than the latest message is left out")
}

func TestConversationActivity_PreviewKindAndMentions(t *testing.T) {
	k, other := mentionTalk(t)
	ctx := context.Background()
	svc := k.f.svc

	html, err := svc.SendMessage(ctx, domain.SendMessageInput{
		ChannelID: k.chID, SenderID: k.f.userID, SenderNodeID: userNode,
		Content: "<p>xem <strong>kế hoạch</strong></p>", ContentFormat: "html", Mentions: []string{other},
	})
	require.NoError(t, err)
	got := activityOf(t, k, userNode)[k.chID]
	assert.Equal(t, html.Id, got.Latest.Id)
	assert.Equal(t, "xem kế hoạch", got.Latest.Preview, "markup never reaches a preview")
	assert.Equal(t, []string{other}, got.Latest.Mentions)

	k.send(t, k.f.userID, userNode, strings.Repeat("từ ", 100), "")
	got = activityOf(t, k, userNode)[k.chID]
	assert.LessOrEqual(t, len([]rune(got.Latest.Preview)), 140)

	for kind, typ := range map[string]string{"poll": "poll", "task": "task", "card": "document"} {
		_, err := svc.SendMessage(ctx, domain.SendMessageInput{
			ChannelID: k.chID, SenderID: k.f.userID, SenderNodeID: userNode,
			Content: "about " + kind, LinkedEntityType: typ, LinkedEntityID: "obj-" + kind,
		})
		require.NoError(t, err, kind)
		got = activityOf(t, k, userNode)[k.chID]
		assert.Equal(t, kind, got.Latest.Kind, typ)
	}
}

func TestConversationActivity_AttachmentNamesItsFirstFile(t *testing.T) {
	k := newTalk(t, "activity-attach")
	ctx := context.Background()
	root := k.driveItem(t, "folder", "", "channel", k.chID, "active", "", true)
	a := k.driveItem(t, "file", root, "channel", k.chID, "active", "application/pdf", false)
	b := k.driveItem(t, "file", root, "channel", k.chID, "active", "image/png", false)
	t.Cleanup(func() { k.f.pool.Exec(ctx, `DELETE FROM drive_items WHERE parent_id = $1`, root) })
	_, err := k.f.svc.SendMessage(ctx, domain.SendMessageInput{
		ChannelID: k.chID, SenderID: k.f.userID, SenderNodeID: userNode, Attachments: attach(a, b),
	})
	require.NoError(t, err)

	got := activityOf(t, k, userNode)[k.chID]
	assert.Equal(t, "attachment", got.Latest.Kind)
	assert.Equal(t, "file-"+a[:8], got.Latest.AttachmentName)
}

func TestConversationActivity_OmitsDeletedMessagesAndEmptyChannels(t *testing.T) {
	k := newTalk(t, "activity-deleted")
	ctx := context.Background()
	empty, emptyOA := k.f.insertChannel(t, "activity-empty")
	k.f.read.allow[grant{userNode, emptyOA, ngac.OpRead}] = true

	m := k.send(t, k.f.userID, userNode, "to be removed", "")
	require.Equal(t, m.Id, activityOf(t, k, userNode)[k.chID].Latest.Id)
	_, err := k.f.svc.DeleteMessage(ctx, m.Id, k.f.userID, userNode)
	require.NoError(t, err)

	all := activityOf(t, k, userNode)
	assert.Equal(t, k.msgID, all[k.chID].Latest.Id, "the deleted message is skipped")
	assert.NotContains(t, all, empty, "a conversation with no message has no entry")
}

// Deny: the preview carries message text, so it follows read access exactly —
// no member, a removed member and another person's DM show nothing.
func TestConversationActivity_FollowsReadAccess(t *testing.T) {
	k := newTalk(t, "activity-deny")
	ctx := context.Background()

	// Not a member / no read: nothing named.
	assert.Empty(t, activityOf(t, k, "ngac-outsider"))

	// A member who is then removed loses the conversation from the list.
	member, memberNode := testutil.CreateUser(t, k.f.pool)
	k.f.read.allow[grant{memberNode, k.oaID, ngac.OpRead}] = true
	list, err := k.f.svc.ConversationActivity(ctx, member, memberNode, k.f.wsID)
	require.NoError(t, err)
	require.Len(t, list, 1)
	assert.Equal(t, k.chID, list[0].ChannelId)
	delete(k.f.read.allow, grant{memberNode, k.oaID, ngac.OpRead})
	list, err = k.f.svc.ConversationActivity(ctx, member, memberNode, k.f.wsID)
	require.NoError(t, err)
	assert.Empty(t, list, "a removed member sees no preview of the group")

	// Someone else's DM never shows, however busy it is.
	dmID, dmOA := k.f.insertChannel(t, "dm-of-others")
	_, err = k.f.pool.Exec(ctx, `UPDATE channels SET channel_type = 'dm' WHERE id = $1`, dmID)
	require.NoError(t, err)
	_, err = k.f.pool.Exec(ctx, `INSERT INTO messages (id, channel_id, sender_id, content, message_type, created_at)
		VALUES ($1, $2, $3, 'private words', 'user', now())`, "dm-msg-"+dmID, dmID, k.f.userID)
	require.NoError(t, err)
	t.Cleanup(func() { k.f.pool.Exec(ctx, `DELETE FROM messages WHERE channel_id = $1`, dmID) })
	assert.NotContains(t, activityOf(t, k, userNode), dmID, "a DM the caller has no read on is never named")
	k.f.read.allow[grant{userNode, dmOA, ngac.OpRead}] = true
	assert.NotContains(t, activityOf(t, k, userNode), dmID, "a DM the caller is not a member of is not even a candidate, whatever the policy says")
	addDMMember(t, k.f, dmID, userNode)
	assert.Contains(t, activityOf(t, k, userNode), dmID, "the same DM appears once the caller is in it and may read it")

	// The policy service unreachable: fail closed, nothing named.
	k.f.read.err = assert.AnError
	assert.Empty(t, activityOf(t, k, userNode))
}

func TestConversationActivity_RejectsAnUnknownCaller(t *testing.T) {
	k := newTalk(t, "activity-input")
	_, err := k.f.svc.ConversationActivity(context.Background(), "", userNode, k.f.wsID)
	require.ErrorIs(t, err, domain.ErrInvalidInput)
	_, err = k.f.svc.ConversationActivity(context.Background(), k.f.userID, "", k.f.wsID)
	require.ErrorIs(t, err, domain.ErrInvalidInput)
}

func TestFollowedThreads_AcrossConversationsAndFollowsAccess(t *testing.T) {
	k := newThreadTalk(t) // the caller started a thread: followed implicitly
	ctx := context.Background()
	svc := k.f.svc

	list, err := svc.FollowedThreads(ctx, k.f.userID, userNode, k.f.wsID, "", "", 10)
	require.NoError(t, err)
	require.Len(t, list.Messages, 1)
	assert.Equal(t, k.rootID, list.Messages[0].Id)
	assert.True(t, list.Messages[0].Following)
	assert.False(t, list.HasMore)

	// Read threads are still followed, and a muted conversation's too.
	_, err = svc.MarkThreadRead(ctx, k.rootID, k.f.userID, userNode)
	require.NoError(t, err)
	require.NoError(t, svc.SetNotificationLevel(ctx, k.chID, k.f.userID, userNode, domain.NotifyOff))
	list, err = svc.FollowedThreads(ctx, k.f.userID, userNode, k.f.wsID, "", "", 10)
	require.NoError(t, err)
	assert.Len(t, list.Messages, 1)

	// Unfollowed: gone.
	_, err = svc.UnfollowThread(ctx, k.rootID, k.f.userID, userNode)
	require.NoError(t, err)
	list, err = svc.FollowedThreads(ctx, k.f.userID, userNode, k.f.wsID, "", "", 10)
	require.NoError(t, err)
	assert.Empty(t, list.Messages)
	_, err = svc.FollowThread(ctx, k.rootID, k.f.userID, userNode)
	require.NoError(t, err)

	// Deny: someone who cannot read the conversation sees none of its threads,
	// even following the thread; so does a caller whose read was withdrawn.
	_, err = svc.FollowThread(ctx, k.rootID, k.replier, replierNode)
	require.NoError(t, err)
	other, err := svc.FollowedThreads(ctx, k.replier, "ngac-no-read", k.f.wsID, "", "", 10)
	require.NoError(t, err)
	assert.Empty(t, other.Messages)
	delete(k.f.read.allow, grant{replierNode, k.oaID, ngac.OpRead})
	removed, err := svc.FollowedThreads(ctx, k.replier, replierNode, k.f.wsID, "", "", 10)
	require.NoError(t, err)
	assert.Empty(t, removed.Messages, "a removed member's followed threads stay hidden")

	k.f.read.err = assert.AnError
	failed, err := svc.FollowedThreads(ctx, k.f.userID, userNode, k.f.wsID, "", "", 10)
	require.NoError(t, err)
	assert.Empty(t, failed.Messages, "an unreachable policy service names nothing")
}

func TestFollowedThreads_RejectsBadInput(t *testing.T) {
	k := newThreadTalk(t)
	ctx := context.Background()
	_, err := k.f.svc.FollowedThreads(ctx, "", userNode, k.f.wsID, "", "", 10)
	require.ErrorIs(t, err, domain.ErrInvalidInput)
	_, err = k.f.svc.FollowedThreads(ctx, k.f.userID, userNode, k.f.wsID, "yesterday", "", 10)
	require.ErrorIs(t, err, domain.ErrInvalidInput)
}

// addDMMember records the caller in a DM's member cache, which is what narrows
// the DMs a batch read has to put to the policy service.
func addDMMember(t *testing.T, f *fixture, dmID, node string) {
	t.Helper()
	_, err := f.pool.Exec(context.Background(),
		`INSERT INTO channel_members (channel_id, ngac_node_id) VALUES ($1, $2) ON CONFLICT DO NOTHING`, dmID, node)
	require.NoError(t, err)
}
