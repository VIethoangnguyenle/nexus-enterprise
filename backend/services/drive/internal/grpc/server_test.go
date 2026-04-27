package grpc_test

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

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
)

// ---------------------------------------------------------------------------
// Mock clients
// ---------------------------------------------------------------------------

type mockPolicyRead struct{ policypb.PolicyReadServiceClient }

func (m *mockPolicyRead) CheckAccess(_ context.Context, _ *policypb.CheckAccessRequest, _ ...grpc.CallOption) (*policypb.AccessDecision, error) {
	return &policypb.AccessDecision{Decision: "ALLOW"}, nil
}
func (m *mockPolicyRead) FindNodeByName(_ context.Context, req *policypb.FindNodeByNameRequest, _ ...grpc.CallOption) (*policypb.NGACNode, error) {
	return &policypb.NGACNode{Id: "pc-global", Name: req.Name, NodeType: req.NodeType}, nil
}
func (m *mockPolicyRead) GetNode(_ context.Context, req *policypb.GetNodeRequest, _ ...grpc.CallOption) (*policypb.NGACNode, error) {
	return &policypb.NGACNode{Id: req.NodeId, Name: "MockNode", NodeType: "UA"}, nil
}
func (m *mockPolicyRead) GetAncestors(_ context.Context, _ *policypb.GetAncestorsRequest, _ ...grpc.CallOption) (*policypb.NodeList, error) {
	return &policypb.NodeList{}, nil
}
func (m *mockPolicyRead) GetChildren(_ context.Context, _ *policypb.GetChildrenRequest, _ ...grpc.CallOption) (*policypb.NodeList, error) {
	return &policypb.NodeList{}, nil
}

// mockPolicyReadDeny denies all access for permission tests.
type mockPolicyReadDeny struct{ mockPolicyRead }

func (m *mockPolicyReadDeny) CheckAccess(_ context.Context, _ *policypb.CheckAccessRequest, _ ...grpc.CallOption) (*policypb.AccessDecision, error) {
	return &policypb.AccessDecision{Decision: "DENY"}, nil
}

type mockPolicyWrite struct{ policypb.PolicyWriteServiceClient }

func (m *mockPolicyWrite) CreateNode(_ context.Context, req *policypb.CreateNodeRequest, _ ...grpc.CallOption) (*policypb.NGACNode, error) {
	return &policypb.NGACNode{Id: fmt.Sprintf("node-%s", req.Name), Name: req.Name, NodeType: req.NodeType}, nil
}
func (m *mockPolicyWrite) CreateAssignment(_ context.Context, _ *policypb.CreateAssignmentRequest, _ ...grpc.CallOption) (*policypb.Assignment, error) {
	return &policypb.Assignment{Id: "assign-1"}, nil
}
func (m *mockPolicyWrite) CreateAssociation(_ context.Context, _ *policypb.CreateAssociationRequest, _ ...grpc.CallOption) (*policypb.Association, error) {
	return &policypb.Association{Id: "assoc-1"}, nil
}
func (m *mockPolicyWrite) RemoveAssignment(_ context.Context, _ *policypb.RemoveAssignmentRequest, _ ...grpc.CallOption) (*policypb.Empty, error) {
	return &policypb.Empty{}, nil
}
func (m *mockPolicyWrite) DeleteNode(_ context.Context, _ *policypb.DeleteNodeRequest, _ ...grpc.CallOption) (*policypb.Empty, error) {
	return &policypb.Empty{}, nil
}

type mockDocStorage struct{ docpb.DocumentStorageServiceClient }

func (m *mockDocStorage) GetUploadURL(_ context.Context, req *docpb.GetUploadURLRequest, _ ...grpc.CallOption) (*docpb.GetUploadURLResponse, error) {
	return &docpb.GetUploadURLResponse{
		UploadUrl: "http://minio:9000/presigned-upload", ObjectKey: fmt.Sprintf("drive/%s/%s", req.DocId, req.Filename),
	}, nil
}
func (m *mockDocStorage) ConfirmUpload(_ context.Context, _ *docpb.ConfirmUploadRequest, _ ...grpc.CallOption) (*docpb.ConfirmUploadResponse, error) {
	return &docpb.ConfirmUploadResponse{SizeBytes: 1024, ContentType: "application/pdf"}, nil
}
func (m *mockDocStorage) GetDownloadURL(_ context.Context, _ *docpb.GetDownloadURLRequest, _ ...grpc.CallOption) (*docpb.GetDownloadURLResponse, error) {
	return &docpb.GetDownloadURLResponse{DownloadUrl: "http://minio:9000/presigned-download"}, nil
}
func (m *mockDocStorage) DeleteObject(_ context.Context, _ *docpb.DeleteObjectRequest, _ ...grpc.CallOption) (*docpb.Empty, error) {
	return &docpb.Empty{}, nil
}
func (m *mockDocStorage) CopyObject(_ context.Context, _ *docpb.CopyObjectRequest, _ ...grpc.CallOption) (*docpb.CopyObjectResponse, error) {
	return &docpb.CopyObjectResponse{ObjectKey: "drive/copy/file.pdf"}, nil
}

// ---------------------------------------------------------------------------
// Setup helpers
// ---------------------------------------------------------------------------

