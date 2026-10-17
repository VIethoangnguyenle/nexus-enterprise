package domain_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"ngac-platform/pkg/grpcauth"
	"ngac-platform/services/messaging/internal/domain"
)

// An approval card links a message to an approval request. A personal workspace
// has no approvals, so a message that links one is refused there — before the
// policy is asked and before anything is stored.

func (k *talk) sendApprovalCard(ctx context.Context) error {
	_, err := k.f.svc.SendMessage(ctx, domain.SendMessageInput{
		ChannelID: k.chID, SenderID: k.f.userID, SenderNodeID: userNode, Content: "Đề nghị phê duyệt",
		LinkedEntityType: "approval", LinkedEntityID: "request-1",
	})
	return err
}

func (k *talk) messageCount(t *testing.T) int {
	t.Helper()
	var n int
	require.NoError(t, k.f.pool.QueryRow(context.Background(), `SELECT count(*) FROM messages WHERE channel_id = $1`, k.chID).Scan(&n))
	return n
}

func (k *talk) setWorkspaceType(t *testing.T, kind string) {
	t.Helper()
	_, err := k.f.pool.Exec(context.Background(), `UPDATE workspaces SET type = $1 WHERE id = $2`, kind, k.f.wsID)
	require.NoError(t, err)
}

func TestSendMessage_ApprovalCardRefusedInPersonalWorkspace(t *testing.T) {
	k := newTalk(t, "approval-card-personal")
	k.setWorkspaceType(t, "personal")
	before := k.messageCount(t)

	err := k.sendApprovalCard(context.Background())

	require.ErrorIs(t, err, domain.ErrAccessDenied)
	assert.Contains(t, err.Error(), "personal workspace")
	assert.NotContains(t, err.Error(), k.f.wsID)
	assert.Equal(t, before, k.messageCount(t), "a refused card stores nothing")
}

// The rule is about approval cards alone: everything else a personal workspace
// sends, other cards and plain text included, goes through as before.
func TestSendMessage_OtherMessagesStillSendInPersonalWorkspace(t *testing.T) {
	k := newTalk(t, "approval-card-personal-other")
	k.setWorkspaceType(t, "personal")
	ctx := context.Background()

	for _, linked := range []string{"", "document", "poll", "task"} {
		_, err := k.f.svc.SendMessage(ctx, domain.SendMessageInput{
			ChannelID: k.chID, SenderID: k.f.userID, SenderNodeID: userNode, Content: "hi",
			LinkedEntityType: linked, LinkedEntityID: map[bool]string{true: "x", false: ""}[linked != ""],
		})
		require.NoError(t, err, "linked_entity_type %q", linked)
	}
}

func TestSendMessage_ApprovalCardAllowedInOrganization(t *testing.T) {
	k := newTalk(t, "approval-card-org")
	before := k.messageCount(t)

	require.NoError(t, k.sendApprovalCard(context.Background()))

	assert.Equal(t, before+1, k.messageCount(t))
}

// A direct message has no workspace of its own; the workspace the sender is
// signed in to decides.
func TestSendMessage_ApprovalCardInDMFollowsTheSignedInWorkspace(t *testing.T) {
	k := newTalk(t, "approval-card-dm")
	_, err := k.f.pool.Exec(context.Background(), `UPDATE channels SET workspace_id = NULL WHERE id = $1`, k.chID)
	require.NoError(t, err)
	k.f.withDMPeer(t, k.chID)
	k.setWorkspaceType(t, "personal")
	inWorkspace := grpcauth.WithCaller(context.Background(), grpcauth.Caller{UserID: k.f.userID, NGACNodeID: userNode, TenantID: k.f.wsID})

	require.ErrorIs(t, k.sendApprovalCard(inWorkspace), domain.ErrAccessDenied)
	require.ErrorIs(t, k.sendApprovalCard(context.Background()), domain.ErrAccessDenied, "no workspace to judge by is a refusal, not an allow")

	k.setWorkspaceType(t, "organization")
	require.NoError(t, k.sendApprovalCard(inWorkspace))
}

func TestSendMessage_ApprovalCardFailsClosedOnAnUnknownWorkspace(t *testing.T) {
	k := newTalk(t, "approval-card-unknown")
	_, err := k.f.pool.Exec(context.Background(), `UPDATE channels SET workspace_id = NULL WHERE id = $1`, k.chID)
	require.NoError(t, err)
	k.f.withDMPeer(t, k.chID)
	ctx := grpcauth.WithCaller(context.Background(), grpcauth.Caller{UserID: k.f.userID, NGACNodeID: userNode, TenantID: "no-such-workspace"})

	require.ErrorIs(t, k.sendApprovalCard(ctx), domain.ErrAccessDenied)
}
