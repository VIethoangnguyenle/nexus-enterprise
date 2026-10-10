package grpc_test

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"ngac-platform/ngac"
	pb "ngac-platform/proto/drive"
	policypb "ngac-platform/proto/policy"
)

// fakePolicyRead answers from an explicit node table and an allow rule.
type fakePolicyRead struct {
	mockPolicyRead
	nodes     map[string]*policypb.NGACNode // by id
	names     map[string]*policypb.NGACNode // by name|type
	ancestors map[string][]*policypb.NGACNode
	allow     func(user, object, op string) bool
}

func newFakePolicyRead() *fakePolicyRead {
	return &fakePolicyRead{
		nodes:     map[string]*policypb.NGACNode{},
		names:     map[string]*policypb.NGACNode{},
		ancestors: map[string][]*policypb.NGACNode{},
		allow:     func(string, string, string) bool { return true },
	}
}

func (f *fakePolicyRead) add(id, name, nodeType string) {
	n := &policypb.NGACNode{Id: id, Name: name, NodeType: nodeType}
	f.nodes[id] = n
	f.names[name+"|"+nodeType] = n
}

func (f *fakePolicyRead) GetAncestors(_ context.Context, req *policypb.GetAncestorsRequest, _ ...grpc.CallOption) (*policypb.NodeList, error) {
	return &policypb.NodeList{Nodes: f.ancestors[req.NodeId]}, nil
}

func (f *fakePolicyRead) CheckAccess(_ context.Context, req *policypb.CheckAccessRequest, _ ...grpc.CallOption) (*policypb.AccessDecision, error) {
	if f.allow(req.UserNodeId, req.ObjectNodeId, req.Operation) {
		return &policypb.AccessDecision{Decision: ngac.DecisionAllow}, nil
	}
	return &policypb.AccessDecision{Decision: ngac.DecisionDeny}, nil
}

func (f *fakePolicyRead) GetNode(_ context.Context, req *policypb.GetNodeRequest, _ ...grpc.CallOption) (*policypb.NGACNode, error) {
	if n, ok := f.nodes[req.NodeId]; ok {
		return n, nil
	}
	return nil, status.Errorf(codes.NotFound, "node not found")
}

func (f *fakePolicyRead) FindNodeByName(_ context.Context, req *policypb.FindNodeByNameRequest, _ ...grpc.CallOption) (*policypb.NGACNode, error) {
	if n, ok := f.names[req.Name+"|"+req.NodeType]; ok {
		return n, nil
	}
	return nil, status.Errorf(codes.NotFound, "node not found")
}

// recWrite records every policy write in order.
type recWrite struct {
	mockPolicyWrite
	mu          sync.Mutex
	calls       []string
	failAssign  func(child, parent string) bool
	failAssoc   bool
	assocByUA   map[string][]string
	createdNode map[string]string // id -> node type
	nodeProps   map[string]map[string]string
	failRemove  func(child, parent string) bool
	edges       map[string]map[string]bool // child -> parents, as the graph would hold them
	delay       time.Duration              // widens the window between policy calls
}

func (w *recWrite) rec(format string, a ...any) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.calls = append(w.calls, fmt.Sprintf(format, a...))
}

func (w *recWrite) CreateNode(_ context.Context, req *policypb.CreateNodeRequest, _ ...grpc.CallOption) (*policypb.NGACNode, error) {
	w.rec("node %s %s", req.NodeType, req.Name)
	w.mu.Lock()
	if w.nodeProps == nil {
		w.nodeProps = map[string]map[string]string{}
	}
	w.nodeProps["id:"+req.Name] = req.Properties
	w.mu.Unlock()
	return &policypb.NGACNode{Id: "id:" + req.Name, Name: req.Name, NodeType: req.NodeType}, nil
}