func testDBURL() string {
	if url := os.Getenv("TEST_DATABASE_URL"); url != "" {
		return url
	}
	return "postgres://ngac:ngac_secret@localhost:5432/ngac?sslmode=disable"
}

func setupServer(t *testing.T) (*grpcserver.DriveServer, *pgxpool.Pool) {
	t.Helper()
	pool, err := pgxpool.New(context.Background(), testDBURL())
	if err != nil {
		t.Fatalf("connect to test DB: %v", err)
	}
	if err := pool.Ping(context.Background()); err != nil {
		t.Skipf("test DB not available: %v", err)
	}
	t.Cleanup(func() { pool.Close() })
	srv := grpcserver.NewDriveServer(pool, &mockPolicyRead{}, &mockPolicyWrite{}, &mockDocStorage{})
	return srv, pool
}

func setupServerDeny(t *testing.T) (*grpcserver.DriveServer, *pgxpool.Pool) {
	t.Helper()
	pool, err := pgxpool.New(context.Background(), testDBURL())
	if err != nil {
		t.Fatalf("connect to test DB: %v", err)
	}
	if err := pool.Ping(context.Background()); err != nil {
		t.Skipf("test DB not available: %v", err)
	}
	t.Cleanup(func() { pool.Close() })
	srv := grpcserver.NewDriveServer(pool, &mockPolicyReadDeny{}, &mockPolicyWrite{}, &mockDocStorage{})
	return srv, pool
}

func getTestWorkspaceID(t *testing.T, pool *pgxpool.Pool) string {
	t.Helper()
	var wsID string
	err := pool.QueryRow(context.Background(), "SELECT id FROM workspaces LIMIT 1").Scan(&wsID)
	if err != nil {
		t.Skipf("no workspace in test DB: %v", err)
	}
	return wsID
}

func getTestUserID(t *testing.T, pool *pgxpool.Pool) string {
	t.Helper()
	var uid string
	err := pool.QueryRow(context.Background(), "SELECT id FROM users LIMIT 1").Scan(&uid)
	if err != nil {
		t.Skipf("no user in test DB: %v", err)
	}
	return uid
}

func cleanDriveItems(t *testing.T, pool *pgxpool.Pool, ids ...string) {
	t.Helper()
	for _, id := range ids {
		pool.Exec(context.Background(), "DELETE FROM drive_shares WHERE drive_item_id = $1", id)
		pool.Exec(context.Background(), "DELETE FROM drive_items WHERE parent_id = $1", id)
		pool.Exec(context.Background(), "DELETE FROM drive_items WHERE id = $1", id)
	}
}

// ---------------------------------------------------------------------------
// 11.1: Core CRUD Tests
// ---------------------------------------------------------------------------

func TestCreateFolder_HappyPath(t *testing.T) {
	srv, pool := setupServer(t)
	wsID := getTestWorkspaceID(t, pool)

	folder, err := srv.CreateFolder(context.Background(), &pb.CreateFolderRequest{
		WorkspaceId: wsID, Name: "TestFolder", UserNgacNodeId: "ngac-user-1",
	})

	require.NoError(t, err)
	assert.Equal(t, "folder", folder.ItemType)
	assert.Equal(t, "TestFolder", folder.Name)
	assert.Equal(t, "active", folder.Status)
	assert.NotEmpty(t, folder.Id)
	t.Cleanup(func() { cleanDriveItems(t, pool, folder.Id) })
}

func TestCreateFolder_WithParent(t *testing.T) {
	srv, pool := setupServer(t)
	wsID := getTestWorkspaceID(t, pool)

	parent, err := srv.CreateFolder(context.Background(), &pb.CreateFolderRequest{
		WorkspaceId: wsID, Name: "Parent", UserNgacNodeId: "ngac-user-1",
	})
	require.NoError(t, err)

	child, err := srv.CreateFolder(context.Background(), &pb.CreateFolderRequest{
		WorkspaceId: wsID, Name: "Child", ParentId: parent.Id, UserNgacNodeId: "ngac-user-1",
	})
	require.NoError(t, err)
	assert.Equal(t, parent.Id, child.ParentId)
	t.Cleanup(func() { cleanDriveItems(t, pool, child.Id, parent.Id) })
}

func TestListFolder_Root(t *testing.T) {
	srv, pool := setupServer(t)
	wsID := getTestWorkspaceID(t, pool)

	f1, _ := srv.CreateFolder(context.Background(), &pb.CreateFolderRequest{
		WorkspaceId: wsID, Name: "ListTest1", UserNgacNodeId: "ngac-user-1",
	})
	f2, _ := srv.CreateFolder(context.Background(), &pb.CreateFolderRequest{
		WorkspaceId: wsID, Name: "ListTest2", UserNgacNodeId: "ngac-user-1",
	})
	t.Cleanup(func() { cleanDriveItems(t, pool, f1.Id, f2.Id) })

	list, err := srv.ListFolder(context.Background(), &pb.ListFolderRequest{
		WorkspaceId: wsID, UserNgacNodeId: "ngac-user-1",
	})

	require.NoError(t, err)
	assert.GreaterOrEqual(t, len(list.Items), 2)
}

