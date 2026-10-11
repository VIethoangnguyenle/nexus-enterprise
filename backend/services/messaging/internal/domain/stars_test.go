package domain_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"ngac-platform/ngac"
	pb "ngac-platform/proto/messaging"
	"ngac-platform/services/messaging/internal/domain"
	"ngac-platform/testutil"
)

func starredIn(t *testing.T, k *talk, channelID string) bool {
	t.Helper()
	counts, err := k.f.svc.GetUnreadCounts(context.Background(), k.f.userID, userNode, k.f.wsID)
	require.NoError(t, err)
	for _, c := range counts {
		if c.ChannelId == channelID {
			return c.Starred
		}
	}
	t.Fatalf("channel %s not listed", channelID)
	return false
}

func TestStarConversation_AllowedWithReadAndIdempotent(t *testing.T) {
	k := newTalk(t, "star-conv")
	ctx := context.Background()
	svc := k.f.svc
	t.Cleanup(func() { k.f.pool.Exec(ctx, `DELETE FROM starred_conversations WHERE channel_id = $1`, k.chID) })

	assert.False(t, starredIn(t, k, k.chID))
	require.NoError(t, svc.StarConversation(ctx, k.chID, k.f.userID, userNode))
	require.NoError(t, svc.StarConversation(ctx, k.chID, k.f.userID, userNode), "starring twice is idempotent")
	assert.True(t, starredIn(t, k, k.chID))

	ch, err := svc.GetChannelFor(ctx, k.chID, k.f.userID, userNode)
	require.NoError(t, err)
	assert.True(t, ch.Starred)

	require.NoError(t, svc.UnstarConversation(ctx, k.chID, k.f.userID, userNode))
	require.NoError(t, svc.UnstarConversation(ctx, k.chID, k.f.userID, userNode), "unstarring twice is idempotent")
	assert.False(t, starredIn(t, k, k.chID))
	ch, err = svc.GetChannelFor(ctx, k.chID, k.f.userID, userNode)
	require.NoError(t, err)
	assert.False(t, ch.Starred)
}

func TestStarConversation_DeniedWithoutRead(t *testing.T) {
	k := newTalk(t, "star-conv-deny")
	ctx := context.Background()
	svc := k.f.svc

	require.ErrorIs(t, svc.StarConversation(ctx, k.chID, k.f.userID, outsider), domain.ErrAccessDenied)
	require.ErrorIs(t, svc.UnstarConversation(ctx, k.chID, k.f.userID, outsider), domain.ErrAccessDenied)
	require.ErrorIs(t, svc.StarConversation(ctx, "no-such-channel", k.f.userID, userNode), domain.ErrNotFound)
	require.ErrorIs(t, svc.StarConversation(ctx, "", k.f.userID, userNode), domain.ErrInvalidInput)
	var n int
	require.NoError(t, k.f.pool.QueryRow(ctx, `SELECT count(*) FROM starred_conversations WHERE channel_id = $1`, k.chID).Scan(&n))
	assert.Zero(t, n, "a refused star leaves nothing behind")
}

