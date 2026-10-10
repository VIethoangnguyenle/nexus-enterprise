package store_test

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"ngac-platform/services/workspace/internal/store"
	"ngac-platform/testutil"
)

func TestDirectoryStore(t *testing.T) {
	pool := testutil.SetupTestDB(t)
	ctx := context.Background()
	st := store.New(pool)

	ownerID, _ := testutil.CreateUser(t, pool)
	wsA, _ := testutil.CreateWorkspace(t, pool, ownerID)
	wsB, _ := testutil.CreateWorkspace(t, pool, ownerID)
	userID, nodeID := testutil.CreateUser(t, pool)
	email := "Lan." + uuid.NewString()[:8] + "@Example.vn"
	_, err := pool.Exec(ctx, `UPDATE users SET email = $2, display_name = 'Nguyễn Thu Lan', title = 'Chuyên viên', avatar_url = '' WHERE id = $1`, userID, email)
	require.NoError(t, err)
	t.Cleanup(func() { pool.Exec(context.Background(), `DELETE FROM tenant_users WHERE user_id = $1`, userID) })

	t.Run("profile of a person with no listing is active", func(t *testing.T) {
		got, err := st.ProfilesByNodeIDs(ctx, wsA, []string{nodeID, "no-such-node"})
		require.NoError(t, err)
		require.Len(t, got, 1, "an unknown node is absent, not an error")
		p := got[nodeID]
		assert.Equal(t, "Nguyễn Thu Lan", p.DisplayName)
		assert.Equal(t, "Chuyên viên", p.Title)
		assert.Equal(t, userID, p.UserID)
		assert.Equal(t, "active", p.Status)
	})

	t.Run("listing a person is per workspace and idempotent", func(t *testing.T) {
		require.NoError(t, st.EnsureTenantUser(ctx, wsA, userID, nodeID))
		require.NoError(t, st.EnsureTenantUser(ctx, wsA, userID, nodeID))

		var n int
		require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM tenant_users WHERE user_id = $1`, userID).Scan(&n))
		assert.Equal(t, 1, n)

		// A person an administrator disabled is not re-enabled by being invited again.
		_, err := pool.Exec(ctx, `UPDATE tenant_users SET status = 'disabled' WHERE tenant_id = $1 AND user_id = $2`, wsA, userID)
		require.NoError(t, err)
		require.NoError(t, st.EnsureTenantUser(ctx, wsA, userID, nodeID))
		got, err := st.ProfilesByNodeIDs(ctx, wsA, []string{nodeID})
		require.NoError(t, err)
		assert.Equal(t, "disabled", got[nodeID].Status)

		// The same person in another workspace has their own standing there.
		got, err = st.ProfilesByNodeIDs(ctx, wsB, []string{nodeID})
		require.NoError(t, err)
		assert.Equal(t, "active", got[nodeID].Status)
	})

	t.Run("department is set and cleared in one workspace only", func(t *testing.T) {
		require.NoError(t, st.EnsureTenantUser(ctx, wsB, userID, nodeID))
		deptA := "dept-" + uuid.NewString()[:8]
		deptB := "dept-" + uuid.NewString()[:8]
		for _, d := range []struct{ id, ws string }{{deptA, wsA}, {deptB, wsB}} {
			_, err := pool.Exec(ctx, `INSERT INTO ngac_nodes (id, name, node_type, properties) VALUES ($1, $1, 'UA', '{}')`, "ua-"+d.id)
			require.NoError(t, err)
			require.NoError(t, st.InsertDepartment(ctx, &store.Department{ID: d.id, WorkspaceID: d.ws, Name: d.id, NGACUaID: "ua-" + d.id}))
			t.Cleanup(func() {
				pool.Exec(context.Background(), `UPDATE tenant_users SET department_id = NULL WHERE department_id = $1`, d.id)
				pool.Exec(context.Background(), `DELETE FROM departments WHERE id = $1`, d.id)
				pool.Exec(context.Background(), `DELETE FROM ngac_nodes WHERE id = $1`, "ua-"+d.id)
			})
		}
		require.NoError(t, st.UpdateUserDepartment(ctx, wsA, nodeID, &deptA))
		require.NoError(t, st.UpdateUserDepartment(ctx, wsB, nodeID, &deptB))
		n, err := st.CountMembersByDepartment(ctx, deptA)
		require.NoError(t, err)
		assert.Equal(t, 1, n)

		require.NoError(t, st.UpdateUserDepartment(ctx, wsA, nodeID, nil))
		n, _ = st.CountMembersByDepartment(ctx, deptA)
		assert.Equal(t, 0, n, "cleared")
		n, _ = st.CountMembersByDepartment(ctx, deptB)
		assert.Equal(t, 1, n, "the other workspace is untouched")
	})

	t.Run("removing a person drops their listing in that workspace only", func(t *testing.T) {
		require.NoError(t, st.EnsureTenantUser(ctx, wsA, userID, nodeID))
		require.NoError(t, st.RemoveTenantUser(ctx, wsA, nodeID))
		var a, b int
		require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM tenant_users WHERE tenant_id = $1 AND user_id = $2`, wsA, userID).Scan(&a))
		require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM tenant_users WHERE tenant_id = $1 AND user_id = $2`, wsB, userID).Scan(&b))
		assert.Equal(t, 0, a)
		assert.Equal(t, 1, b)
	})
}
