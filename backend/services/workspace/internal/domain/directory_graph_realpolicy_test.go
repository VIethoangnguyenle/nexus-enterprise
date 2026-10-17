//go:build realpolicy

package domain_test

import (
	"context"
	"slices"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"ngac-platform/ngac"
	policypb "ngac-platform/proto/policy"
	"ngac-platform/services/workspace/internal/domain"
	"ngac-platform/services/workspace/internal/store"
	"ngac-platform/testutil"
)

// The contacts directory graph (docs/specs/contacts-visibility) kept by the
// workspace service, against the real policy service: a workspace is born with
// its directory nodes, every department mirrors into them, and a person who
// changes department or leaves loses exactly the edges that gave them a view.
// Each DENY is asserted on the same fixture as its ALLOW.

type dirWorld struct {
	t     *testing.T
	read  policypb.PolicyReadServiceClient
	write policypb.PolicyWriteServiceClient
	pool  *pgxpool.Pool
	svc   *domain.Service
	ws    *domain.WorkspaceResult
	id    ngac.WorkspaceID
	owner wsPerson
}

func (w *dirWorld) admin() context.Context { return testutil.PolicyAdmin(context.Background()) }

func (w *dirWorld) as(p wsPerson) context.Context {
	return testutil.PolicyCaller(context.Background(), p.node)
}

func (w *dirWorld) newPerson(parents ...string) wsPerson {
	w.t.Helper()
	node := testutil.NewUser(w.t, w.write, parents...)
	id := "dir-rp-" + uuid.NewString()
	_, err := w.pool.Exec(context.Background(),
		`INSERT INTO users (id, username, password, ngac_node, display_name) VALUES ($1, $1, '', $2, $1)`, id, node)
	require.NoError(w.t, err)
	w.t.Cleanup(func() { w.pool.Exec(context.Background(), `DELETE FROM users WHERE id = $1`, id) })
	return wsPerson{id: id, node: node}
}

// newDirWorld makes a workspace through CreateWorkspace, the way signup does.
func newDirWorld(t *testing.T, typ string) *dirWorld {
	t.Helper()
	read, write := testutil.StartPolicy(t)
	w := &dirWorld{t: t, read: read, write: write, pool: testutil.SetupTestDB(t)}
	st := store.New(w.pool)
	w.svc = domain.NewService(st, st, read, write, nil, nil)
	w.owner = w.newPerson()
	ws, err := w.svc.CreateWorkspace(w.as(w.owner), domain.CreateWorkspaceInput{
		Name: "Dir " + uuid.NewString()[:8], Type: typ, UserID: w.owner.id, UserNGACNodeID: w.owner.node,
	})
	require.NoError(t, err)
	w.ws, w.id = ws, ngac.WorkspaceID(ws.ID)
	t.Cleanup(w.teardown)
	return w
}

// teardown removes what the service built: its prohibition (which has no foreign
// key to the graph), the rows and every node below the class.
func (w *dirWorld) teardown() {
	purgeWorkspace(w.write, w.pool, w.ws.ID, w.ws.PcNodeID)
}

func purgeWorkspace(write policypb.PolicyWriteServiceClient, pool *pgxpool.Pool, wsID, pcID string) {
	admin := testutil.PolicyAdmin(context.Background())
	_, _ = write.RemoveProhibition(admin, &policypb.RemoveProhibitionRequest{Name: ngac.DirectoryLimitedProhibitionName(ngac.WorkspaceID(wsID))})
	_, _ = pool.Exec(context.Background(), `DELETE FROM workspaces WHERE id = $1`, wsID)
	desc, err := write.GetDescendants(admin, &policypb.GetDescendantsRequest{NodeId: pcID})
	if err == nil {
		for _, n := range desc.GetNodes() {
			if n.GetNodeType() != ngac.TypeU {
				_, _ = write.DeleteNode(admin, &policypb.DeleteNodeRequest{NodeId: n.GetId()})
			}
		}
	}
	_, _ = write.DeleteNode(admin, &policypb.DeleteNodeRequest{NodeId: pcID})
}

func (w *dirWorld) find(name, nodeType string) (string, bool) {
	w.t.Helper()
	n, err := w.write.FindNodeByName(w.admin(), &policypb.FindNodeByNameRequest{Name: name, NodeType: nodeType})
	if status.Code(err) == codes.NotFound {
		return "", false
	}
	require.NoError(w.t, err, "look up %s", name)
	return n.GetId(), true
}

func (w *dirWorld) node(name, nodeType string) string {
	w.t.Helper()
	id, ok := w.find(name, nodeType)
	require.True(w.t, ok, "%s %s must exist", nodeType, name)
	return id
}

func (w *dirWorld) parents(id string) []string {
	w.t.Helper()
	r, err := w.write.GetParents(w.admin(), &policypb.GetParentsRequest{NodeId: id})
	require.NoError(w.t, err)
	var out []string
	for _, n := range r.GetNodes() {
		out = append(out, n.GetName())
	}
	slices.Sort(out)
	return out
}

func (w *dirWorld) under(child, parent string) bool {
	w.t.Helper()
	r, err := w.write.IsAssigned(w.admin(), &policypb.IsAssignedRequest{ChildId: child, ParentId: parent})
	require.NoError(w.t, err)
	return r.GetValue()
}