func (w *recWrite) CreateAssignment(_ context.Context, req *policypb.CreateAssignmentRequest, _ ...grpc.CallOption) (*policypb.Assignment, error) {
	w.rec("assign %s>%s", req.ChildId, req.ParentId)
	if w.failAssign != nil && w.failAssign(req.ChildId, req.ParentId) {
		return nil, errors.New("assignment would create a cycle")
	}
	time.Sleep(w.delay)
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.edges == nil {
		w.edges = map[string]map[string]bool{}
	}
	if w.edges[req.ChildId] == nil {
		w.edges[req.ChildId] = map[string]bool{}
	}
	w.edges[req.ChildId][req.ParentId] = true
	return &policypb.Assignment{Id: "a"}, nil
}

// parentsOf returns the parent OAs the graph holds for a child.
func (w *recWrite) parentsOf(child string) []string {
	w.mu.Lock()
	defer w.mu.Unlock()
	var out []string
	for p := range w.edges[child] {
		out = append(out, p)
	}
	return out
}

func (w *recWrite) RemoveAssignment(_ context.Context, req *policypb.RemoveAssignmentRequest, _ ...grpc.CallOption) (*policypb.Empty, error) {
	w.rec("unassign %s>%s", req.ChildId, req.ParentId)
	if w.failRemove != nil && w.failRemove(req.ChildId, req.ParentId) {
		return nil, errors.New("policy unavailable")
	}
	time.Sleep(w.delay)
	w.mu.Lock()
	delete(w.edges[req.ChildId], req.ParentId)
	w.mu.Unlock()
	return &policypb.Empty{}, nil
}

func (w *recWrite) CreateAssociation(_ context.Context, req *policypb.CreateAssociationRequest, _ ...grpc.CallOption) (*policypb.Association, error) {
	w.rec("assoc %s>%s %s", req.UaId, req.OaId, strings.Join(req.Operations, ","))
	if w.failAssoc {
		return nil, errors.New("source must be UA")
	}
	return &policypb.Association{Id: "x"}, nil
}

func (w *recWrite) DeleteNode(_ context.Context, req *policypb.DeleteNodeRequest, _ ...grpc.CallOption) (*policypb.Empty, error) {
	w.rec("delete %s", req.NodeId)
	return &policypb.Empty{}, nil
}

func (w *recWrite) snapshot() []string {
	w.mu.Lock()
	defer w.mu.Unlock()
	return append([]string(nil), w.calls...)
}

func (w *recWrite) reset() {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.calls = nil
}

func (w *recWrite) has(prefix string) bool {
	for _, c := range w.snapshot() {
		if strings.HasPrefix(c, prefix) {
			return true
		}
	}
	return false
}

const actor = "ngac-actor"

func personalUA(userNodeID string) string { return ngac.PersonalUAName(ngac.UserNodeID(userNodeID)) }

// oaOf is the policy node ID the fake hands out for a folder: its node is named
// by the folder's own ID, so two folders called "A" are two nodes.
func oaOf(it *pb.DriveItem) string { return "id:" + ngac.FolderNodeName(ngac.FolderID(it.Id)) }

// ---------------------------------------------------------------------------
// MoveItem
// ---------------------------------------------------------------------------

type moveFixture struct {
	pw      *recWrite
	pr      *fakePolicyRead
	srv     pb.DriveServiceServer
	wsID    string
	a, b, c *pb.DriveItem // a (top) > b > c
	other   *pb.DriveItem // a second top-level folder
	pool    *pgxpool.Pool
}

func newMoveFixture(t *testing.T) *moveFixture {
	t.Helper()
	pr := newFakePolicyRead()
	pw := &recWrite{}
	srv, pool := newServerWith(t, pr, pw)
	wsID := getTestWorkspaceID(t, pool)
	mk := func(name, parent string) *pb.DriveItem {
		it, err := srv.CreateFolder(asCaller("", actor), &pb.CreateFolderRequest{
			WorkspaceId: wsID, Name: name, ParentId: parent,
		})
		require.NoError(t, err)
		t.Cleanup(func() { cleanDriveItems(t, pool, it.Id) })
		return it
	}
	f := &moveFixture{pw: pw, pr: pr, srv: srv, wsID: wsID, pool: pool}
	f.a = mk("A", "")
	f.b = mk("B", f.a.Id)
	f.c = mk("C", f.b.Id)
	f.other = mk("Other", "")
	pw.reset()
	return f
}

