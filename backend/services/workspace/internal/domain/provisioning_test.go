package domain_test

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"ngac-platform/ngac"
	policypb "ngac-platform/proto/policy"
	"ngac-platform/services/workspace/internal/domain"
	"ngac-platform/services/workspace/internal/store"
	"ngac-platform/testutil"
)

// Provisioning is a run of separate policy writes followed by a database insert.
// These tests break the run at every step and require that nothing it created is
// left in the graph, and that it is removed newest first.

type failingWSStore struct {
	*fakeWSStore
	err error
}

func (f *failingWSStore) Insert(context.Context, *store.Workspace) error { return f.err }

type failingDeptStore struct {
	*fakeDeptStore
	err error
}

func (f *failingDeptStore) InsertDepartment(context.Context, *store.Department) error { return f.err }

// created returns the names of the nodes the log shows created, in order.
func created(log []string) []string {
	var out []string
	for _, l := range log {
		if rest, ok := strings.CutPrefix(l, "create "); ok {
			out = append(out, rest[strings.Index(rest, " ")+1:])
		}
	}
	return out
}

func deleted(log []string) []string {
	var out []string
	for _, l := range log {
		if name, ok := strings.CutPrefix(l, "delete "); ok {
			out = append(out, name)
		}
	}
	return out
}

func reversed(in []string) []string {
	out := slices.Clone(in)
	slices.Reverse(out)
	return out
}

func newWriteFixture(t *testing.T, w *testutil.FakePolicyWrite) *fixture {
	t.Helper()
	f := newFixture(t)
	f.svc = domain.NewService(f.wsStore, f.depts, f.read, w, nil, nil)
	return f
}

func assertNothingLeft(t *testing.T, w *testutil.FakePolicyWrite, what string) {
	t.Helper()
	assert.Empty(t, w.LiveNodes(), "%s: nodes left behind", what)
	assert.Equal(t, reversed(created(w.Log())), deleted(w.Log()), "%s: rolled back newest first", what)
}

func TestCreateWorkspace_Succeeds(t *testing.T) {
	w := testutil.NewFakePolicyWrite()
	f := newWriteFixture(t, w)

	ws, err := f.svc.CreateWorkspace(context.Background(), domain.CreateWorkspaceInput{Name: "Acme", UserID: "u1", UserNGACNodeID: owner})

	require.NoError(t, err)
	assert.Len(t, w.LiveNodes(), 8, "PC, Owners, Members, Mgmt, Documents, DraftDocs, ApprovedDocs, Channels")
	row := f.wsStore.ws[ws.ID]
	require.NotNil(t, row)
	assert.Equal(t, ws.DocumentsOaID, row.DocumentsOAID, "the Documents OA is stored on the workspace so nothing has to guess it from a name")
	assert.NotEmpty(t, row.DocumentsOAID)
	assert.Contains(t, w.LiveNodes(), "OA "+ngac.DocumentsOAName(ngac.WorkspaceID(ws.ID)))
}

func TestCreateWorkspace_RollsBackWhereverItFails(t *testing.T) {
	probe := testutil.NewFakePolicyWrite()
	_, err := newWriteFixture(t, probe).svc.CreateWorkspace(context.Background(), domain.CreateWorkspaceInput{Name: "Acme", UserID: "u1", UserNGACNodeID: owner})
	require.NoError(t, err)
	steps := probe.Calls()
	require.Greater(t, steps, 15, "the probe run must exercise every node, assignment and association")

	for k := 1; k <= steps; k++ {
		w := testutil.NewFakePolicyWrite()
		w.FailAt = k
		f := newWriteFixture(t, w)

		_, err := f.svc.CreateWorkspace(context.Background(), domain.CreateWorkspaceInput{Name: "Acme", UserID: "u1", UserNGACNodeID: owner})

		require.Errorf(t, err, "failure injected at write %d must surface", k)
		assertNothingLeft(t, w, fmt.Sprintf("failure at write %d", k))
		assert.Len(t, f.wsStore.ws, 2, "no workspace row for a failed provisioning (the two fixture rows only)")
	}
}

func TestCreateWorkspace_RollsBackWhenTheRowCannotBeStored(t *testing.T) {
	w := testutil.NewFakePolicyWrite()
	f := newWriteFixture(t, w)
	f.svc = domain.NewService(&failingWSStore{fakeWSStore: f.wsStore, err: errors.New("db down")}, f.depts, f.read, w, nil, nil)

	_, err := f.svc.CreateWorkspace(context.Background(), domain.CreateWorkspaceInput{Name: "Acme", UserID: "u1", UserNGACNodeID: owner})

	require.ErrorContains(t, err, "db down")
	assertNothingLeft(t, w, "row insert")
	assert.NotEmpty(t, created(w.Log()), "the graph writes did happen before the insert failed")
}

func createSales(t *testing.T, f *fixture, caller, ws string) *domain.DepartmentResult {
	t.Helper()
	d, err := f.svc.CreateDepartment(context.Background(), caller, domain.CreateDepartmentInput{WorkspaceID: ws, Name: "Sales"})
	require.NoError(t, err)
	return d
}

