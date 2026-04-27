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

	policypb "ngac-platform/proto/policy"
	pb "ngac-platform/proto/workspace"
	grpcserver "ngac-platform/services/workspace/internal/grpc"
)

// ---------------------------------------------------------------------------
// Mock policy client
// ---------------------------------------------------------------------------

type mockPolicyReadClient struct {
	policypb.PolicyReadServiceClient
}

func (m *mockPolicyReadClient) GetDescendants(_ context.Context, _ *policypb.GetDescendantsRequest, _ ...grpc.CallOption) (*policypb.NodeList, error) {
	return &policypb.NodeList{}, nil
}

func (m *mockPolicyReadClient) GetChildren(_ context.Context, _ *policypb.GetChildrenRequest, _ ...grpc.CallOption) (*policypb.NodeList, error) {
	return &policypb.NodeList{}, nil
}

func (m *mockPolicyReadClient) FindNodeByName(_ context.Context, _ *policypb.FindNodeByNameRequest, _ ...grpc.CallOption) (*policypb.NGACNode, error) {
	return nil, nil
}

type mockPolicyWriteClient struct {
	policypb.PolicyWriteServiceClient
	pool  *pgxpool.Pool
	nodes map[string]*policypb.NGACNode
}

func newMockPolicyWriteClient(pool *pgxpool.Pool) *mockPolicyWriteClient {
	return &mockPolicyWriteClient{pool: pool, nodes: make(map[string]*policypb.NGACNode)}
}

func (m *mockPolicyWriteClient) CreateNode(_ context.Context, req *policypb.CreateNodeRequest, _ ...grpc.CallOption) (*policypb.NGACNode, error) {
	nodeID := fmt.Sprintf("ngac-%s-%d", req.Name, time.Now().UnixNano())
	m.pool.Exec(context.Background(),
		"INSERT INTO ngac_nodes (id, name, node_type) VALUES ($1, $2, $3) ON CONFLICT DO NOTHING",
		nodeID, req.Name, req.NodeType,
	)
	node := &policypb.NGACNode{Id: nodeID, Name: req.Name, NodeType: req.NodeType}
	m.nodes[nodeID] = node
	return node, nil
}

func (m *mockPolicyWriteClient) CreateAssignment(_ context.Context, req *policypb.CreateAssignmentRequest, _ ...grpc.CallOption) (*policypb.Assignment, error) {
	asgID := fmt.Sprintf("asg-%d", time.Now().UnixNano())
	m.pool.Exec(context.Background(),
		"INSERT INTO ngac_assignments (id, child_id, parent_id) VALUES ($1, $2, $3) ON CONFLICT DO NOTHING",
		asgID, req.ChildId, req.ParentId,
	)
	return &policypb.Assignment{Id: asgID}, nil
}

func (m *mockPolicyWriteClient) CreateAssociation(_ context.Context, _ *policypb.CreateAssociationRequest, _ ...grpc.CallOption) (*policypb.Association, error) {
	return &policypb.Association{Id: fmt.Sprintf("assoc-%d", time.Now().UnixNano())}, nil
}

func (m *mockPolicyWriteClient) DeleteNode(_ context.Context, _ *policypb.DeleteNodeRequest, _ ...grpc.CallOption) (*policypb.Empty, error) {
	return &policypb.Empty{}, nil
}