func TestMoveItem_FolderIntoItselfIsRefusedBeforeAnyPolicyWrite(t *testing.T) {
	f := newMoveFixture(t)
	_, err := f.srv.MoveItem(asCaller("", actor), &pb.MoveItemRequest{ItemId: f.b.Id, NewParentId: f.b.Id})
	assert.Equal(t, codes.InvalidArgument, status.Code(err))
	assert.Empty(t, f.pw.snapshot(), "no policy edge may be touched")
}

func TestMoveItem_FolderIntoDescendantIsRefusedBeforeAnyPolicyWrite(t *testing.T) {
	f := newMoveFixture(t)
	for name, dest := range map[string]string{"child": f.b.Id, "grandchild": f.c.Id} {
		_, err := f.srv.MoveItem(asCaller("", actor), &pb.MoveItemRequest{ItemId: f.a.Id, NewParentId: dest})
		assert.Equal(t, codes.InvalidArgument, status.Code(err), name)
	}
	assert.Empty(t, f.pw.snapshot(), "the subtree must stay attached to the graph")
}

func TestMoveItem_SiblingMoveStillWorksAndSwapsTheEdge(t *testing.T) {
	f := newMoveFixture(t)
	moved, err := f.srv.MoveItem(asCaller("", actor), &pb.MoveItemRequest{ItemId: f.c.Id, NewParentId: f.other.Id})
	require.NoError(t, err)
	assert.Equal(t, f.other.Id, moved.ParentId)
	// New edge first, old edge second: the folder is never detached.
	assert.Equal(t, []string{
		fmt.Sprintf("assign %s>%s", oaOf(f.c), oaOf(f.other)),
		fmt.Sprintf("unassign %s>%s", oaOf(f.c), oaOf(f.b)),
	}, f.pw.snapshot())
}

func TestMoveItem_RefusedNewEdgeNeverTouchesTheOldOne(t *testing.T) {
	f := newMoveFixture(t)
	f.pw.failAssign = func(child, parent string) bool { return parent == oaOf(f.other) }

	_, err := f.srv.MoveItem(asCaller("", actor), &pb.MoveItemRequest{ItemId: f.c.Id, NewParentId: f.other.Id})
	require.Error(t, err)
	assert.Equal(t, []string{
		fmt.Sprintf("assign %s>%s", oaOf(f.c), oaOf(f.other)), // refused; nothing detached
	}, f.pw.snapshot())

	// The row did not move either.
	got, err := f.srv.GetItem(asCaller("", actor), &pb.GetItemRequest{ItemId: f.c.Id})
	require.NoError(t, err)
	assert.Equal(t, f.b.Id, got.ParentId)
}

func TestMoveItem_WithdrawsTheNewEdgeWhenTheOldCannotBeRemoved(t *testing.T) {
	f := newMoveFixture(t)
	f.pw.failRemove = func(child, parent string) bool { return parent == oaOf(f.b) }

	_, err := f.srv.MoveItem(asCaller("", actor), &pb.MoveItemRequest{ItemId: f.c.Id, NewParentId: f.other.Id})
	require.Error(t, err)
	assert.Equal(t, []string{
		fmt.Sprintf("assign %s>%s", oaOf(f.c), oaOf(f.other)),
		fmt.Sprintf("unassign %s>%s", oaOf(f.c), oaOf(f.b)),     // fails
		fmt.Sprintf("unassign %s>%s", oaOf(f.c), oaOf(f.other)), // new edge withdrawn
	}, f.pw.snapshot())
	got, gerr := f.srv.GetItem(asCaller("", actor), &pb.GetItemRequest{ItemId: f.c.Id})
	require.NoError(t, gerr)
	assert.Equal(t, f.b.Id, got.ParentId, "the row did not move")
}