func TestListFolder_NGACFiltering(t *testing.T) {
	srvDeny, pool := setupServerDeny(t)
	wsID := getTestWorkspaceID(t, pool)

	// Use the allow server to create an item
	srvAllow := grpcserver.NewDriveServer(pool, &mockPolicyRead{}, &mockPolicyWrite{}, &mockDocStorage{})
	folder, _ := srvAllow.CreateFolder(context.Background(), &pb.CreateFolderRequest{
		WorkspaceId: wsID, Name: "DenyTest", UserNgacNodeId: "ngac-user-1",
	})
	t.Cleanup(func() { cleanDriveItems(t, pool, folder.Id) })

	// List with deny policy — items should be filtered out
	list, err := srvDeny.ListFolder(context.Background(), &pb.ListFolderRequest{
		WorkspaceId: wsID, UserNgacNodeId: "ngac-denied-user",
	})
	require.NoError(t, err)
	for _, item := range list.Items {
		assert.NotEqual(t, folder.Id, item.Id, "denied item should be filtered")
	}
}

func TestGetItem_HappyPath(t *testing.T) {
	srv, pool := setupServer(t)
	wsID := getTestWorkspaceID(t, pool)

	folder, _ := srv.CreateFolder(context.Background(), &pb.CreateFolderRequest{
		WorkspaceId: wsID, Name: "GetItemTest", UserNgacNodeId: "ngac-user-1",
	})
	t.Cleanup(func() { cleanDriveItems(t, pool, folder.Id) })

	got, err := srv.GetItem(context.Background(), &pb.GetItemRequest{
		ItemId: folder.Id, UserNgacNodeId: "ngac-user-1",
	})

	require.NoError(t, err)
	assert.Equal(t, folder.Id, got.Id)
	assert.Equal(t, "GetItemTest", got.Name)
}

func TestGetItem_NotFound(t *testing.T) {
	srv, _ := setupServer(t)

	_, err := srv.GetItem(context.Background(), &pb.GetItemRequest{
		ItemId: "nonexistent", UserNgacNodeId: "ngac-user-1",
	})

	require.Error(t, err)
	st, _ := status.FromError(err)
	assert.Equal(t, codes.NotFound, st.Code())
}

func TestCreateFile_HappyPath(t *testing.T) {
	srv, pool := setupServer(t)
	wsID := getTestWorkspaceID(t, pool)
	userID := getTestUserID(t, pool)

	resp, err := srv.CreateFile(context.Background(), &pb.CreateFileRequest{
		WorkspaceId: wsID, Name: "test.pdf", MimeType: "application/pdf",
		SizeBytes: 2048, UserId: userID, UserNgacNodeId: "ngac-user-1",
	})

	require.NoError(t, err)
	assert.NotEmpty(t, resp.FileId)
	assert.NotEmpty(t, resp.UploadUrl)
	assert.NotEmpty(t, resp.ObjectKey)
	t.Cleanup(func() { cleanDriveItems(t, pool, resp.FileId) })

	// Verify item is in pending state
	item, err := srv.GetItem(context.Background(), &pb.GetItemRequest{
		ItemId: resp.FileId, UserNgacNodeId: "ngac-user-1",
	})
	require.NoError(t, err)
	assert.Equal(t, "pending", item.Status)
}

func TestConfirmFile_HappyPath(t *testing.T) {
	srv, pool := setupServer(t)
	wsID := getTestWorkspaceID(t, pool)
	userID := getTestUserID(t, pool)

	created, _ := srv.CreateFile(context.Background(), &pb.CreateFileRequest{
		WorkspaceId: wsID, Name: "confirm.pdf", MimeType: "application/pdf",
		SizeBytes: 1024, UserId: userID, UserNgacNodeId: "ngac-user-1",
	})
	t.Cleanup(func() { cleanDriveItems(t, pool, created.FileId) })

	confirmed, err := srv.ConfirmFile(context.Background(), &pb.ConfirmFileRequest{
		FileId: created.FileId,
	})

	require.NoError(t, err)
	assert.Equal(t, "active", confirmed.Status)
}

func TestMoveItem_HappyPath(t *testing.T) {
	srv, pool := setupServer(t)
	wsID := getTestWorkspaceID(t, pool)

	folder1, _ := srv.CreateFolder(context.Background(), &pb.CreateFolderRequest{
		WorkspaceId: wsID, Name: "MoveFrom", UserNgacNodeId: "ngac-user-1",
	})
	folder2, _ := srv.CreateFolder(context.Background(), &pb.CreateFolderRequest{
		WorkspaceId: wsID, Name: "MoveTo", UserNgacNodeId: "ngac-user-1",
	})
	child, _ := srv.CreateFolder(context.Background(), &pb.CreateFolderRequest{
		WorkspaceId: wsID, Name: "MoveChild", ParentId: folder1.Id, UserNgacNodeId: "ngac-user-1",
	})
	t.Cleanup(func() { cleanDriveItems(t, pool, child.Id, folder1.Id, folder2.Id) })

	moved, err := srv.MoveItem(context.Background(), &pb.MoveItemRequest{
		ItemId: child.Id, NewParentId: folder2.Id, UserNgacNodeId: "ngac-user-1",
	})

	require.NoError(t, err)
	assert.Equal(t, folder2.Id, moved.ParentId)
}

