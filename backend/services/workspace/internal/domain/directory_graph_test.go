package domain_test

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"ngac-platform/ngac"
	policypb "ngac-platform/proto/policy"
	"ngac-platform/services/workspace/internal/domain"
	"ngac-platform/testutil"
)

// The workspace directory graph (docs/specs/contacts-visibility) kept by the
// department upkeep, against fakes that record every write: what is written, in
// what order, what is undone when a step fails, what is looked up and cached, and
// what is refused. The decisions themselves are proved on the real policy service
// in directory_graph_realpolicy_test.go.

// seedFakeDirectory registers a workspace's five directory nodes in the fake, as
// they stand once the workspace has a directory.
func seedFakeDirectory(w *testutil.FakePolicyWrite, ws string) {
	id := ngac.WorkspaceID(ws)
	for _, n := range []struct{ id, name, typ string }{
		{"dir-root-" + ws, ngac.DirectoryOAName(id), ngac.TypeOA},
		{"dir-open-" + ws, ngac.DirectoryOpenOAName(id), ngac.TypeOA},
		{"dir-nodept-" + ws, ngac.DirectoryNoDeptOAName(id), ngac.TypeOA},
		{"dir-owners-" + ws, ngac.DirectoryOwnersOAName(id), ngac.TypeOA},
		{"dir-limited-" + ws, ngac.DirectoryLimitedUAName(id), ngac.TypeUA},
	} {
		w.Seed(&policypb.NGACNode{Id: n.id, Name: n.name, NodeType: n.typ, Properties: ngac.DirectoryProps(id)})
	}
}

// seedFakeDeptDirectory registers a department's three directory nodes.
func seedFakeDeptDirectory(w *testutil.FakePolicyWrite, ws, dept string) {
	id := ngac.WorkspaceID(ws)
	d := ngac.DeptID(dept)
	for _, n := range []struct{ id, name, typ string }{
		{"dir-" + dept, ngac.DeptDirOAName(d), ngac.TypeOA},
		{"peer-" + dept, ngac.DeptPeerOAName(d), ngac.TypeOA},
		{"peers-" + dept, ngac.DeptPeersUAName(d), ngac.TypeUA},
	} {
		w.Seed(&policypb.NGACNode{Id: n.id, Name: n.name, NodeType: n.typ, Properties: ngac.DirectoryProps(id)})
	}
}

func liveBaseline(w *testutil.FakePolicyWrite) []string { return w.LiveNodes() }

// A new department gets its three nodes, under the right parents, and the peers UA
// reads its peer OA; a child sits under its parent's.
func TestCreateDepartment_MirrorsIntoTheDirectory(t *testing.T) {
	w := testutil.NewFakePolicyWrite()
	f := newWriteFixture(t, w)
	seedFakeDirectory(w, ws1)
	seedFakeDeptDirectory(w, ws1, "dept-root-1")

	top := createSales(t, f, owner, ws1)
	topID := ngac.DeptID(top.ID)
	log := w.Log()
	assert.Contains(t, log, "create OA "+ngac.DeptDirOAName(topID))
	assert.Contains(t, log, "create OA "+ngac.DeptPeerOAName(topID))
	assert.Contains(t, log, "create UA "+ngac.DeptPeersUAName(topID))
	assert.Contains(t, log, "assign "+ngac.DeptDirOAName(topID)+">"+ngac.DirectoryOpenOAName(ngac.WorkspaceID(ws1)), "a top-level Dir sits under Open")
	assert.Contains(t, log, "assign "+ngac.DeptPeerOAName(topID)+">"+ngac.DirectoryOAName(ngac.WorkspaceID(ws1)), "a top-level DirPeer sits under Directory")
	assert.Contains(t, log, "assign "+ngac.DeptPeersUAName(topID)+">"+pc1, "the peers UA is flat under the class")
	assert.NotContains(t, log, "assign "+ngac.DeptPeersUAName(topID)+">"+members1)
	assert.Contains(t, log, "associate "+ngac.DeptPeersUAName(topID)+">"+ngac.DeptPeerOAName(topID)+" [read]")

	child, err := f.svc.CreateDepartment(context.Background(), owner, domain.CreateDepartmentInput{WorkspaceID: ws1, Name: "Child", ParentID: "dept-root-1"})
	require.NoError(t, err)
	childID := ngac.DeptID(child.ID)
	log = w.Log()
	assert.Contains(t, log, "assign "+ngac.DeptDirOAName(childID)+">"+ngac.DeptDirOAName("dept-root-1"), "a child Dir sits under its parent's")
	assert.Contains(t, log, "assign "+ngac.DeptPeerOAName(childID)+">"+ngac.DeptPeerOAName("dept-root-1"))
	n, ok := w.Node(child.NGACUaID)
	require.True(t, ok)
	assert.NotContains(t, n.Name, "Child", "names are ID-keyed")
	for _, name := range []string{ngac.DeptDirOAName(childID), ngac.DeptPeerOAName(childID)} {
		assert.Contains(t, w.LiveNodes(), "OA "+name)
	}
}