func TestMoveItem_SurfacesAFailedWithdrawal(t *testing.T) {
	f := newMoveFixture(t)
	f.pw.failRemove = func(string, string) bool { return true }

	_, err := f.srv.MoveItem(asCaller("", actor), &pb.MoveItemRequest{ItemId: f.c.Id, NewParentId: f.other.Id})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "under both parents", "an unrepaired state must be reported, not only logged")
}

// Two moves of one item at once must leave it under exactly one parent in the
// graph, the one the row records. Without serialisation both read the same
// starting parent, each adds its own edge and only one removes the old one:
// the folder ends up under both destinations.
func TestMoveItem_ConcurrentMovesOfOneItemLeaveOneParent(t *testing.T) {
	f := newMoveFixture(t)
	f.pw.delay = 15 * time.Millisecond
	ctx := asCaller("", actor)

	dests := []string{f.other.Id, f.a.Id}
	var wg sync.WaitGroup
	errs := make([]error, len(dests))
	for i, d := range dests {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, errs[i] = f.srv.MoveItem(ctx, &pb.MoveItemRequest{ItemId: f.c.Id, NewParentId: d})
		}()
	}
	wg.Wait()
	for i, err := range errs {
		if err != nil {
			assert.Equal(t, codes.Aborted, status.Code(err), "move %d may only lose with Aborted, got %v", i, err)
		}
	}

	got, err := f.srv.GetItem(ctx, &pb.GetItemRequest{ItemId: f.c.Id})
	require.NoError(t, err)
	parents := f.pw.parentsOf(oaOf(f.c))
	require.Len(t, parents, 1, "the folder must hang under exactly one parent, got %v", parents)
	want := map[string]string{f.other.Id: oaOf(f.other), f.a.Id: oaOf(f.a)}[got.ParentId]
	assert.Equal(t, want, parents[0], "the graph edge must be the one the row records")
}

// A lost race on the row (the parent changed after this move read it) undoes
// this move's policy edges and reports Aborted.
func TestMoveItem_StaleParentRollsBackItsEdges(t *testing.T) {
	f := newMoveFixture(t)
	ctx := asCaller("", actor)
	// Another writer re-parents the row behind the lock, as a different drive
	// instance without the lock would.
	_, err := f.pool.Exec(context.Background(), `UPDATE drive_items SET parent_id = $1 WHERE id = $2`, f.other.Id, f.c.Id)
	require.NoError(t, err)
	f.pw.reset()
	f.pw.failAssign = nil

	// This call re-reads under its lock, so it sees the new parent and
	// succeeds from there: the old edge it removes is Other's, not B's.
	_, err = f.srv.MoveItem(ctx, &pb.MoveItemRequest{ItemId: f.c.Id, NewParentId: f.a.Id})
	require.NoError(t, err)
	assert.Contains(t, f.pw.snapshot(), fmt.Sprintf("unassign %s>%s", oaOf(f.c), oaOf(f.other)))
}

func TestMoveItem_TrashedItemIsNotFound(t *testing.T) {
	f := newMoveFixture(t)
	ctx := asCaller("", actor)
	_, err := f.srv.TrashItem(ctx, &pb.TrashItemRequest{ItemId: f.c.Id})
	require.NoError(t, err)
	_, err = f.srv.MoveItem(ctx, &pb.MoveItemRequest{ItemId: f.c.Id, NewParentId: f.other.Id})
	assert.Equal(t, codes.NotFound, status.Code(err))
	assert.Empty(t, f.pw.snapshot())
}

func TestMoveItem_ToDriveRoot(t *testing.T) {
	f := newMoveFixture(t)
	// A top-level folder hangs under the root OA; ensureRoot makes one when the
	// workspace has none, and its OA is what the folder must move under.
	moved, err := f.srv.MoveItem(asCaller("", actor), &pb.MoveItemRequest{ItemId: f.c.Id, NewParentId: ""})
	require.NoError(t, err)
	assert.Empty(t, moved.ParentId, "top-level items have no parent row")
	assert.True(t, f.pw.has(fmt.Sprintf("unassign %s>%s", oaOf(f.c), oaOf(f.b))))
	assert.True(t, f.pw.has(fmt.Sprintf("assign %s>", oaOf(f.c))))
}