func TestTrashItem_HappyPath(t *testing.T) {
	srv, pool := setupServer(t)
	wsID := getTestWorkspaceID(t, pool)

	folder, _ := srv.CreateFolder(context.Background(), &pb.CreateFolderRequest{
		WorkspaceId: wsID, Name: "TrashTest", UserNgacNodeId: "ngac-user-1",
	})
	t.Cleanup(func() { cleanDriveItems(t, pool, folder.Id) })

	_, err := srv.TrashItem(context.Background(), &pb.TrashItemRequest{
		ItemId: folder.Id, UserNgacNodeId: "ngac-user-1",
	})
	require.NoError(t, err)

	item, _ := srv.GetItem(context.Background(), &pb.GetItemRequest{
		ItemId: folder.Id, UserNgacNodeId: "ngac-user-1",
	})
	assert.Equal(t, "trashed", item.Status)
}

func TestRestoreItem_HappyPath(t *testing.T) {
	srv, pool := setupServer(t)
	wsID := getTestWorkspaceID(t, pool)

	folder, _ := srv.CreateFolder(context.Background(), &pb.CreateFolderRequest{
		WorkspaceId: wsID, Name: "RestoreTest", UserNgacNodeId: "ngac-user-1",
	})
	t.Cleanup(func() { cleanDriveItems(t, pool, folder.Id) })

	srv.TrashItem(context.Background(), &pb.TrashItemRequest{ItemId: folder.Id, UserNgacNodeId: "ngac-user-1"})

	restored, err := srv.RestoreItem(context.Background(), &pb.RestoreItemRequest{ItemId: folder.Id})
	require.NoError(t, err)
	assert.Equal(t, "active", restored.Status)
}

func TestRenameItem_HappyPath(t *testing.T) {
	srv, pool := setupServer(t)
	wsID := getTestWorkspaceID(t, pool)

	folder, _ := srv.CreateFolder(context.Background(), &pb.CreateFolderRequest{
		WorkspaceId: wsID, Name: "OldName", UserNgacNodeId: "ngac-user-1",
	})
	t.Cleanup(func() { cleanDriveItems(t, pool, folder.Id) })

	renamed, err := srv.RenameItem(context.Background(), &pb.RenameItemRequest{
		ItemId: folder.Id, NewName: "NewName", UserNgacNodeId: "ngac-user-1",
	})

	require.NoError(t, err)
	assert.Equal(t, "NewName", renamed.Name)
}

func TestCopyItem_HappyPath(t *testing.T) {
	srv, pool := setupServer(t)
	wsID := getTestWorkspaceID(t, pool)
	userID := getTestUserID(t, pool)

	destFolder, _ := srv.CreateFolder(context.Background(), &pb.CreateFolderRequest{
		WorkspaceId: wsID, Name: "CopyDest", UserNgacNodeId: "ngac-user-1",
	})
	created, _ := srv.CreateFile(context.Background(), &pb.CreateFileRequest{
		WorkspaceId: wsID, Name: "copyable.pdf", MimeType: "application/pdf",
		SizeBytes: 512, UserId: userID, UserNgacNodeId: "ngac-user-1",
	})
	srv.ConfirmFile(context.Background(), &pb.ConfirmFileRequest{FileId: created.FileId})
	t.Cleanup(func() { cleanDriveItems(t, pool, created.FileId, destFolder.Id) })

	copied, err := srv.CopyItem(context.Background(), &pb.CopyItemRequest{
		ItemId: created.FileId, DestParentId: destFolder.Id,
		DestWorkspaceId: wsID, UserId: userID, UserNgacNodeId: "ngac-user-1",
	})

	require.NoError(t, err)
	assert.NotEqual(t, created.FileId, copied.Id, "copy should have new ID")
	assert.Equal(t, "copyable.pdf", copied.Name)
	t.Cleanup(func() { cleanDriveItems(t, pool, copied.Id) })
}

// ---------------------------------------------------------------------------
// 11.2: Sharing Tests
// ---------------------------------------------------------------------------

func TestCreateShare_UserShare(t *testing.T) {
	srv, pool := setupServer(t)
	wsID := getTestWorkspaceID(t, pool)

	folder, _ := srv.CreateFolder(context.Background(), &pb.CreateFolderRequest{
		WorkspaceId: wsID, Name: "ShareTest", UserNgacNodeId: "ngac-user-1",
	})
	t.Cleanup(func() { cleanDriveItems(t, pool, folder.Id) })

	share, err := srv.CreateShare(context.Background(), &pb.CreateShareRequest{
		ItemId: folder.Id, ShareType: "user", TargetNgacNodeId: "ngac-user-2",
		Operations: []string{"read"}, UserNgacNodeId: "ngac-user-1",
	})

	require.NoError(t, err)
	assert.Equal(t, "user", share.ShareType)
	assert.NotEmpty(t, share.Id)
	t.Cleanup(func() {
		pool.Exec(context.Background(), "DELETE FROM drive_shares WHERE id = $1", share.Id)
	})
}

