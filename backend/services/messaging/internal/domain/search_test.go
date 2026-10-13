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

func searchIDs(t *testing.T, k *talk, node, q string) []string {
	t.Helper()
	list, err := k.f.svc.SearchAll(context.Background(), k.f.userID, node, k.f.wsID, domain.SearchQuery{Text: q, Limit: 25})
	require.NoError(t, err)
	out := []string{}
	for _, m := range list.Messages {
		out = append(out, m.Id)
	}
	return out
}

func TestSearchAll_FindsMainMessagesAndRepliesWithTheirThread(t *testing.T) {
	k := newThreadTalk(t) // "hello needle", then "a reply" by someone else
	ctx := context.Background()
	reply, err := k.f.svc.SendMessage(ctx, domain.SendMessageInput{
		ChannelID: k.chID, SenderID: k.replier, SenderNodeID: replierNode, Content: "needle again", ParentMessageID: k.rootID,
	})
	require.NoError(t, err)

	list, err := k.f.svc.SearchAll(ctx, k.f.userID, userNode, k.f.wsID, domain.SearchQuery{Text: "needle", Limit: 25})
	require.NoError(t, err)
	require.Len(t, list.Messages, 2)
	assert.Equal(t, reply.Id, list.Messages[0].Id, "newest first")
	assert.Equal(t, k.rootID, list.Messages[0].ParentMessageId, "a reply names its thread")
	assert.Equal(t, k.msgID, list.Messages[1].Id)
	assert.Empty(t, list.Messages[1].ParentMessageId)
	assert.NotEmpty(t, list.Messages[0].SenderName)
	assert.False(t, list.HasMore)
}

func TestSearchAll_ShortOrEmptyTextIsAnEmptyList(t *testing.T) {
	k := newTalk(t, "search-short")
	ctx := context.Background()
	for _, q := range []string{"", " ", "n", "  n  ", "!!"} {
		list, err := k.f.svc.SearchAll(ctx, k.f.userID, userNode, k.f.wsID, domain.SearchQuery{Text: q})
		require.NoError(t, err, "%q", q)
		assert.Empty(t, list.Messages, "%q", q)
	}
	assert.Equal(t, []string{k.msgID}, searchIDs(t, k, userNode, "ne"), "two characters is enough")
}

func TestSearchAll_PagesAndRejectsABadCursor(t *testing.T) {
	k := newTalk(t, "search-page")
	ctx := context.Background()
	for range 2 {
		k.send(t, k.f.userID, userNode, "needle more", "")
	}
	first, err := k.f.svc.SearchAll(ctx, k.f.userID, userNode, k.f.wsID, domain.SearchQuery{Text: "needle", Limit: 2})
	require.NoError(t, err)
	require.Len(t, first.Messages, 2)
	assert.True(t, first.HasMore)
	last := first.Messages[1]
	rest, err := k.f.svc.SearchAll(ctx, k.f.userID, userNode, k.f.wsID, domain.SearchQuery{
		Text: "needle", Limit: 2, Before: last.CreatedAt.AsTime().Format(time.RFC3339Nano), BeforeID: last.Id})
	require.NoError(t, err)
	require.Len(t, rest.Messages, 1)
	assert.False(t, rest.HasMore)
	assert.NotContains(t, []string{first.Messages[0].Id, last.Id}, rest.Messages[0].Id)

	_, err = k.f.svc.SearchAll(ctx, k.f.userID, userNode, k.f.wsID, domain.SearchQuery{Text: "needle", Before: "yesterday"})
	require.ErrorIs(t, err, domain.ErrInvalidInput)
	_, err = k.f.svc.SearchAll(ctx, "", userNode, k.f.wsID, domain.SearchQuery{Text: "needle"})
	require.ErrorIs(t, err, domain.ErrInvalidInput)
	_, err = k.f.svc.SearchAll(ctx, k.f.userID, "", k.f.wsID, domain.SearchQuery{Text: "needle"})
	require.ErrorIs(t, err, domain.ErrInvalidInput)
}

func TestSearchAll_AnUnsizedPageHoldsTwentyFiveHits(t *testing.T) {
	k := newTalk(t, "search-size")
	for range 25 { // with the opening message, 26 hits
		k.send(t, k.f.userID, userNode, "needle more", "")
	}
	list, err := k.f.svc.SearchAll(context.Background(), k.f.userID, userNode, k.f.wsID, domain.SearchQuery{Text: "needle"})
	require.NoError(t, err)
	assert.Len(t, list.Messages, 25)
	assert.True(t, list.HasMore)
}