func TestMoveItem_TopLevelFolderLeavesTheRootEdgeBehind(t *testing.T) {
	f := newMoveFixture(t)
	_, err := f.srv.MoveItem(asCaller("", actor), &pb.MoveItemRequest{ItemId: f.other.Id, NewParentId: f.b.Id})
	require.NoError(t, err)
	calls := f.pw.snapshot()
	require.Len(t, calls, 2, "old (root) edge removed, new edge made: %v", calls)
	assert.Equal(t, fmt.Sprintf("assign %s>%s", oaOf(f.other), oaOf(f.b)), calls[0])
	assert.True(t, strings.HasPrefix(calls[1], "unassign "+oaOf(f.other)+">"), calls[1])
}

func TestMoveItem_RefusedDestinations(t *testing.T) {
	f := newMoveFixture(t)
	ctx := asCaller("", actor)

	// Trashed folder: not a place to move into.
	_, err := f.srv.TrashItem(ctx, &pb.TrashItemRequest{ItemId: f.other.Id})
	require.NoError(t, err)
	_, err = f.srv.MoveItem(ctx, &pb.MoveItemRequest{ItemId: f.c.Id, NewParentId: f.other.Id})
	assert.Equal(t, codes.NotFound, status.Code(err))

	// A file is not a folder.
	file, err := f.srv.CreateFile(asCaller("u", actor), &pb.CreateFileRequest{
		WorkspaceId: f.wsID, Name: "f.txt", MimeType: "text/plain", SizeBytes: 1, ParentId: f.b.Id,
	})
	require.NoError(t, err)
	t.Cleanup(func() { cleanDriveItems(t, f.pool, file.FileId) })
	_, err = f.srv.ConfirmFile(ctx, &pb.ConfirmFileRequest{FileId: file.FileId})
	require.NoError(t, err)
	_, err = f.srv.MoveItem(ctx, &pb.MoveItemRequest{ItemId: f.c.Id, NewParentId: file.FileId})
	assert.Equal(t, codes.InvalidArgument, status.Code(err))

	assert.Empty(t, f.pw.snapshot())
}

func TestMoveItem_DeniedWithoutWriteOnDestination(t *testing.T) {
	f := newMoveFixture(t)
	f.pr.allow = func(_, object, op string) bool { return object != oaOf(f.other) }
	_, err := f.srv.MoveItem(asCaller("", actor), &pb.MoveItemRequest{ItemId: f.c.Id, NewParentId: f.other.Id})
	assert.Equal(t, codes.PermissionDenied, status.Code(err))
	assert.Empty(t, f.pw.snapshot())
}

// ---------------------------------------------------------------------------
// Trashed and foreign folders
// ---------------------------------------------------------------------------

func TestTrashedFolderIsNotFound(t *testing.T) {
	f := newMoveFixture(t)
	ctx := asCaller("", actor)
	_, err := f.srv.TrashItem(ctx, &pb.TrashItemRequest{ItemId: f.b.Id})
	require.NoError(t, err)

	_, err = f.srv.ListFolder(ctx, &pb.ListFolderRequest{WorkspaceId: f.wsID, FolderId: f.b.Id})
	assert.Equal(t, codes.NotFound, status.Code(err), "ListFolder")
	_, err = f.srv.GetItem(ctx, &pb.GetItemRequest{ItemId: f.b.Id})
	assert.Equal(t, codes.NotFound, status.Code(err), "GetItem")
	_, err = f.srv.CreateFolder(ctx, &pb.CreateFolderRequest{WorkspaceId: f.wsID, Name: "x", ParentId: f.b.Id})
	assert.Equal(t, codes.NotFound, status.Code(err), "CreateFolder under trashed")
	_, err = f.srv.CreateFile(ctx, &pb.CreateFileRequest{WorkspaceId: f.wsID, Name: "x", SizeBytes: 1, ParentId: f.b.Id})
	assert.Equal(t, codes.NotFound, status.Code(err), "CreateFile under trashed")

	// Restoring it makes it open again.
	_, err = f.srv.RestoreItem(ctx, &pb.RestoreItemRequest{ItemId: f.b.Id})
	require.NoError(t, err)
	_, err = f.srv.ListFolder(ctx, &pb.ListFolderRequest{WorkspaceId: f.wsID, FolderId: f.b.Id})
	assert.NoError(t, err)
}

