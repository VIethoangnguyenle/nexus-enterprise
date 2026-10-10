package grpc_test

import (
	"context"
	"sync"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"ngac-platform/ngac"
	"ngac-platform/pkg/grpcauth"
	policypb "ngac-platform/proto/policy"
	pb "ngac-platform/proto/workspace"
	"ngac-platform/services/workspace/internal/domain"
	grpcserver "ngac-platform/services/workspace/internal/grpc"
	"ngac-platform/services/workspace/internal/store"
	"ngac-platform/testutil"
)

func TestOverTheWire_MissingCallerIsUnauthenticated(t *testing.T) {
	srv, _, _ := setupTestServer(t)
	conn := testutil.ServeGRPC(t, grpcauth.ServerPolicy{}, func(s *grpc.Server) { pb.RegisterWorkspaceServiceServer(s, srv) })
	c := pb.NewWorkspaceServiceClient(conn)

	_, err := c.ListWorkspaces(context.Background(), &pb.ListWorkspacesRequest{})
	assert.Equal(t, codes.Unauthenticated, status.Code(err))
	_, err = c.GetWorkspace(context.Background(), &pb.GetWorkspaceRequest{WorkspaceId: "ws"})
	assert.Equal(t, codes.Unauthenticated, status.Code(err))
}

// userGatedRead allows access checks only for one user node and records which
// user nodes it was asked about.
type userGatedRead struct {
	*mockPolicyReadClient
	allowed string
	mu      sync.Mutex
	seen    []string
}

func (g *userGatedRead) CheckAccess(_ context.Context, req *policypb.CheckAccessRequest, _ ...grpc.CallOption) (*policypb.AccessDecision, error) {
	g.mu.Lock()
	g.seen = append(g.seen, req.UserNodeId)
	g.mu.Unlock()
	if req.UserNodeId == g.allowed {
		return &policypb.AccessDecision{Decision: ngac.DecisionAllow}, nil
	}
	return &policypb.AccessDecision{Decision: ngac.DecisionDeny}, nil
}

func serveGated(t *testing.T, pool *pgxpool.Pool, base *mockPolicyReadClient, allowed string) (pb.WorkspaceServiceClient, *userGatedRead) {
	t.Helper()
	gated := &userGatedRead{mockPolicyReadClient: base, allowed: allowed}
	svc := domain.NewService(store.New(pool), nil, gated, base.w, nil, nil)
	srv := grpcserver.NewWorkspaceServer(svc)
	conn := testutil.ServeGRPC(t, grpcauth.ServerPolicy{}, func(s *grpc.Server) { pb.RegisterWorkspaceServiceServer(s, srv) })
	return pb.NewWorkspaceServiceClient(conn), gated
}

// The body names the workspace owner, who may create roles; the metadata names
// a stranger, who may not. The stranger is who is authorized.
func TestOverTheWire_BodyCallerIsIgnored_Deny(t *testing.T) {
	srv, pool, pr := setupTestServer(t)
	ws, ownerNode := createTestWorkspace(t, srv, pool, "WireDeny")
	strangerID, strangerNode := getTestUserNGACNodeID(t, pool)
	c, gated := serveGated(t, pool, pr, ownerNode)

	_, err := c.CreateRole(asCaller(strangerID, strangerNode), &pb.CreateRoleRequest{
		WorkspaceId:         ws.Id,
		Name:                "Intruder",
		RequesterNgacNodeId: ownerNode, //lint:ignore SA1019 proves the deprecated body field is not trusted
	})

	assert.Equal(t, codes.PermissionDenied, status.Code(err))
	assert.NotContains(t, gated.seen, ownerNode, "the body's user must never reach the policy check")
	assert.Contains(t, gated.seen, strangerNode)
}

func TestOverTheWire_MetadataCallerDecides_Allow(t *testing.T) {
	srv, pool, pr := setupTestServer(t)
	ws, ownerNode := createTestWorkspace(t, srv, pool, "WireAllow")
	ownerID, _ := getTestUserNGACNodeID(t, pool)
	_, strangerNode := getTestUserNGACNodeID(t, pool)
	c, _ := serveGated(t, pool, pr, ownerNode)

	role, err := c.CreateRole(asCaller(ownerID, ownerNode), &pb.CreateRoleRequest{
		WorkspaceId:         ws.Id,
		Name:                "Editor",
		RequesterNgacNodeId: strangerNode, //lint:ignore SA1019 proves the deprecated body field is not trusted
	})
	require.NoError(t, err)
	assert.Equal(t, "Editor", role.Name)
}
