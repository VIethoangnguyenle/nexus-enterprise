package domain_test

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"ngac-platform/ngac"
	"ngac-platform/pkg/realtime"
	policypb "ngac-platform/proto/policy"
	"ngac-platform/services/workspace/internal/domain"
	"ngac-platform/services/workspace/internal/store"
	"ngac-platform/testutil"
)

// The directory scope against fakes: what it asks the policy service and how it fails.
// A decision the scope cannot get is an outage (503), never "show everyone"; a department
// the directory does not know is hidden from members, not shown. What the real policy
// decides is proved in directory_scope_realpolicy_test.go.

const (
	dirRoot   = "dir-root-" + ws1
	dirOwners = "dir-owners-" + ws1
	dirNoDept = "dir-nodept-" + ws1
	dirLim    = "dir-limited-" + ws1
	limitedU  = "u-limited"
)

// scopeRead adds the prohibition list to the fixture's read side.
type scopeRead struct {
	*fakePolicyRead
	rules []*policypb.Prohibition
}

func (r *scopeRead) ListProhibitions(_ context.Context, req *policypb.ListProhibitionsRequest, _ ...grpc.CallOption) (*policypb.ProhibitionList, error) {
	var out []*policypb.Prohibition
	for _, p := range r.rules {
		if req.GetSubjectId() == "" || p.SubjectId == req.GetSubjectId() {
			out = append(out, p)
		}
	}
	return &policypb.ProhibitionList{Prohibitions: out}, nil
}

// failingFind answers the lookup of one directory node by name with err.
type failingFind struct {
	*testutil.FakePolicyWrite
	name string
	err  error
}

func (f failingFind) FindNodeByName(ctx context.Context, in *policypb.FindNodeByNameRequest, o ...grpc.CallOption) (*policypb.NGACNode, error) {
	if f.err != nil && in.Name == f.name {
		return nil, f.err
	}
	return f.FakePolicyWrite.FindNodeByName(ctx, in, o...)
}

type scopeFx struct {
	*fixture
	w *testutil.FakePolicyWrite
	r *scopeRead
}

// newScopeFx builds ws-1 with a directory and three departments; dept-root-1 and
// dept-child-1 have directory nodes, dept-other-1 does not.
func newScopeFx(t *testing.T, mutate ...func(*scopeFx)) *scopeFx {
	t.Helper()
	w := testutil.NewFakePolicyWrite()
	f := newWriteFixture(t, w)
	seedFakeDirectory(w, ws1)
	seedFakeDeptDirectory(w, ws1, "dept-root-1")
	seedFakeDeptDirectory(w, ws1, "dept-child-1")
	id := ngac.WorkspaceID(ws1)
	var tree []*policypb.NGACNode
	for _, name := range []string{
		ngac.DirectoryOpenOAName(id), ngac.DirectoryNoDeptOAName(id), ngac.DirectoryOwnersOAName(id),
		ngac.DeptDirOAName("dept-root-1"), ngac.DeptPeerOAName("dept-root-1"),
		ngac.DeptDirOAName("dept-child-1"), ngac.DeptPeerOAName("dept-child-1"),
	} {
		n, err := w.FindNodeByName(context.Background(), &policypb.FindNodeByNameRequest{Name: name, NodeType: ngac.TypeOA})
		require.NoError(t, err, name)
		tree = append(tree, n)
	}
	f.read.descendants[dirRoot] = tree
	// The children of Directory: the open branch, the owners, no hidden department.
	f.read.children[dirRoot] = tree[:3]
	sr := &scopeRead{fakePolicyRead: f.read}
	w.Reads = sr
	f.svc = domain.NewService(f.wsStore, f.depts, sr, w, nil, nil)
	fx := &scopeFx{fixture: f, w: w, r: sr}
	for _, m := range mutate {
		m(fx)
	}
	return fx
}

func (fx *scopeFx) allow(user string, objs ...string) {
	for _, o := range objs {
		fx.read.grants[grant{user, o, ngac.OpRead}] = true
	}
}

func (fx *scopeFx) scope(user string) (*domain.DirectoryScope, error) {
	return fx.svc.DirectoryScope(context.Background(), user, ws1)
}