func TestFolderOfAnotherWorkspaceIsNotFound(t *testing.T) {
	f := newMoveFixture(t)
	ctx := asCaller("", actor)
	// A real second workspace, so the refusal is about the parent and not
	// about the workspace missing.
	foreign := "drive-test-foreign-" + f.wsID
	_, err := f.pool.Exec(context.Background(),
		`INSERT INTO workspaces (id, name, owner_id) VALUES ($1, $1, $2)`, foreign, getTestUserID(t, f.pool))
	require.NoError(t, err)
	t.Cleanup(func() {
		f.pool.Exec(context.Background(), `DELETE FROM drive_quotas WHERE workspace_id = $1`, foreign)
		f.pool.Exec(context.Background(), `DELETE FROM workspaces WHERE id = $1`, foreign)
	})

	_, err = f.srv.CreateFolder(ctx, &pb.CreateFolderRequest{WorkspaceId: foreign, Name: "x", ParentId: f.b.Id})
	assert.Equal(t, codes.NotFound, status.Code(err), "CreateFolder")
	_, err = f.srv.CreateFile(ctx, &pb.CreateFileRequest{WorkspaceId: foreign, Name: "x", SizeBytes: 1, ParentId: f.b.Id})
	assert.Equal(t, codes.NotFound, status.Code(err), "CreateFile")
	_, err = f.srv.ListFolder(ctx, &pb.ListFolderRequest{WorkspaceId: foreign, FolderId: f.b.Id})
	assert.Equal(t, codes.NotFound, status.Code(err), "ListFolder")
	assert.Empty(t, f.pw.snapshot(), "nothing may be written for a foreign parent")
}

// ---------------------------------------------------------------------------
// CreateShare
// ---------------------------------------------------------------------------

type shareFixture struct {
	pw     *recWrite
	pr     *fakePolicyRead
	srv    pb.DriveServiceServer
	item   *pb.DriveItem
	person string // U node id
}

func newShareFixture(t *testing.T) *shareFixture {
	t.Helper()
	pr := newFakePolicyRead()
	pw := &recWrite{}
	srv, pool := newServerWith(t, pr, pw)
	wsID := getTestWorkspaceID(t, pool)
	item, err := srv.CreateFolder(asCaller("", actor), &pb.CreateFolderRequest{WorkspaceId: wsID, Name: "Shared"})
	require.NoError(t, err)
	t.Cleanup(func() { cleanDriveItems(t, pool, item.Id) })

	f := &shareFixture{pw: pw, pr: pr, srv: srv, item: item, person: "u-person"}
	pr.add(f.person, "person", ngac.TypeU)
	pr.add("ua-role", "Role", ngac.TypeUA)
	pr.add("oa-1", "SomeOA", ngac.TypeOA)
	pr.add("pcg", ngac.NodePCGlobal, ngac.TypePC)
	pr.add("pub", ngac.NodePublicUsers, ngac.TypeUA)
	pw.reset()
	return f
}

func (f *shareFixture) share(shareType, target string, ops ...string) (*pb.ShareInfo, error) {
	return f.srv.CreateShare(asCaller("", actor), &pb.CreateShareRequest{
		ItemId: f.item.Id, ShareType: shareType, TargetNgacNodeId: target, Operations: ops,
	})
}

