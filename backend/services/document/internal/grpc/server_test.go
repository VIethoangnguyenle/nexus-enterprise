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
	grpcstatus "google.golang.org/grpc/status"

	pb "ngac-platform/proto/document"
	policypb "ngac-platform/proto/policy"
	grpcserver "ngac-platform/services/document/internal/grpc"
)

// ---------------------------------------------------------------------------
// Mock clients — split into read and write per CQRS architecture
// ---------------------------------------------------------------------------

type mockPolicyReadClient struct {
	policypb.PolicyReadServiceClient
}

func (m *mockPolicyReadClient) CheckAccess(_ context.Context, _ *policypb.CheckAccessRequest, _ ...grpc.CallOption) (*policypb.AccessDecision, error) {
	return &policypb.AccessDecision{Decision: "ALLOW"}, nil
}

func (m *mockPolicyReadClient) FindNodeByName(_ context.Context, _ *policypb.FindNodeByNameRequest, _ ...grpc.CallOption) (*policypb.NGACNode, error) {
	return &policypb.NGACNode{Id: "oa-public-docs", Name: "PublicDocs", NodeType: "OA"}, nil
}

func (m *mockPolicyReadClient) GetDescendants(_ context.Context, _ *policypb.GetDescendantsRequest, _ ...grpc.CallOption) (*policypb.NodeList, error) {
	return &policypb.NodeList{}, nil
}

func (m *mockPolicyReadClient) GetParents(_ context.Context, _ *policypb.GetParentsRequest, _ ...grpc.CallOption) (*policypb.NodeList, error) {
	return &policypb.NodeList{}, nil
}

type mockPolicyWriteClient struct {
	policypb.PolicyWriteServiceClient
	pool *pgxpool.Pool
}

func (m *mockPolicyWriteClient) CreateNode(_ context.Context, req *policypb.CreateNodeRequest, _ ...grpc.CallOption) (*policypb.NGACNode, error) {
	nodeID := fmt.Sprintf("ngac-%s-%d", req.Name, time.Now().UnixNano())
	m.pool.Exec(context.Background(),
		"INSERT INTO ngac_nodes (id, name, node_type) VALUES ($1, $2, $3) ON CONFLICT DO NOTHING",
		nodeID, req.Name, req.NodeType,
	)
	return &policypb.NGACNode{Id: nodeID, Name: req.Name, NodeType: req.NodeType}, nil
}

func (m *mockPolicyWriteClient) CreateAssignment(_ context.Context, _ *policypb.CreateAssignmentRequest, _ ...grpc.CallOption) (*policypb.Assignment, error) {
	return &policypb.Assignment{Id: "assign-1"}, nil
}

func (m *mockPolicyWriteClient) CreateAssociation(_ context.Context, _ *policypb.CreateAssociationRequest, _ ...grpc.CallOption) (*policypb.Association, error) {
	return &policypb.Association{Id: "assoc-1"}, nil
}

func (m *mockPolicyWriteClient) DeleteNode(_ context.Context, _ *policypb.DeleteNodeRequest, _ ...grpc.CallOption) (*policypb.Empty, error) {
	return &policypb.Empty{}, nil
}

func (m *mockPolicyWriteClient) RemoveAssignment(_ context.Context, _ *policypb.RemoveAssignmentRequest, _ ...grpc.CallOption) (*policypb.Empty, error) {
	return &policypb.Empty{}, nil
}

// mockPolicyDenyReadClient always denies access checks.
type mockPolicyDenyReadClient struct {
	mockPolicyReadClient
}

func (m *mockPolicyDenyReadClient) CheckAccess(_ context.Context, _ *policypb.CheckAccessRequest, _ ...grpc.CallOption) (*policypb.AccessDecision, error) {
	return &policypb.AccessDecision{Decision: "DENY"}, nil
}

// ---------------------------------------------------------------------------
// Test setup
// ---------------------------------------------------------------------------

func testDBURL() string {
	if url := os.Getenv("TEST_DATABASE_URL"); url != "" {
		return url
	}
	return "postgres://ngac:ngac_secret@localhost:5432/ngac?sslmode=disable"
}

