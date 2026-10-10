package store_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"ngac-platform/services/workspace/internal/store"
	"ngac-platform/testutil"
)

// tablesOfAWorkspace lists, for each table that holds a workspace's data, the
// query that counts its rows for one workspace id ($1).
var tablesOfAWorkspace = map[string]string{
	"workspaces":      "SELECT count(*) FROM workspaces WHERE id = $1",
	"tenant_users":    "SELECT count(*) FROM tenant_users WHERE tenant_id = $1",
	"channels":        "SELECT count(*) FROM channels WHERE workspace_id = $1",
	"channel_members": "SELECT count(*) FROM channel_members WHERE channel_id IN (SELECT id FROM channels WHERE workspace_id = $1)",
	"drive_items":     "SELECT count(*) FROM drive_items WHERE workspace_id = $1",
	"drive_quotas":    "SELECT count(*) FROM drive_quotas WHERE workspace_id = $1",
	"text_documents":  "SELECT count(*) FROM text_documents WHERE workspace_id = $1",
	"documents":       "SELECT count(*) FROM documents WHERE workspace_id = $1",
	"asset_types":     "SELECT count(*) FROM asset_types WHERE workspace_id = $1",
	"assets":          "SELECT count(*) FROM assets WHERE workspace_id = $1",
	"asset_requests":  "SELECT count(*) FROM asset_requests WHERE workspace_id = $1",
	"departments":     "SELECT count(*) FROM departments WHERE workspace_id = $1",
	"tenant_schemas":  "SELECT count(*) FROM tenant_schemas WHERE tenant_id = $1",
}

func rowsOf(t *testing.T, ctx context.Context, q string, ws string) int {
	t.Helper()
	pool := testutil.SetupTestDB(t)
	var n int
	require.NoError(t, pool.QueryRow(ctx, q, ws).Scan(&n))
	return n
}

// fill puts one row in every table that holds a workspace's data, as a workspace
// that has been used a little (a #general channel, its drive root, a
// provisioned approval schema) would have.
func fill(t *testing.T, ctx context.Context, wsID, userID, nodeID, tag string) string {
	pool := testutil.SetupTestDB(t)
	schema := "tenant_td_" + tag
	exec := func(q string, args ...any) {
		t.Helper()
		_, err := pool.Exec(ctx, q, args...)
		require.NoError(t, err, q)
	}
	exec(`INSERT INTO tenant_users (tenant_id, user_id, role, status, ngac_node_id) VALUES ($1, $2, 'owner', 'active', $3)`, wsID, userID, nodeID)
	exec(`INSERT INTO channels (id, name, channel_type, workspace_id) VALUES ($1, 'general', 'workspace', $2)`, "ch-"+tag, wsID)
	exec(`INSERT INTO channel_members (channel_id, ngac_node_id) VALUES ($1, $2)`, "ch-"+tag, nodeID)
	exec(`INSERT INTO drive_items (id, workspace_id, item_type, name, ngac_node_id, owner_id) VALUES ($1, $2, 'folder', 'root', $3, $4)`, "di-"+tag, wsID, nodeID, userID)
	exec(`INSERT INTO drive_quotas (workspace_id) VALUES ($1)`, wsID)
	exec(`INSERT INTO text_documents (workspace_id, title) VALUES ($1, 'Biên bản')`, wsID)
	exec(`INSERT INTO documents (id, title, filename, workspace_id) VALUES ($1, 'Hợp đồng', 'a.pdf', $2)`, "doc-"+tag, wsID)
	exec(`INSERT INTO asset_types (id, name, category, workspace_id) VALUES ($1, 'Laptop', 'it', $2)`, "at-"+tag, wsID)
	exec(`INSERT INTO assets (id, name, type_id, workspace_id, created_by) VALUES ($1, 'Máy 1', $2, $3, $4)`, "as-"+tag, "at-"+tag, wsID, userID)
	exec(`INSERT INTO asset_requests (id, type_id, workspace_id, requester_id) VALUES ($1, $2, $3, $4)`, "ar-"+tag, "at-"+tag, wsID, userID)
	exec(`INSERT INTO departments (id, workspace_id, name, ngac_ua_id) VALUES ($1, $2, 'Kế toán', $3)`, "dp-"+tag, wsID, nodeID)
	exec(`CREATE SCHEMA ` + schema)
	exec(`CREATE TABLE ` + schema + `.approval_requests (id text)`)
	exec(`INSERT INTO tenant_schemas (tenant_id, schema_name) VALUES ($1, $2)`, wsID, schema)
	t.Cleanup(func() { _, _ = pool.Exec(context.Background(), `DROP SCHEMA IF EXISTS `+schema+` CASCADE`) })
	return schema
}

