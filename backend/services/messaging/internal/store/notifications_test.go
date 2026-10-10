package store_test

import (
	"context"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"ngac-platform/services/messaging/internal/store"
	"ngac-platform/testutil"
)

// join makes userID a member of the workspace.
func join(t *testing.T, pool *pgxpool.Pool, wsID, userID, nodeID string) {
	t.Helper()
	_, err := pool.Exec(context.Background(),
		`INSERT INTO tenant_users (tenant_id, user_id, role, status, ngac_node_id) VALUES ($1, $2, 'member', 'active', $3)`,
		wsID, userID, nodeID)
	require.NoError(t, err)
}

func TestNotifications_RoundTripAndOwnership(t *testing.T) {
	pool := testutil.SetupTestDB(t)
	st := store.NewStore(pool)
	ctx := context.Background()
	alice, _ := testutil.CreateUser(t, pool)
	bob, _ := testutil.CreateUser(t, pool)
	ws, _ := testutil.CreateWorkspace(t, pool, alice)

	base := time.Now().Add(-time.Hour)
	for i, n := range []*store.Notification{
		{ID: "n-" + alice + "-1", UserID: alice, WorkspaceID: ws, Type: "asset_assigned", TargetType: "asset", TargetID: "a-0", TargetName: "first", CreatedAt: base},
		{ID: "n-" + alice + "-2", UserID: alice, WorkspaceID: ws, Type: "approval_rejected", ActorUserID: bob, ActorName: "Bob",
			TargetType: "approval", TargetID: "r-1", TargetName: "Tạm ứng", Params: map[string]string{"reason": "no budget"}, CreatedAt: base.Add(time.Minute)},
		{ID: "n-" + bob + "-1", UserID: bob, WorkspaceID: ws, Type: "asset_assigned", TargetName: "bobs", CreatedAt: base},
	} {
		require.NoError(t, st.InsertNotification(ctx, n), "insert %d", i)
	}

	list, err := st.ListNotifications(ctx, alice, ws, 10, 0)
	require.NoError(t, err)
	require.Len(t, list, 2)
	assert.Equal(t, "Tạm ứng", list[0].TargetName, "newest first")
	assert.Equal(t, bob, list[0].ActorUserID)
	assert.Equal(t, "Bob", list[0].ActorName)
	assert.Equal(t, "approval", list[0].TargetType)
	assert.Equal(t, map[string]string{"reason": "no budget"}, list[0].Params)
	assert.Equal(t, "", list[1].ActorUserID, "NULL reads back as empty")
	assert.Empty(t, list[1].Params, "no params read back as an empty map")

	page, err := st.ListNotifications(ctx, alice, ws, 1, 1)
	require.NoError(t, err)
	require.Len(t, page, 1)
	assert.Equal(t, "first", page[0].TargetName)

	total, unread, err := st.NotificationCounts(ctx, alice, ws)
	require.NoError(t, err)
	assert.Equal(t, 2, total)
	assert.Equal(t, 2, unread)

	// Bob cannot mark Alice's notification read.
	require.NoError(t, st.MarkNotificationRead(ctx, "n-"+alice+"-1", bob, ws))
	_, unread, err = st.NotificationCounts(ctx, alice, ws)
	require.NoError(t, err)
	assert.Equal(t, 2, unread, "another user's mark changes nothing")

	require.NoError(t, st.MarkNotificationRead(ctx, "n-"+alice+"-1", alice, ws))
	_, unread, _ = st.NotificationCounts(ctx, alice, ws)
	assert.Equal(t, 1, unread)

	require.NoError(t, st.MarkAllNotificationsRead(ctx, alice, ws))
	_, unread, _ = st.NotificationCounts(ctx, alice, ws)
	assert.Equal(t, 0, unread)
	_, bobUnread, _ := st.NotificationCounts(ctx, bob, ws)
	assert.Equal(t, 1, bobUnread, "marking all read is the caller's own")
}