func setupTestServer(t *testing.T) (*grpcserver.DocumentServer, *pgxpool.Pool) {
	t.Helper()
	pool, err := pgxpool.New(context.Background(), testDBURL())
	if err != nil {
		t.Fatalf("connect to test DB: %v", err)
	}
	if err := pool.Ping(context.Background()); err != nil {
		t.Skipf("test DB not available: %v", err)
	}
	t.Cleanup(func() { pool.Close() })

	pr := &mockPolicyReadClient{}
	pw := &mockPolicyWriteClient{pool: pool}
	// Pass nil for MinIO — tests that need it will be skipped
	srv := grpcserver.NewDocumentServer(pool, pr, pw, nil, nil)
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

// ---------------------------------------------------------------------------
// 7.1: TestUpload + TestList (access filtering)
// ---------------------------------------------------------------------------

func TestUpload_HappyPath(t *testing.T) {
	srv, pool := setupTestServer(t)
	wsID := getTestWorkspaceID(t, pool)
	userID := getTestUserID(t, pool)

	doc, err := srv.Upload(context.Background(), &pb.UploadRequest{
		Title: "Test Document", Filename: "test.pdf",
		MimeType: "application/pdf", UserId: userID,
		WorkspaceId: wsID, Content: []byte("hello"),
	})
	require.NoError(t, err)
	t.Cleanup(func() {
		pool.Exec(context.Background(), "DELETE FROM documents WHERE id = $1", doc.Id)
	})

	assert.NotEmpty(t, doc.Id)
	assert.Equal(t, "Test Document", doc.Title)
	assert.Equal(t, wsID, doc.WorkspaceId)
}

func TestList_ReturnsAccessibleDocs(t *testing.T) {
	srv, pool := setupTestServer(t)
	wsID := getTestWorkspaceID(t, pool)
	userID := getTestUserID(t, pool)

	doc, err := srv.Upload(context.Background(), &pb.UploadRequest{
		Title: "List Test Doc", Filename: "list.pdf",
		MimeType: "application/pdf", UserId: userID,
		WorkspaceId: wsID,
	})
	require.NoError(t, err)
	t.Cleanup(func() {
		pool.Exec(context.Background(), "DELETE FROM documents WHERE id = $1", doc.Id)
	})

	result, err := srv.List(context.Background(), &pb.ListDocumentsRequest{
		WorkspaceId: wsID, UserNgacNodeId: "ngac-user-1",
	})
	require.NoError(t, err)

	found := false
	for _, d := range result.Documents {
		if d.Id == doc.Id {
			found = true
		}
	}
	assert.True(t, found, "uploaded doc should appear in list")
}

// ---------------------------------------------------------------------------
// 7.3: TestGetDownloadURL (access check) — needs MinIO, skip
// ---------------------------------------------------------------------------

// TestGetDownloadURL is skipped because it requires MinIO presign client
func TestGetDownloadURL_NoMinIO(t *testing.T) {
	t.Skip("requires MinIO presign client — tested via test_app.sh integration")
}

// ---------------------------------------------------------------------------
// 7.4: TestApprove + TestPublish + TestShare
// ---------------------------------------------------------------------------

func TestApprove_HappyPath(t *testing.T) {
	srv, pool := setupTestServer(t)
	wsID := getTestWorkspaceID(t, pool)
	userID := getTestUserID(t, pool)

	doc, err := srv.Upload(context.Background(), &pb.UploadRequest{
		Title: "Approve Test", Filename: "approve.pdf",
		MimeType: "application/pdf", UserId: userID,
		WorkspaceId: wsID,
	})
	require.NoError(t, err)
	t.Cleanup(func() {
		pool.Exec(context.Background(), "DELETE FROM documents WHERE id = $1", doc.Id)
	})

	approved, err := srv.Approve(context.Background(), &pb.ApproveDocumentRequest{
		DocumentId: doc.Id, UserNgacNodeId: "ngac-user-1",
	})
	require.NoError(t, err)
	assert.Equal(t, doc.Id, approved.Id)
}

func TestPublish_HappyPath(t *testing.T) {
	srv, pool := setupTestServer(t)
	wsID := getTestWorkspaceID(t, pool)
	userID := getTestUserID(t, pool)

	doc, err := srv.Upload(context.Background(), &pb.UploadRequest{
		Title: "Publish Test", Filename: "pub.pdf",
		MimeType: "application/pdf", UserId: userID,
		WorkspaceId: wsID,
	})
	require.NoError(t, err)
	t.Cleanup(func() {
		pool.Exec(context.Background(), "DELETE FROM documents WHERE id = $1", doc.Id)
	})

	_, err = srv.Publish(context.Background(), &pb.PublishDocumentRequest{
		DocumentId: doc.Id,
	})
	require.NoError(t, err)
}

func TestList_AccessDeniedFilters(t *testing.T) {
	// Create server with deny-all policy
	pool, err := pgxpool.New(context.Background(), testDBURL())
	if err != nil {
		t.Fatalf("connect to test DB: %v", err)
	}
	if err := pool.Ping(context.Background()); err != nil {
		t.Skipf("test DB not available: %v", err)
	}
	t.Cleanup(func() { pool.Close() })

	denyRead := &mockPolicyDenyReadClient{}
	pw := &mockPolicyWriteClient{pool: pool}
	srv := grpcserver.NewDocumentServer(pool, denyRead, pw, nil, nil)

	wsID := getTestWorkspaceID(t, pool)

	result, err := srv.List(context.Background(), &pb.ListDocumentsRequest{
		WorkspaceId: wsID, UserNgacNodeId: "ngac-denied-user",
	})
	require.NoError(t, err)
	assert.Empty(t, result.Documents, "deny-all policy should filter out all docs")
}

func TestGet_AccessDenied(t *testing.T) {
	pool, err := pgxpool.New(context.Background(), testDBURL())
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	if err := pool.Ping(context.Background()); err != nil {
		t.Skipf("test DB not available: %v", err)
	}
	t.Cleanup(func() { pool.Close() })

	// Upload with allow-all, then try to get with deny-all
	allowRead := &mockPolicyReadClient{}
	allowWrite := &mockPolicyWriteClient{pool: pool}
	allowSrv := grpcserver.NewDocumentServer(pool, allowRead, allowWrite, nil, nil)

	wsID := getTestWorkspaceID(t, pool)
	userID := getTestUserID(t, pool)

	doc, err := allowSrv.Upload(context.Background(), &pb.UploadRequest{
		Title: "Denied Doc", Filename: "denied.pdf",
		MimeType: "application/pdf", UserId: userID,
		WorkspaceId: wsID,
	})
	require.NoError(t, err)
	t.Cleanup(func() {
		pool.Exec(context.Background(), "DELETE FROM documents WHERE id = $1", doc.Id)
	})

	denyRead := &mockPolicyDenyReadClient{}
	denyWrite := &mockPolicyWriteClient{pool: pool}
	denySrv := grpcserver.NewDocumentServer(pool, denyRead, denyWrite, nil, nil)

	_, err = denySrv.Get(context.Background(), &pb.GetDocumentRequest{
		DocumentId: doc.Id, UserNgacNodeId: "denied-user",
	})
	require.Error(t, err)
	st, ok := grpcstatus.FromError(err)
	require.True(t, ok)
	assert.Equal(t, codes.PermissionDenied, st.Code())
}