func TestCreateShare_PersonGetsAPersonalUAAndTheMappedOperations(t *testing.T) {
	for _, tc := range []struct {
		permission string
		want       []string
	}{
		{ngac.SharePermissionRead, []string{ngac.OpRead}},
		{ngac.SharePermissionWrite, []string{ngac.OpRead, ngac.OpWrite}},
	} {
		t.Run(tc.permission, func(t *testing.T) {
			f := newShareFixture(t)
			info, err := f.share("user", f.person, tc.permission)
			require.NoError(t, err)
			assert.Equal(t, tc.want, info.Operations)

			ua := "id:" + personalUA(f.person)
			calls := strings.Join(f.pw.snapshot(), "\n")
			assert.Contains(t, calls, "node UA "+personalUA(f.person))
			assert.Contains(t, calls, fmt.Sprintf("assign %s>%s", f.person, ua), "the person joins their own UA")
			assert.Contains(t, calls, fmt.Sprintf("assoc %s>", ua), "the association starts from the UA, not the user node")
			assert.NotContains(t, calls, fmt.Sprintf("assoc %s>", f.person))
			assert.Contains(t, calls, strings.Join(tc.want, ","))
		})
	}
}

func TestCreateShare_ReusesTheExistingPersonalUA(t *testing.T) {
	f := newShareFixture(t)
	f.pr.ancestors[f.person] = []*policypb.NGACNode{{
		Id: "ua-existing", Name: personalUA(f.person), NodeType: ngac.TypeUA,
		Properties: ngac.PersonalUAProperties(f.person),
	}}
	_, err := f.share("user", f.person, ngac.SharePermissionRead)
	require.NoError(t, err)
	assert.False(t, f.pw.has("node UA "), "no second personal UA")
	assert.True(t, f.pw.has("assoc ua-existing>"))
}

func TestCreateShare_PersonalUAIsCreatedWithItsMarkingProperties(t *testing.T) {
	f := newShareFixture(t)
	_, err := f.share("user", f.person, ngac.SharePermissionRead)
	require.NoError(t, err)
	assert.Equal(t, ngac.PersonalUAProperties(f.person), f.pw.nodeProps["id:"+personalUA(f.person)])
}

// A role is a UA whose name a workspace administrator picks. One named exactly
// like a person's personal UA, with the victim's share going to whoever is in
// it, must never be mistaken for that person's attribute.
func TestCreateShare_SameNamedNonPersonalUAIsNeverReused(t *testing.T) {
	squatter := &policypb.NGACNode{
		Id: "ua-squatter", Name: "", NodeType: ngac.TypeUA, Properties: map[string]string{"type": "role"},
	}
	for name, setup := range map[string]func(f *shareFixture){
		"found by name": func(f *shareFixture) {
			squatter.Name = personalUA(f.person)
			f.pr.add(squatter.Id, squatter.Name, ngac.TypeUA)
			f.pr.names[squatter.Name+"|"+ngac.TypeUA] = squatter
		},
		"no properties at all": func(f *shareFixture) {
			sq := &policypb.NGACNode{Id: "ua-squatter", Name: personalUA(f.person), NodeType: ngac.TypeUA}
			f.pr.names[sq.Name+"|"+ngac.TypeUA] = sq
		},
		"personal UA of someone else": func(f *shareFixture) {
			sq := &policypb.NGACNode{
				Id: "ua-squatter", Name: personalUA(f.person), NodeType: ngac.TypeUA,
				Properties: ngac.PersonalUAProperties("another-user"),
			}
			f.pr.names[sq.Name+"|"+ngac.TypeUA] = sq
			f.pr.ancestors[f.person] = []*policypb.NGACNode{sq} // even among the victim's own attributes
		},
	} {
		t.Run(name, func(t *testing.T) {
			f := newShareFixture(t)
			setup(f)
			_, err := f.share("user", f.person, ngac.SharePermissionRead)
			require.NoError(t, err)

			calls := strings.Join(f.pw.snapshot(), "\n")
			assert.NotContains(t, calls, "assoc ua-squatter>", "the share must not go to the squatter")
			assert.Contains(t, calls, "node UA "+personalUA(f.person), "a genuine personal UA is created instead")
			assert.Contains(t, calls, fmt.Sprintf("assign %s>id:%s", f.person, personalUA(f.person)))
		})
	}
}

