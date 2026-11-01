package grpc_test

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"ngac-platform/ngac"
	pb "ngac-platform/proto/drive"
	policypb "ngac-platform/proto/policy"
	"ngac-platform/services/drive/internal/domain"
	grpcserver "ngac-platform/services/drive/internal/grpc"
	"ngac-platform/services/drive/internal/store"
)

// Shares made before a shared file had an OA of its own hold the file's folder
// OA (or the Documents OA) and so grant everything in it. RepairFileShares
// narrows them at startup.

// GetChildren and GetParents read recWrite's edges the way the policy writer
// reads its graph.
func (w *recWrite) GetChildren(_ context.Context, req *policypb.GetChildrenRequest, _ ...grpc.CallOption) (*policypb.NodeList, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	out := &policypb.NodeList{}
	for child, parents := range w.edges {
		if parents[req.NodeId] {
			out.Nodes = append(out.Nodes, &policypb.NGACNode{Id: child})
		}
	}
	return out, nil
}

// GetNode knows every node recWrite created: their IDs are "id:" + name.
func (w *recWrite) GetNode(_ context.Context, req *policypb.GetNodeRequest, _ ...grpc.CallOption) (*policypb.NGACNode, error) {
	if name, ok := strings.CutPrefix(req.NodeId, "id:"); ok {
		return &policypb.NGACNode{Id: req.NodeId, Name: name, NodeType: ngac.TypeOA}, nil
	}
	return nil, status.Errorf(codes.NotFound, "node not found")
}

func (w *recWrite) GetParents(_ context.Context, req *policypb.GetParentsRequest, _ ...grpc.CallOption) (*policypb.NodeList, error) {
	out := &policypb.NodeList{}
	for _, p := range w.parentsOf(req.NodeId) {
		out.Nodes = append(out.Nodes, &policypb.NGACNode{Id: p})
	}
	return out, nil
}

// reaches reports whether from is to, or is assigned (transitively) under it:
// whether a grant on to covers from.
func (w *recWrite) reaches(from, to string) bool {
	seen := map[string]bool{}
	queue := []string{from}
	for len(queue) > 0 {
		n := queue[0]
		queue = queue[1:]
		if n == to {
			return true
		}
		if seen[n] {
			continue
		}
		seen[n] = true
		queue = append(queue, w.parentsOf(n)...)
	}
	return false
}

type repairFixture struct {
	*boundaryFixture
	svc       *domain.Service
	actorUser string
	actorNode string
}

func newRepairFixture(t *testing.T) *repairFixture {
	t.Helper()
	pr, pw := newFakePolicyRead(), &recWrite{}
	_, pool := newServerWith(t, linkedRead{pr}, pw)
	svc := domain.NewService(store.NewStore(pool), linkedRead{pr}, pw, &mockDocStorage{})
	b := &boundaryFixture{pr: pr, pw: pw, srv: grpcserver.NewServer(svc), pool: pool, wsID: getTestWorkspaceID(t, pool)}
	pr.add("role-ua", "Role_reviewers", ngac.TypeUA)

	// The person who made the old shares, with a graph identity the repair can
	// act as.
	f := &repairFixture{boundaryFixture: b, svc: svc, actorUser: getTestUserID(t, pool), actorNode: "drive-test-unode-" + uuid.NewString()}
	ctx := context.Background()
	_, err := pool.Exec(ctx, `INSERT INTO ngac_nodes (id, name, node_type) VALUES ($1, $1, 'U')`, f.actorNode)
	require.NoError(t, err)
	_, err = pool.Exec(ctx, `UPDATE users SET ngac_node = $1 WHERE id = $2`, f.actorNode, f.actorUser)
	require.NoError(t, err)
	t.Cleanup(func() {
		_, _ = pool.Exec(ctx, `UPDATE users SET ngac_node = NULL WHERE id = $1`, f.actorUser)
		_, _ = pool.Exec(ctx, `DELETE FROM ngac_nodes WHERE id = $1`, f.actorNode)
	})
	return f
}