func (w *dirWorld) can(p wsPerson, oa, op string) bool {
	w.t.Helper()
	return testutil.Allowed(w.t, w.read, p.node, oa, op)
}

func (w *dirWorld) directory() string { return w.node(ngac.DirectoryOAName(w.id), ngac.TypeOA) }
func (w *dirWorld) open() string      { return w.node(ngac.DirectoryOpenOAName(w.id), ngac.TypeOA) }
func (w *dirWorld) owners() string    { return w.node(ngac.DirectoryOwnersOAName(w.id), ngac.TypeOA) }
func (w *dirWorld) limited() string   { return w.node(ngac.DirectoryLimitedUAName(w.id), ngac.TypeUA) }

func (w *dirWorld) dir(d *domain.DepartmentResult) string {
	return w.node(ngac.DeptDirOAName(ngac.DeptID(d.ID)), ngac.TypeOA)
}
func (w *dirWorld) peer(d *domain.DepartmentResult) string {
	return w.node(ngac.DeptPeerOAName(ngac.DeptID(d.ID)), ngac.TypeOA)
}
func (w *dirWorld) peers(d *domain.DepartmentResult) string {
	return w.node(ngac.DeptPeersUAName(ngac.DeptID(d.ID)), ngac.TypeUA)
}

func (w *dirWorld) dirName(d *domain.DepartmentResult) string {
	return ngac.DeptDirOAName(ngac.DeptID(d.ID))
}
func (w *dirWorld) peerName(d *domain.DepartmentResult) string {
	return ngac.DeptPeerOAName(ngac.DeptID(d.ID))
}

func (w *dirWorld) dept(name string, parent *domain.DepartmentResult) *domain.DepartmentResult {
	w.t.Helper()
	in := domain.CreateDepartmentInput{WorkspaceID: w.ws.ID, Name: name}
	if parent != nil {
		in.ParentID = parent.ID
	}
	d, err := w.svc.CreateDepartment(w.as(w.owner), w.owner.node, in)
	require.NoError(w.t, err, "create department %s", name)
	return d
}

func (w *dirWorld) member(parents ...string) wsPerson {
	w.t.Helper()
	return w.newPerson(append([]string{w.ws.MembersUaID}, parents...)...)
}

// place puts a member in a department the way the admin screen does.
func (w *dirWorld) place(p wsPerson, d *domain.DepartmentResult) {
	w.t.Helper()
	deptID := ""
	if d != nil {
		deptID = d.ID
	}
	require.NoError(w.t, w.svc.UpdateMemberDepartment(w.as(w.owner), w.owner.node, w.ws.ID, p.node, deptID))
}

// hide does to the graph what hiding a department does (docs/specs/contacts-visibility):
// its Dir and DirPeer move to sit directly under Directory, and the department's
// own UA reads the Dir. The endpoint that does it ships with the rules API; the
// upkeep under test has to keep this shape.
func (w *dirWorld) hide(d *domain.DepartmentResult) {
	w.t.Helper()
	directory := w.directory()
	for _, oa := range []string{w.dir(d), w.peer(d)} {
		r, err := w.write.GetParents(w.admin(), &policypb.GetParentsRequest{NodeId: oa})
		require.NoError(w.t, err)
		for _, p := range r.GetNodes() {
			testutil.Unassign(w.t, w.write, oa, p.GetId())
		}
		testutil.Assign(w.t, w.write, oa, directory)
	}
	testutil.Associate(w.t, w.write, d.NGACUaID, w.dir(d), []string{ngac.OpRead})
}