func TestStarMessage_AllowedDeniedAndDeleted(t *testing.T) {
	k := newTalk(t, "star-msg")
	ctx := context.Background()
	svc := k.f.svc

	require.ErrorIs(t, svc.StarMessage(ctx, k.msgID, k.f.userID, outsider), domain.ErrAccessDenied)
	require.ErrorIs(t, svc.UnstarMessage(ctx, k.msgID, k.f.userID, outsider), domain.ErrAccessDenied)
	require.ErrorIs(t, svc.StarMessage(ctx, "no-such-message", k.f.userID, userNode), domain.ErrNotFound)
	require.ErrorIs(t, svc.StarMessage(ctx, "", k.f.userID, userNode), domain.ErrInvalidInput)

	require.NoError(t, svc.StarMessage(ctx, k.msgID, k.f.userID, userNode))
	require.NoError(t, svc.StarMessage(ctx, k.msgID, k.f.userID, userNode), "starring twice is idempotent")
	list, err := svc.StarredMessages(ctx, k.f.userID, userNode, k.f.wsID, "", "", 10)
	require.NoError(t, err)
	require.Len(t, list.Messages, 1)
	assert.Equal(t, k.msgID, list.Messages[0].Id)
	assert.True(t, list.Messages[0].Starred)
	require.NotNil(t, list.Messages[0].StarredAt)

	require.NoError(t, svc.UnstarMessage(ctx, k.msgID, k.f.userID, userNode))
	require.NoError(t, svc.UnstarMessage(ctx, k.msgID, k.f.userID, userNode), "unstarring twice is idempotent")
	list, err = svc.StarredMessages(ctx, k.f.userID, userNode, k.f.wsID, "", "", 10)
	require.NoError(t, err)
	assert.Empty(t, list.Messages)

	// A deleted message cannot be starred; one starred earlier drops out of the feed.
	require.NoError(t, svc.StarMessage(ctx, k.msgID, k.f.userID, userNode))
	_, err = k.f.pool.Exec(ctx, `UPDATE messages SET deleted_at = now() WHERE id = $1`, k.msgID)
	require.NoError(t, err)
	list, err = svc.StarredMessages(ctx, k.f.userID, userNode, k.f.wsID, "", "", 10)
	require.NoError(t, err)
	assert.Empty(t, list.Messages, "a deleted message is left out")
	require.NoError(t, svc.UnstarMessage(ctx, k.msgID, k.f.userID, userNode))
	require.ErrorIs(t, svc.StarMessage(ctx, k.msgID, k.f.userID, userNode), domain.ErrInvalidInput)
}

// The feed lists newest star first, pages with (starred_at, message id), and
// shows only what the caller can still read, in the workspace they are in.
func TestStarredMessages_PagesAndFollowsAccess(t *testing.T) {
	k := newTalk(t, "star-feed")
	ctx := context.Background()
	svc := k.f.svc
	ids := []string{k.msgID}
	for _, text := range []string{"two", "three", "four"} {
		m, err := svc.SendMessage(ctx, domain.SendMessageInput{ChannelID: k.chID, SenderID: k.f.userID, SenderNodeID: userNode, Content: text})
		require.NoError(t, err)
		ids = append(ids, m.Id)
	}
	for _, id := range ids {
		require.NoError(t, svc.StarMessage(ctx, id, k.f.userID, userNode))
	}
	// ids[1] and ids[2] were starred at the same instant; ids[3] is the newest star.
	_, err := k.f.pool.Exec(ctx, `UPDATE starred_messages SET created_at = '2026-01-01T10:00:00Z' WHERE user_id = $1 AND message_id = ANY($2)`,
		k.f.userID, []string{ids[1], ids[2]})
	require.NoError(t, err)
	_, err = k.f.pool.Exec(ctx, `UPDATE starred_messages SET created_at = '2026-01-01T09:00:00Z' WHERE user_id = $1 AND message_id = $2`, k.f.userID, ids[0])
	require.NoError(t, err)
	_, err = k.f.pool.Exec(ctx, `UPDATE starred_messages SET created_at = '2026-01-01T11:00:00Z' WHERE user_id = $1 AND message_id = $2`, k.f.userID, ids[3])
	require.NoError(t, err)

	// Order: ids[3], then the tie broken by id descending, then ids[0].
	tied := []string{ids[1], ids[2]}
	if tied[0] < tied[1] {
		tied[0], tied[1] = tied[1], tied[0]
	}
	want := []string{ids[3], tied[0], tied[1], ids[0]}

	var got []string
	before, beforeID := "", ""
	for pages := 0; pages < 10; pages++ {
		page, err := svc.StarredMessages(ctx, k.f.userID, userNode, k.f.wsID, before, beforeID, 1)
		require.NoError(t, err)
		require.Len(t, page.Messages, 1)
		last := page.Messages[0]
		got = append(got, last.Id)
		if !page.HasMore {
			break
		}
		before, beforeID = last.StarredAt.AsTime().Format(time.RFC3339Nano), last.Id
	}
	assert.Equal(t, want, got, "a tie on starred_at neither repeats nor skips a message")

	_, err = svc.StarredMessages(ctx, k.f.userID, userNode, k.f.wsID, "yesterday", "", 10)
	require.ErrorIs(t, err, domain.ErrInvalidInput)

	// Another workspace's session does not see this workspace's channel.
	other, err := svc.StarredMessages(ctx, k.f.userID, userNode, "some-other-workspace", "", "", 10)
	require.NoError(t, err)
	assert.Empty(t, other.Messages)

	// A member removed from the conversation no longer sees what they starred.
	delete(k.f.read.allow, grant{userNode, k.oaID, ngac.OpRead})
	gone, err := svc.StarredMessages(ctx, k.f.userID, userNode, k.f.wsID, "", "", 10)
	require.NoError(t, err)
	assert.Empty(t, gone.Messages)
}