// Whatever step fails, nothing the run created stays: the department, its three
// directory nodes, and no row.
func TestCreateDepartment_RollsBackTheDirectoryNodesToo(t *testing.T) {
	newWorld := func() (*testutil.FakePolicyWrite, *fixture) {
		w := testutil.NewFakePolicyWrite()
		f := newWriteFixture(t, w)
		seedFakeDirectory(w, ws1)
		seedFakeDeptDirectory(w, ws1, "dept-root-1")
		return w, f
	}
	probe, pf := newWorld()
	_, err := pf.svc.CreateDepartment(context.Background(), owner, domain.CreateDepartmentInput{WorkspaceID: ws1, Name: "Sales", ParentID: "dept-root-1"})
	require.NoError(t, err)
	steps := probe.Calls()
	require.Equal(t, 9, steps, "UA + its assignment, then Dir, DirPeer and DeptPeers with an assignment each, and the association")

	for k := 1; k <= steps; k++ {
		w, f := newWorld()
		w.FailAt = k
		baseline := liveBaseline(w)
		before := len(f.depts.depts)

		_, err := f.svc.CreateDepartment(context.Background(), owner, domain.CreateDepartmentInput{WorkspaceID: ws1, Name: "Sales", ParentID: "dept-root-1"})

		require.Errorf(t, err, "failure at write %d must surface", k)
		assert.ElementsMatchf(t, baseline, w.LiveNodes(), "failure at write %d leaves only what was there before", k)
		assert.Len(t, f.depts.depts, before, "failure at write %d: no row", k)
	}

	w, f := newWorld()
	baseline := liveBaseline(w)
	f.svc = domain.NewService(f.wsStore, &failingDeptStore{fakeDeptStore: f.depts, err: errors.New("db down")}, f.read, w, nil, nil)
	_, err = f.svc.CreateDepartment(context.Background(), owner, domain.CreateDepartmentInput{WorkspaceID: ws1, Name: "Sales"})
	require.ErrorContains(t, err, "db down")
	assert.ElementsMatch(t, baseline, w.LiveNodes(), "the directory nodes go with the department when its row cannot be written")
}

// A workspace without a directory writes no directory node. A department of a workspace
// that has one but whose own nodes are missing (it was created while the migration ran, or
// before it) gets them the first time something needs them, its parent's first.
func TestCreateDepartment_BuildsMissingDirectoryNodesOfItsParent(t *testing.T) {
	w := testutil.NewFakePolicyWrite()
	f := newWriteFixture(t, w)
	d := createSales(t, f, owner, ws1)
	assert.Equal(t, 2, w.Calls(), "only the department's UA and its assignment")
	assert.NotContains(t, w.LiveNodes(), "OA "+ngac.DeptDirOAName(ngac.DeptID(d.ID)), "no directory, no nodes")

	seedFakeDirectory(w, ws1) // the workspace has a directory, "dept-root-1" does not
	child, err := f.svc.CreateDepartment(context.Background(), owner, domain.CreateDepartmentInput{WorkspaceID: ws1, Name: "Child", ParentID: "dept-root-1"})
	require.NoError(t, err)
	live := w.LiveNodes()
	for _, id := range []ngac.DeptID{"dept-root-1", ngac.DeptID(child.ID)} {
		assert.Contains(t, live, "OA "+ngac.DeptDirOAName(id))
		assert.Contains(t, live, "OA "+ngac.DeptPeerOAName(id))
		assert.Contains(t, live, "UA "+ngac.DeptPeersUAName(id))
	}
	log := w.Log()
	assert.Contains(t, log, "assign "+ngac.DeptDirOAName("dept-root-1")+">"+ngac.DirectoryOpenOAName(ngac.WorkspaceID(ws1)), "the parent is a top-level department")
	assert.Contains(t, log, "assign "+ngac.DeptDirOAName(ngac.DeptID(child.ID))+">"+ngac.DeptDirOAName("dept-root-1"), "and the child sits under it")

	again := w.Calls()
	_, err = f.svc.CreateDepartment(context.Background(), owner, domain.CreateDepartmentInput{WorkspaceID: ws1, Name: "Second", ParentID: "dept-root-1"})
	require.NoError(t, err)
	assert.Equal(t, again+9, w.Calls(), "the parent's nodes exist now: only the new department's are written (UA + 8 for its own)")
}

