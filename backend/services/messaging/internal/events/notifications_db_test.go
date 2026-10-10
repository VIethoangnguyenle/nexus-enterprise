package events

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"ngac-platform/services/messaging/internal/domain"
	"ngac-platform/services/messaging/internal/store"
	"ngac-platform/testutil"
)

// These run the consumer against the real notification service and database: an
// event in, the person's list out, as the screen would read it.

type fixture struct {
	pool *pgxpool.Pool
	svc  *domain.NotificationService
	c    *Consumer
}

func newFixture(t *testing.T) fixture {
	t.Helper()
	pool := testutil.SetupTestDB(t)
	svc := domain.NewNotificationService(store.NewStore(pool), nil)
	return fixture{pool: pool, svc: svc, c: &Consumer{notifSv: svc}}
}

func (f fixture) join(t *testing.T, ws, user, node, display string) {
	t.Helper()
	ctx := context.Background()
	_, err := f.pool.Exec(ctx, `INSERT INTO tenant_users (tenant_id, user_id, role, status, ngac_node_id) VALUES ($1, $2, 'member', 'active', $3)`, ws, user, node)
	require.NoError(t, err)
	_, err = f.pool.Exec(ctx, `UPDATE users SET display_name = $2 WHERE id = $1`, user, display)
	require.NoError(t, err)
}

func (f fixture) list(t *testing.T, user, ws string) []*store.Notification {
	t.Helper()
	page, err := f.svc.List(context.Background(), user, ws, 50, 0)
	require.NoError(t, err)
	return page.Items
}

func jsonOf(t *testing.T, v any) []byte {
	t.Helper()
	b, err := json.Marshal(v)
	require.NoError(t, err)
	return b
}

// The requester is addressed by NGAC node (created_by). The notification must
// land in the requester's own list, under their user id, with the decider named.
func TestApprovalDecision_ReachesTheRequestersList(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	requester, requesterNode := testutil.CreateUser(t, f.pool)
	boss, bossNode := testutil.CreateUser(t, f.pool)
	ws, _ := testutil.CreateWorkspace(t, f.pool, requester)
	otherWS, _ := testutil.CreateWorkspace(t, f.pool, requester)
	f.join(t, ws, requester, requesterNode, "Lê Thị Hoa")
	f.join(t, ws, boss, bossNode, "Trần Minh Đức")
	f.join(t, otherWS, requester, requesterNode, "Lê Thị Hoa")

	f.c.handleApprovalEvent(ctx, jsonOf(t, ApprovalEvent{
		RequestID: "req-1", TemplateName: "Tạm ứng", Status: "rejected", Action: "rejected", Comment: "Đã có màn hình dự phòng",
		ActorNodeID: bossNode, CreatedBy: requesterNode, TenantID: ws, WorkspaceID: ws,
	}))

	got := f.list(t, requester, ws)
	require.Len(t, got, 1, "the requester receives it")
	n := got[0]
	assert.Equal(t, requester, n.UserID, "stored under the user id the list reads by, not the node id")
	assert.Equal(t, "approval_rejected", n.Type)
	assert.Equal(t, boss, n.ActorUserID)
	assert.Equal(t, "Trần Minh Đức", n.ActorName)
	assert.Equal(t, "Tạm ứng", n.TargetName)
	assert.Equal(t, "approval", n.TargetType)
	assert.Equal(t, "req-1", n.TargetID)
	assert.Equal(t, "Đã có màn hình dự phòng", n.Params["reason"])
	assert.False(t, n.Read)

	assert.Empty(t, f.list(t, requester, otherWS), "not in the requester's other workspace")
	assert.Empty(t, f.list(t, boss, ws), "the decider is not told about their own decision")
}