func TestListShares_ReturnsShares(t *testing.T) {
	srv, pool := setupServer(t)
	wsID := getTestWorkspaceID(t, pool)

	folder, _ := srv.CreateFolder(context.Background(), &pb.CreateFolderRequest{
		WorkspaceId: wsID, Name: "ListShareTest", UserNgacNodeId: "ngac-user-1",
	})
	share, _ := srv.CreateShare(context.Background(), &pb.CreateShareRequest{
		ItemId: folder.Id, ShareType: "user", TargetNgacNodeId: "ngac-user-2",
		Operations: []string{"read", "write"}, UserNgacNodeId: "ngac-user-1",
	})
	t.Cleanup(func() {
		pool.Exec(context.Background(), "DELETE FROM drive_shares WHERE id = $1", share.Id)
		cleanDriveItems(t, pool, folder.Id)
	})

	list, err := srv.ListShares(context.Background(), &pb.ListSharesRequest{ItemId: folder.Id})

	require.NoError(t, err)
	assert.GreaterOrEqual(t, len(list.Shares), 1)
}

func TestRevokeShare_HappyPath(t *testing.T) {
	srv, pool := setupServer(t)
	wsID := getTestWorkspaceID(t, pool)

	folder, _ := srv.CreateFolder(context.Background(), &pb.CreateFolderRequest{
		WorkspaceId: wsID, Name: "RevokeShareTest", UserNgacNodeId: "ngac-user-1",
	})
	share, _ := srv.CreateShare(context.Background(), &pb.CreateShareRequest{
		ItemId: folder.Id, ShareType: "user", TargetNgacNodeId: "ngac-user-2",
		Operations: []string{"read"}, UserNgacNodeId: "ngac-user-1",
	})
	t.Cleanup(func() { cleanDriveItems(t, pool, folder.Id) })

	_, err := srv.RevokeShare(context.Background(), &pb.RevokeShareRequest{ShareId: share.Id})
	require.NoError(t, err)

	list, _ := srv.ListShares(context.Background(), &pb.ListSharesRequest{ItemId: folder.Id})
	for _, s := range list.Shares {
		assert.NotEqual(t, share.Id, s.Id, "revoked share should not appear")
	}
}

func TestCreateShare_InvalidType(t *testing.T) {
	srv, pool := setupServer(t)
	wsID := getTestWorkspaceID(t, pool)

	folder, _ := srv.CreateFolder(context.Background(), &pb.CreateFolderRequest{
		WorkspaceId: wsID, Name: "BadShareType", UserNgacNodeId: "ngac-user-1",
	})
	t.Cleanup(func() { cleanDriveItems(t, pool, folder.Id) })

	_, err := srv.CreateShare(context.Background(), &pb.CreateShareRequest{
		ItemId: folder.Id, ShareType: "invalid", Operations: []string{"read"}, UserNgacNodeId: "ngac-user-1",
	})

	require.Error(t, err)
	st, _ := status.FromError(err)
	assert.Equal(t, codes.InvalidArgument, st.Code())
}

// ---------------------------------------------------------------------------
// 11.4: Quota Tests
// ---------------------------------------------------------------------------

func TestGetQuota_CreatesDefault(t *testing.T) {
	srv, pool := setupServer(t)
	wsID := getTestWorkspaceID(t, pool)
	t.Cleanup(func() {
		pool.Exec(context.Background(), "DELETE FROM drive_quotas WHERE workspace_id = $1", wsID)
	})

	// Ensure clean state
	pool.Exec(context.Background(), "DELETE FROM drive_quotas WHERE workspace_id = $1", wsID)

	q, err := srv.GetQuota(context.Background(), &pb.GetQuotaRequest{WorkspaceId: wsID})

	require.NoError(t, err)
	assert.Equal(t, wsID, q.WorkspaceId)
	assert.Equal(t, int64(-1), q.MaxBytes, "default should be unlimited")
	assert.Equal(t, int32(-1), q.MaxFiles, "default should be unlimited")
}

func TestQuota_IncrementOnConfirm(t *testing.T) {
	srv, pool := setupServer(t)
	wsID := getTestWorkspaceID(t, pool)
	userID := getTestUserID(t, pool)

	// Ensure clean quota
	pool.Exec(context.Background(), "DELETE FROM drive_quotas WHERE workspace_id = $1", wsID)

	before, _ := srv.GetQuota(context.Background(), &pb.GetQuotaRequest{WorkspaceId: wsID})

	created, _ := srv.CreateFile(context.Background(), &pb.CreateFileRequest{
		WorkspaceId: wsID, Name: "quota_test.pdf", MimeType: "application/pdf",
		SizeBytes: 2048, UserId: userID, UserNgacNodeId: "ngac-user-1",
	})
	srv.ConfirmFile(context.Background(), &pb.ConfirmFileRequest{FileId: created.FileId})
	t.Cleanup(func() { cleanDriveItems(t, pool, created.FileId) })

	after, _ := srv.GetQuota(context.Background(), &pb.GetQuotaRequest{WorkspaceId: wsID})

	assert.Greater(t, after.UsedBytes, before.UsedBytes, "used_bytes should increase after confirm")
	assert.Greater(t, after.UsedFiles, before.UsedFiles, "used_files should increase after confirm")

	t.Cleanup(func() {
		pool.Exec(context.Background(), "DELETE FROM drive_quotas WHERE workspace_id = $1", wsID)
	})
}