func schemaExists(t *testing.T, ctx context.Context, name string) bool {
	pool := testutil.SetupTestDB(t)
	var n int
	require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM information_schema.schemata WHERE schema_name = $1`, name).Scan(&n))
	return n > 0
}

func TestPurgeWorkspace_LeavesNothingOfTheWorkspaceAndOnlyThat(t *testing.T) {
	pool := testutil.SetupTestDB(t)
	ctx := context.Background()
	st := store.New(pool)

	userID, nodeID := testutil.CreateUser(t, pool)
	doomed, _ := testutil.CreateWorkspace(t, pool, userID)
	kept, _ := testutil.CreateWorkspace(t, pool, userID)
	// Rows the test adds must be removed before the fixtures' own cleanups run.
	t.Cleanup(func() {
		for _, ws := range []string{doomed, kept} {
			_ = st.PurgeWorkspace(context.Background(), ws)
		}
	})
	doomedSchema := fill(t, ctx, doomed, userID, nodeID, "d"+doomed[len(doomed)-6:])
	keptSchema := fill(t, ctx, kept, userID, nodeID, "k"+kept[len(kept)-6:])

	for table, q := range tablesOfAWorkspace {
		require.Greater(t, rowsOf(t, ctx, q, doomed), 0, "the fixture put a row in %s", table)
	}
	require.True(t, schemaExists(t, ctx, doomedSchema))

	require.NoError(t, st.PurgeWorkspace(ctx, doomed))

	for table, q := range tablesOfAWorkspace {
		assert.Zero(t, rowsOf(t, ctx, q, doomed), "%s: rows left behind", table)
		assert.Greater(t, rowsOf(t, ctx, q, kept), 0, "%s: another workspace's rows must stay", table)
	}
	assert.False(t, schemaExists(t, ctx, doomedSchema), "the approval schema is dropped")
	assert.True(t, schemaExists(t, ctx, keptSchema), "another tenant's schema stays")

	t.Run("a second call has nothing to do and succeeds", func(t *testing.T) {
		require.NoError(t, st.PurgeWorkspace(ctx, doomed))
		require.NoError(t, st.PurgeWorkspace(ctx, "never-existed"))
	})
}

func TestWorkspaceCreatorAndOtherMembers(t *testing.T) {
	pool := testutil.SetupTestDB(t)
	ctx := context.Background()
	st := store.New(pool)

	owner, ownerNode := testutil.CreateUser(t, pool)
	other, otherNode := testutil.CreateUser(t, pool)
	ws, _ := testutil.CreateWorkspace(t, pool, owner)
	t.Cleanup(func() { _, _ = pool.Exec(context.Background(), `DELETE FROM tenant_users WHERE tenant_id = $1`, ws) })

	got, found, err := st.WorkspaceCreator(ctx, ws)
	require.NoError(t, err)
	assert.True(t, found)
	assert.Equal(t, owner, got)
	_, found, err = st.WorkspaceCreator(ctx, "nope")
	require.NoError(t, err)
	assert.False(t, found)

	add := func(user, node string) {
		_, err := pool.Exec(ctx, `INSERT INTO tenant_users (tenant_id, user_id, role, status, ngac_node_id) VALUES ($1, $2, 'member', 'active', $3)`, ws, user, node)
		require.NoError(t, err)
	}
	add(owner, ownerNode)
	n, err := st.OtherMembers(ctx, ws, owner)
	require.NoError(t, err)
	assert.Zero(t, n, "the owner alone has no other members")
	add(other, otherNode)
	n, err = st.OtherMembers(ctx, ws, owner)
	require.NoError(t, err)
	assert.Equal(t, 1, n)
}