func TestApprovalDecision_StepAndCompletionAreDifferentNotifications(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	requester, requesterNode := testutil.CreateUser(t, f.pool)
	boss, bossNode := testutil.CreateUser(t, f.pool)
	ws, _ := testutil.CreateWorkspace(t, f.pool, requester)
	f.join(t, ws, requester, requesterNode, "Hoa")
	f.join(t, ws, boss, bossNode, "Đức")

	for _, status := range []string{"pending", "approved"} {
		f.c.handleApprovalEvent(ctx, jsonOf(t, ApprovalEvent{
			RequestID: "req-" + status, TemplateName: "Mua sắm", Status: status, Action: "approved",
			ActorNodeID: bossNode, CreatedBy: requesterNode, TenantID: ws,
		}))
	}
	types := map[string]string{}
	for _, n := range f.list(t, requester, ws) {
		types[n.TargetID] = n.Type
	}
	assert.Equal(t, map[string]string{"req-pending": "approval_step_approved", "req-approved": "approval_approved"}, types)
}

// An approval raised in one workspace is never addressed to a person who only
// belongs to another: the node is looked up inside the event's workspace.
func TestApprovalDecision_NeverReachesSomeoneOutsideItsWorkspace(t *testing.T) {
	f := newFixture(t)
	requester, requesterNode := testutil.CreateUser(t, f.pool)
	_, bossNode := testutil.CreateUser(t, f.pool)
	wsA, _ := testutil.CreateWorkspace(t, f.pool, requester)
	wsB, _ := testutil.CreateWorkspace(t, f.pool, requester)
	f.join(t, wsB, requester, requesterNode, "Hoa") // a member of B only

	f.c.handleApprovalEvent(context.Background(), jsonOf(t, ApprovalEvent{
		RequestID: "req-1", TemplateName: "Tạm ứng", Status: "approved", Action: "approved",
		ActorNodeID: bossNode, CreatedBy: requesterNode, TenantID: wsA,
	}))

	assert.Empty(t, f.list(t, requester, wsA))
	assert.Empty(t, f.list(t, requester, wsB))
}

func TestAssignment_NamesTheGiverAndIsNotSentToThemselves(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	holder, holderNode := testutil.CreateUser(t, f.pool)
	admin, adminNode := testutil.CreateUser(t, f.pool)
	ws, _ := testutil.CreateWorkspace(t, f.pool, admin)
	f.join(t, ws, holder, holderNode, "Phạm Hải Yến")
	f.join(t, ws, admin, adminNode, "Nguyễn Thu Lan")

	f.c.handleAssignmentEvent(ctx, jsonOf(t, AssetAssignmentEvent{
		AssetID: "a1", AssetName: "MacBook Pro 14", Action: "assign", ToUserID: holder, ActorID: admin, TenantID: ws,
	}))
	got := f.list(t, holder, ws)
	require.Len(t, got, 1)
	assert.Equal(t, "asset_assigned", got[0].Type)
	assert.Equal(t, "Nguyễn Thu Lan", got[0].ActorName)
	assert.Equal(t, "MacBook Pro 14", got[0].TargetName)

	// Giving yourself an asset is not news.
	f.c.handleAssignmentEvent(ctx, jsonOf(t, AssetAssignmentEvent{
		AssetID: "a2", AssetName: "Máy in", Action: "assign", ToUserID: admin, ActorID: admin, TenantID: ws,
	}))
	assert.Empty(t, f.list(t, admin, ws))
}

// invite stores a pending invitation of ws to the address and returns its id.
func (f fixture) invite(t *testing.T, ws, inviterNode, email string) string {
	t.Helper()
	var id string
	require.NoError(t, f.pool.QueryRow(context.Background(),
		`INSERT INTO workspace_invitations (workspace_id, email, invited_by, status, expires_at)
		 VALUES ($1, $2, $3, 'pending', NOW() + interval '7 days') RETURNING id`, ws, email, inviterNode).Scan(&id))
	return id
}

func (f fixture) accountWithEmail(t *testing.T, email string, verified bool) string {
	t.Helper()
	user, _ := testutil.CreateUser(t, f.pool)
	q := `UPDATE users SET email = $2 WHERE id = $1`
	if verified {
		q = `UPDATE users SET email = $2, email_verified_at = NOW() WHERE id = $1`
	}
	_, err := f.pool.Exec(context.Background(), q, user, email)
	require.NoError(t, err)
	return user
}