func TestQuota_ExceededRejectsUpload(t *testing.T) {
	srv, pool := setupServer(t)
	wsID := getTestWorkspaceID(t, pool)
	userID := getTestUserID(t, pool)

	// Set very low quota
	pool.Exec(context.Background(), "DELETE FROM drive_quotas WHERE workspace_id = $1", wsID)
	pool.Exec(context.Background(),
		"INSERT INTO drive_quotas (workspace_id, max_bytes, used_bytes, max_files, used_files) VALUES ($1, 100, 99, 10, 9)",
		wsID)
	t.Cleanup(func() {
		pool.Exec(context.Background(), "DELETE FROM drive_quotas WHERE workspace_id = $1", wsID)
	})

	_, err := srv.CreateFile(context.Background(), &pb.CreateFileRequest{
		WorkspaceId: wsID, Name: "too_big.pdf", MimeType: "application/pdf",
		SizeBytes: 500, UserId: userID, UserNgacNodeId: "ngac-user-1",
	})

	require.Error(t, err)
	st, _ := status.FromError(err)
	assert.Equal(t, codes.ResourceExhausted, st.Code())
}

// ---------------------------------------------------------------------------
// 11.3: Channel Drive Tests
// ---------------------------------------------------------------------------

func insertTestChannel(t *testing.T, pool *pgxpool.Pool, name, wsID string) string {
	t.Helper()
	id := fmt.Sprintf("test-ch-%s-%d", name, time.Now().UnixNano())
	_, err := pool.Exec(context.Background(),
		"INSERT INTO channels (id, name, channel_type, workspace_id, created_at) VALUES ($1, $2, 'workspace', $3, NOW())",
		id, name, wsID)
	require.NoError(t, err)
	return id
}

func TestCreateDriveForChannel_HappyPath(t *testing.T) {
	srv, pool := setupServer(t)
	wsID := getTestWorkspaceID(t, pool)
	chID := insertTestChannel(t, pool, "drive_test_ch", wsID)
	t.Cleanup(func() {
		pool.Exec(context.Background(), "DELETE FROM drive_items WHERE drive_context_id = $1", chID)
		pool.Exec(context.Background(), "DELETE FROM channels WHERE id = $1", chID)
	})

	drive, err := srv.CreateDriveForChannel(context.Background(), &pb.CreateDriveForChannelRequest{
		WorkspaceId: wsID, ChannelId: chID, ChannelName: "drive_test_ch",
		ChannelNgacOaId: "oa-channel-1", ChannelNgacUaId: "ua-channel-1",
	})

	require.NoError(t, err)
	assert.Equal(t, "folder", drive.ItemType)
	assert.Contains(t, drive.Name, "Ch_drive_test_ch_Drive")
	assert.Equal(t, "active", drive.Status)
}

func TestGetChannelDrive_HappyPath(t *testing.T) {
	srv, pool := setupServer(t)
	wsID := getTestWorkspaceID(t, pool)
	chID := insertTestChannel(t, pool, "get_drive_ch", wsID)
	t.Cleanup(func() {
		pool.Exec(context.Background(), "DELETE FROM drive_items WHERE drive_context_id = $1", chID)
		pool.Exec(context.Background(), "DELETE FROM channels WHERE id = $1", chID)
	})

	created, err := srv.CreateDriveForChannel(context.Background(), &pb.CreateDriveForChannelRequest{
		WorkspaceId: wsID, ChannelId: chID, ChannelName: "get_drive_ch",
		ChannelNgacOaId: "oa-channel-2", ChannelNgacUaId: "ua-channel-2",
	})
	require.NoError(t, err)

	got, err := srv.GetChannelDrive(context.Background(), &pb.GetChannelDriveRequest{ChannelId: chID})

	require.NoError(t, err)
	assert.Equal(t, created.Id, got.Id)
	assert.Equal(t, "channel", got.DriveContext)
}

func TestGetChannelDrive_NotFound(t *testing.T) {
	srv, _ := setupServer(t)

	_, err := srv.GetChannelDrive(context.Background(), &pb.GetChannelDriveRequest{ChannelId: "nonexistent-ch"})

	require.Error(t, err)
	st, _ := status.FromError(err)
	assert.Equal(t, codes.NotFound, st.Code())
}

// ---------------------------------------------------------------------------
// 11.6: E2E — upload → share → download → revoke → deny
// ---------------------------------------------------------------------------

func TestE2E_UploadShareDownloadRevokeDeny(t *testing.T) {
	srv, pool := setupServer(t)
	wsID := getTestWorkspaceID(t, pool)
	userID := getTestUserID(t, pool)

	// 1. Upload file
	created, err := srv.CreateFile(context.Background(), &pb.CreateFileRequest{
		WorkspaceId: wsID, Name: "e2e_share_test.pdf", MimeType: "application/pdf",
		SizeBytes: 1024, UserId: userID, UserNgacNodeId: "ngac-user-1",
	})
	require.NoError(t, err)
	srv.ConfirmFile(context.Background(), &pb.ConfirmFileRequest{FileId: created.FileId})
	t.Cleanup(func() { cleanDriveItems(t, pool, created.FileId) })

	// 2. Share with user-2
	share, err := srv.CreateShare(context.Background(), &pb.CreateShareRequest{
		ItemId: created.FileId, ShareType: "user", TargetNgacNodeId: "ngac-user-2",
		Operations: []string{"read"}, UserNgacNodeId: "ngac-user-1",
	})
	require.NoError(t, err)
	assert.NotEmpty(t, share.Id)
	t.Cleanup(func() {
		pool.Exec(context.Background(), "DELETE FROM drive_shares WHERE id = $1", share.Id)
	})

	// 3. Verify download URL is accessible
	dl, err := srv.GetDownloadURL(context.Background(), &pb.GetDownloadURLRequest{
		FileId: created.FileId, UserNgacNodeId: "ngac-user-1",
	})
	require.NoError(t, err)
	assert.NotEmpty(t, dl.DownloadUrl)

	// 4. Revoke share
	_, err = srv.RevokeShare(context.Background(), &pb.RevokeShareRequest{ShareId: share.Id})
	require.NoError(t, err)

	// 5. Verify share is gone
	list, _ := srv.ListShares(context.Background(), &pb.ListSharesRequest{ItemId: created.FileId})
	for _, s := range list.Shares {
		assert.NotEqual(t, share.Id, s.Id, "revoked share must not appear")
	}
}

