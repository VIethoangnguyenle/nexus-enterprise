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

	pb "ngac-platform/proto/auth"
	policypb "ngac-platform/proto/policy"
	"ngac-platform/services/auth/internal/auth"
	grpcserver "ngac-platform/services/auth/internal/grpc"
	"ngac-platform/services/auth/internal/store"
)

// ---------------------------------------------------------------------------
// Mock clients
// ---------------------------------------------------------------------------

type mockPolicyReadClient struct {
	policypb.PolicyReadServiceClient
}

func (m *mockPolicyReadClient) FindNodeByName(_ context.Context, _ *policypb.FindNodeByNameRequest, _ ...grpc.CallOption) (*policypb.NGACNode, error) {
	return &policypb.NGACNode{Id: "ua-public-users", Name: "PublicUsers", NodeType: "UA"}, nil
}

type mockPolicyWriteClient struct {
	policypb.PolicyWriteServiceClient
	pool *pgxpool.Pool
}

func (m *mockPolicyWriteClient) CreateNode(_ context.Context, req *policypb.CreateNodeRequest, _ ...grpc.CallOption) (*policypb.NGACNode, error) {
	nodeID := fmt.Sprintf("ngac-%s-%d", req.Name, time.Now().UnixNano())
	// Insert into real DB so FK constraint on users.ngac_node is satisfied
	_, _ = m.pool.Exec(context.Background(),
		"INSERT INTO ngac_nodes (id, name, node_type) VALUES ($1, $2, $3) ON CONFLICT DO NOTHING",
		nodeID, req.Name, req.NodeType,
	)
	return &policypb.NGACNode{Id: nodeID, Name: req.Name, NodeType: req.NodeType}, nil
}

