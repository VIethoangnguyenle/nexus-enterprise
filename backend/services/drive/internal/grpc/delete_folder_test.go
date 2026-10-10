package grpc_test

import (
	"context"
	"sync"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	docpb "ngac-platform/proto/document"
	pb "ngac-platform/proto/drive"
	policypb "ngac-platform/proto/policy"
	grpcserver "ngac-platform/services/drive/internal/grpc"
	"ngac-platform/services/drive/internal/reason"
	"ngac-platform/services/drive/internal/store"
)

// graphDeletes remembers which nodes were deleted from the graph.
type graphDeletes struct {
	mockPolicyWrite
	mu      sync.Mutex
	deleted []string
}

func (r *graphDeletes) DeleteNode(_ context.Context, req *policypb.DeleteNodeRequest, _ ...grpc.CallOption) (*policypb.Empty, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.deleted = append(r.deleted, req.NodeId)
	return &policypb.Empty{}, nil
}

func (r *graphDeletes) deletedNodes() []string {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]string(nil), r.deleted...)
}

// recordingDocStorage remembers which stored objects were removed.
type recordingDocStorage struct {
	mockDocStorage
	mu      sync.Mutex
	removed []string
}

func (r *recordingDocStorage) DeleteObject(_ context.Context, req *docpb.DeleteObjectRequest, _ ...grpc.CallOption) (*docpb.Empty, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.removed = append(r.removed, req.ObjectKey)
	return &docpb.Empty{}, nil
}

func (r *recordingDocStorage) removedKeys() []string {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]string(nil), r.removed...)
}

type deleteFixture struct {
	srv     *grpcserver.DriveServer
	pool    *pgxpool.Pool
	policy  *graphDeletes
	storage *recordingDocStorage
	wsID    string
	userID  string
}

func newDeleteFixture(t *testing.T) *deleteFixture {
	t.Helper()
	_, pool := setupServer(t)
	f := &deleteFixture{
		pool: pool, policy: &graphDeletes{}, storage: &recordingDocStorage{},
		wsID: getTestWorkspaceID(t, pool), userID: getTestUserID(t, pool),
	}
	f.srv = grpcserver.NewDriveServer(pool, &mockPolicyRead{}, f.policy, f.storage)
	return f
}

func (f *deleteFixture) folder(t *testing.T, name, parentID string) *pb.DriveItem {
	t.Helper()
	item, err := f.srv.CreateFolder(asCaller("", "ngac-user-1"), &pb.CreateFolderRequest{
		WorkspaceId: f.wsID, Name: name, ParentId: parentID,
	})
	require.NoError(t, err)
	return item
}

// document puts a text document in the folder, as the document service does.
func (f *deleteFixture) document(t *testing.T, folderID string) {
	t.Helper()
	_, err := f.pool.Exec(context.Background(),
		`INSERT INTO text_documents (workspace_id, folder_id, title, owner_id) VALUES ($1, $2, 'Biên bản', $3)`,
		f.wsID, folderID, f.userID)
	require.NoError(t, err)
	t.Cleanup(func() {
		_, _ = f.pool.Exec(context.Background(), `DELETE FROM text_documents WHERE workspace_id = $1`, f.wsID)
	})
}

// file puts a stored file in the folder; the object key is what the storage
// service would be asked to remove.
func (f *deleteFixture) file(t *testing.T, folderID, key string, size int64) string {
	t.Helper()
	id := uuid.New().String()
	_, err := f.pool.Exec(context.Background(),
		`INSERT INTO drive_items (id, workspace_id, drive_context, parent_id, item_type, name, size_bytes, object_key, ngac_node_id, owner_id, status)
		 VALUES ($1, $2, 'workspace', $3, 'file', $4, $5, $6, $7, $8, 'active')`,
		id, f.wsID, folderID, "tep-"+id[:8], size, key, "node-file-"+id, f.userID)
	require.NoError(t, err)
	return id
}

func (f *deleteFixture) exists(t *testing.T, id string) bool {
	t.Helper()
	var n int
	require.NoError(t, f.pool.QueryRow(context.Background(), `SELECT count(*) FROM drive_items WHERE id = $1`, id).Scan(&n))
	return n == 1
}

func requireFolderHasDocuments(t *testing.T, err error) {
	t.Helper()
	require.Error(t, err)
	assert.Equal(t, codes.FailedPrecondition, status.Code(err))
	assert.Equal(t, reason.FolderHasDocuments, status.Convert(err).Message())
}