// A workspace made by CreateWorkspace has the directory, whatever its type, with
// the default visibility in force and nothing that makes anyone a member.
func TestRealPolicy_NewWorkspaceCarriesTheDirectory(t *testing.T) {
	for _, typ := range []string{domain.TypeOrganization, domain.TypePersonal} {
		t.Run(typ, func(t *testing.T) {
			w := newDirWorld(t, typ)
			other := newDirWorld(t, domain.TypeOrganization)
			directory, open, owners, limited := w.directory(), w.open(), w.owners(), w.limited()
			noDept := w.node(ngac.DirectoryNoDeptOAName(w.id), ngac.TypeOA)

			// Shape.
			assert.Equal(t, []string{ngac.MgmtOAName(w.id)}, w.parents(directory))
			assert.Equal(t, []string{ngac.DirectoryOAName(w.id)}, w.parents(open))
			assert.Equal(t, []string{ngac.DirectoryOpenOAName(w.id)}, w.parents(noDept))
			assert.Equal(t, []string{ngac.DirectoryOAName(w.id)}, w.parents(owners), "owners sit beside Open, never under it")
			assert.Equal(t, []string{ngac.PCName(w.id)}, w.parents(limited), "Limited hangs under the class, not under Members")
			for _, n := range []string{directory, open, owners, noDept} {
				got, err := w.write.GetNode(w.admin(), &policypb.GetNodeRequest{NodeId: n})
				require.NoError(t, err)
				assert.Equal(t, ngac.PropTypeDirectory, got.GetProperties()[ngac.PropType])
				assert.Equal(t, w.ws.ID, got.GetProperties()["workspace_id"])
			}

			plain := w.member()
			limitedPerson := w.member(limited)
			foreign := other.member()

			// DENY first, each paired with an ALLOW on the same fixture.
			assert.False(t, w.can(foreign, open, ngac.OpRead), "another workspace's member reads nothing of this directory")
			assert.False(t, w.can(foreign, owners, ngac.OpRead))
			assert.False(t, w.can(plain, directory, ngac.OpManage), "an ordinary member does not administer the directory")
			assert.False(t, w.can(plain, directory, ngac.OpRead), "one hop short: the root itself is not open")
			assert.False(t, w.can(limitedPerson, open, ngac.OpRead), "the prohibition takes the open tree from a limited person")
			assert.False(t, w.can(limitedPerson, noDept, ngac.OpRead))

			assert.True(t, w.can(plain, open, ngac.OpRead))
			assert.True(t, w.can(plain, noDept, ngac.OpRead))
			assert.True(t, w.can(plain, owners, ngac.OpRead))
			assert.True(t, w.can(limitedPerson, owners, ngac.OpRead), "a limited person always sees the owners")
			assert.True(t, w.can(w.owner, directory, ngac.OpManage), "owners administer the directory")

			// The only rule: Limited, read, Open, not an intersection.
			rules, err := w.read.ListProhibitions(w.admin(), &policypb.ListProhibitionsRequest{SubjectId: limited})
			require.NoError(t, err)
			require.Len(t, rules.GetProhibitions(), 1)
			rule := rules.GetProhibitions()[0]
			assert.Equal(t, ngac.DirectoryLimitedProhibitionName(w.id), rule.GetName())
			assert.Equal(t, []string{ngac.OpRead}, rule.GetOperations())
			assert.Equal(t, []string{open}, rule.GetTargetOaIds())
			assert.False(t, rule.GetIntersection())
		})
	}
}

// Departments mirror into the directory, a person placed directly in one reads its
// peer subtree and nothing above or beside it, and the peers UA is not membership.
func TestRealPolicy_DepartmentsMirrorIntoTheDirectory(t *testing.T) {
	w := newDirWorld(t, domain.TypeOrganization)
	a := w.dept("A", nil)
	a1 := w.dept("A1", a)
	a2 := w.dept("A2", a)

	assert.Equal(t, []string{ngac.DirectoryOpenOAName(w.id)}, w.parents(w.dir(a)))
	assert.Equal(t, []string{w.dirName(a)}, w.parents(w.dir(a1)))
	assert.Equal(t, []string{ngac.DirectoryOAName(w.id)}, w.parents(w.peer(a)))
	assert.Equal(t, []string{w.peerName(a)}, w.parents(w.peer(a1)))
	assert.Equal(t, []string{ngac.PCName(w.id)}, w.parents(w.peers(a1)), "the peers UA is flat, directly under the class")

	pA, pA1, pA2 := w.member(), w.member(), w.member()
	w.place(pA, a)
	w.place(pA1, a1)
	w.place(pA2, a2)
	assert.True(t, w.under(pA1.node, w.peers(a1)), "a person placed in a department is under its peers UA")

	// DENY: one hop short and the sibling; the parent's peers read the subtree below.
	assert.False(t, w.can(pA1, w.peer(a), ngac.OpRead), "A1's people do not read the parent's peer OA")
	assert.False(t, w.can(pA1, w.peer(a2), ngac.OpRead), "nor a sibling's")
	assert.False(t, w.can(pA2, w.peer(a1), ngac.OpRead))
	// ALLOW on the same fixture.
	assert.True(t, w.can(pA1, w.peer(a1), ngac.OpRead))
	assert.True(t, w.can(pA, w.peer(a), ngac.OpRead))
	assert.True(t, w.can(pA, w.peer(a1), ngac.OpRead), "the parent's people read the sub-department's peer OA below it")
	assert.True(t, w.can(pA1, w.dir(a2), ngac.OpRead), "everyone reads the open tree")

	// Peers are not members: someone only under a peers UA reads nothing.
	ghost := w.newPerson(w.peers(a1))
	assert.False(t, w.can(ghost, w.open(), ngac.OpRead))
	assert.False(t, w.can(ghost, w.owners(), ngac.OpRead))
	_, err := w.svc.ViewWorkspace(w.as(ghost), ghost.node, w.ws.ID)
	assert.ErrorIs(t, err, domain.ErrAccessDenied, "and the workspace does not take it for a member")
	_, err = w.svc.ViewWorkspace(w.as(pA1), pA1.node, w.ws.ID)
	assert.NoError(t, err, "while a person placed in the same department is one")
	assert.True(t, w.can(ghost, w.peer(a1), ngac.OpRead), "the peers UA grants exactly its own peer OA")
}