type countingFinder struct {
	*testutil.FakePolicyWrite
	mu    sync.Mutex
	finds map[string]int
	err   error
}

func (c *countingFinder) FindNodeByName(ctx context.Context, in *policypb.FindNodeByNameRequest, o ...grpc.CallOption) (*policypb.NGACNode, error) {
	c.mu.Lock()
	c.finds[in.Name]++
	err := c.err
	c.mu.Unlock()
	if err != nil {
		return nil, err
	}
	return c.FakePolicyWrite.FindNodeByName(ctx, in, o...)
}

func newCountingWorld(t *testing.T) (*countingFinder, *fixture) {
	t.Helper()
	cf := &countingFinder{FakePolicyWrite: testutil.NewFakePolicyWrite(), finds: map[string]int{}}
	f := newFixture(t)
	cf.Reads = f.read
	f.svc = domain.NewService(f.wsStore, f.depts, f.read, cf, nil, nil)
	return cf, f
}

// A lookup that finds a node is remembered; one that does not is asked again, so a
// directory built after a miss is picked up without a restart.
func TestDirectoryLookup_CachesHitsAndNeverMisses(t *testing.T) {
	cf, f := newCountingWorld(t)
	directory := ngac.DirectoryOAName(ngac.WorkspaceID(ws1))
	sales := func() { createSales(t, f, owner, ws1) }

	sales()
	sales()
	assert.Equal(t, 2, cf.finds[directory], "a miss is not cached: the second department asks again")

	seedFakeDirectory(cf.FakePolicyWrite, ws1)
	sales()
	assert.Equal(t, 3, cf.finds[directory], "the lookup that finds it")
	sales()
	sales()
	assert.Equal(t, 3, cf.finds[directory], "a hit is cached for the life of the service")
	assert.Equal(t, 1, cf.finds[ngac.DirectoryOpenOAName(ngac.WorkspaceID(ws1))])
}

// The directory is trusted only when the node carries the directory mark of this
// workspace; a lookup that cannot say is not read as "none". Either way nothing is
// written.
func TestDirectoryLookup_FailsClosed(t *testing.T) {
	id := ngac.WorkspaceID(ws1)
	cases := map[string]func(cf *countingFinder){
		"a node with the name but no directory mark": func(cf *countingFinder) {
			cf.Seed(&policypb.NGACNode{Id: "x", Name: ngac.DirectoryOAName(id), NodeType: ngac.TypeOA})
		},
		"a node marked for another workspace": func(cf *countingFinder) {
			cf.Seed(&policypb.NGACNode{Id: "x", Name: ngac.DirectoryOAName(id), NodeType: ngac.TypeOA, Properties: ngac.DirectoryProps(ngac.WorkspaceID(ws2))})
		},
		"the policy service cannot answer": func(cf *countingFinder) { cf.err = status.Error(codes.Unavailable, "down") },
	}
	for name, setup := range cases {
		t.Run(name, func(t *testing.T) {
			cf, f := newCountingWorld(t)
			setup(cf)
			_, err := f.svc.CreateDepartment(context.Background(), owner, domain.CreateDepartmentInput{WorkspaceID: ws1, Name: "Sales"})
			require.ErrorIs(t, err, domain.ErrUnavailable)
			assert.Zero(t, cf.Calls(), "nothing was written")
			assert.Empty(t, f.depts.mutations)
		})
	}

	// ALLOW on the same fixture: with the real, marked directory the department is created.
	cf, f := newCountingWorld(t)
	seedFakeDirectory(cf.FakePolicyWrite, ws1)
	createSales(t, f, owner, ws1)
	assert.Contains(t, cf.LiveNodes(), "OA "+ngac.DirectoryOAName(id), "fixture sanity")
}