// The refusal comes before anything is touched: the folder row stays, its access
// node stays in the graph, and no stored file is removed.
func TestDeleteFolder_RefusedWhenItHoldsDocuments(t *testing.T) {
	f := newDeleteFixture(t)
	folder := f.folder(t, "Hồ sơ đối soát", "")
	f.document(t, folder.Id)
	fileID := f.file(t, folder.Id, "drive/keep-me.xlsx", 2048)

	_, err := f.srv.DeleteItem(asCaller("", "ngac-user-1"), &pb.DeleteItemRequest{ItemId: folder.Id})

	requireFolderHasDocuments(t, err)
	assert.True(t, f.exists(t, folder.Id), "folder row must survive")
	assert.True(t, f.exists(t, fileID), "its files must survive")
	assert.Empty(t, f.policy.deletedNodes(), "the folder's access node must not be removed")
	assert.Empty(t, f.storage.removedKeys(), "no stored object may be removed")
}

// A document anywhere beneath the folder blocks it just the same.
func TestDeleteFolder_RefusedWhenADescendantHoldsDocuments(t *testing.T) {
	f := newDeleteFixture(t)
	top := f.folder(t, "Năm 2026", "")
	mid := f.folder(t, "Quý 4", top.Id)
	deep := f.folder(t, "Tháng 10", mid.Id)
	f.document(t, deep.Id)

	_, err := f.srv.DeleteItem(asCaller("", "ngac-user-1"), &pb.DeleteItemRequest{ItemId: top.Id})

	requireFolderHasDocuments(t, err)
	for _, id := range []string{top.Id, mid.Id, deep.Id} {
		assert.True(t, f.exists(t, id), "the whole tree must survive")
	}
	assert.Empty(t, f.policy.deletedNodes())
}

// The refusal is about documents only: a folder of plain files still deletes,
// its stored objects are removed and its access node leaves the graph.
func TestDeleteFolder_WithoutDocumentsStillDeletes(t *testing.T) {
	f := newDeleteFixture(t)
	folder := f.folder(t, "Tạm", "")
	f.file(t, folder.Id, "drive/gone.xlsx", 1024)

	_, err := f.srv.DeleteItem(asCaller("", "ngac-user-1"), &pb.DeleteItemRequest{ItemId: folder.Id})

	require.NoError(t, err)
	assert.False(t, f.exists(t, folder.Id))
	assert.Equal(t, []string{"drive/gone.xlsx"}, f.storage.removedKeys())
	assert.Equal(t, []string{folder.NgacNodeId}, f.policy.deletedNodes())
}

// Once the documents are gone, the same folder deletes.
func TestDeleteFolder_SucceedsAfterTheDocumentsAreRemoved(t *testing.T) {
	f := newDeleteFixture(t)
	folder := f.folder(t, "Sẽ dọn", "")
	f.document(t, folder.Id)
	_, err := f.srv.DeleteItem(asCaller("", "ngac-user-1"), &pb.DeleteItemRequest{ItemId: folder.Id})
	requireFolderHasDocuments(t, err)

	_, err = f.pool.Exec(context.Background(), `DELETE FROM text_documents WHERE folder_id = $1`, folder.Id)
	require.NoError(t, err)
	_, err = f.srv.DeleteItem(asCaller("", "ngac-user-1"), &pb.DeleteItemRequest{ItemId: folder.Id})

	require.NoError(t, err)
	assert.False(t, f.exists(t, folder.Id))
}

// The denied caller is told no before the documents are even counted, so the
// refusal cannot be used to learn what a folder holds.
func TestDeleteFolder_DeniedCallerLearnsNothingAboutDocuments(t *testing.T) {
	f := newDeleteFixture(t)
	folder := f.folder(t, "Kín", "")
	f.document(t, folder.Id)
	denied := grpcserver.NewDriveServer(f.pool, &mockPolicyReadDeny{}, f.policy, f.storage)

	_, err := denied.DeleteItem(asCaller("", "ngac-denied-user"), &pb.DeleteItemRequest{ItemId: folder.Id})

	require.Error(t, err)
	assert.Equal(t, codes.PermissionDenied, status.Code(err))
}

// A document saved between the check and the delete is caught by the database,
// and the store reports it as the same refusal rather than a bare 23503.
func TestStoreDeleteItem_ReportsDocumentsAsErrFolderHasDocuments(t *testing.T) {
	f := newDeleteFixture(t)
	folder := f.folder(t, "Vừa có văn bản", "")
	f.document(t, folder.Id)

	err := store.NewStore(f.pool).DeleteItem(context.Background(), folder.Id)

	assert.ErrorIs(t, err, store.ErrFolderHasDocuments)
	assert.True(t, f.exists(t, folder.Id), "the refused delete must leave the row")
}