// legacyShare records a share the way the drive made one before shared files had
// an OA of their own: the share OA holds heldOA (the file's folder OA, or the
// Documents OA), not the file.
func (f *repairFixture) legacyShare(t *testing.T, itemID, heldOA, createdBy string) string {
	t.Helper()
	shareOA := "id:" + ngac.ShareOAName(ngac.ShareID("legacy-"+uuid.NewString()))
	_, err := f.pool.Exec(context.Background(),
		`INSERT INTO drive_shares (id, drive_item_id, share_type, target_ngac_id, target_label, operations, ngac_share_oa, created_by)
		 VALUES ($1, $2, 'role', 'role-ua', 'Reviewers', $3, $4, $5)`,
		uuid.NewString(), itemID, []string{ngac.OpRead}, shareOA, createdBy)
	require.NoError(t, err)
	_, err = f.pw.CreateAssignment(context.Background(), &policypb.CreateAssignmentRequest{ChildId: heldOA, ParentId: shareOA})
	require.NoError(t, err)
	return shareOA
}

func authNode(t *testing.T, pool *pgxpool.Pool, id string) string {
	t.Helper()
	var n string
	require.NoError(t, pool.QueryRow(context.Background(), `SELECT ngac_node_id FROM drive_items WHERE id = $1`, id).Scan(&n))
	return n
}

// fileAs uploads a file owned by the fixture's actor.
func (f *repairFixture) fileAs(t *testing.T, name, parent string) *pb.DriveItem {
	t.Helper()
	ctx := asCaller(f.actorUser, f.actorNode)
	created, err := f.srv.CreateFile(ctx, &pb.CreateFileRequest{
		WorkspaceId: f.wsID, Name: name, MimeType: "text/plain", SizeBytes: 1, ParentId: parent,
	})
	require.NoError(t, err)
	t.Cleanup(func() { cleanDriveItems(t, f.pool, created.FileId) })
	it, err := f.srv.ConfirmFile(ctx, &pb.ConfirmFileRequest{FileId: created.FileId})
	require.NoError(t, err)
	return it
}

func TestRepairFileShares_TheSharedFileStaysReadableAndTheRestOfTheFolderDoesNot(t *testing.T) {
	f := newRepairFixture(t)
	folder := f.folder(t, "Private", "")
	shared, sibling := f.fileAs(t, "a.pdf", folder.Id), f.fileAs(t, "b.pdf", folder.Id)
	share := f.legacyShare(t, shared.Id, oaOf(folder), f.actorNode)

	// Before: the old share hands the target the whole folder.
	require.True(t, f.pw.reaches(authNode(t, f.pool, shared.Id), share))
	require.True(t, f.pw.reaches(authNode(t, f.pool, sibling.Id), share), "precondition: the old share over-grants")
	f.pw.reset()

	n, err := f.svc.RepairFileShares(context.Background(), f.wsID)
	require.NoError(t, err)
	assert.Equal(t, 1, n)

	// After: the target still reaches the shared file, and nothing else in the folder.
	assert.Equal(t, fileOA(shared), authNode(t, f.pool, shared.Id))
	assert.True(t, f.pw.reaches(authNode(t, f.pool, shared.Id), share), "the shared file stays readable")
	assert.False(t, f.pw.reaches(authNode(t, f.pool, sibling.Id), share), "the rest of the folder is not")
	assert.False(t, f.pw.reaches(oaOf(folder), share))
	assert.True(t, f.pw.reaches(fileOA(shared), oaOf(folder)), "the file keeps what its folder grants")

	// The file is put under the share before the folder is taken out, so the
	// grantee never loses the file and never gains anything.
	calls := f.pw.snapshot()
	add := indexOf(calls, fmt.Sprintf("assign %s>%s", fileOA(shared), share))
	remove := indexOf(calls, fmt.Sprintf("unassign %s>%s", oaOf(folder), share))
	require.GreaterOrEqual(t, add, 0, "%v", calls)
	require.GreaterOrEqual(t, remove, 0, "%v", calls)
	assert.Less(t, add, remove)
}

func TestRepairFileShares_ATopLevelFileStopsGrantingTheDocumentsOA(t *testing.T) {
	f := newRepairFixture(t)
	f.folder(t, "Elsewhere", "") // a top-level neighbour, authorized on the Documents OA too
	docs := getTestDocumentsOA(t)
	top := f.fileAs(t, "top.pdf", "")
	require.Equal(t, docs, authNode(t, f.pool, top.Id), "precondition: a top-level file sits on the Documents OA")
	share := f.legacyShare(t, top.Id, docs, f.actorNode)

	_, err := f.svc.RepairFileShares(context.Background(), f.wsID)
	require.NoError(t, err)

	assert.False(t, f.pw.reaches(docs, share), "the whole drive is no longer shared")
	assert.True(t, f.pw.reaches(fileOA(top), share))
	assert.True(t, f.pw.reaches(fileOA(top), docs))
}