// A person's department change writes the peers edge with the department edge, and
// takes it back when a later step fails.
func TestUpdateMemberDepartment_WritesAndUndoesThePeersEdge(t *testing.T) {
	newWorld := func() (*testutil.FakePolicyWrite, *fixture) {
		w := testutil.NewFakePolicyWrite()
		f := newWriteFixture(t, w)
		seedFakeDirectory(w, ws1)
		for _, d := range []string{"dept-root-1", "dept-other-1"} {
			seedFakeDeptDirectory(w, ws1, d)
		}
		return w, f
	}
	oldPeers := &policypb.NGACNode{Id: "peers-dept-other-1", Name: ngac.DeptPeersUAName("dept-other-1"), NodeType: ngac.TypeUA}
	oldDept := &policypb.NGACNode{Id: "ua-dept-other-1", Name: ngac.DeptUAName("dept-other-1"), NodeType: ngac.TypeUA}

	w, f := newWorld()
	f.read.parents[target] = []*policypb.NGACNode{oldDept, oldPeers, node(owners1, "o", ngac.TypeUA)}
	require.NoError(t, f.svc.UpdateMemberDepartment(context.Background(), owner, ws1, target, "dept-root-1"))
	log := w.Log()
	assert.Contains(t, log, "assign "+target+">ua-dept-root-1")
	assert.Contains(t, log, "assign "+target+">"+ngac.DeptPeersUAName("dept-root-1"), "the new department's peers edge")
	assert.Contains(t, log, "unassign "+target+">ua-dept-other-1")
	assert.Contains(t, log, "unassign "+target+">"+ngac.DeptPeersUAName("dept-other-1"), "and the old one's is taken away")
	assert.NotContains(t, log, "unassign "+target+">"+owners1, "Owners is not touched")
	assert.Equal(t, []string{"user-dept " + ws1 + " " + target + " dept-root-1"}, f.depts.mutations)

	// Out of any department: both of the old edges go, none is added.
	w, f = newWorld()
	f.read.parents[target] = []*policypb.NGACNode{oldDept, oldPeers}
	require.NoError(t, f.svc.UpdateMemberDepartment(context.Background(), owner, ws1, target, ""))
	for _, l := range w.Log() {
		assert.False(t, strings.HasPrefix(l, "assign "+target+">"), "no edge is added: %s", l)
	}
	assert.Contains(t, w.Log(), "unassign "+target+">"+ngac.DeptPeersUAName("dept-other-1"))

	// The peers edge cannot be added (second write): the department edge is taken back, the row is untouched.
	w, f = newWorld()
	w.FailAt = 2
	err := f.svc.UpdateMemberDepartment(context.Background(), owner, ws1, target, "dept-root-1")
	require.Error(t, err)
	assert.Contains(t, w.Log(), "unassign "+target+">ua-dept-root-1", "the new department edge is undone")
	assert.Empty(t, f.depts.mutations, "the row follows the graph, which did not change")
}