// Moving a department moves its directory nodes with it: the old parent's people
// stop reading it as a sub-department of theirs, the new parent's start.
func TestRealPolicy_MovingADepartmentMovesItsDirectoryNodes(t *testing.T) {
	w := newDirWorld(t, domain.TypeOrganization)
	a := w.dept("A", nil)
	b := w.dept("B", nil)
	a1 := w.dept("A1", a)
	pA, pB := w.member(), w.member()
	w.place(pA, a)
	w.place(pB, b)

	assert.True(t, w.can(pA, w.peer(a1), ngac.OpRead))
	assert.False(t, w.can(pB, w.peer(a1), ngac.OpRead))

	_, err := w.svc.MoveDepartment(w.as(w.owner), w.owner.node, domain.MoveDepartmentInput{WorkspaceID: w.ws.ID, DeptID: a1.ID, NewParentID: b.ID})
	require.NoError(t, err)

	assert.Equal(t, []string{w.dirName(b)}, w.parents(w.dir(a1)), "exactly one parent: the old edge is gone")
	assert.Equal(t, []string{w.peerName(b)}, w.parents(w.peer(a1)))
	assert.False(t, w.can(pA, w.peer(a1), ngac.OpRead), "A's people no longer read it")
	assert.True(t, w.can(pB, w.peer(a1), ngac.OpRead), "B's people do")

	_, err = w.svc.MoveDepartment(w.as(w.owner), w.owner.node, domain.MoveDepartmentInput{WorkspaceID: w.ws.ID, DeptID: a1.ID})
	require.NoError(t, err, "to the root")
	assert.Equal(t, []string{ngac.DirectoryOpenOAName(w.id)}, w.parents(w.dir(a1)))
	assert.Equal(t, []string{ngac.DirectoryOAName(w.id)}, w.parents(w.peer(a1)))
	assert.False(t, w.can(pB, w.peer(a1), ngac.OpRead))
}

// A hidden department stays hidden when it is moved or when something is created
// under it: its Dir and its DirPeer both stay under Directory.
func TestRealPolicy_HiddenDepartmentsStayHidden(t *testing.T) {
	w := newDirWorld(t, domain.TypeOrganization)
	a := w.dept("A", nil)
	h := w.dept("H", nil)
	w.hide(h)
	pA, pH := w.member(), w.member()
	w.place(pA, a)
	w.place(pH, h)

	assert.False(t, w.can(pA, w.dir(h), ngac.OpRead), "fixture: H is hidden from A's people")
	assert.True(t, w.can(pH, w.dir(h), ngac.OpRead))

	// A department created under a hidden one is hidden with it.
	h1 := w.dept("H1", h)
	pH1 := w.member()
	w.place(pH1, h1)
	assert.Equal(t, []string{w.dirName(h)}, w.parents(w.dir(h1)))
	assert.Equal(t, []string{w.peerName(h)}, w.parents(w.peer(h1)))
	assert.False(t, w.can(pA, w.dir(h1), ngac.OpRead))
	assert.True(t, w.can(pH1, w.dir(h1), ngac.OpRead))
	assert.True(t, w.can(pH, w.dir(h1), ngac.OpRead), "H's people read their sub-departments")

	// Moving H under A leaves both its nodes under Directory: A's people, who sit
	// directly in the new parent, still read neither.
	_, err := w.svc.MoveDepartment(w.as(w.owner), w.owner.node, domain.MoveDepartmentInput{WorkspaceID: w.ws.ID, DeptID: h.ID, NewParentID: a.ID})
	require.NoError(t, err)
	assert.Equal(t, []string{ngac.DirectoryOAName(w.id)}, w.parents(w.dir(h)), "Dir stays under Directory")
	assert.Equal(t, []string{ngac.DirectoryOAName(w.id)}, w.parents(w.peer(h)), "DirPeer stays under Directory too")
	assert.False(t, w.can(pA, w.dir(h), ngac.OpRead), "the new parent's people still do not see it")
	assert.False(t, w.can(pA, w.peer(h), ngac.OpRead), "nor through the peer tree")
	assert.False(t, w.can(pA, w.peer(h1), ngac.OpRead))
	assert.True(t, w.can(pH, w.peer(h), ngac.OpRead))
	assert.True(t, w.can(pH1, w.dir(h), ngac.OpRead), "its own people keep their view")
}

// Deleting a department moves its sub-departments and its people up, and removes
// its three nodes; a hidden department's sub-departments stay hidden.
func TestRealPolicy_DeletingADepartmentMovesItsSubtreeUp(t *testing.T) {
	w := newDirWorld(t, domain.TypeOrganization)
	a := w.dept("A", nil)
	a1 := w.dept("A1", a)
	a11 := w.dept("A11", a1)
	pA, pA1, pA11 := w.member(), w.member(), w.member()
	w.place(pA, a)
	w.place(pA1, a1)
	w.place(pA11, a11)

	// A middle department: its people move up into A, its sub-department under A.
	require.NoError(t, w.svc.DeleteDepartment(w.as(w.owner), w.owner.node, w.ws.ID, a1.ID))
	for _, name := range []string{w.dirName(a1), w.peerName(a1)} {
		_, ok := w.find(name, ngac.TypeOA)
		assert.False(t, ok, "%s is gone", name)
	}
	_, ok := w.find(ngac.DeptPeersUAName(ngac.DeptID(a1.ID)), ngac.TypeUA)
	assert.False(t, ok, "the peers UA is gone")
	assert.Equal(t, []string{w.dirName(a)}, w.parents(w.dir(a11)))
	assert.Equal(t, []string{w.peerName(a)}, w.parents(w.peer(a11)))
	assert.True(t, w.under(pA1.node, w.peers(a)), "the deleted department's people are A's peers now")
	assert.True(t, w.can(pA1, w.peer(a), ngac.OpRead))
	assert.False(t, w.can(pA11, w.peer(a), ngac.OpRead), "the sub-department's people stay below A")
	assert.True(t, w.can(pA11, w.peer(a11), ngac.OpRead))

	// A top-level department: sub-departments go to the root, its people to none.
	require.NoError(t, w.svc.DeleteDepartment(w.as(w.owner), w.owner.node, w.ws.ID, a.ID))
	assert.Equal(t, []string{ngac.DirectoryOpenOAName(w.id)}, w.parents(w.dir(a11)))
	assert.Equal(t, []string{ngac.DirectoryOAName(w.id)}, w.parents(w.peer(a11)))
	parentsOfA := w.parents(pA.node)
	for _, n := range parentsOfA {
		assert.NotContains(t, n, "DeptPeers_", "a person left with no department sits under no peers UA")
	}
	assert.True(t, w.can(pA11, w.peer(a11), ngac.OpRead))
	assert.False(t, w.can(pA, w.peer(a11), ngac.OpRead), "someone with no department reads no peer subtree")
}