func TestRepairFileShares_IsIdempotent(t *testing.T) {
	f := newRepairFixture(t)
	folder := f.folder(t, "Private", "")
	shared := f.fileAs(t, "a.pdf", folder.Id)
	share := f.legacyShare(t, shared.Id, oaOf(folder), f.actorNode)

	_, err := f.svc.RepairFileShares(context.Background(), f.wsID)
	require.NoError(t, err)
	f.pw.reset()

	n, err := f.svc.RepairFileShares(context.Background(), f.wsID)
	require.NoError(t, err)
	assert.Zero(t, n)
	assert.Empty(t, f.pw.snapshot(), "a second run finds nothing and writes nothing")
	assert.True(t, f.pw.reaches(fileOA(shared), share))
	assert.False(t, f.pw.reaches(oaOf(folder), share))
}

func TestRepairFileShares_AFolderShareIsLeftAlone(t *testing.T) {
	f := newRepairFixture(t)
	folder := f.folder(t, "Shared", "")
	f.shareWithRole(t, folder.Id)
	var share string
	require.NoError(t, f.pool.QueryRow(context.Background(),
		`SELECT ngac_share_oa FROM drive_shares WHERE drive_item_id = $1`, folder.Id).Scan(&share))
	inside := f.fileAs(t, "in.pdf", folder.Id)
	f.pw.reset()

	n, err := f.svc.RepairFileShares(context.Background(), f.wsID)
	require.NoError(t, err)
	assert.Zero(t, n)
	assert.Empty(t, f.pw.snapshot())
	assert.True(t, f.pw.reaches(oaOf(folder), share), "a folder share keeps the folder")
	assert.True(t, f.pw.reaches(authNode(t, f.pool, inside.Id), share), "and everything in it")
}

// Deny: a share that holds a folder the file is no longer in (it was moved
// after it was shared) loses that folder and does not gain the file.
func TestRepairFileShares_NeverWidensAShare(t *testing.T) {
	f := newRepairFixture(t)
	folder := f.folder(t, "Now", "")
	moved := f.fileAs(t, "moved.pdf", folder.Id)
	stale := "id:" + ngac.FolderNodeName(ngac.FolderID("left-behind"))
	share := f.legacyShare(t, moved.Id, stale, f.actorNode)

	_, err := f.svc.RepairFileShares(context.Background(), f.wsID)
	require.NoError(t, err)

	assert.False(t, f.pw.reaches(stale, share), "the folder the file left is no longer shared")
	assert.False(t, f.pw.reaches(fileOA(moved), share), "and the file, which the share did not reach, is not added")
	assert.False(t, f.pw.reaches(oaOf(folder), share))
}

// Deny: with no one to act as, nothing is written and the run reports it.
func TestRepairFileShares_WithoutAnIdentityWritesNothing(t *testing.T) {
	f := newRepairFixture(t)
	folder := f.folder(t, "Private", "")
	orphan := f.file(t, "orphan.pdf", folder.Id) // owner "u" is no user
	f.legacyShare(t, orphan.Id, oaOf(folder), "ngac-gone")
	f.pw.reset()

	n, err := f.svc.RepairFileShares(context.Background(), f.wsID)
	assert.Error(t, err)
	assert.Zero(t, n)
	assert.Empty(t, f.pw.snapshot())
}

// A new share of a file that still has an old one narrows the old one too.
func TestCreateShare_NarrowsTheFilesOlderShares(t *testing.T) {
	f := newRepairFixture(t)
	folder := f.folder(t, "Private", "")
	shared, sibling := f.fileAs(t, "a.pdf", folder.Id), f.fileAs(t, "b.pdf", folder.Id)
	old := f.legacyShare(t, shared.Id, oaOf(folder), f.actorNode)

	f.shareWithRole(t, shared.Id)

	assert.True(t, f.pw.reaches(fileOA(shared), old))
	assert.False(t, f.pw.reaches(authNode(t, f.pool, sibling.Id), old))
}

func indexOf(calls []string, want string) int {
	for i, c := range calls {
		if c == want {
			return i
		}
	}
	return -1
}

// rootLegacyShare records a share of a drive root as CreateShare made them
// before roots were refused: the share OA holds the root's OA.
func (f *repairFixture) rootLegacyShare(t *testing.T, rootID, rootOA, createdBy string) string {
	t.Helper()
	return f.legacyShare(t, rootID, rootOA, createdBy)
}

func shareRows(t *testing.T, pool *pgxpool.Pool, itemID string) int {
	t.Helper()
	var n int
	require.NoError(t, pool.QueryRow(context.Background(), `SELECT count(*) FROM drive_shares WHERE drive_item_id = $1`, itemID).Scan(&n))
	return n
}