// Deleting a department moves its sub-departments' directory nodes and its people's
// peers edges up, and removes its own three nodes last.
func TestDeleteDepartment_MovesTheDirectoryUpAndRemovesItsNodes(t *testing.T) {
	w := testutil.NewFakePolicyWrite()
	f := newWriteFixture(t, w)
	seedFakeDirectory(w, ws1)
	for _, d := range []string{"dept-root-1", "dept-child-1"} {
		seedFakeDeptDirectory(w, ws1, d)
	}
	f.read.children["ua-dept-child-1"] = []*policypb.NGACNode{node(target, "target", ngac.TypeU)}

	require.NoError(t, f.svc.DeleteDepartment(context.Background(), owner, ws1, "dept-child-1"))

	log := w.Log()
	assert.Contains(t, log, "assign "+target+">"+ngac.DeptPeersUAName("dept-root-1"), "its people become the parent's peers")
	live := w.LiveNodes()
	for _, gone := range []string{"OA " + ngac.DeptDirOAName("dept-child-1"), "OA " + ngac.DeptPeerOAName("dept-child-1"), "UA " + ngac.DeptPeersUAName("dept-child-1")} {
		assert.NotContains(t, live, gone)
	}
	assert.Contains(t, live, "OA "+ngac.DeptDirOAName("dept-root-1"), "the parent's nodes stay")

	// A root with a child: the child's nodes go to Open and Directory.
	w2 := testutil.NewFakePolicyWrite()
	f2 := newWriteFixture(t, w2)
	seedFakeDirectory(w2, ws1)
	for _, d := range []string{"dept-root-1", "dept-child-1"} {
		seedFakeDeptDirectory(w2, ws1, d)
	}
	require.NoError(t, f2.svc.DeleteDepartment(context.Background(), owner, ws1, "dept-root-1"))
	log = w2.Log()
	assert.Contains(t, log, "assign "+ngac.DeptDirOAName("dept-child-1")+">"+ngac.DirectoryOpenOAName(ngac.WorkspaceID(ws1)))
	assert.Contains(t, log, "assign "+ngac.DeptPeerOAName("dept-child-1")+">"+ngac.DirectoryOAName(ngac.WorkspaceID(ws1)))
	assert.NotContains(t, w2.LiveNodes(), "OA "+ngac.DeptDirOAName("dept-root-1"))
}

// Placing a person in, or a department under, a hidden department needs read on its
// Dir OA, which manage on that OA stands in for: the directory administrator holds
// it through Mgmt. The stand-in applies to a directory OA and to read only.
func TestGuardDelegation_ADirectoryAdministratorMayHandOutReadOfADirectoryOA(t *testing.T) {
	const dirOA = "oa-dir-hidden"
	setup := func(props map[string]string, ops ...string) *adminFixture {
		f := newAdminFixture(t)
		f.read.nodes[dirOA] = &policypb.NGACNode{Id: dirOA, Name: "Dir_hidden", NodeType: ngac.TypeOA, Properties: props}
		f.read.assocs["ua-dept-root-1"] = []*policypb.Association{{Id: "as-h", UaId: "ua-dept-root-1", OaId: dirOA, Operations: ops}}
		return f
	}
	directoryMark := ngac.DirectoryProps(ngac.WorkspaceID(ws1))

	// DENY: the caller holds neither read nor manage on the hidden Dir (the manager
	// holds manage on Mgmt only, in the fixture).
	f := setup(directoryMark, ngac.OpRead)
	err := f.svc.UpdateMemberDepartment(ctx(), manager, ws1, target, "dept-root-1")
	require.ErrorIs(t, err, domain.ErrAccessDenied, "manage on Mgmt alone, with no grant on the Dir, is not enough")
	assert.False(t, f.mutated())

	// ALLOW, same fixture: manage on the directory OA.
	f.read.grants[grant{manager, dirOA, ngac.OpManage}] = true
	require.NoError(t, f.svc.UpdateMemberDepartment(ctx(), manager, ws1, target, "dept-root-1"))

	// Manage stands in for read and for nothing else.
	f = setup(directoryMark, ngac.OpWrite)
	f.read.grants[grant{manager, dirOA, ngac.OpManage}] = true
	err = f.svc.UpdateMemberDepartment(ctx(), manager, ws1, target, "dept-root-1")
	require.ErrorIs(t, err, domain.ErrAccessDenied, "manage does not stand in for write")
	assert.False(t, f.mutated())

	// Nor on an OA that is not part of the directory.
	f = setup(nil, ngac.OpRead)
	f.read.grants[grant{manager, dirOA, ngac.OpManage}] = true
	err = f.svc.UpdateMemberDepartment(ctx(), manager, ws1, target, "dept-root-1")
	require.ErrorIs(t, err, domain.ErrAccessDenied, "manage does not stand in for read of an OA with no directory mark")
	assert.False(t, f.mutated())

	// Reading it for real is still enough.
	f = setup(nil, ngac.OpRead)
	f.read.grants[grant{manager, dirOA, ngac.OpRead}] = true
	require.NoError(t, f.svc.UpdateMemberDepartment(ctx(), manager, ws1, target, "dept-root-1"))

	// A directory OA that cannot be read to check its mark is not assumed to be one.
	f = setup(directoryMark, ngac.OpRead)
	delete(f.read.nodes, dirOA)
	f.read.grants[grant{manager, dirOA, ngac.OpManage}] = true
	err = f.svc.UpdateMemberDepartment(ctx(), manager, ws1, target, "dept-root-1")
	require.ErrorIs(t, err, domain.ErrAccessDenied)
}