func TestRealPolicy_DeletingAHiddenDepartmentKeepsItsSubDepartmentsHidden(t *testing.T) {
	w := newDirWorld(t, domain.TypeOrganization)
	a := w.dept("A", nil)
	h := w.dept("H", a)
	h1 := w.dept("H1", h)
	w.hide(h)
	pA, pH1 := w.member(), w.member()
	w.place(pA, a)
	w.place(pH1, h1)
	require.False(t, w.can(pA, w.dir(h1), ngac.OpRead), "fixture: H1 is hidden with H")
	require.True(t, w.can(pH1, w.dir(h1), ngac.OpRead))

	require.NoError(t, w.svc.DeleteDepartment(w.as(w.owner), w.owner.node, w.ws.ID, h.ID))

	assert.False(t, w.can(pA, w.dir(h1), ngac.OpRead), "deleting the hidden parent does not publish what it hid")
	assert.False(t, w.can(pA, w.peer(h1), ngac.OpRead))
	assert.True(t, w.can(pH1, w.dir(h1), ngac.OpRead), "H1's own people keep their view")
	assert.True(t, w.can(pH1, w.peer(h1), ngac.OpRead))
}

// A change of department swaps the peers edge with it, in every direction.
func TestRealPolicy_ChangingDepartmentSwapsThePeersEdge(t *testing.T) {
	w := newDirWorld(t, domain.TypeOrganization)
	a1 := w.dept("A1", nil)
	a2 := w.dept("A2", nil)
	p := w.member()

	w.place(p, a1)
	assert.True(t, w.under(p.node, w.peers(a1)))
	assert.True(t, w.can(p, w.peer(a1), ngac.OpRead))

	w.place(p, a2)
	assert.False(t, w.under(p.node, w.peers(a1)), "the old department's peers edge is gone")
	assert.False(t, w.can(p, w.peer(a1), ngac.OpRead))
	assert.True(t, w.under(p.node, w.peers(a2)))
	assert.True(t, w.can(p, w.peer(a2), ngac.OpRead))

	w.place(p, nil)
	assert.False(t, w.under(p.node, w.peers(a2)), "out of any department: no peers edge at all")
	assert.False(t, w.can(p, w.peer(a2), ngac.OpRead))
	assert.True(t, w.can(p, w.open(), ngac.OpRead), "still a member")

	w.place(p, a1)
	assert.True(t, w.can(p, w.peer(a1), ngac.OpRead), "and back in")
}

// Someone removed from the workspace keeps no view: not the department edge (the
// association on a hidden Dir is on the department UA), not the peers edge, not
// Limited, not Members.
func TestRealPolicy_ARemovedMemberKeepsNoEdgeThatGaveAView(t *testing.T) {
	w := newDirWorld(t, domain.TypeOrganization)
	h := w.dept("H", nil)
	w.hide(h)
	leaver := w.member(w.limited())
	stray := w.newPerson() // never a member, but holds a stale department edge
	w.place(leaver, h)
	testutil.Assign(t, w.write, stray.node, h.NGACUaID)

	assert.True(t, w.can(leaver, w.dir(h), ngac.OpRead), "before: H's own person reads the hidden Dir")
	assert.True(t, w.can(leaver, w.peer(h), ngac.OpRead))
	assert.True(t, w.can(stray, w.dir(h), ngac.OpRead), "a stale department edge alone is enough to read: the reason it must go")

	require.NoError(t, w.svc.RemoveMember(w.as(w.owner), w.owner.node, w.ws.ID, leaver.node))
	require.NoError(t, w.svc.RemoveMember(w.as(w.owner), w.owner.node, w.ws.ID, stray.node))

	for name, p := range map[string]wsPerson{"leaver": leaver, "stray": stray} {
		assert.False(t, w.under(p.node, h.NGACUaID), name+": the department edge is removed")
		assert.False(t, w.under(p.node, w.peers(h)), name+": the peers edge is removed")
		assert.False(t, w.under(p.node, w.limited()), name+": the Limited edge is removed")
		assert.False(t, w.under(p.node, w.ws.MembersUaID), name+": and Members")
		assert.False(t, w.can(p, w.dir(h), ngac.OpRead), name+": DENY on the hidden Dir")
		assert.False(t, w.can(p, w.peer(h), ngac.OpRead), name+": DENY on the peer OA")
		assert.False(t, w.can(p, w.owners(), ngac.OpRead), name+": DENY on the owners")
	}
	member := w.member()
	assert.True(t, w.can(member, w.owners(), ngac.OpRead), "an ordinary member still reads the owners")
}