func TestRepairShares_RevokesSharesOfADriveRoot(t *testing.T) {
	f := newRepairFixture(t)
	f.folder(t, "Top", "")
	root := f.rootID(t)
	docs := getTestDocumentsOA(t)
	share := f.rootLegacyShare(t, root, docs, f.actorNode)
	require.True(t, f.pw.reaches(docs, share), "precondition: the share hands over the whole drive")
	f.pw.reset()

	res, err := f.svc.RepairShares(context.Background(), f.wsID)
	require.NoError(t, err)
	assert.Equal(t, 1, res.RootSharesRevoked)
	assert.Equal(t, []string{"delete " + share}, f.pw.snapshot(), "the share OA, and with it the association, goes")
	assert.Zero(t, shareRows(t, f.pool, root))

	f.pw.reset()
	res, err = f.svc.RepairShares(context.Background(), f.wsID)
	require.NoError(t, err)
	assert.Equal(t, domain.ShareRepairResult{}, res, "a second run finds nothing")
	assert.Empty(t, f.pw.snapshot())
}

// Deny: with no one to act as, a root share is not touched and the run says so.
func TestRepairShares_ARootShareWithoutAnIdentityIsReported(t *testing.T) {
	f := newRepairFixture(t)
	f.folder(t, "Top", "")
	root := f.rootID(t)
	f.rootLegacyShare(t, root, getTestDocumentsOA(t), "ngac-gone")
	f.pw.reset()

	res, err := f.svc.RepairShares(context.Background(), f.wsID)
	assert.Error(t, err)
	assert.Equal(t, 1, res.Failed)
	assert.Empty(t, f.pw.snapshot())
	assert.Equal(t, 1, shareRows(t, f.pool, root))
}

// A run that fails (the policy service is restarting) is retried until one
// completes clean.
func TestRepairSharesUntilClean_RetriesUntilARunIsClean(t *testing.T) {
	f := newRepairFixture(t)
	folder := f.folder(t, "Private", "")
	shared := f.fileAs(t, "a.pdf", folder.Id)
	share := f.legacyShare(t, shared.Id, oaOf(folder), f.actorNode)
	failures := 2
	f.pw.failAssign = func(child, _ string) bool {
		if child == fileOA(shared) && failures > 0 {
			failures--
			return true
		}
		return false
	}

	err := f.svc.RepairSharesUntilClean(context.Background(), f.wsID, []time.Duration{time.Millisecond, time.Millisecond, time.Millisecond})
	require.NoError(t, err)
	assert.Zero(t, failures, "the first two runs failed")
	assert.True(t, f.pw.reaches(fileOA(shared), share))
	assert.False(t, f.pw.reaches(oaOf(folder), share))
}

// Deny: retries are bounded, and a run that never comes clean is an error.
func TestRepairSharesUntilClean_GivesUpAfterItsLastRetry(t *testing.T) {
	f := newRepairFixture(t)
	folder := f.folder(t, "Private", "")
	shared := f.fileAs(t, "a.pdf", folder.Id)
	share := f.legacyShare(t, shared.Id, oaOf(folder), f.actorNode)
	runs := 0
	f.pw.failAssign = func(child, _ string) bool {
		if child == fileOA(shared) {
			runs++
			return true
		}
		return false
	}

	err := f.svc.RepairSharesUntilClean(context.Background(), f.wsID, []time.Duration{time.Millisecond, time.Millisecond})
	assert.Error(t, err)
	assert.Equal(t, 3, runs, "one run and two retries")
	assert.True(t, f.pw.reaches(oaOf(folder), share), "a share that could not be narrowed is left as it was, never widened")
}

func TestRepairSharesUntilClean_StopsWhenCancelled(t *testing.T) {
	f := newRepairFixture(t)
	folder := f.folder(t, "Private", "")
	shared := f.fileAs(t, "a.pdf", folder.Id)
	f.legacyShare(t, shared.Id, oaOf(folder), f.actorNode)
	f.pw.failAssign = func(child, _ string) bool { return child == fileOA(shared) }
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	done := make(chan error, 1)
	go func() { done <- f.svc.RepairSharesUntilClean(ctx, f.wsID, []time.Duration{time.Hour}) }()
	select {
	case err := <-done:
		assert.True(t, errors.Is(err, context.Canceled), "%v", err)
	case <-time.After(5 * time.Second):
		t.Fatal("a cancelled repair kept waiting")
	}
}