// staleParent answers NotFound to an assignment under a node that no longer exists, as the
// policy service does.
type staleParent struct {
	*testutil.FakePolicyWrite
	gone map[string]bool
	used []string
}

func (c *staleParent) CreateAssignment(ctx context.Context, in *policypb.CreateAssignmentRequest, o ...grpc.CallOption) (*policypb.Assignment, error) {
	if c.gone[in.ParentId] {
		return nil, status.Error(codes.NotFound, "node not found")
	}
	c.used = append(c.used, in.ParentId)
	return c.FakePolicyWrite.CreateAssignment(ctx, in, o...)
}

// A cached id whose node was deleted and rebuilt (a migration reversed and applied again) is
// dropped when a write using it is answered NotFound, and the operation is tried once more
// with a fresh lookup.
func TestDirectoryLookup_ADroppedNodeIsLookedUpAgain(t *testing.T) {
	w := testutil.NewFakePolicyWrite()
	sp := &staleParent{FakePolicyWrite: w, gone: map[string]bool{}}
	f := newFixture(t)
	w.Reads = f.read
	f.svc = domain.NewService(f.wsStore, f.depts, f.read, sp, nil, nil)
	seedFakeDirectory(w, ws1)
	createSales(t, f, owner, ws1) // caches the ids

	oldOpen := "dir-open-" + ws1
	w.DeleteNode(context.Background(), &policypb.DeleteNodeRequest{NodeId: oldOpen})
	w.Seed(&policypb.NGACNode{Id: "open-rebuilt", Name: ngac.DirectoryOpenOAName(ngac.WorkspaceID(ws1)), NodeType: ngac.TypeOA, Properties: ngac.DirectoryProps(ngac.WorkspaceID(ws1))})
	sp.gone[oldOpen] = true
	sp.used = nil

	createSales(t, f, owner, ws1)
	assert.Contains(t, sp.used, "open-rebuilt", "the retry found the rebuilt node")
	assert.NotContains(t, sp.used, oldOpen)
}

// A department that moves leaves graph, directory and row consistent: when its directory
// nodes cannot be moved, the department goes back where it was.
func TestMoveDepartment_UndoesTheMoveWhenTheDirectoryCannotFollow(t *testing.T) {
	newWorld := func() (*testutil.FakePolicyWrite, *fixture) {
		w := testutil.NewFakePolicyWrite()
		f := newWriteFixture(t, w)
		seedFakeDirectory(w, ws1)
		for _, d := range []string{"dept-root-1", "dept-other-1"} {
			seedFakeDeptDirectory(w, ws1, d)
		}
		return w, f
	}
	in := domain.MoveDepartmentInput{WorkspaceID: ws1, DeptID: "dept-other-1", NewParentID: "dept-root-1"}

	w, f := newWorld()
	_, err := f.svc.MoveDepartment(context.Background(), owner, in)
	require.NoError(t, err)
	steps := w.Calls()
	require.Greater(t, steps, 3, "the department's two edges, then its directory nodes")

	// Fail each directory write in turn.
	for k := 3; k <= steps; k++ {
		w, f = newWorld()
		w.FailAt = k
		_, err = f.svc.MoveDepartment(context.Background(), owner, in)
		require.Errorf(t, err, "failure at write %d", k)
		log := w.Log()
		assert.Contains(t, log, "unassign ua-dept-other-1>ua-dept-root-1", "the new parent edge is taken back (write %d)", k)
		assert.Contains(t, log, "assign ua-dept-other-1>"+pc1, "and the old one restored (write %d)", k)
		for _, m := range f.depts.mutations {
			assert.NotContains(t, m, "move", "the row did not move (write %d)", k)
		}
	}
}