// Tearing a workspace down removes its prohibition, which has no foreign key to
// the nodes it names; another workspace's rule stays.
func TestRealPolicy_DeletingTheWorkspaceRemovesItsProhibition(t *testing.T) {
	w := newDirWorld(t, domain.TypeOrganization)
	other := newDirWorld(t, domain.TypeOrganization)
	has := func(id ngac.WorkspaceID) bool {
		r, err := w.read.ListProhibitions(w.admin(), &policypb.ListProhibitionsRequest{})
		require.NoError(t, err)
		for _, p := range r.GetProhibitions() {
			if p.GetName() == ngac.DirectoryLimitedProhibitionName(id) {
				return true
			}
		}
		return false
	}
	require.True(t, has(w.id), "fixture: the rule exists")

	require.NoError(t, w.svc.DeleteWorkspace(w.as(w.owner), w.owner.node, w.owner.id, w.ws.ID))
	assert.False(t, has(w.id), "the workspace's rule is gone with it")
	assert.True(t, has(other.id), "another workspace's rule is untouched")
	_, ok := w.find(ngac.DirectoryOAName(w.id), ngac.TypeOA)
	assert.False(t, ok, "and its directory nodes")

	require.NoError(t, w.svc.DeleteWorkspace(w.as(w.owner), w.owner.node, w.owner.id, w.ws.ID), "a retry is harmless")
}

// A role that holds manage (and invite) on Mgmt administers the directory; placing
// a person in, or moving a department under, a hidden department needs read on
// its Dir, which manage on the Directory OA stands in for.
func TestRealPolicy_AManageRoleCanPlacePeopleInAHiddenDepartment(t *testing.T) {
	w := newDirWorld(t, domain.TypeOrganization)
	a := w.dept("A", nil)
	h := w.dept("H", nil)
	w.hide(h)
	role := testutil.NewNode(t, w.write, ngac.RoleUAName(ngac.RoleID("rp-"+uuid.NewString())), ngac.TypeUA, w.ws.PcNodeID)
	testutil.Associate(t, w.write, role, w.ws.MgmtOaID, []string{ngac.OpManage, ngac.OpInvite})
	admin := w.member(role)
	plain := w.member()
	require.False(t, w.can(admin, w.dir(h), ngac.OpRead), "fixture: manage does not include read of the hidden Dir")
	require.True(t, w.can(admin, w.directory(), ngac.OpManage))

	// A department UA that grants write on the hidden Dir, and one that grants read
	// on something that is not part of the directory, still need the real operation.
	loud := w.dept("Loud", nil)
	testutil.Associate(t, w.write, loud.NGACUaID, w.dir(h), []string{ngac.OpWrite})
	// Mgmt is an OA the role manages but cannot read, and it is no directory node.
	require.False(t, w.can(admin, w.ws.MgmtOaID, ngac.OpRead), "fixture: the role does not read Mgmt")
	other := w.dept("Other", nil)
	testutil.Associate(t, w.write, other.NGACUaID, w.ws.MgmtOaID, []string{ngac.OpRead})

	// DENY.
	err := w.svc.UpdateMemberDepartment(w.as(admin), admin.node, w.ws.ID, plain.node, loud.ID)
	assert.ErrorIs(t, err, domain.ErrAccessDenied, "manage does not stand in for write")
	err = w.svc.UpdateMemberDepartment(w.as(admin), admin.node, w.ws.ID, plain.node, other.ID)
	assert.ErrorIs(t, err, domain.ErrAccessDenied, "nor for read of an OA outside the directory")
	assert.False(t, w.under(plain.node, loud.NGACUaID))

	// ALLOW, same caller.
	require.NoError(t, w.svc.UpdateMemberDepartment(w.as(admin), admin.node, w.ws.ID, plain.node, h.ID),
		"a directory administrator places a person in a hidden department")
	assert.True(t, w.under(plain.node, h.NGACUaID))
	_, err = w.svc.MoveDepartment(w.as(admin), admin.node, domain.MoveDepartmentInput{WorkspaceID: w.ws.ID, DeptID: a.ID, NewParentID: h.ID})
	assert.NoError(t, err, "and moves a department under it")
}

