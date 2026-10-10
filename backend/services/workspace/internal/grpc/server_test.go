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

	"ngac-platform/ngac"
	"ngac-platform/pkg/grpcauth"
	policypb "ngac-platform/proto/policy"
	pb "ngac-platform/proto/workspace"
	"ngac-platform/services/workspace/internal/domain"
	grpcserver "ngac-platform/services/workspace/internal/grpc"
	"ngac-platform/services/workspace/internal/store"
	"ngac-platform/testutil"
)

// ---------------------------------------------------------------------------
// Mock policy clients
//
// The write mock persists nodes/assignments to the test DB (the workspace row
// needs them) and also keeps the graph in memory, so the read mock can answer
// GetChildren / GetAncestors from what was actually written. CheckAccess
// returns ALLOW unless deny is set — the decision boundary itself is pinned by
// the domain tests, which use an explicit grant table.
// ---------------------------------------------------------------------------

type mockPolicyReadClient struct {
	policypb.PolicyReadServiceClient
	w    *mockPolicyWriteClient
	deny bool
}

func (m *mockPolicyReadClient) GetDescendants(_ context.Context, _ *policypb.GetDescendantsRequest, _ ...grpc.CallOption) (*policypb.NodeList, error) {
	return &policypb.NodeList{}, nil
}

func (m *mockPolicyReadClient) GetChildren(_ context.Context, req *policypb.GetChildrenRequest, _ ...grpc.CallOption) (*policypb.NodeList, error) {
	var out []*policypb.NGACNode
	for _, id := range m.w.children[req.NodeId] {
		if n, ok := m.w.nodes[id]; ok {
			out = append(out, n)
		}
	}
	return &policypb.NodeList{Nodes: out}, nil
}

func (m *mockPolicyReadClient) GetAncestors(_ context.Context, req *policypb.GetAncestorsRequest, _ ...grpc.CallOption) (*policypb.NodeList, error) {
	var out []*policypb.NGACNode
	seen := map[string]bool{}
	queue := append([]string{}, m.w.parents[req.NodeId]...)
	for len(queue) > 0 {
		id := queue[0]
		queue = queue[1:]
		if seen[id] {
			continue
		}
		seen[id] = true
		out = append(out, &policypb.NGACNode{Id: id})
		queue = append(queue, m.w.parents[id]...)
	}
	return &policypb.NodeList{Nodes: out}, nil
}

func (m *mockPolicyReadClient) CheckAccess(_ context.Context, _ *policypb.CheckAccessRequest, _ ...grpc.CallOption) (*policypb.AccessDecision, error) {
	if m.deny {
		return &policypb.AccessDecision{Decision: ngac.DecisionDeny}, nil
	}
	return &policypb.AccessDecision{Decision: ngac.DecisionAllow}, nil
}

func (m *mockPolicyReadClient) FindNodeByName(_ context.Context, _ *policypb.FindNodeByNameRequest, _ ...grpc.CallOption) (*policypb.NGACNode, error) {
	return nil, nil
}

type mockPolicyWriteClient struct {
	policypb.PolicyWriteServiceClient
	pool     *pgxpool.Pool
	nodes    map[string]*policypb.NGACNode
	children map[string][]string
	parents  map[string][]string
}