// mentionTalk is a talk where a second user can write and the caller can read.
func mentionTalk(t *testing.T) (*talk, string) {
	t.Helper()
	k := newTalk(t, "mentions")
	other, otherNode := testutil.CreateUser(t, k.f.pool)
	k.f.read.allow[grant{otherNode, k.oaID, ngac.OpRead}] = true
	k.f.read.allow[grant{otherNode, k.oaID, ngac.OpWrite}] = true
	k.f.read.allow[grant{replierNode, k.oaID, ngac.OpRead}] = true
	k.f.read.allow[grant{replierNode, k.oaID, ngac.OpWrite}] = true
	ctx := context.Background()
	t.Cleanup(func() {
		k.f.pool.Exec(ctx, `DELETE FROM thread_follows WHERE root_message_id IN (SELECT id FROM messages WHERE channel_id = $1)`, k.chID)
		k.f.pool.Exec(ctx, `DELETE FROM channel_notification_levels WHERE channel_id = $1`, k.chID)
		k.f.pool.Exec(ctx, `DELETE FROM messages WHERE channel_id = $1 AND parent_message_id IS NOT NULL`, k.chID)
		k.f.pool.Exec(ctx, `DELETE FROM messages WHERE channel_id = $1`, k.chID)
	})
	return k, other
}

func (k *talk) send(t *testing.T, from, fromNode, text, parent string, mentions ...string) *pb.Message {
	t.Helper()
	m, err := k.f.svc.SendMessage(context.Background(), domain.SendMessageInput{
		ChannelID: k.chID, SenderID: from, SenderNodeID: fromNode, Content: text, ParentMessageID: parent, Mentions: mentions,
	})
	require.NoError(t, err)
	return m
}

func TestMentions_ListsEveryMessageThatMentionsTheCaller(t *testing.T) {
	k, other := mentionTalk(t)
	ctx := context.Background()
	svc := k.f.svc
	me := k.f.userID
	otherNode := "ngac-other-sender"
	k.f.read.allow[grant{otherNode, k.oaID, ngac.OpWrite}] = true

	main := k.send(t, other, otherNode, "hi @me", "", me)
	k.send(t, other, otherNode, "no mention", "")
	k.send(t, me, userNode, "talking to myself", "", me)              // own message: left out
	root := k.send(t, other, otherNode, "thread root", "")            // no mention
	reply := k.send(t, other, otherNode, "reply to @me", root.Id, me) // a reply counts
	gone := k.send(t, other, otherNode, "deleted mention", "", me)    // deleted: left out
	_, err := k.f.pool.Exec(ctx, `UPDATE messages SET deleted_at = now() WHERE id = $1`, gone.Id)
	require.NoError(t, err)

	list, err := svc.Mentions(ctx, me, userNode, k.f.wsID, "", "", 10)
	require.NoError(t, err)
	var got []string
	for _, m := range list.Messages {
		got = append(got, m.Id)
	}
	assert.Equal(t, []string{reply.Id, main.Id}, got, "newest first; replies included; own, deleted and unmentioning left out")
	assert.False(t, list.HasMore)
	assert.True(t, list.Messages[0].Unread)
	assert.True(t, list.Messages[1].Unread)

	// Deny: without read nothing is listed; a different workspace sees nothing.
	delete(k.f.read.allow, grant{userNode, k.oaID, ngac.OpRead})
	none, err := svc.Mentions(ctx, me, userNode, k.f.wsID, "", "", 10)
	require.NoError(t, err)
	assert.Empty(t, none.Messages, "a member removed from the conversation no longer sees its mentions")
	k.f.read.allow[grant{userNode, k.oaID, ngac.OpRead}] = true
	elsewhere, err := svc.Mentions(ctx, me, userNode, "some-other-workspace", "", "", 10)
	require.NoError(t, err)
	assert.Empty(t, elsewhere.Messages)
}