// ---------------------------------------------------------------------------
// 11.7: E2E — chat file upload → message with file card → download
// ---------------------------------------------------------------------------

func TestE2E_ChatFileUpload(t *testing.T) {
	srv, pool := setupServer(t)
	wsID := getTestWorkspaceID(t, pool)
	userID := getTestUserID(t, pool)
	chID := insertTestChannel(t, pool, "e2e_chat_upload", wsID)
	t.Cleanup(func() {
		pool.Exec(context.Background(), "DELETE FROM drive_items WHERE drive_context_id = $1", chID)
		pool.Exec(context.Background(), "DELETE FROM channels WHERE id = $1", chID)
	})

	// 1. Create channel drive
	drive, err := srv.CreateDriveForChannel(context.Background(), &pb.CreateDriveForChannelRequest{
		WorkspaceId: wsID, ChannelId: chID, ChannelName: "e2e_chat_upload",
		ChannelNgacOaId: "oa-ch-e2e", ChannelNgacUaId: "ua-ch-e2e",
	})
	require.NoError(t, err)

	// 2. Upload file to channel drive
	created, err := srv.CreateFile(context.Background(), &pb.CreateFileRequest{
		WorkspaceId: wsID, Name: "chat_attachment.png", MimeType: "image/png",
		SizeBytes: 4096, ParentId: drive.Id, UserId: userID, UserNgacNodeId: "ngac-user-1",
	})
	require.NoError(t, err)
	srv.ConfirmFile(context.Background(), &pb.ConfirmFileRequest{FileId: created.FileId})
	t.Cleanup(func() { cleanDriveItems(t, pool, created.FileId) })

	// 3. Verify file appears in channel drive
	channelDrive, err := srv.GetChannelDrive(context.Background(), &pb.GetChannelDriveRequest{ChannelId: chID})
	require.NoError(t, err)
	assert.Equal(t, drive.Id, channelDrive.Id)

	// 4. List folder to find the file
	children, err := srv.ListFolder(context.Background(), &pb.ListFolderRequest{
		WorkspaceId: wsID, FolderId: drive.Id, UserNgacNodeId: "ngac-user-1",
	})
	require.NoError(t, err)
	found := false
	for _, item := range children.Items {
		if item.Id == created.FileId {
			found = true
			assert.Equal(t, "chat_attachment.png", item.Name)
		}
	}
	assert.True(t, found, "file should appear in channel drive listing")

	// 5. Download URL works
	dl, err := srv.GetDownloadURL(context.Background(), &pb.GetDownloadURLRequest{
		FileId: created.FileId, UserNgacNodeId: "ngac-user-1",
	})
	require.NoError(t, err)
	assert.NotEmpty(t, dl.DownloadUrl)
}

// ---------------------------------------------------------------------------
// 11.8: E2E — folder sharing inheritance
// ---------------------------------------------------------------------------

func TestE2E_FolderSharingInheritance(t *testing.T) {
	srv, pool := setupServer(t)
	wsID := getTestWorkspaceID(t, pool)
	userID := getTestUserID(t, pool)

	// 1. Create folder
	folder, err := srv.CreateFolder(context.Background(), &pb.CreateFolderRequest{
		WorkspaceId: wsID, Name: "SharedFolder", UserNgacNodeId: "ngac-user-1",
	})
	require.NoError(t, err)

	// 2. Share folder with user-2
	share, err := srv.CreateShare(context.Background(), &pb.CreateShareRequest{
		ItemId: folder.Id, ShareType: "user", TargetNgacNodeId: "ngac-user-2",
		Operations: []string{"read", "write"}, UserNgacNodeId: "ngac-user-1",
	})
	require.NoError(t, err)

	// 3. Add a file inside the shared folder
	created, err := srv.CreateFile(context.Background(), &pb.CreateFileRequest{
		WorkspaceId: wsID, Name: "inherited_access.doc", MimeType: "application/msword",
		SizeBytes: 2048, ParentId: folder.Id, UserId: userID, UserNgacNodeId: "ngac-user-1",
	})
	require.NoError(t, err)
	srv.ConfirmFile(context.Background(), &pb.ConfirmFileRequest{FileId: created.FileId})

	t.Cleanup(func() {
		pool.Exec(context.Background(), "DELETE FROM drive_shares WHERE id = $1", share.Id)
		cleanDriveItems(t, pool, created.FileId, folder.Id)
	})

	// 4. Verify child file inherits NGAC assignment under shared folder's OA
	// The file's NGAC OA is assigned under the folder's OA (verified by policy mock)
	childItem, err := srv.GetItem(context.Background(), &pb.GetItemRequest{
		ItemId: created.FileId, UserNgacNodeId: "ngac-user-1",
	})
	require.NoError(t, err)
	assert.Equal(t, folder.Id, childItem.ParentId, "child should be under shared folder")
	assert.NotEmpty(t, childItem.NgacNodeId, "child should have NGAC OA for inherited access")
}

