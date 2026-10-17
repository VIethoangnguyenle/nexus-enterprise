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

func versionsOf(t *testing.T, v *memVersions, tenant, user string) []int64 {
	t.Helper()
	got, err := v.Get(context.Background(), domain.ChatVersionKeys(tenant, user)...)
	require.NoError(t, err)
	return got
}

func TestChatState_MessageWritesMoveTheTenantVersion(t *testing.T) {
	k := newTalk(t, "etag-writes")
	v := &memVersions{}
	k.f.svc.SetVersions(v)
	ctx := context.Background()
	start := versionsOf(t, v, k.f.wsID, "someone")[0]

	msg, err := k.f.svc.SendMessage(ctx, domain.SendMessageInput{
		ChannelID: k.chID, SenderID: k.f.userID, SenderNodeID: userNode, Content: "second",
	})
	require.NoError(t, err)
	afterSend := versionsOf(t, v, k.f.wsID, "someone")[0]
	assert.Greater(t, afterSend, start, "a sent message changes everybody's previews and unread")

	_, err = k.f.svc.EditMessage(ctx, msg.Id, k.f.userID, userNode, "second, edited", "markdown", nil)
	require.NoError(t, err)
	afterEdit := versionsOf(t, v, k.f.wsID, "someone")[0]
	assert.Greater(t, afterEdit, afterSend, "an edit changes the preview")

	_, err = k.f.svc.DeleteMessage(ctx, msg.Id, k.f.userID, userNode)
	require.NoError(t, err)
	assert.Greater(t, versionsOf(t, v, k.f.wsID, "someone")[0], afterEdit, "a delete changes the preview")
}

func TestChatState_RefusedMessageMovesNothing(t *testing.T) {
	k := newTalk(t, "etag-refused")
	v := &memVersions{}
	k.f.svc.SetVersions(v)

	_, err := k.f.svc.SendMessage(context.Background(), domain.SendMessageInput{
		ChannelID: k.chID, SenderID: "x", SenderNodeID: outsider, Content: "let me in",
	})
	require.ErrorIs(t, err, domain.ErrAccessDenied)
	assert.Equal(t, []int64{0, 0}, versionsOf(t, v, k.f.wsID, "x"))
}

func TestChatState_DirectMessageMovesTheOtherPersonsVersionInAnyWorkspace(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	other, otherNode := testutil.CreateUser(t, f.pool)
	dm, oaID := f.insertChannel(t, "dm-etag")
	_, err := f.pool.Exec(ctx, `UPDATE channels SET channel_type = 'dm', workspace_id = NULL WHERE id = $1`, dm)
	require.NoError(t, err)
	_, err = f.pool.Exec(ctx, `INSERT INTO channel_members (channel_id, ngac_node_id) VALUES ($1, $2)`, dm, otherNode)
	require.NoError(t, err)
	t.Cleanup(func() {
		f.pool.Exec(ctx, `DELETE FROM messages WHERE channel_id = $1`, dm)
		f.pool.Exec(ctx, `DELETE FROM channel_members WHERE channel_id = $1`, dm)
	})
	f.read.allow[grant{userNode, oaID, ngac.OpWrite}] = true
	f.read.addMember(f.wsID, userNode, otherNode)
	v := &memVersions{}
	f.svc.SetVersions(v)

	_, err = f.svc.SendMessage(f.callerCtx(), domain.SendMessageInput{
		ChannelID: dm, SenderID: f.userID, SenderNodeID: userNode, Content: "psst",
	})
	require.NoError(t, err)

	// The other person looks from a workspace of their own.
	got := versionsOf(t, v, "their-own-workspace", other)
	assert.Positive(t, got[1], "their own version moves: DMs are not scoped to a workspace")
	assert.Zero(t, versionsOf(t, v, "their-own-workspace", "a-bystander")[1], "nobody else's does")
}