func (m *mockPolicyWriteClient) RemoveAssignment(_ context.Context, _ *policypb.RemoveAssignmentRequest, _ ...grpc.CallOption) (*policypb.Empty, error) {
	return &policypb.Empty{}, nil
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

func setupTestServer(t *testing.T) (*grpcserver.WorkspaceServer, *pgxpool.Pool) {
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
	pw := newMockPolicyWriteClient(pool)
	srv := grpcserver.NewWorkspaceServer(pool, pr, pw, nil)
	return srv, pool
}

func getTestUserNGACNodeID(t *testing.T, pool *pgxpool.Pool) (userID, ngacNodeID string) {
	t.Helper()
	err := pool.QueryRow(context.Background(), "SELECT id, ngac_node FROM users LIMIT 1").Scan(&userID, &ngacNodeID)
	if err != nil {
		t.Skipf("no user in test DB: %v", err)
	}
	return
}

// ---------------------------------------------------------------------------
// 8.1: TestCreateWorkspace + TestGetWorkspace
// ---------------------------------------------------------------------------

func TestCreateWorkspace_HappyPath(t *testing.T) {
	srv, pool := setupTestServer(t)
	userID, ngacNodeID := getTestUserNGACNodeID(t, pool)

	wsName := fmt.Sprintf("TestWS_%d", time.Now().UnixNano())
	ws, err := srv.CreateWorkspace(context.Background(), &pb.CreateWorkspaceRequest{
		Name: wsName, UserId: userID, UserNgacNodeId: ngacNodeID,
	})
	require.NoError(t, err)
	t.Cleanup(func() {
		pool.Exec(context.Background(), "DELETE FROM workspaces WHERE id = $1", ws.Id)
	})

	assert.NotEmpty(t, ws.Id)
	assert.Equal(t, wsName, ws.Name)
	assert.NotEmpty(t, ws.PcNodeId)
}

func TestGetWorkspace_HappyPath(t *testing.T) {
	srv, pool := setupTestServer(t)
	userID, ngacNodeID := getTestUserNGACNodeID(t, pool)

	wsName := fmt.Sprintf("GetWS_%d", time.Now().UnixNano())
	ws, err := srv.CreateWorkspace(context.Background(), &pb.CreateWorkspaceRequest{
		Name: wsName, UserId: userID, UserNgacNodeId: ngacNodeID,
	})
	require.NoError(t, err)
	t.Cleanup(func() {
		pool.Exec(context.Background(), "DELETE FROM workspaces WHERE id = $1", ws.Id)
	})

	got, err := srv.GetWorkspace(context.Background(), &pb.GetWorkspaceRequest{
		WorkspaceId: ws.Id,
	})
	require.NoError(t, err)
	assert.Equal(t, ws.Id, got.Id)
	assert.Equal(t, wsName, got.Name)
}

func TestGetWorkspace_NotFound(t *testing.T) {
	srv, _ := setupTestServer(t)
	_, err := srv.GetWorkspace(context.Background(), &pb.GetWorkspaceRequest{
		WorkspaceId: "nonexistent-ws",
	})
	require.Error(t, err)
	st, ok := grpcstatus.FromError(err)
	require.True(t, ok)
	assert.Equal(t, codes.NotFound, st.Code())
}

// ---------------------------------------------------------------------------
// 8.2: TestCreateRole + TestListRoles
// ---------------------------------------------------------------------------

func TestCreateRole(t *testing.T) {
	srv, pool := setupTestServer(t)
	userID, ngacNodeID := getTestUserNGACNodeID(t, pool)

	ws, err := srv.CreateWorkspace(context.Background(), &pb.CreateWorkspaceRequest{
		Name: fmt.Sprintf("RoleWS_%d", time.Now().UnixNano()),
		UserId: userID, UserNgacNodeId: ngacNodeID,
	})
	require.NoError(t, err)
	t.Cleanup(func() {
		pool.Exec(context.Background(), "DELETE FROM workspaces WHERE id = $1", ws.Id)
	})

	role, err := srv.CreateRole(context.Background(), &pb.CreateRoleRequest{
		WorkspaceId: ws.Id, Name: "Editor",
	})
	require.NoError(t, err)
	assert.NotEmpty(t, role.Id)
	assert.Equal(t, "Editor", role.Name)
}

// ---------------------------------------------------------------------------
// 8.3: TestCreateFolder
// ---------------------------------------------------------------------------

func TestCreateFolder(t *testing.T) {
	srv, pool := setupTestServer(t)
	userID, ngacNodeID := getTestUserNGACNodeID(t, pool)

	ws, err := srv.CreateWorkspace(context.Background(), &pb.CreateWorkspaceRequest{
		Name: fmt.Sprintf("FolderWS_%d", time.Now().UnixNano()),
		UserId: userID, UserNgacNodeId: ngacNodeID,
	})
	require.NoError(t, err)
	t.Cleanup(func() {
		pool.Exec(context.Background(), "DELETE FROM workspaces WHERE id = $1", ws.Id)
	})

	folder, err := srv.CreateFolder(context.Background(), &pb.CreateFolderRequest{
		WorkspaceId: ws.Id, Name: "Engineering",
	})
	require.NoError(t, err)
	assert.NotEmpty(t, folder.Id)
	assert.Equal(t, "Engineering", folder.Name)
}