func TestMentions_UnreadFollowsTheChannelAndThreadReadPositions(t *testing.T) {
	k, other := mentionTalk(t)
	ctx := context.Background()
	svc := k.f.svc
	me := k.f.userID
	otherNode := "ngac-other-sender"
	k.f.read.allow[grant{otherNode, k.oaID, ngac.OpWrite}] = true

	root := k.send(t, other, otherNode, "thread root", "")
	oldMain := k.send(t, other, otherNode, "old @me", "", me)
	oldReply := k.send(t, other, otherNode, "old reply @me", root.Id, me)
	time.Sleep(10 * time.Millisecond)
	require.NoError(t, svc.MarkChannelRead(ctx, me, userNode, k.chID, ""))
	time.Sleep(10 * time.Millisecond)
	newMain := k.send(t, other, otherNode, "new @me", "", me)
	newReply := k.send(t, other, otherNode, "new reply @me", root.Id, me)

	unread := func() map[string]bool {
		list, err := svc.Mentions(ctx, me, userNode, k.f.wsID, "", "", 10)
		require.NoError(t, err)
		out := map[string]bool{}
		for _, m := range list.Messages {
			out[m.Id] = m.Unread
		}
		return out
	}
	// The channel read position covers main messages only: a reply is judged by
	// its thread's position, which is the root's creation until the thread is read.
	got := unread()
	assert.False(t, got[oldMain.Id], "main message before the channel read position")
	assert.True(t, got[newMain.Id], "main message after the channel read position")
	assert.True(t, got[oldReply.Id], "the thread was never read")
	assert.True(t, got[newReply.Id])

	_, err := svc.MarkThreadRead(ctx, root.Id, me, userNode)
	require.NoError(t, err)
	got = unread()
	assert.False(t, got[oldReply.Id])
	assert.False(t, got[newReply.Id])
	assert.True(t, got[newMain.Id])

	// A muted conversation still lists its mentions.
	require.NoError(t, svc.SetNotificationLevel(ctx, k.chID, me, userNode, domain.NotifyOff))
	assert.Len(t, unread(), 4)
}

func TestMentions_PagesWithATieBreak(t *testing.T) {
	k, other := mentionTalk(t)
	ctx := context.Background()
	svc := k.f.svc
	me := k.f.userID
	otherNode := "ngac-other-sender"
	k.f.read.allow[grant{otherNode, k.oaID, ngac.OpWrite}] = true

	var ids []string
	for _, text := range []string{"a", "b", "c"} {
		ids = append(ids, k.send(t, other, otherNode, text, "", me).Id)
	}
	_, err := k.f.pool.Exec(ctx, `UPDATE messages SET created_at = '2026-01-01T10:00:00Z' WHERE id = ANY($1)`, ids)
	require.NoError(t, err)

	var got []string
	before, beforeID := "", ""
	for pages := 0; pages < 10; pages++ {
		page, err := svc.Mentions(ctx, me, userNode, k.f.wsID, before, beforeID, 1)
		require.NoError(t, err)
		require.Len(t, page.Messages, 1)
		last := page.Messages[0]
		got = append(got, last.Id)
		if !page.HasMore {
			break
		}
		before, beforeID = last.CreatedAt.AsTime().Format(time.RFC3339Nano), last.Id
	}
	require.Len(t, got, 3)
	assert.ElementsMatch(t, ids, got, "no message repeated or skipped across the tie")
	assert.IsDecreasing(t, got)

	_, err = svc.Mentions(ctx, me, userNode, k.f.wsID, "yesterday", "", 10)
	require.ErrorIs(t, err, domain.ErrInvalidInput)
	capped, err := svc.Mentions(ctx, me, userNode, k.f.wsID, "", "", 1000)
	require.NoError(t, err)
	assert.Len(t, capped.Messages, 3)
}