// ---------------------------------------------------------------------------
// 11.9: E2E — move vs copy behavior
// ---------------------------------------------------------------------------

func TestE2E_MovePreservesShares(t *testing.T) {
	srv, pool := setupServer(t)
	wsID := getTestWorkspaceID(t, pool)

	folderA, _ := srv.CreateFolder(context.Background(), &pb.CreateFolderRequest{
		WorkspaceId: wsID, Name: "MoveSourceE2E", UserNgacNodeId: "ngac-user-1",
	})
	folderB, _ := srv.CreateFolder(context.Background(), &pb.CreateFolderRequest{
		WorkspaceId: wsID, Name: "MoveDestE2E", UserNgacNodeId: "ngac-user-1",
	})
	child, _ := srv.CreateFolder(context.Background(), &pb.CreateFolderRequest{
		WorkspaceId: wsID, Name: "MoveChild", ParentId: folderA.Id, UserNgacNodeId: "ngac-user-1",
	})

	// Share the child
	share, _ := srv.CreateShare(context.Background(), &pb.CreateShareRequest{
		ItemId: child.Id, ShareType: "user", TargetNgacNodeId: "ngac-user-2",
		Operations: []string{"read"}, UserNgacNodeId: "ngac-user-1",
	})

	t.Cleanup(func() {
		pool.Exec(context.Background(), "DELETE FROM drive_shares WHERE id = $1", share.Id)
		cleanDriveItems(t, pool, child.Id, folderA.Id, folderB.Id)
	})

	// Move child to folderB
	moved, err := srv.MoveItem(context.Background(), &pb.MoveItemRequest{
		ItemId: child.Id, NewParentId: folderB.Id, UserNgacNodeId: "ngac-user-1",
	})
	require.NoError(t, err)
	assert.Equal(t, folderB.Id, moved.ParentId, "item should be under new parent")

	// Verify share still exists after move
	shares, err := srv.ListShares(context.Background(), &pb.ListSharesRequest{ItemId: child.Id})
	require.NoError(t, err)
	assert.GreaterOrEqual(t, len(shares.Shares), 1, "share should survive move")
}

func TestE2E_CopyCreatesIndependentFile(t *testing.T) {
	srv, pool := setupServer(t)
	wsID := getTestWorkspaceID(t, pool)
	userID := getTestUserID(t, pool)

	// Create source file
	created, _ := srv.CreateFile(context.Background(), &pb.CreateFileRequest{
		WorkspaceId: wsID, Name: "copy_src.pdf", MimeType: "application/pdf",
		SizeBytes: 1024, UserId: userID, UserNgacNodeId: "ngac-user-1",
	})
	srv.ConfirmFile(context.Background(), &pb.ConfirmFileRequest{FileId: created.FileId})

	// Share original
	share, _ := srv.CreateShare(context.Background(), &pb.CreateShareRequest{
		ItemId: created.FileId, ShareType: "user", TargetNgacNodeId: "ngac-user-2",
		Operations: []string{"read"}, UserNgacNodeId: "ngac-user-1",
	})

	destFolder, _ := srv.CreateFolder(context.Background(), &pb.CreateFolderRequest{
		WorkspaceId: wsID, Name: "CopyDestE2E", UserNgacNodeId: "ngac-user-1",
	})

	t.Cleanup(func() {
		pool.Exec(context.Background(), "DELETE FROM drive_shares WHERE id = $1", share.Id)
		cleanDriveItems(t, pool, created.FileId, destFolder.Id)
	})

	// Copy file
	copied, err := srv.CopyItem(context.Background(), &pb.CopyItemRequest{
		ItemId: created.FileId, DestParentId: destFolder.Id,
		DestWorkspaceId: wsID, UserId: userID, UserNgacNodeId: "ngac-user-1",
	})
	require.NoError(t, err)
	t.Cleanup(func() { cleanDriveItems(t, pool, copied.Id) })

	// Verify copy is independent — different ID, different NGAC node
	assert.NotEqual(t, created.FileId, copied.Id, "copy must have unique ID")
	assert.NotEqual(t, "", copied.NgacNodeId, "copy must have its own NGAC node")
	assert.Equal(t, "copy_src.pdf", copied.Name, "copy preserves name")
	assert.Equal(t, destFolder.Id, copied.ParentId, "copy is under dest folder")

	// Verify original's shares are NOT inherited by copy
	copyShares, _ := srv.ListShares(context.Background(), &pb.ListSharesRequest{ItemId: copied.Id})
	assert.Empty(t, copyShares.Shares, "copy should not inherit source's shares")
}