func (m *mockPolicyWriteClient) CreateAssignment(_ context.Context, req *policypb.CreateAssignmentRequest, _ ...grpc.CallOption) (*policypb.Assignment, error) {
	// Insert assignment into real DB so cleanup works
	assignID := fmt.Sprintf("asg-test-%d", time.Now().UnixNano())
	m.pool.Exec(context.Background(),
		"INSERT INTO ngac_assignments (id, child_id, parent_id) VALUES ($1, $2, $3) ON CONFLICT DO NOTHING",
		assignID, req.ChildId, req.ParentId,
	)
	return &policypb.Assignment{Id: assignID}, nil
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

func setupTestServer(t *testing.T) (*grpcserver.AuthServer, *pgxpool.Pool) {
	t.Helper()
	auth.SetJWTSecret("test-secret-key-for-testing-only")

	pool, err := pgxpool.New(context.Background(), testDBURL())
	if err != nil {
		t.Fatalf("connect to test DB: %v", err)
	}
	if err := pool.Ping(context.Background()); err != nil {
		t.Skipf("test DB not available: %v", err)
	}
	t.Cleanup(func() { pool.Close() })

	s := store.New(pool)
	srv := grpcserver.NewAuthServer(s, &mockPolicyReadClient{}, &mockPolicyWriteClient{pool: pool}, nil)
	return srv, pool
}

// ---------------------------------------------------------------------------
// 5.4: TestRegister (happy + duplicate username)
// ---------------------------------------------------------------------------

func TestRegister_HappyPath(t *testing.T) {
	srv, pool := setupTestServer(t)
	username := fmt.Sprintf("regtest_%d", time.Now().UnixNano())
	t.Cleanup(func() {
		// Clean up user and their NGAC node
		var ngacNode string
		pool.QueryRow(context.Background(), "SELECT ngac_node FROM users WHERE username = $1", username).Scan(&ngacNode)
		pool.Exec(context.Background(), "DELETE FROM ngac_assignments WHERE child_id = $1 OR parent_id = $1", ngacNode)
		pool.Exec(context.Background(), "DELETE FROM users WHERE username = $1", username)
		pool.Exec(context.Background(), "DELETE FROM ngac_nodes WHERE id = $1", ngacNode)
	})

	resp, err := srv.Register(context.Background(), &pb.RegisterRequest{
		Username: username,
		Password: "StrongPassword123!",
	})

	require.NoError(t, err)
	assert.NotEmpty(t, resp.Token)
	assert.Equal(t, username, resp.User.Username)
	assert.NotEmpty(t, resp.User.Id)
	assert.NotEmpty(t, resp.User.NgacNodeId)
}

func TestRegister_DuplicateUsername(t *testing.T) {
	srv, pool := setupTestServer(t)
	username := fmt.Sprintf("duptest_%d", time.Now().UnixNano())
	t.Cleanup(func() {
		var ngacNode string
		pool.QueryRow(context.Background(), "SELECT ngac_node FROM users WHERE username = $1", username).Scan(&ngacNode)
		pool.Exec(context.Background(), "DELETE FROM ngac_assignments WHERE child_id = $1 OR parent_id = $1", ngacNode)
		pool.Exec(context.Background(), "DELETE FROM users WHERE username = $1", username)
		pool.Exec(context.Background(), "DELETE FROM ngac_nodes WHERE id = $1", ngacNode)
	})

	// First register should succeed
	_, err := srv.Register(context.Background(), &pb.RegisterRequest{
		Username: username, Password: "Password123!",
	})
	require.NoError(t, err)

	// Second register with same username should fail
	_, err = srv.Register(context.Background(), &pb.RegisterRequest{
		Username: username, Password: "Password123!",
	})
	require.Error(t, err)
	st, ok := grpcstatus.FromError(err)
	require.True(t, ok)
	assert.Equal(t, codes.AlreadyExists, st.Code())
}

// ---------------------------------------------------------------------------
// 5.5: TestLogin (happy + wrong password + user not found)
// ---------------------------------------------------------------------------

func TestLogin_HappyPath(t *testing.T) {
	srv, pool := setupTestServer(t)
	username := fmt.Sprintf("logintest_%d", time.Now().UnixNano())
	password := "CorrectPassword123!"
	t.Cleanup(func() {
		var ngacNode string
		pool.QueryRow(context.Background(), "SELECT ngac_node FROM users WHERE username = $1", username).Scan(&ngacNode)
		pool.Exec(context.Background(), "DELETE FROM ngac_assignments WHERE child_id = $1 OR parent_id = $1", ngacNode)
		pool.Exec(context.Background(), "DELETE FROM users WHERE username = $1", username)
		pool.Exec(context.Background(), "DELETE FROM ngac_nodes WHERE id = $1", ngacNode)
	})

	// Register first
	_, err := srv.Register(context.Background(), &pb.RegisterRequest{
		Username: username, Password: password,
	})
	require.NoError(t, err)

	// Login
	resp, err := srv.Login(context.Background(), &pb.LoginRequest{
		Username: username, Password: password,
	})
	require.NoError(t, err)
	assert.NotEmpty(t, resp.Token)
	assert.Equal(t, username, resp.User.Username)
}

func TestLogin_WrongPassword(t *testing.T) {
	srv, pool := setupTestServer(t)
	username := fmt.Sprintf("wrongpw_%d", time.Now().UnixNano())
	t.Cleanup(func() {
		var ngacNode string
		pool.QueryRow(context.Background(), "SELECT ngac_node FROM users WHERE username = $1", username).Scan(&ngacNode)
		pool.Exec(context.Background(), "DELETE FROM ngac_assignments WHERE child_id = $1 OR parent_id = $1", ngacNode)
		pool.Exec(context.Background(), "DELETE FROM users WHERE username = $1", username)
		pool.Exec(context.Background(), "DELETE FROM ngac_nodes WHERE id = $1", ngacNode)
	})

	_, err := srv.Register(context.Background(), &pb.RegisterRequest{
		Username: username, Password: "RealPassword123!",
	})
	require.NoError(t, err)

	_, err = srv.Login(context.Background(), &pb.LoginRequest{
		Username: username, Password: "WrongPassword!",
	})
	require.Error(t, err)
	st, ok := grpcstatus.FromError(err)
	require.True(t, ok)
	assert.Equal(t, codes.Unauthenticated, st.Code())
}

func TestLogin_UserNotFound(t *testing.T) {
	srv, _ := setupTestServer(t)

	_, err := srv.Login(context.Background(), &pb.LoginRequest{
		Username: "nonexistent_user_xyz", Password: "anything",
	})
	require.Error(t, err)
	st, ok := grpcstatus.FromError(err)
	require.True(t, ok)
	assert.Equal(t, codes.NotFound, st.Code())
}