// seedDirectory builds the directory nodes of a workspace the way migration 064
// does, with the harness write client.
func seedDirectory(t *testing.T, write policypb.PolicyWriteServiceClient, g testutil.WorkspaceGraph) {
	t.Helper()
	id := ngac.WorkspaceID(g.WorkspaceID)
	mk := func(name, typ string, parent string) string {
		n, err := write.CreateNode(testutil.PolicyAdmin(context.Background()), &policypb.CreateNodeRequest{Name: name, NodeType: typ, Properties: ngac.DirectoryProps(id)})
		require.NoError(t, err)
		testutil.Assign(t, write, n.GetId(), parent)
		return n.GetId()
	}
	directory := mk(ngac.DirectoryOAName(id), ngac.TypeOA, g.Mgmt)
	open := mk(ngac.DirectoryOpenOAName(id), ngac.TypeOA, directory)
	mk(ngac.DirectoryNoDeptOAName(id), ngac.TypeOA, open)
	owners := mk(ngac.DirectoryOwnersOAName(id), ngac.TypeOA, directory)
	mk(ngac.DirectoryLimitedUAName(id), ngac.TypeUA, g.PC)
	testutil.Associate(t, write, g.Members, open, []string{ngac.OpRead})
	testutil.Associate(t, write, g.Members, owners, []string{ngac.OpRead})
}

// A workspace without a directory (before the migration that builds it) is
// neither refused nor given directory nodes; once the directory exists the same
// code mirrors new departments, and leaves a department without nodes alone.
func TestRealPolicy_AWorkspaceWithoutADirectoryIsToleratedThenMirrored(t *testing.T) {
	read, write := testutil.StartPolicy(t)
	pool := testutil.SetupTestDB(t)
	ctx := context.Background()
	g := testutil.SeedWorkspaceGraph(t, write)
	w := &dirWorld{t: t, read: read, write: write, pool: pool, id: ngac.WorkspaceID(g.WorkspaceID)}
	w.ws = &domain.WorkspaceResult{ID: g.WorkspaceID, PcNodeID: g.PC, MembersUaID: g.Members, MgmtOaID: g.Mgmt}
	w.owner = w.newPerson(g.Owners)
	_, err := pool.Exec(ctx, `INSERT INTO workspaces (id, name, owner_id, ngac_pc_id, type) VALUES ($1, 'ws-pre', $2, $3, 'organization')`,
		g.WorkspaceID, w.owner.id, g.PC)
	require.NoError(t, err)
	t.Cleanup(func() { purgeWorkspace(write, pool, g.WorkspaceID, g.PC) })
	st := store.New(pool)
	w.svc = domain.NewService(st, st, read, write, nil, nil)

	directoryNodes := func() []string {
		desc, err := write.GetDescendants(w.admin(), &policypb.GetDescendantsRequest{NodeId: g.PC})
		require.NoError(t, err)
		var out []string
		for _, n := range desc.GetNodes() {
			if n.GetProperties()[ngac.PropType] == ngac.PropTypeDirectory {
				out = append(out, n.GetName())
			}
		}
		return out
	}

	// Before: every department operation works and writes no directory node.
	a := w.dept("A", nil)
	a1 := w.dept("A1", a)
	p := w.member()
	w.place(p, a1)
	w.place(p, a)
	_, err = w.svc.MoveDepartment(w.as(w.owner), w.owner.node, domain.MoveDepartmentInput{WorkspaceID: g.WorkspaceID, DeptID: a1.ID})
	require.NoError(t, err)
	w.place(p, nil)
	require.NoError(t, w.svc.DeleteDepartment(w.as(w.owner), w.owner.node, g.WorkspaceID, a1.ID))
	member := w.member()
	require.NoError(t, w.svc.RemoveMember(w.as(w.owner), w.owner.node, g.WorkspaceID, member.node))
	assert.Empty(t, directoryNodes(), "no directory node is written into a workspace that has none")
	_, ok := w.find(ngac.DeptPeersUAName(ngac.DeptID(a.ID)), ngac.TypeUA)
	assert.False(t, ok, "nor a peers UA")

	// After: the directory exists (as the migration leaves it).
	seedDirectory(t, write, g)
	top := w.dept("Top", nil)
	assert.Equal(t, []string{ngac.DirectoryOpenOAName(w.id)}, w.parents(w.dir(top)), "a new top-level department is mirrored")
	w.place(p, top)
	assert.True(t, w.under(p.node, w.peers(top)))

	// A department that predates the directory has no nodes: the first thing that needs
	// them builds them, its parent's first, and puts the people already in it under its peers.
	require.False(t, w.nodeExists(ngac.DeptDirOAName(ngac.DeptID(a.ID)), ngac.TypeOA), "fixture: A has no nodes")
	earlier := w.member(a.NGACUaID) // placed in A while it had none
	child := w.dept("Child", a)     // creating a child needs the parent's nodes: they are built
	w.place(w.member(), child)
	assert.Equal(t, []string{ngac.DirectoryOpenOAName(w.id)}, w.parents(w.dir(a)), "the parent was built first, as a top-level department")
	assert.Equal(t, []string{w.dirName(a)}, w.parents(w.dir(child)), "and the child under it")
	assert.True(t, w.under(earlier.node, w.peers(a)), "the people already in the department get its peers edge")
	require.NoError(t, w.svc.DeleteDepartment(w.as(w.owner), w.owner.node, g.WorkspaceID, child.ID))
}

func (w *dirWorld) nodeExists(name, nodeType string) bool {
	_, ok := w.find(name, nodeType)
	return ok
}