func (f fixture) inviteEvent(t *testing.T, ws string, ids ...string) []byte {
	return jsonOf(t, map[string]any{"domain": "workspace", "kind": "invitation_created", "tenant_id": ws, "workspace_id": ws, "ids": ids})
}

func TestInvitation_NotifiesAnExistingVerifiedAccountInAnyWorkspaceOfTheirs(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	owner, ownerNode := testutil.CreateUser(t, f.pool)
	ws, _ := testutil.CreateWorkspace(t, f.pool, owner)
	home, _ := testutil.CreateWorkspace(t, f.pool, owner)
	f.join(t, ws, owner, ownerNode, "Lê Quang Vinh")
	email := "guest-" + ws + "@example.test"
	guest := f.accountWithEmail(t, "Guest-"+ws+"@Example.test", true)

	id := f.invite(t, ws, ownerNode, email)
	f.c.handleWorkspaceEvent(ctx, f.inviteEvent(t, ws, id))

	// The guest is in no workspace of the invitation's, and has another open:
	// the notification is personal, so it still shows.
	got := f.list(t, guest, home)
	require.Len(t, got, 1)
	n := got[0]
	assert.Equal(t, "workspace_invitation", n.Type)
	assert.Equal(t, id, n.TargetID)
	assert.Equal(t, "Workspace "+ws, n.TargetName)
	assert.Equal(t, "Lê Quang Vinh", n.ActorName)
	assert.Equal(t, owner, n.ActorUserID)
	unread, err := f.svc.UnreadCount(ctx, guest, home)
	require.NoError(t, err)
	assert.Equal(t, 1, unread)

	// Inviting again refreshes the one notice rather than stacking another.
	f.c.handleWorkspaceEvent(ctx, f.inviteEvent(t, ws, id))
	assert.Len(t, f.list(t, guest, home), 1)

	// Answering the invitation reads it, from whichever workspace is open.
	require.NoError(t, f.svc.MarkAboutRead(ctx, guest, home, "workspace_invitation", id))
	unread, _ = f.svc.UnreadCount(ctx, guest, home)
	assert.Equal(t, 0, unread)

	// The owner, who sent it, has nothing.
	assert.Empty(t, f.list(t, owner, ws))
}

func TestInvitation_NoNotificationForAnUnverifiedOrNonexistentAccount(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	owner, ownerNode := testutil.CreateUser(t, f.pool)
	ws, _ := testutil.CreateWorkspace(t, f.pool, owner)
	f.join(t, ws, owner, ownerNode, "Vinh")

	unverified := f.accountWithEmail(t, "unverified-"+ws+"@example.test", false)
	idUnverified := f.invite(t, ws, ownerNode, "unverified-"+ws+"@example.test")
	idNobody := f.invite(t, ws, ownerNode, "nobody-"+ws+"@example.test")
	f.c.handleWorkspaceEvent(ctx, f.inviteEvent(t, ws, idUnverified, idNobody))

	var count int
	require.NoError(t, f.pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM notifications WHERE type = 'workspace_invitation' AND target_id = ANY($1)`,
		[]string{idUnverified, idNobody}).Scan(&count))
	assert.Zero(t, count, "an address with no verified account is told nothing, and nothing records that")
	assert.Empty(t, f.list(t, unverified, ws))
}

// A revoked, answered or expired invitation announces nothing.
func TestInvitation_OnlyAnOpenInvitationNotifies(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	owner, ownerNode := testutil.CreateUser(t, f.pool)
	ws, _ := testutil.CreateWorkspace(t, f.pool, owner)
	f.join(t, ws, owner, ownerNode, "Vinh")
	guest := f.accountWithEmail(t, "late-"+ws+"@example.test", true)
	id := f.invite(t, ws, ownerNode, "late-"+ws+"@example.test")
	_, err := f.pool.Exec(ctx, `UPDATE workspace_invitations SET status = 'revoked' WHERE id = $1`, id)
	require.NoError(t, err)

	f.c.handleWorkspaceEvent(ctx, f.inviteEvent(t, ws, id))
	assert.Empty(t, f.list(t, guest, ws))
}
