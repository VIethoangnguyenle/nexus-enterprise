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

func newDept(ws, name string, parent *string, order int) *store.Department {
	return &store.Department{ID: uuid.NewString(), WorkspaceID: ws, Name: name, ParentID: parent, NGACUaID: "ua-" + uuid.NewString(), SortOrder: order}
}

func TestDepartments_InsertListGetRenameMoveDelete(t *testing.T) {
	pool := testutil.SetupTestDB(t)
	st := store.New(pool)
	ctx := context.Background()
	owner, _ := testutil.CreateUser(t, pool)
	ws, _ := testutil.CreateWorkspace(t, pool, owner)
	other, _ := testutil.CreateWorkspace(t, pool, owner)

	root := newDept(ws, "Khối Vận hành", nil, 1)
	child := newDept(ws, "Đối soát", &root.ID, 2)
	foreign := newDept(other, "Của nơi khác", nil, 0)
	for _, d := range []*store.Department{root, child, foreign} {
		require.NoError(t, st.InsertDepartment(ctx, d))
	}
	t.Cleanup(func() {
		pool.Exec(ctx, `DELETE FROM departments WHERE id = ANY($1)`, []string{child.ID, root.ID, foreign.ID})
	})

	list, err := st.ListDepartmentsByWorkspace(ctx, ws)
	require.NoError(t, err)
	require.Len(t, list, 2, "only this workspace's departments")
	assert.Equal(t, root.ID, list[0].ID, "ordered by sort_order")
	assert.Equal(t, root.ID, *list[1].ParentID)

	got, err := st.GetDepartment(ctx, child.ID)
	require.NoError(t, err)
	assert.Equal(t, "Đối soát", got.Name)
	_, err = st.GetDepartment(ctx, "no-such-department")
	assert.Error(t, err)

	require.NoError(t, st.UpdateDepartmentName(ctx, child.ID, "Thanh toán"))
	got, _ = st.GetDepartment(ctx, child.ID)
	assert.Equal(t, "Thanh toán", got.Name)

	require.NoError(t, st.MoveDepartment(ctx, child.ID, nil))
	got, _ = st.GetDepartment(ctx, child.ID)
	assert.Nil(t, got.ParentID, "moved to the top level")

	assert.Error(t, st.InsertDepartment(ctx, child), "a duplicate id is refused")

	require.NoError(t, st.DeleteDepartment(ctx, child.ID))
	_, err = st.GetDepartment(ctx, child.ID)
	assert.Error(t, err)
	still, _ := st.GetDepartment(ctx, root.ID)
	assert.NotNil(t, still, "deleting one leaves its parent")
}

func TestDepartments_ReassignChildrenAndUsers(t *testing.T) {
	pool := testutil.SetupTestDB(t)
	st := store.New(pool)
	ctx := context.Background()
	owner, ownerNode := testutil.CreateUser(t, pool)
	ws, _ := testutil.CreateWorkspace(t, pool, owner)

	old := newDept(ws, "Cũ", nil, 0)
	target := newDept(ws, "Mới", nil, 1)
	kid := newDept(ws, "Con", &old.ID, 2)
	for _, d := range []*store.Department{old, target, kid} {
		require.NoError(t, st.InsertDepartment(ctx, d))
	}
	t.Cleanup(func() {
		pool.Exec(ctx, `UPDATE tenant_users SET department_id = NULL WHERE tenant_id = $1`, ws)
		pool.Exec(ctx, `DELETE FROM departments WHERE id = ANY($1)`, []string{kid.ID, old.ID, target.ID})
	})

	require.NoError(t, st.ReassignDepartmentChildren(ctx, old.ID, &target.ID))
	got, _ := st.GetDepartment(ctx, kid.ID)
	require.NotNil(t, got.ParentID)
	assert.Equal(t, target.ID, *got.ParentID)

	// The workspace owner is a member row in this workspace.
	_, err := pool.Exec(ctx, `INSERT INTO tenant_users (tenant_id, user_id, role, ngac_node_id) VALUES ($1, $2, 'member', $3)
		ON CONFLICT (tenant_id, user_id) DO NOTHING`, ws, owner, ownerNode)
	require.NoError(t, err)
	require.NoError(t, st.UpdateUserDepartment(ctx, ws, ownerNode, &old.ID))
	n, err := st.CountMembersByDepartment(ctx, old.ID)
	require.NoError(t, err)
	assert.Equal(t, 1, n)

	require.NoError(t, st.ReassignDepartmentUsers(ctx, old.ID, &target.ID))
	n, _ = st.CountMembersByDepartment(ctx, old.ID)
	assert.Zero(t, n)
	n, _ = st.CountMembersByDepartment(ctx, target.ID)
	assert.Equal(t, 1, n)

	// A person of another workspace is not touched by this workspace's edit.
	require.NoError(t, st.UpdateUserDepartment(ctx, "another-workspace", ownerNode, nil))
	n, _ = st.CountMembersByDepartment(ctx, target.ID)
	assert.Equal(t, 1, n)

	require.NoError(t, st.UpdateUserDepartment(ctx, ws, ownerNode, nil))
	n, _ = st.CountMembersByDepartment(ctx, target.ID)
	assert.Zero(t, n)
}