// A person in two workspaces sees, counts and marks only the one they are in.
func TestNotifications_AreScopedToTheWorkspace(t *testing.T) {
	pool := testutil.SetupTestDB(t)
	st := store.NewStore(pool)
	ctx := context.Background()
	alice, _ := testutil.CreateUser(t, pool)
	wsA, _ := testutil.CreateWorkspace(t, pool, alice)
	wsB, _ := testutil.CreateWorkspace(t, pool, alice)

	now := time.Now()
	require.NoError(t, st.InsertNotification(ctx, &store.Notification{ID: "a-" + alice, UserID: alice, WorkspaceID: wsA, Type: "asset_assigned", TargetName: "in A", CreatedAt: now}))
	require.NoError(t, st.InsertNotification(ctx, &store.Notification{ID: "b-" + alice, UserID: alice, WorkspaceID: wsB, Type: "asset_assigned", TargetName: "in B", CreatedAt: now}))
	// A row from before notifications had a workspace.
	_, err := pool.Exec(ctx, `INSERT INTO notifications (id, user_id, type) VALUES ($1, $2, 'asset_assigned')`, "legacy-"+alice, alice)
	require.NoError(t, err)

	inA, err := st.ListNotifications(ctx, alice, wsA, 10, 0)
	require.NoError(t, err)
	require.Len(t, inA, 1)
	assert.Equal(t, "in A", inA[0].TargetName)
	total, unread, _ := st.NotificationCounts(ctx, alice, wsA)
	assert.Equal(t, 1, total)
	assert.Equal(t, 1, unread, "the other workspace and the legacy row are not counted")

	// Marking through the wrong workspace matches nothing.
	require.NoError(t, st.MarkNotificationRead(ctx, "b-"+alice, alice, wsA))
	_, unreadB, _ := st.NotificationCounts(ctx, alice, wsB)
	assert.Equal(t, 1, unreadB, "a notification of workspace B is not marked from workspace A")

	require.NoError(t, st.MarkAllNotificationsRead(ctx, alice, wsA))
	_, unreadA, _ := st.NotificationCounts(ctx, alice, wsA)
	_, unreadB, _ = st.NotificationCounts(ctx, alice, wsB)
	assert.Equal(t, 0, unreadA)
	assert.Equal(t, 1, unreadB, "mark-all stays in its workspace")

	var legacyRead bool
	require.NoError(t, pool.QueryRow(ctx, `SELECT read FROM notifications WHERE id = $1`, "legacy-"+alice).Scan(&legacyRead))
	assert.False(t, legacyRead, "mark-all never touches the legacy row")
}

func TestInsertNotification_RequiresAWorkspace(t *testing.T) {
	pool := testutil.SetupTestDB(t)
	st := store.NewStore(pool)
	alice, _ := testutil.CreateUser(t, pool)
	err := st.InsertNotification(context.Background(), &store.Notification{ID: "x-" + alice, UserID: alice, Type: "asset_assigned", CreatedAt: time.Now()})
	require.Error(t, err)
}

func TestFindMember_ResolvesByUserOrNodeWithinTheWorkspaceOnly(t *testing.T) {
	pool := testutil.SetupTestDB(t)
	st := store.NewStore(pool)
	ctx := context.Background()
	owner, _ := testutil.CreateUser(t, pool)
	member, memberNode := testutil.CreateUser(t, pool)
	outsider, outsiderNode := testutil.CreateUser(t, pool)
	ws, _ := testutil.CreateWorkspace(t, pool, owner)
	join(t, pool, ws, member, memberNode)
	_, err := pool.Exec(ctx, `UPDATE users SET display_name = 'Lê Quang Vinh' WHERE id = $1`, member)
	require.NoError(t, err)

	for _, key := range []string{member, memberNode} {
		m, ok, err := st.FindMember(ctx, ws, key)
		require.NoError(t, err)
		require.True(t, ok, key)
		assert.Equal(t, member, m.UserID)
		assert.Equal(t, "Lê Quang Vinh", m.Name)
	}

	// Without a display name the login names them, never their id.
	_, err = pool.Exec(ctx, `UPDATE users SET display_name = '' WHERE id = $1`, member)
	require.NoError(t, err)
	m, ok, err := st.FindMember(ctx, ws, member)
	require.NoError(t, err)
	require.True(t, ok)
	assert.Equal(t, member, m.Name, "the login (which the fixture sets to the id) is the fallback, not an id column")

	for _, key := range []string{outsider, outsiderNode, "no-such-key", ""} {
		_, ok, err := st.FindMember(ctx, ws, key)
		require.NoError(t, err)
		assert.False(t, ok, "%q is not a member of this workspace", key)
	}
	_, ok, err = st.FindMember(ctx, "", member)
	require.NoError(t, err)
	assert.False(t, ok, "no workspace, no member")
}