func TestCreateShare_TrashedItemIsNotFound(t *testing.T) {
	f := newShareFixture(t)
	_, err := f.srv.TrashItem(asCaller("", actor), &pb.TrashItemRequest{ItemId: f.item.Id})
	require.NoError(t, err)
	_, err = f.share("user", f.person, ngac.SharePermissionRead)
	assert.Equal(t, codes.NotFound, status.Code(err))
	assert.Empty(t, f.pw.snapshot())
}

func TestCreateShare_RoleAndPublicTargetsKeepWorking(t *testing.T) {
	f := newShareFixture(t)
	_, err := f.share("role", "ua-role", ngac.SharePermissionRead)
	require.NoError(t, err)
	assert.True(t, f.pw.has("assoc ua-role>"))
	f.pw.reset()
	_, err = f.share("public", "", ngac.SharePermissionRead)
	require.NoError(t, err)
	assert.True(t, f.pw.has("assoc pub>"))
}

func TestCreateShare_RejectsPermissionsOtherThanReadAndWriteBeforeAnyWrite(t *testing.T) {
	f := newShareFixture(t)
	for _, ops := range [][]string{
		{ngac.OpManage}, {ngac.OpShare}, {ngac.OpApprove}, {"READ"}, {""},
		{ngac.OpRead, ngac.OpWrite}, {ngac.OpRead, ngac.OpManage}, nil,
	} {
		_, err := f.share("user", f.person, ops...)
		assert.Equal(t, codes.InvalidArgument, status.Code(err), "ops %v", ops)
	}
	assert.Empty(t, f.pw.snapshot(), "a refused share must not create nodes")
}

func TestCreateShare_RejectsBadTargetsBeforeAnyWrite(t *testing.T) {
	f := newShareFixture(t)
	for name, tc := range map[string]struct{ shareType, target string }{
		"unknown node":        {"user", "no-such-node"},
		"empty target":        {"user", ""},
		"object attribute":    {"user", "oa-1"},
		"person as role":      {"role", f.person},
		"person as workspace": {"workspace", f.person},
		"unknown share type":  {"everyone", "ua-role"},
	} {
		_, err := f.share(tc.shareType, tc.target, ngac.SharePermissionRead)
		assert.Equal(t, codes.InvalidArgument, status.Code(err), name)
	}
	assert.Empty(t, f.pw.snapshot())
}

func TestCreateShare_RequiresTheShareRightNotWrite(t *testing.T) {
	f := newShareFixture(t)
	// Holds everything except share.
	f.pr.allow = func(_, _, op string) bool { return op != ngac.OpShare }
	_, err := f.share("user", f.person, ngac.SharePermissionRead)
	assert.Equal(t, codes.PermissionDenied, status.Code(err))
	assert.Empty(t, f.pw.snapshot())

	// Holds only share.
	f.pr.allow = func(_, _, op string) bool { return op == ngac.OpShare }
	_, err = f.share("user", f.person, ngac.SharePermissionRead)
	assert.NoError(t, err)
}

func TestCreateShare_RemovesTheShareOAWhenTheAssociationFails(t *testing.T) {
	f := newShareFixture(t)
	f.pw.failAssoc = true
	_, err := f.share("role", "ua-role", ngac.SharePermissionRead)
	require.Error(t, err)

	calls := f.pw.snapshot()
	require.NotEmpty(t, calls)
	last := calls[len(calls)-1]
	assert.True(t, strings.HasPrefix(last, "delete id:Share_"), "share OA must be deleted, got %q", last)

	// And no row says the item is shared.
	list, lerr := f.srv.ListShares(asCaller("", actor), &pb.ListSharesRequest{ItemId: f.item.Id})
	require.NoError(t, lerr)
	assert.Empty(t, list.Shares)
}