// A department without nodes under a hidden parent is built hidden with it, whichever
// operation reaches it first.
func TestRealPolicy_ADepartmentWithoutNodesIsBuiltPerItsParentsHiddenState(t *testing.T) {
	read, write := testutil.StartPolicy(t)
	pool := testutil.SetupTestDB(t)
	g := testutil.SeedWorkspaceGraph(t, write)
	w := &dirWorld{t: t, read: read, write: write, pool: pool, id: ngac.WorkspaceID(g.WorkspaceID)}
	w.ws = &domain.WorkspaceResult{ID: g.WorkspaceID, PcNodeID: g.PC, MembersUaID: g.Members, MgmtOaID: g.Mgmt}
	w.owner = w.newPerson(g.Owners)
	_, err := pool.Exec(context.Background(), `INSERT INTO workspaces (id, name, owner_id, ngac_pc_id, type) VALUES ($1, 'ws-heal', $2, $3, 'organization')`,
		g.WorkspaceID, w.owner.id, g.PC)
	require.NoError(t, err)
	t.Cleanup(func() { purgeWorkspace(write, pool, g.WorkspaceID, g.PC) })
	st := store.New(pool)
	w.svc = domain.NewService(st, st, read, write, nil, nil)

	h := w.dept("H", nil)
	c := w.dept("C", h) // both created before the directory exists
	seedDirectory(t, write, g)
	pH, pC, other := w.member(), w.member(), w.member()
	w.place(pH, h) // builds H's nodes
	w.hide(h)
	w.place(pC, c) // builds C's, under the hidden H's

	assert.Equal(t, []string{w.dirName(h)}, w.parents(w.dir(c)))
	assert.Equal(t, []string{w.peerName(h)}, w.parents(w.peer(c)))
	assert.False(t, w.can(other, w.dir(c), ngac.OpRead), "built hidden: a member elsewhere cannot read it")
	assert.True(t, w.can(pC, w.dir(c), ngac.OpRead))
	assert.True(t, w.can(pH, w.dir(c), ngac.OpRead), "and the hidden parent's people read their sub-department")
}

// Deleting a hidden department that still has people placed directly in it is refused,
// with nothing changed: they would otherwise land in the visible parent.
func TestRealPolicy_ADeletedHiddenDepartmentMustBeEmptyOfPeople(t *testing.T) {
	w := newDirWorld(t, domain.TypeOrganization)
	a := w.dept("A", nil)
	h := w.dept("H", a)
	h1 := w.dept("H1", h)
	w.hide(h)
	pA, pH, pH1 := w.member(), w.member(), w.member()
	w.place(pA, a)
	w.place(pH, h)
	w.place(pH1, h1)

	err := w.svc.DeleteDepartment(w.as(w.owner), w.owner.node, w.ws.ID, h.ID)
	require.ErrorIs(t, err, domain.ErrInvalidInput)
	require.ErrorIs(t, err, domain.ErrHiddenDepartmentHasPeople, "the screen needs to tell this refusal from any other 400")
	assert.Contains(t, err.Error(), "Chuyển những người trong phòng này sang phòng khác trước khi xoá")
	assert.True(t, w.nodeExists(w.dirName(h), ngac.TypeOA), "nothing was deleted")
	assert.True(t, w.under(pH.node, h.NGACUaID), "the person is where they were")
	assert.True(t, w.under(pH.node, w.peers(h)))
	assert.False(t, w.can(pA, w.dir(h1), ngac.OpRead), "and still hidden")
	assert.Equal(t, []string{w.dirName(h)}, w.parents(w.dir(h1)))

	// ALLOW on the same fixture: once nobody is placed directly in it, it goes, and its
	// sub-department stays hidden.
	w.place(pH, nil)
	require.NoError(t, w.svc.DeleteDepartment(w.as(w.owner), w.owner.node, w.ws.ID, h.ID))
	assert.False(t, w.nodeExists(w.dirName(h), ngac.TypeOA))
	assert.False(t, w.can(pA, w.dir(h1), ngac.OpRead))
	assert.True(t, w.can(pH1, w.dir(h1), ngac.OpRead))
}

// A delete that was interrupted after the sub-department's nodes moved under Directory, but
// before its UA was given read on its Dir, finishes on the retry: the grant does not depend
// on the nodes still having to move.
func TestRealPolicy_DeletingAHiddenDepartmentAgainRestoresTheGrant(t *testing.T) {
	w := newDirWorld(t, domain.TypeOrganization)
	h := w.dept("H", nil)
	h1 := w.dept("H1", h)
	w.hide(h)
	pH1 := w.member()
	w.place(pH1, h1)
	// What the interrupted attempt left: H1's nodes already directly under Directory, no grant.
	testutil.Unassign(t, w.write, w.dir(h1), w.dir(h))
	testutil.Assign(t, w.write, w.dir(h1), w.directory())
	testutil.Unassign(t, w.write, w.peer(h1), w.peer(h))
	testutil.Assign(t, w.write, w.peer(h1), w.directory())

	require.NoError(t, w.svc.DeleteDepartment(w.as(w.owner), w.owner.node, w.ws.ID, h.ID))
	assert.True(t, w.can(pH1, w.dir(h1), ngac.OpRead), "the sub-department's people keep their view")
	assert.False(t, w.can(w.member(), w.dir(h1), ngac.OpRead))
}
