package store_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"ngac-platform/services/messaging/internal/store"
	"ngac-platform/testutil"
)

// wsChannel inserts a channel row (with its nodes) for the workspace and returns it.
func wsChannel(t *testing.T, pool *pgxpool.Pool, st *store.Store, wsID, creator, name, chType string, at time.Time, members ...string) *store.Channel {
	t.Helper()
	ctx := context.Background()
	oa, ua := "oa-"+uuid.NewString(), "ua-"+uuid.NewString()
	for _, n := range [][2]string{{oa, "OA"}, {ua, "UA"}} {
		_, err := pool.Exec(ctx, `INSERT INTO ngac_nodes (id, name, node_type) VALUES ($1, $1, $2)`, n[0], n[1])
		require.NoError(t, err)
	}
	ch := &store.Channel{ID: "ch-" + uuid.NewString(), Name: name, ChannelType: chType, WorkspaceID: wsID, NGACOaID: oa, NGACUaID: ua, CreatedBy: creator, CreatedAt: at}
	t.Cleanup(func() {
		pool.Exec(ctx, `DELETE FROM channels WHERE id = $1`, ch.ID)
		pool.Exec(ctx, `DELETE FROM ngac_nodes WHERE id IN ($1, $2)`, oa, ua)
	})
	require.NoError(t, st.InsertChannelWithMembers(ctx, ch, members...))
	if !at.IsZero() {
		_, err := pool.Exec(ctx, `UPDATE channels SET created_at = $2 WHERE id = $1`, ch.ID, at)
		require.NoError(t, err)
	}
	return ch
}

// Before migration 060 the workspace-wide channel is the owner's channel named general; a group of the
// same name made by someone else, or a differently named one of the owner's, is
// not it. Both halves run on one workspace.
func TestWorkspaceWideChannel_BeforeTheIndexIsTheOwnersGeneral(t *testing.T) {
	pool := testutil.SetupTestDB(t)
	st := store.NewStore(pool)
	st.ForceWideIndex(false) // migration 060 not applied: the name and creator say which channel it is
	ctx := context.Background()
	owner, _ := testutil.CreateUser(t, pool)
	other, _ := testutil.CreateUser(t, pool)
	wsID, _ := testutil.CreateWorkspace(t, pool, owner)

	got, err := st.WorkspaceWideChannel(ctx, wsID)
	require.NoError(t, err)
	assert.Nil(t, got, "a workspace with no channel has none")

	now := time.Now()
	wsChannel(t, pool, st, wsID, other, "general", "private", now.Add(-3*time.Hour))
	wsChannel(t, pool, st, wsID, owner, "design", "private", now.Add(-2*time.Hour))
	got, err = st.WorkspaceWideChannel(ctx, wsID)
	require.NoError(t, err)
	assert.Nil(t, got, "neither a stranger's general nor a private channel is it")

	general := wsChannel(t, pool, st, wsID, owner, "general", "workspace", now.Add(-time.Hour))
	got, err = st.WorkspaceWideChannel(ctx, wsID)
	require.NoError(t, err)
	require.NotNil(t, got)
	assert.Equal(t, general.ID, got.ID)
	assert.Equal(t, general.NGACUaID, got.NGACUaID)

	exists, err := st.HasWorkspaceTypeChannel(ctx, wsID)
	require.NoError(t, err)
	assert.True(t, exists)
	otherWS, _ := testutil.CreateWorkspace(t, pool, owner)
	exists, err = st.HasWorkspaceTypeChannel(ctx, otherWS)
	require.NoError(t, err)
	assert.False(t, exists, "another workspace is not affected")
}