// Two tenants each call a department "Sales". The node is keyed by the
// department's own ID, so they cannot resolve onto one node.
func TestCreateDepartment_SameNameInTwoTenantsDoesNotCollide(t *testing.T) {
	w := testutil.NewFakePolicyWrite()
	f := newWriteFixture(t, w)

	a := createSales(t, f, owner, ws1)
	b := createSales(t, f, otherOwner, ws2)

	live := w.LiveNodes()
	require.Len(t, live, 2)
	assert.NotEqual(t, live[0], live[1], "two tenants' departments must be two distinct node names")
	for _, d := range []*domain.DepartmentResult{a, b} {
		assert.Contains(t, live, "UA "+ngac.DeptUAName(ngac.DeptID(d.ID)), "the node is named by the department's ID")
		n, ok := w.Node(d.NGACUaID)
		require.True(t, ok)
		assert.Equal(t, "Sales", n.Properties[ngac.PropDisplayName], "the display name lives in the properties")
		assert.NotContains(t, n.Name, "Sales", "the display name is never part of the node name")
	}
	assert.NotEqual(t, a.ID, b.ID)
}

func TestCreateDepartment_RollsBackWhereverItFails(t *testing.T) {
	probe := testutil.NewFakePolicyWrite()
	createSales(t, newWriteFixture(t, probe), owner, ws1)
	steps := probe.Calls()
	require.Equal(t, 2, steps, "one node and one assignment")

	for k := 1; k <= steps; k++ {
		w := testutil.NewFakePolicyWrite()
		w.FailAt = k
		f := newWriteFixture(t, w)
		before := len(f.depts.depts)

		_, err := f.svc.CreateDepartment(context.Background(), owner, domain.CreateDepartmentInput{WorkspaceID: ws1, Name: "Sales"})

		require.Error(t, err)
		assertNothingLeft(t, w, "department write failure")
		assert.Len(t, f.depts.depts, before)
	}

	w := testutil.NewFakePolicyWrite()
	f := newWriteFixture(t, w)
	f.svc = domain.NewService(f.wsStore, &failingDeptStore{fakeDeptStore: f.depts, err: errors.New("db down")}, f.read, w, nil, nil)
	_, err := f.svc.CreateDepartment(context.Background(), owner, domain.CreateDepartmentInput{WorkspaceID: ws1, Name: "Sales"})
	require.ErrorContains(t, err, "db down")
	assertNothingLeft(t, w, "department row insert")
}

// A caller who may not manage the workspace creates nothing, and so has nothing
// to roll back.
func TestCreateDepartment_DeniedCreatesNothing(t *testing.T) {
	w := testutil.NewFakePolicyWrite()
	f := newWriteFixture(t, w)

	for _, caller := range []string{member, otherOwner, outsider} {
		_, err := f.svc.CreateDepartment(context.Background(), caller, domain.CreateDepartmentInput{WorkspaceID: ws1, Name: "Sales"})
		require.ErrorIs(t, err, domain.ErrAccessDenied)
	}
	assert.Zero(t, w.Calls())
}

func TestCreateRole_NodeNameIsAGeneratedIDAndTheDisplayNameIsAProperty(t *testing.T) {
	w := testutil.NewFakePolicyWrite()
	f := newWriteFixture(t, w)

	r, err := f.svc.CreateRole(context.Background(), owner, ws1, "Reviewer")

	require.NoError(t, err)
	assert.Equal(t, "Reviewer", r.Name, "callers keep seeing the name the administrator typed")
	n, ok := w.Node(r.NGACNodeID)
	require.True(t, ok)
	assert.True(t, strings.HasPrefix(n.Name, "Role_"), "node name %q is ID-keyed", n.Name)
	assert.NotContains(t, n.Name, "Reviewer")
	assert.Equal(t, "Reviewer", n.Properties[ngac.PropDisplayName])
	assert.Equal(t, ws1, n.Properties["workspace_id"])
}

// Two roles with the same display name are two nodes; neither can take over the
// other's name.
func TestCreateRole_SameDisplayNameTwiceGivesTwoNodes(t *testing.T) {
	w := testutil.NewFakePolicyWrite()
	f := newWriteFixture(t, w)
	_, err := f.svc.CreateRole(context.Background(), owner, ws1, "Reviewer")
	require.NoError(t, err)
	_, err = f.svc.CreateRole(context.Background(), owner, ws1, "Reviewer")
	require.NoError(t, err)

	live := w.LiveNodes()
	require.Len(t, live, 2)
	assert.NotEqual(t, live[0], live[1])
}

func TestCreateRole_RollsBackWhenItCannotBeAssigned(t *testing.T) {
	w := testutil.NewFakePolicyWrite()
	w.FailAt = 2 // the assignment under the workspace PC
	f := newWriteFixture(t, w)

	_, err := f.svc.CreateRole(context.Background(), owner, ws1, "Reviewer")

	require.Error(t, err)
	assertNothingLeft(t, w, "role assignment")
}

