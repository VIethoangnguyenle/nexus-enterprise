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
		// The department counts active members, so the person is active here (the
		// subtest above left them disabled in wsA).
		_, err := pool.Exec(ctx, `UPDATE tenant_users SET status = 'active' WHERE tenant_id = $1 AND user_id = $2`, wsA, userID)
		require.NoError(t, err)
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

	status := func(ws string) (string, bool) {
		var s string
		var dept *string
		err := pool.QueryRow(ctx, `SELECT status, department_id FROM tenant_users WHERE tenant_id = $1 AND user_id = $2`, ws, userID).Scan(&s, &dept)
		if err != nil {
			return "", false
		}
		return s, true
	}

	t.Run("removing a person tombstones their listing in that workspace only", func(t *testing.T) {
		_, err := pool.Exec(ctx, `UPDATE tenant_users SET status = 'active' WHERE tenant_id = $1 AND user_id = $2`, wsA, userID)
		require.NoError(t, err)
		require.NoError(t, st.EnsureTenantUser(ctx, wsA, userID, nodeID))
		require.NoError(t, st.MarkTenantUserRemoved(ctx, wsA, nodeID))

		a, found := status(wsA)
		require.True(t, found, "the row stays: a deleted row is what let a domain sign-in re-add the person")
		assert.Equal(t, "removed", a)
		b, _ := status(wsB)
		assert.Equal(t, "active", b, "the other workspace is untouched")

		var dept *string
		require.NoError(t, pool.QueryRow(ctx, `SELECT department_id FROM tenant_users WHERE tenant_id = $1 AND user_id = $2`, wsA, userID).Scan(&dept))
		assert.Nil(t, dept, "a removed person is in no department")

		require.NoError(t, st.MarkTenantUserRemoved(ctx, wsA, nodeID), "removing twice is not an error")
	})

	t.Run("an invitation brings a removed person back, and only a removed one", func(t *testing.T) {
		// They were an owner when they left; coming back they are a member until someone says otherwise.
		_, err := pool.Exec(ctx, `UPDATE tenant_users SET role = 'owner' WHERE tenant_id = $1 AND user_id = $2`, wsA, userID)
		require.NoError(t, err)
		require.NoError(t, st.EnsureTenantUser(ctx, wsA, userID, nodeID))
		got, _ := status(wsA)
		assert.Equal(t, "active", got, "removed -> active on a new invitation")
		var role string
		require.NoError(t, pool.QueryRow(ctx, `SELECT role FROM tenant_users WHERE tenant_id = $1 AND user_id = $2`, wsA, userID).Scan(&role))
		assert.Equal(t, "member", role, "a returning person does not get back the role they left with")

		for _, standing := range []string{"disabled", "invited"} {
			_, err := pool.Exec(ctx, `UPDATE tenant_users SET status = $3, role = 'admin' WHERE tenant_id = $1 AND user_id = $2`, wsA, userID, standing)
			require.NoError(t, err)
			require.NoError(t, st.EnsureTenantUser(ctx, wsA, userID, nodeID))
			got, _ = status(wsA)
			assert.Equal(t, standing, got, "an invitation must not lift %q", standing)
			require.NoError(t, pool.QueryRow(ctx, `SELECT role FROM tenant_users WHERE tenant_id = $1 AND user_id = $2`, wsA, userID).Scan(&role))
			assert.Equal(t, "admin", role, "and leaves the role of a person who did not leave")
		}
	})
}

// People who are no longer in the workspace are not members for the two counts
// the service takes: who else is in a workspace being deleted, and who is in a
// department.
func TestDirectoryStore_RemovedPeopleAreNotCounted(t *testing.T) {
	pool := testutil.SetupTestDB(t)
	ctx := context.Background()
	st := store.New(pool)

	ownerID, ownerNode := testutil.CreateUser(t, pool)
	ws, _ := testutil.CreateWorkspace(t, pool, ownerID)
	stayID, stayNode := testutil.CreateUser(t, pool)
	goneID, goneNode := testutil.CreateUser(t, pool)
	t.Cleanup(func() { pool.Exec(context.Background(), `DELETE FROM tenant_users WHERE tenant_id = $1`, ws) })

	dept := "dept-" + uuid.NewString()[:8]
	_, err := pool.Exec(ctx, `INSERT INTO ngac_nodes (id, name, node_type, properties) VALUES ($1, $1, 'UA', '{}')`, "ua-"+dept)
	require.NoError(t, err)
	require.NoError(t, st.InsertDepartment(ctx, &store.Department{ID: dept, WorkspaceID: ws, Name: dept, NGACUaID: "ua-" + dept}))
	t.Cleanup(func() {
		pool.Exec(context.Background(), `UPDATE tenant_users SET department_id = NULL WHERE department_id = $1`, dept)
		pool.Exec(context.Background(), `DELETE FROM departments WHERE id = $1`, dept)
		pool.Exec(context.Background(), `DELETE FROM ngac_nodes WHERE id = $1`, "ua-"+dept)
	})

	for _, p := range []struct{ user, node string }{{ownerID, ownerNode}, {stayID, stayNode}, {goneID, goneNode}} {
		require.NoError(t, st.EnsureTenantUser(ctx, ws, p.user, p.node))
		require.NoError(t, st.UpdateUserDepartment(ctx, ws, p.node, &dept))
	}
	n, err := st.OtherMembers(ctx, ws, ownerID)
	require.NoError(t, err)
	assert.Equal(t, 2, n, "before the removal both others are members")
	n, err = st.CountMembersByDepartment(ctx, dept)
	require.NoError(t, err)
	assert.Equal(t, 3, n)

	// A removal clears the department itself; a row that kept one (written by an
	// older release) must not count either.
	_, err = pool.Exec(ctx, `UPDATE tenant_users SET status = 'removed' WHERE tenant_id = $1 AND user_id = $2`, ws, goneID)
	require.NoError(t, err)

	n, err = st.OtherMembers(ctx, ws, ownerID)
	require.NoError(t, err)
	assert.Equal(t, 1, n, "the removed person is not another member: the workspace can be deleted once everyone else has gone")
	n, err = st.CountMembersByDepartment(ctx, dept)
	require.NoError(t, err)
	assert.Equal(t, 2, n, "a removed person does not keep a department occupied")
}