// A second workspace-wide channel is refused by the database whoever races to
// insert it, and the refusal is the store's own error, not a driver one.
func TestInsertChannel_SecondWorkspaceWideChannelIsRefused(t *testing.T) {
	pool := testutil.SetupTestDB(t)
	st := store.NewStore(pool)
	ctx := context.Background()
	owner, _ := testutil.CreateUser(t, pool)
	wsID, _ := testutil.CreateWorkspace(t, pool, owner)
	other, _ := testutil.CreateWorkspace(t, pool, owner)

	wsChannel(t, pool, st, wsID, owner, "general", "workspace", time.Time{})

	// Another workspace may have its own; a private channel is unaffected.
	wsChannel(t, pool, st, other, owner, "general", "workspace", time.Time{})
	wsChannel(t, pool, st, wsID, owner, "design", "private", time.Time{})

	oa, ua := "oa-"+uuid.NewString(), "ua-"+uuid.NewString()
	for _, n := range [][2]string{{oa, "OA"}, {ua, "UA"}} {
		_, err := pool.Exec(ctx, `INSERT INTO ngac_nodes (id, name, node_type) VALUES ($1, $1, $2)`, n[0], n[1])
		require.NoError(t, err)
	}
	t.Cleanup(func() { pool.Exec(ctx, `DELETE FROM ngac_nodes WHERE id IN ($1, $2)`, oa, ua) })
	second := &store.Channel{ID: "ch-" + uuid.NewString(), Name: "general-2", ChannelType: "workspace", WorkspaceID: wsID, NGACOaID: oa, NGACUaID: ua, CreatedBy: owner}

	t.Cleanup(func() { pool.Exec(ctx, `DELETE FROM channels WHERE id = $1`, second.ID) })

	err := st.InsertChannelWithMembers(ctx, second, "node-x")

	require.ErrorIs(t, err, store.ErrWorkspaceChannelExists)
	var rows int
	require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM channel_members WHERE channel_id = $1`, second.ID).Scan(&rows))
	assert.Equal(t, 0, rows, "nothing of the refused channel remains")
}

// Leaving a workspace drops the person's cache rows for every channel of that
// workspace, and only theirs, and only that workspace's.
func TestDeleteChannelMembersInWorkspace_DropsOnlyThatPersonInThatWorkspace(t *testing.T) {
	pool := testutil.SetupTestDB(t)
	st := store.NewStore(pool)
	ctx := context.Background()
	owner, _ := testutil.CreateUser(t, pool)
	wsA, _ := testutil.CreateWorkspace(t, pool, owner)
	wsB, _ := testutil.CreateWorkspace(t, pool, owner)

	general := wsChannel(t, pool, st, wsA, owner, "general", "workspace", time.Time{}, "leaver", "stayer")
	group := wsChannel(t, pool, st, wsA, owner, "design", "private", time.Time{}, "leaver")
	elsewhere := wsChannel(t, pool, st, wsB, owner, "design", "private", time.Time{}, "leaver")
	dm := wsChannel(t, pool, st, "", owner, "dm", "dm", time.Time{}, "leaver", "stayer")

	n, err := st.DeleteChannelMembersInWorkspace(ctx, wsA, "leaver")

	require.NoError(t, err)
	assert.EqualValues(t, 2, n)
	has := func(ch *store.Channel, node string) bool {
		var c int
		require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM channel_members WHERE channel_id = $1 AND ngac_node_id = $2`, ch.ID, node).Scan(&c))
		return c == 1
	}
	assert.False(t, has(general, "leaver"))
	assert.False(t, has(group, "leaver"))
	assert.True(t, has(general, "stayer"), "another person keeps theirs")
	assert.True(t, has(elsewhere, "leaver"), "another workspace is untouched")
	assert.True(t, has(dm, "leaver"), "a DM belongs to no workspace")
}

// Once migration 060 has enforced one workspace-wide channel per workspace, the type
// alone names it: a #general renamed afterwards is still found, and a private channel
// that happens to be called general is not.
func TestWorkspaceWideChannel_AfterTheIndexIsTheWorkspacesOnlyWorkspaceChannel(t *testing.T) {
	pool := testutil.SetupTestDB(t)
	st := store.NewStore(pool)
	st.ForceWideIndex(true)
	ctx := context.Background()
	owner, _ := testutil.CreateUser(t, pool)
	wsID, _ := testutil.CreateWorkspace(t, pool, owner)

	wsChannel(t, pool, st, wsID, owner, "general", "private", time.Now().Add(-time.Hour))
	got, err := st.WorkspaceWideChannel(ctx, wsID)
	require.NoError(t, err)
	assert.Nil(t, got, "a private channel called general is not the workspace-wide one")

	renamed := wsChannel(t, pool, st, wsID, owner, "Toàn công ty", "workspace", time.Now())
	got, err = st.WorkspaceWideChannel(ctx, wsID)
	require.NoError(t, err)
	require.NotNil(t, got, "the workspace's workspace-typed channel is found by its type, whatever it is now called")
	assert.Equal(t, renamed.ID, got.ID)
}

// Taking someone out of a channel drops their cache row there, and only that one:
// the same person in another channel and another person in this one keep theirs.
// Doing it again is not an error.
func TestDeleteChannelMember_DropsOnlyThatRow(t *testing.T) {
	pool := testutil.SetupTestDB(t)
	st := store.NewStore(pool)
	ctx := context.Background()
	owner, _ := testutil.CreateUser(t, pool)
	wsID, _ := testutil.CreateWorkspace(t, pool, owner)

	group := wsChannel(t, pool, st, wsID, owner, "design", "private", time.Time{}, "leaver", "stayer")
	other := wsChannel(t, pool, st, wsID, owner, "ops", "private", time.Time{}, "leaver")
	has := func(ch *store.Channel, node string) bool {
		var c int
		require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM channel_members WHERE channel_id = $1 AND ngac_node_id = $2`, ch.ID, node).Scan(&c))
		return c == 1
	}

	require.NoError(t, st.DeleteChannelMember(ctx, group.ID, "leaver"))

	assert.False(t, has(group, "leaver"))
	assert.True(t, has(group, "stayer"), "another person keeps theirs")
	assert.True(t, has(other, "leaver"), "another channel keeps theirs")
	require.NoError(t, st.DeleteChannelMember(ctx, group.ID, "leaver"), "again is not an error")
}