// A department is visible when its Dir or its DirPeer is readable; every other department,
// and one the directory has no nodes for, is not visible to a member and is to an administrator.
func TestDirectoryScope_DepartmentVisibility(t *testing.T) {
	logs := captureDomainLogs(t)
	fx := newScopeFx(t)
	fx.allow(member, dirNoDept, dirOwners, "dir-dept-root-1", "peer-dept-child-1")
	fx.read.grants[grant{owner, dirRoot, ngac.OpManage}] = true

	s, err := fx.scope(member)
	require.NoError(t, err)
	assert.True(t, s.Ready)
	assert.False(t, s.Admin)
	assert.Equal(t, map[string]bool{"dept-root-1": true, "dept-child-1": true}, s.VisibleDeptIDs,
		"one through its Dir, one through its DirPeer; the department with no nodes is not shown")
	assert.True(t, s.NoDeptVisible)
	assert.True(t, s.OwnersVisible)
	assert.Contains(t, logs.String(), "level=ERROR", "a department the directory does not know is logged loudly")
	assert.Contains(t, logs.String(), "dept-other-1")

	s, err = fx.scope(owner)
	require.NoError(t, err)
	assert.True(t, s.Admin)
	assert.Len(t, s.VisibleDeptIDs, 3, "an administrator sees every department, the one without nodes too")
	assert.True(t, s.NoDeptVisible)
	assert.True(t, s.OwnersVisible)

	// DENY on the same fixture: the member who is allowed nothing sees nothing.
	fx2 := newScopeFx(t)
	s, err = fx2.scope(member)
	require.NoError(t, err)
	assert.Empty(t, s.VisibleDeptIDs)
	assert.False(t, s.NoDeptVisible)
	assert.False(t, s.OwnersVisible)
}

// Whatever the scope cannot learn it fails on: a lookup that errors is not "no directory",
// a decision that cannot be fetched is not "allowed", and a half-built directory is refused.
func TestDirectoryScope_FailsClosed(t *testing.T) {
	// ALLOW first: a workspace with no directory node at all is simply not ready.
	bare := newWriteFixture(t, testutil.NewFakePolicyWrite())
	s, err := bare.svc.DirectoryScope(context.Background(), member, ws1)
	require.NoError(t, err)
	assert.False(t, s.Ready)
	assert.Len(t, s.VisibleDeptIDs, 3, "no directory: every department is visible")
	assert.True(t, s.NoDeptVisible)
	assert.True(t, s.OwnersVisible)
	assert.False(t, s.Admin)

	// DENY: the same workspace whose lookup fails is an outage.
	fx := newScopeFx(t)
	broken := failingFind{FakePolicyWrite: fx.w, name: ngac.DirectoryOAName(ws1), err: status.Error(codes.Unavailable, "policy down")}
	svc := domain.NewService(fx.wsStore, fx.depts, fx.r, broken, nil, nil)
	_, err = svc.DirectoryScope(context.Background(), member, ws1)
	assert.ErrorIs(t, err, domain.ErrUnavailable, "a lookup that errors is never read as 'no directory'")

	fx = newScopeFx(t)
	fx.read.batchErr = errors.New("policy down")
	_, err = fx.scope(member)
	assert.ErrorIs(t, err, domain.ErrUnavailable, "no decision, no list")

	fx = newScopeFx(t)
	fx.read.checkErr = errors.New("policy down")
	_, err = fx.scope(member)
	assert.ErrorIs(t, err, domain.ErrUnavailable, "an administrator check that cannot be asked is not a 'no'")

	fx = newScopeFx(t)
	halfBuilt := failingFind{FakePolicyWrite: fx.w, name: ngac.DirectoryNoDeptOAName(ws1), err: status.Error(codes.NotFound, "no such node")}
	svc = domain.NewService(fx.wsStore, fx.depts, fx.r, halfBuilt, nil, nil)
	_, err = svc.DirectoryScope(context.Background(), member, ws1)
	assert.ErrorIs(t, err, domain.ErrUnavailable, "a directory missing one of its five nodes is half built")

	// Membership is first: a stranger gets a refusal, not an outage and not a scope.
	fx = newScopeFx(t)
	_, err = fx.scope(outsider)
	assert.ErrorIs(t, err, domain.ErrAccessDenied)
	_, err = fx.scope("")
	assert.ErrorIs(t, err, domain.ErrAccessDenied)
}

// Limited is read from the caller's ancestors; a limited caller with no prohibition to
// back it would see everyone, so that is an outage.
func TestDirectoryScope_LimitedNeedsItsProhibition(t *testing.T) {
	fx := newScopeFx(t)
	fx.read.ancestors[limitedU] = append(append([]*policypb.NGACNode{}, fx.read.ancestors[member]...), node(dirLim, ngac.DirectoryLimitedUAName(ngac.WorkspaceID(ws1)), ngac.TypeUA))
	fx.read.nodes[limitedU] = node(limitedU, "limited", ngac.TypeU)
	fx.allow(limitedU, dirOwners, "peer-dept-root-1")

	_, err := fx.scope(limitedU)
	assert.ErrorIs(t, err, domain.ErrUnavailable, "limited and no prohibition: 503, never the full list")

	fx.r.rules = []*policypb.Prohibition{{Name: "someone-else", SubjectId: "ua-unrelated"}}
	_, err = fx.scope(limitedU)
	assert.ErrorIs(t, err, domain.ErrUnavailable, "another subject's prohibition is not the workspace's rule")

	fx.r.rules = []*policypb.Prohibition{{Name: ngac.DirectoryLimitedProhibitionName(ngac.WorkspaceID(ws1)), SubjectId: dirLim}}
	s, err := fx.scope(limitedU)
	require.NoError(t, err)
	assert.True(t, s.Limited)
	assert.Equal(t, map[string]bool{"dept-root-1": true}, s.VisibleDeptIDs)
	assert.True(t, s.OwnersVisible)
	assert.False(t, s.NoDeptVisible)

	// The caller who is not limited needs no prohibition.
	fx.r.rules = nil
	s, err = fx.scope(member)
	require.NoError(t, err)
	assert.False(t, s.Limited)
}