func TestSearchAll_HasFileKeepsMessagesWithAnAttachment(t *testing.T) {
	k := newTalk(t, "search-file")
	ctx := context.Background()
	root := k.driveItem(t, "folder", "", "channel", k.chID, "active", "", true)
	file := k.driveItem(t, "file", root, "channel", k.chID, "active", "application/pdf", false)
	t.Cleanup(func() { k.f.pool.Exec(ctx, `DELETE FROM drive_items WHERE parent_id = $1`, root) })
	withFile, err := k.f.svc.SendMessage(ctx, domain.SendMessageInput{
		ChannelID: k.chID, SenderID: k.f.userID, SenderNodeID: userNode, Content: "needle attached", Attachments: attach(file),
	})
	require.NoError(t, err)

	list, err := k.f.svc.SearchAll(ctx, k.f.userID, userNode, k.f.wsID, domain.SearchQuery{Text: "needle", HasFile: true})
	require.NoError(t, err)
	require.Len(t, list.Messages, 1)
	assert.Equal(t, withFile.Id, list.Messages[0].Id)
	require.Len(t, list.Messages[0].Attachments, 1, "the hit carries its file")
	assert.Len(t, searchIDs(t, k, userNode, "needle"), 2)
}

// Deny: a hit is message text, so it follows read access exactly. A caller who
// is no member, a member since removed, and a DM between other people never
// get a hit from that conversation, whatever they search for.
func TestSearchAll_NeverReturnsAConversationTheCallerCannotRead(t *testing.T) {
	k := newThreadTalk(t) // the root and the reply both say something findable
	ctx := context.Background()
	require.Len(t, searchIDs(t, k.talk, userNode, "reply"), 1, "control: the caller finds the reply")

	// No member: no read on the channel.
	assert.Empty(t, searchIDs(t, k.talk, "ngac-outsider", "needle"))
	assert.Empty(t, searchIDs(t, k.talk, "ngac-outsider", "reply"))

	// A member who is then removed loses the conversation's messages.
	member, memberNode := testutil.CreateUser(t, k.f.pool)
	k.f.read.allow[grant{memberNode, k.oaID, ngac.OpRead}] = true
	list, err := k.f.svc.SearchAll(ctx, member, memberNode, k.f.wsID, domain.SearchQuery{Text: "needle"})
	require.NoError(t, err)
	require.Len(t, list.Messages, 1)
	delete(k.f.read.allow, grant{memberNode, k.oaID, ngac.OpRead})
	list, err = k.f.svc.SearchAll(ctx, member, memberNode, k.f.wsID, domain.SearchQuery{Text: "needle"})
	require.NoError(t, err)
	assert.Empty(t, list.Messages, "a removed member finds nothing of the group")
	list, err = k.f.svc.SearchAll(ctx, member, memberNode, k.f.wsID, domain.SearchQuery{Text: "reply"})
	require.NoError(t, err)
	assert.Empty(t, list.Messages, "nor a reply in its threads")

	// Someone else's DM never shows, however exact the words.
	dmID, dmOA := k.f.insertChannel(t, "dm-of-others")
	_, err = k.f.pool.Exec(ctx, `UPDATE channels SET channel_type = 'dm' WHERE id = $1`, dmID)
	require.NoError(t, err)
	_, err = k.f.pool.Exec(ctx, `INSERT INTO messages (id, channel_id, sender_id, content, message_type, created_at)
		VALUES ($1, $2, $3, 'private secretword', 'user', now())`, "dm-msg-"+dmID, dmID, k.f.userID)
	require.NoError(t, err)
	t.Cleanup(func() { k.f.pool.Exec(ctx, `DELETE FROM messages WHERE channel_id = $1`, dmID) })
	assert.Empty(t, searchIDs(t, k.talk, userNode, "secretword"), "a DM the caller has no read on is never searched")
	k.f.read.allow[grant{userNode, dmOA, ngac.OpRead}] = true
	assert.Equal(t, []string{"dm-msg-" + dmID}, searchIDs(t, k.talk, userNode, "secretword"), "the same DM is found once the caller may read it")
	delete(k.f.read.allow, grant{userNode, dmOA, ngac.OpRead})

	// Another workspace's session does not search this workspace's channel.
	other, err := k.f.svc.SearchAll(ctx, k.f.userID, userNode, "some-other-workspace", domain.SearchQuery{Text: "needle"})
	require.NoError(t, err)
	assert.Empty(t, other.Messages)

	// The policy service unreachable: fail closed, nothing found.
	k.f.read.err = assert.AnError
	assert.Empty(t, searchIDs(t, k.talk, userNode, "needle"))
}