func TestListRoles_ShowsRolesByDisplayNameAndNothingElse(t *testing.T) {
	w := testutil.NewFakePolicyWrite()
	f := newWriteFixture(t, w)
	f.read.children[pc1] = append(f.read.children[pc1], &policypb.NGACNode{
		Id: "ua-new", Name: "Role_3f2c", NodeType: ngac.TypeUA,
		Properties: map[string]string{ngac.PropType: ngac.PropTypeRole, ngac.PropDisplayName: "Reviewer"},
	})

	roles, err := f.svc.ListRoles(context.Background(), owner, ws1)

	require.NoError(t, err)
	var names []string
	for _, r := range roles {
		names = append(names, r.Name)
	}
	assert.ElementsMatch(t, []string{"Reviewer", "Editor"}, names, "roles only")
	for _, n := range names {
		assert.NotContains(t, n, "_Owners", "platform UAs are not roles")
		assert.NotContains(t, n, "_Members")
		assert.NotEqual(t, "Role_3f2c", n, "the node name never reaches the screen")
	}
}

// Platform UAs (Owners, Members, tenant, department, channel) are not roles:
// the roles API can neither delete them nor put a member into them.
func TestRolesAPI_RefusesPlatformUAs(t *testing.T) {
	for name, id := range map[string]string{"owners": owners1, "members": members1, "unknown": "ua-nope"} {
		t.Run(name, func(t *testing.T) {
			f := newFixture(t)

			err := f.svc.DeleteRole(context.Background(), owner, ws1, id)
			require.Error(t, err)
			assert.NotContains(t, f.write.mutations, "DeleteNode "+id, "the UA must survive")

			err = f.svc.UpdateMemberRoles(context.Background(), owner, ws1, target, []string{role1, id})
			require.Error(t, err)
			assert.Empty(t, f.write.mutations, "nothing is detached or assigned when any role is not a role")
		})
	}
}

func TestRolesAPI_AcceptsARealRole(t *testing.T) {
	f := newFixture(t)

	require.NoError(t, f.svc.UpdateMemberRoles(context.Background(), owner, ws1, target, []string{role1}))
	require.NoError(t, f.svc.DeleteRole(context.Background(), owner, ws1, role1))

	assert.Contains(t, f.write.mutations, "DeleteNode "+role1)
}

func TestCreateFolder_NodeNameIsAGeneratedIDAndTheDisplayNameIsAProperty(t *testing.T) {
	w := testutil.NewFakePolicyWrite()
	f := newWriteFixture(t, w)

	fo, err := f.svc.CreateFolder(context.Background(), owner, ws1, "Legal", "")

	require.NoError(t, err)
	assert.Equal(t, "Legal", fo.Name)
	n, ok := w.Node(fo.NGACNodeID)
	require.True(t, ok)
	assert.True(t, strings.HasPrefix(n.Name, "Folder_"), "node name %q is ID-keyed", n.Name)
	assert.NotContains(t, n.Name, "Legal")
	assert.Equal(t, "Legal", n.Properties[ngac.PropDisplayName])
}

func TestCreateFolder_RollsBackWhenItCannotBeAssigned(t *testing.T) {
	w := testutil.NewFakePolicyWrite()
	w.FailAt = 2
	f := newWriteFixture(t, w)

	_, err := f.svc.CreateFolder(context.Background(), owner, ws1, "Legal", "")

	require.Error(t, err)
	assertNothingLeft(t, w, "folder assignment")
}

func TestListFolders_ShowsDisplayNames(t *testing.T) {
	w := testutil.NewFakePolicyWrite()
	f := newWriteFixture(t, w)
	f.read.descendants[pc1] = append(f.read.descendants[pc1], &policypb.NGACNode{
		Id: "oa-new", Name: "Folder_9a1b", NodeType: ngac.TypeOA,
		Properties: map[string]string{ngac.PropDisplayName: "Legal"},
	})

	folders, err := f.svc.ListFolders(context.Background(), owner, ws1)

	require.NoError(t, err)
	var names []string
	for _, fo := range folders {
		names = append(names, fo.Name)
	}
	assert.Contains(t, names, "Legal")
	assert.Contains(t, names, "Engineering", "a folder written before names were ID-keyed is listed by its node name")
	assert.NotContains(t, names, "Folder_9a1b")
}

func TestListMembers_ShowsUsernamesNotNodeNames(t *testing.T) {
	w := testutil.NewFakePolicyWrite()
	f := newWriteFixture(t, w)
	f.read.descendants[pc1] = append(f.read.descendants[pc1], &policypb.NGACNode{
		Id: "u-new", Name: "U_5d2e", NodeType: ngac.TypeU,
		Properties: map[string]string{ngac.PropDisplayName: "alice"},
	})

	members, err := f.svc.ListMembers(context.Background(), owner, ws1)

	require.NoError(t, err)
	var names []string
	for _, m := range members {
		names = append(names, m.Username)
	}
	assert.Contains(t, names, "alice")
	assert.Contains(t, names, "owner", "a user node written before names were ID-keyed carries the username as its name")
	assert.NotContains(t, names, "U_5d2e")
}