func newMockPolicyWriteClient(pool *pgxpool.Pool) *mockPolicyWriteClient {
	return &mockPolicyWriteClient{
		pool:     pool,
		nodes:    make(map[string]*policypb.NGACNode),
		children: make(map[string][]string),
		parents:  make(map[string][]string),
	}
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
	m.children[req.ParentId] = append(m.children[req.ParentId], req.ChildId)
	m.parents[req.ChildId] = append(m.parents[req.ChildId], req.ParentId)
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

func setupTestServer(t *testing.T) (*grpcserver.WorkspaceServer, *pgxpool.Pool, *mockPolicyReadClient) {
	t.Helper()
	pool, err := pgxpool.New(context.Background(), testDBURL())
	if err != nil {
		t.Fatalf("connect to test DB: %v", err)
	}
	if err := pool.Ping(context.Background()); err != nil {
		t.Skipf("test DB not available: %v", err)
	}
	t.Cleanup(func() { pool.Close() })

	pw := newMockPolicyWriteClient(pool)
	pr := &mockPolicyReadClient{w: pw}
	s := store.New(pool)
	svc := domain.NewService(s, nil, pr, pw, nil, nil)
	srv := grpcserver.NewWorkspaceServer(svc)
	return srv, pool, pr
}

func getTestUserNGACNodeID(t *testing.T, pool *pgxpool.Pool) (userID, ngacNodeID string) {
	t.Helper()
	return testutil.CreateUser(t, pool)
}

func createTestWorkspace(t *testing.T, srv *grpcserver.WorkspaceServer, pool *pgxpool.Pool, prefix string) (ws *pb.Workspace, ngacNodeID string) {
	t.Helper()
	userID, ngacNodeID := getTestUserNGACNodeID(t, pool)
	ws, err := srv.CreateWorkspace(asCaller(userID, ngacNodeID), &pb.CreateWorkspaceRequest{
		Name: fmt.Sprintf("%s_%d", prefix, time.Now().UnixNano()),
	})
	require.NoError(t, err)
	t.Cleanup(func() {
		pool.Exec(context.Background(), "DELETE FROM workspaces WHERE id = $1", ws.Id)
	})
	return ws, ngacNodeID
}

func requireCode(t *testing.T, err error, want codes.Code) {
	t.Helper()
	require.Error(t, err)
	st, ok := grpcstatus.FromError(err)
	require.True(t, ok)
	assert.Equal(t, want, st.Code(), st.Message())
}

// ---------------------------------------------------------------------------
// 8.1: TestCreateWorkspace + TestGetWorkspace
// ---------------------------------------------------------------------------

func TestCreateWorkspace_HappyPath(t *testing.T) {
	srv, pool, _ := setupTestServer(t)
	userID, ngacNodeID := getTestUserNGACNodeID(t, pool)

	wsName := fmt.Sprintf("TestWS_%d", time.Now().UnixNano())
	ws, err := srv.CreateWorkspace(asCaller(userID, ngacNodeID), &pb.CreateWorkspaceRequest{
		Name: wsName,
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
	srv, pool, _ := setupTestServer(t)
	ws, ngacNodeID := createTestWorkspace(t, srv, pool, "GetWS")

	ctx := asCaller("", ngacNodeID)
	got, err := srv.GetWorkspace(ctx, &pb.GetWorkspaceRequest{WorkspaceId: ws.Id})
	require.NoError(t, err)
	assert.Equal(t, ws.Id, got.Id)
	assert.Equal(t, ws.Name, got.Name)
}

func TestGetWorkspace_NotFound(t *testing.T) {
	srv, _, _ := setupTestServer(t)
	ctx := asCaller("", "some-user")
	_, err := srv.GetWorkspace(ctx, &pb.GetWorkspaceRequest{WorkspaceId: "nonexistent-ws"})
	requireCode(t, err, codes.NotFound)
}

func TestGetWorkspace_NonMemberDenied(t *testing.T) {
	srv, pool, _ := setupTestServer(t)
	ws, _ := createTestWorkspace(t, srv, pool, "GetWSDeny")

	ctx := asCaller("", "not-a-member")
	_, err := srv.GetWorkspace(ctx, &pb.GetWorkspaceRequest{WorkspaceId: ws.Id})
	requireCode(t, err, codes.PermissionDenied)
}

func TestGetWorkspace_NoRequesterDenied(t *testing.T) {
	srv, pool, _ := setupTestServer(t)
	ws, _ := createTestWorkspace(t, srv, pool, "GetWSAnon")

	_, err := srv.GetWorkspace(context.Background(), &pb.GetWorkspaceRequest{WorkspaceId: ws.Id})
	requireCode(t, err, codes.PermissionDenied)
}

// ---------------------------------------------------------------------------
// 8.2: TestCreateRole + TestListRoles
// ---------------------------------------------------------------------------

func TestCreateRole(t *testing.T) {
	srv, pool, _ := setupTestServer(t)
	ws, ngacNodeID := createTestWorkspace(t, srv, pool, "RoleWS")

	role, err := srv.CreateRole(asCaller("", ngacNodeID), &pb.CreateRoleRequest{
		WorkspaceId: ws.Id, Name: "Editor",
	})
	require.NoError(t, err)
	assert.NotEmpty(t, role.Id)
	assert.Equal(t, "Editor", role.Name)
}

func TestCreateRole_PDPDenyIsPermissionDenied(t *testing.T) {
	srv, pool, pr := setupTestServer(t)
	ws, ngacNodeID := createTestWorkspace(t, srv, pool, "RoleWSDeny")
	pr.deny = true

	_, err := srv.CreateRole(asCaller("", ngacNodeID), &pb.CreateRoleRequest{
		WorkspaceId: ws.Id, Name: "Editor",
	})
	requireCode(t, err, codes.PermissionDenied)
}

func TestCreateRole_NoRequesterDenied(t *testing.T) {
	srv, pool, _ := setupTestServer(t)
	ws, _ := createTestWorkspace(t, srv, pool, "RoleWSAnon")

	_, err := srv.CreateRole(context.Background(), &pb.CreateRoleRequest{
		WorkspaceId: ws.Id, Name: "Editor",
	})
	requireCode(t, err, codes.PermissionDenied)
}

// ---------------------------------------------------------------------------
// 8.3: TestCreateFolder
// ---------------------------------------------------------------------------

func TestCreateFolder(t *testing.T) {
	srv, pool, _ := setupTestServer(t)
	ws, ngacNodeID := createTestWorkspace(t, srv, pool, "FolderWS")

	folder, err := srv.CreateFolder(asCaller("", ngacNodeID), &pb.CreateFolderRequest{
		WorkspaceId: ws.Id, Name: "Engineering",
	})
	require.NoError(t, err)
	assert.NotEmpty(t, folder.Id)
	assert.Equal(t, "Engineering", folder.Name)
}

func TestCreateFolder_PDPDenyIsPermissionDenied(t *testing.T) {
	srv, pool, pr := setupTestServer(t)
	ws, ngacNodeID := createTestWorkspace(t, srv, pool, "FolderWSDeny")
	pr.deny = true

	_, err := srv.CreateFolder(asCaller("", ngacNodeID), &pb.CreateFolderRequest{
		WorkspaceId: ws.Id, Name: "Engineering",
	})
	requireCode(t, err, codes.PermissionDenied)
}

// ---------------------------------------------------------------------------
// Permissions
// ---------------------------------------------------------------------------

func TestCreatePermission_PDPDenyIsPermissionDenied(t *testing.T) {
	srv, pool, pr := setupTestServer(t)
	ws, ngacNodeID := createTestWorkspace(t, srv, pool, "PermWSDeny")
	pr.deny = true

	_, err := srv.CreatePermission(asCaller("", ngacNodeID), &pb.CreatePermissionRequest{
		WorkspaceId: ws.Id,
		UaId:        ws.MembersUaId, OaId: ws.MgmtOaId, Operations: []string{ngac.OpManage},
	})
	requireCode(t, err, codes.PermissionDenied)
}

func TestCreatePermission_UnknownOperationIsInvalidArgument(t *testing.T) {
	srv, pool, _ := setupTestServer(t)
	ws, ngacNodeID := createTestWorkspace(t, srv, pool, "PermWSBadOp")

	_, err := srv.CreatePermission(asCaller("", ngacNodeID), &pb.CreatePermissionRequest{
		WorkspaceId: ws.Id,
		UaId:        ws.MembersUaId, OaId: ws.DocumentsOaId, Operations: []string{"root"},
	})
	requireCode(t, err, codes.InvalidArgument)
}

func TestCreatePermission_Allowed(t *testing.T) {
	srv, pool, _ := setupTestServer(t)
	ws, ngacNodeID := createTestWorkspace(t, srv, pool, "PermWS")

	p, err := srv.CreatePermission(asCaller("", ngacNodeID), &pb.CreatePermissionRequest{
		WorkspaceId: ws.Id,
		UaId:        ws.MembersUaId, OaId: ws.DocumentsOaId, Operations: []string{ngac.OpRead},
	})
	require.NoError(t, err)
	assert.Equal(t, []string{ngac.OpRead}, p.Operations)
}

func TestDeletePermission_PDPDenyIsPermissionDenied(t *testing.T) {
	srv, pool, pr := setupTestServer(t)
	ws, ngacNodeID := createTestWorkspace(t, srv, pool, "DelPermWSDeny")
	pr.deny = true

	_, err := srv.DeletePermission(asCaller("", ngacNodeID), &pb.DeletePermissionRequest{
		WorkspaceId: ws.Id, PermissionId: "assoc-1",
	})
	requireCode(t, err, codes.PermissionDenied)
}

// ---------------------------------------------------------------------------
// Members
// ---------------------------------------------------------------------------

func TestInviteMember_PDPDenyIsPermissionDenied(t *testing.T) {
	srv, pool, pr := setupTestServer(t)
	ws, ngacNodeID := createTestWorkspace(t, srv, pool, "InviteWSDeny")
	pr.deny = true

	_, err := srv.InviteMember(asCaller("", ngacNodeID), &pb.InviteMemberRequest{
		WorkspaceId: ws.Id, TargetNgacNodeId: "someone",
	})
	requireCode(t, err, codes.PermissionDenied)
}

func TestListMembers_MemberAllowed_NonMemberDenied(t *testing.T) {
	srv, pool, _ := setupTestServer(t)
	ws, ngacNodeID := createTestWorkspace(t, srv, pool, "ListMembersWS")

	_, err := srv.ListMembers(asCaller("", ngacNodeID),
		&pb.ListMembersRequest{WorkspaceId: ws.Id})
	require.NoError(t, err)

	_, err = srv.ListMembers(asCaller("", "not-a-member"),
		&pb.ListMembersRequest{WorkspaceId: ws.Id})
	requireCode(t, err, codes.PermissionDenied)
}

// asCaller returns a context carrying the caller the interceptor would have
// put there from request metadata.
func asCaller(userID, nodeID string) context.Context {
	return grpcauth.WithCaller(context.Background(), grpcauth.Caller{UserID: userID, NGACNodeID: nodeID})
}