// countingWrite counts the graph reads a request makes and how many are in flight at once.
type countingWrite struct {
	*testutil.FakePolicyWrite
	mu                    sync.Mutex
	inflight, maxInflight int
	children, memberReads int
}

func (c *countingWrite) reset() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.inflight, c.maxInflight, c.children, c.memberReads = 0, 0, 0, 0
}

func (c *countingWrite) GetChildren(ctx context.Context, in *policypb.GetChildrenRequest, o ...grpc.CallOption) (*policypb.NodeList, error) {
	c.mu.Lock()
	c.children++
	c.inflight++
	c.maxInflight = max(c.maxInflight, c.inflight)
	c.mu.Unlock()
	time.Sleep(3 * time.Millisecond)
	defer func() { c.mu.Lock(); c.inflight--; c.mu.Unlock() }()
	return c.FakePolicyWrite.GetChildren(ctx, in, o...)
}

func (c *countingWrite) GetDescendants(ctx context.Context, in *policypb.GetDescendantsRequest, o ...grpc.CallOption) (*policypb.NodeList, error) {
	if in.NodeId == members1 {
		c.mu.Lock()
		c.memberReads++
		c.mu.Unlock()
	}
	return c.FakePolicyWrite.GetDescendants(ctx, in, o...)
}

// With many departments a request reads who is in each one once, a few at a time, and
// the people lists of an administrator or of a workspace with no rules are not read at all.
func TestDirectoryScope_ReadsStayBoundedWithManyDepartments(t *testing.T) {
	const n = 50
	fx := newScopeFx(t)
	fx.allow(member, dirNoDept, dirOwners)
	fx.read.grants[grant{owner, dirRoot, ngac.OpManage}] = true
	for i := 0; i < n; i++ {
		id := fmt.Sprintf("bulk-%d", i)
		fx.depts.depts[id] = &store.Department{ID: id, WorkspaceID: ws1, Name: id, NGACUaID: "ua-" + id}
		seedFakeDeptDirectory(fx.w, ws1, id)
		for _, name := range []string{ngac.DeptDirOAName(ngac.DeptID(id)), ngac.DeptPeerOAName(ngac.DeptID(id))} {
			nd, err := fx.w.FindNodeByName(context.Background(), &policypb.FindNodeByNameRequest{Name: name, NodeType: ngac.TypeOA})
			require.NoError(t, err)
			fx.read.descendants[dirRoot] = append(fx.read.descendants[dirRoot], nd)
		}
		fx.allow(member, "dir-"+id)
		fx.read.children["ua-"+id] = []*policypb.NGACNode{node("u-in-"+id, "person", ngac.TypeU)}
	}
	cw := &countingWrite{FakePolicyWrite: fx.w}
	svc := domain.NewService(fx.wsStore, fx.depts, fx.r, cw, nil, nil)
	ctx := context.Background()

	got, err := svc.ListDepartments(ctx, member, ws1)
	require.NoError(t, err)
	assert.GreaterOrEqual(t, len(got), n)
	assert.LessOrEqual(t, cw.children, n+6, "one read per department (and the owners), not two")
	assert.Greater(t, cw.maxInflight, 1, "the reads overlap")
	assert.LessOrEqual(t, cw.maxInflight, 8, "and are bounded")

	cw.reset()
	_, err = svc.ListMembers(ctx, member, ws1)
	require.NoError(t, err)
	assert.LessOrEqual(t, cw.children, n+6)
	assert.Greater(t, cw.maxInflight, 1)
	assert.LessOrEqual(t, cw.maxInflight, 8)

	cw.reset()
	_, err = svc.ListRoleSummaries(ctx, owner, ws1)
	require.NoError(t, err)
	assert.Equal(t, 1, cw.memberReads, "an administrator's role counts read the members once")
	assert.Less(t, cw.children, 10, "and no department is read for them")
}

// The scope's role holders are filtered by the same rule as the people reads, inside the
// workspace service: a limited caller is not told who holds a role among people they cannot see.
func TestDirectoryScope_RoleHoldersFollowTheVisibleSet(t *testing.T) {
	fx := newScopeFx(t)
	fx.read.ancestors[limitedU] = append(append([]*policypb.NGACNode{}, fx.read.ancestors[member]...), node(dirLim, ngac.DirectoryLimitedUAName(ngac.WorkspaceID(ws1)), ngac.TypeUA))
	fx.read.nodes[limitedU] = node(limitedU, "limited", ngac.TypeU)
	fx.r.rules = []*policypb.Prohibition{{Name: ngac.DirectoryLimitedProhibitionName(ngac.WorkspaceID(ws1)), SubjectId: dirLim}}
	fx.allow(limitedU, dirOwners, "peer-dept-root-1")
	fx.read.grants[grant{owner, dirRoot, ngac.OpManage}] = true
	fx.read.children["ua-dept-root-1"] = []*policypb.NGACNode{node(target, "target", ngac.TypeU)}
	fx.read.children["ua-dept-other-1"] = []*policypb.NGACNode{node(leaver, "leaver", ngac.TypeU)}
	fx.read.children[roleRead] = []*policypb.NGACNode{node(target, "target", ngac.TypeU), node(leaver, "leaver", ngac.TypeU)}

	holders := func(user string) []string {
		sc, err := fx.scope(user)
		require.NoError(t, err)
		for _, r := range sc.Roles {
			if r.ID == roleRead {
				return r.HolderNodeIDs
			}
		}
		t.Fatalf("role not in the scope")
		return nil
	}
	assert.ElementsMatch(t, []string{target}, holders(limitedU), "the holder in another department is not named to a limited caller")
	assert.ElementsMatch(t, []string{target, leaver}, holders(owner), "an administrator is told both")
}

type countEmitter struct{ n int }

func (c *countEmitter) Emit(realtime.Event) { c.n++ }

// Hiding what is hidden, and unhiding what is not, writes nothing and announces nothing.
func TestHideDepartment_NothingToDoWritesAndAnnouncesNothing(t *testing.T) {
	const dept, ua = "dept-root-1", "ua-dept-root-1"
	dirID, peerID := "dir-"+dept, "peer-"+dept
	open := node("dir-open-"+ws1, ngac.DirectoryOpenOAName(ngac.WorkspaceID(ws1)), ngac.TypeOA)
	root := node(dirRoot, ngac.DirectoryOAName(ngac.WorkspaceID(ws1)), ngac.TypeOA)
	build := func(hidden bool) (*scopeFx, *countEmitter) {
		fx := newScopeFx(t)
		fx.wsStore.ws[ws1].Type = domain.TypeOrganization
		em := &countEmitter{}
		fx.svc = domain.NewService(fx.wsStore, fx.depts, fx.r, fx.w, nil, nil).WithEmitter(em)
		dirN := node(dirID, ngac.DeptDirOAName(dept), ngac.TypeOA)
		peerN := node(peerID, ngac.DeptPeerOAName(dept), ngac.TypeOA)
		if hidden {
			fx.read.children[dirRoot] = append(fx.read.children[dirRoot], dirN, peerN)
			fx.read.parents[dirID], fx.read.parents[peerID] = []*policypb.NGACNode{root}, []*policypb.NGACNode{root}
			fx.read.assocs[ua] = []*policypb.Association{{UaId: ua, OaId: dirID, Operations: []string{ngac.OpRead}}}
		} else {
			fx.read.children[open.Id] = append(fx.read.children[open.Id], dirN)
			fx.read.children[dirRoot] = append(fx.read.children[dirRoot], peerN)
			fx.read.parents[dirID], fx.read.parents[peerID] = []*policypb.NGACNode{open}, []*policypb.NGACNode{root}
		}
		return fx, em
	}
	ctx := context.Background()

	// Already in the state asked for.
	fx, em := build(true)
	require.NoError(t, fx.svc.HideDepartment(ctx, owner, ws1, dept))
	assert.Zero(t, fx.w.Calls(), "already hidden: no write")
	assert.Zero(t, em.n, "and no announcement")
	fx, em = build(false)
	require.NoError(t, fx.svc.UnhideDepartment(ctx, owner, ws1, dept))
	assert.Zero(t, fx.w.Calls(), "not hidden: no write")
	assert.Zero(t, em.n)

	// The opposite requests on the same fixtures do write, and announce once.
	fx, em = build(false)
	require.NoError(t, fx.svc.HideDepartment(ctx, owner, ws1, dept))
	assert.NotZero(t, fx.w.Calls())
	assert.Equal(t, 1, em.n)
	fx, em = build(true)
	require.NoError(t, fx.svc.UnhideDepartment(ctx, owner, ws1, dept))
	assert.NotZero(t, fx.w.Calls())
	assert.Equal(t, 1, em.n)
}
