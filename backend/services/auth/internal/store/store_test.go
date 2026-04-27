package store_test

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"ngac-platform/services/auth/internal/store"
)

func testDBURL() string {
	if url := os.Getenv("TEST_DATABASE_URL"); url != "" {
		return url
	}
	return "postgres://ngac:ngac_secret@localhost:5432/ngac?sslmode=disable"
}

func setupStore(t *testing.T) *store.Store {
	t.Helper()
	pool, err := pgxpool.New(context.Background(), testDBURL())
	if err != nil {
		t.Fatalf("connect to test DB: %v", err)
	}
	if err := pool.Ping(context.Background()); err != nil {
		t.Skipf("test DB not available: %v", err)
	}
	t.Cleanup(func() { pool.Close() })
	return store.New(pool)
}

// insertTestUser creates a user directly via SQL to avoid NGAC dependency.
func insertTestUser(t *testing.T, s *store.Store, pool *pgxpool.Pool) (id, username, ngacNodeID string) {
	t.Helper()
	id = fmt.Sprintf("test-user-%d", time.Now().UnixNano())
	username = fmt.Sprintf("testuser_%d", time.Now().UnixNano())
	ngacNodeID = fmt.Sprintf("ngac-test-%d", time.Now().UnixNano())

	// Create NGAC node first (FK constraint)
	_, err := pool.Exec(context.Background(),
		"INSERT INTO ngac_nodes (id, name, node_type) VALUES ($1, $2, 'U')",
		ngacNodeID, username,
	)
	require.NoError(t, err)

	err = s.CreateUser(context.Background(), id, username, "$2a$10$fakehash000000000000000000000000000000000000", ngacNodeID)
	require.NoError(t, err)

	t.Cleanup(func() {
		pool.Exec(context.Background(), "DELETE FROM users WHERE id = $1", id)
		pool.Exec(context.Background(), "DELETE FROM ngac_nodes WHERE id = $1", ngacNodeID)
	})
	return
}

// ---------------------------------------------------------------------------
// 5.1: TestCreateUser + TestGetUserByUsername
// ---------------------------------------------------------------------------

func TestCreateUser(t *testing.T) {
	s := setupStore(t)
	pool := getPool(t, s)
	id, username, _ := insertTestUser(t, s, pool)

	got, err := s.GetUserByUsername(context.Background(), username)
	require.NoError(t, err)
	require.NotNil(t, got)
	assert.Equal(t, id, got.ID)
	assert.Equal(t, username, got.Username)
}

func TestGetUserByUsername_NotFound(t *testing.T) {
	s := setupStore(t)
	got, err := s.GetUserByUsername(context.Background(), "nonexistent_user_xyz")
	require.NoError(t, err)
	assert.Nil(t, got, "should return nil for missing user")
}

// ---------------------------------------------------------------------------
// 5.2: TestGetUserByID + TestGetUserByNGACNodeID
// ---------------------------------------------------------------------------

func TestGetUserByID(t *testing.T) {
	s := setupStore(t)
	pool := getPool(t, s)
	id, username, _ := insertTestUser(t, s, pool)

	got, err := s.GetUserByID(context.Background(), id)
	require.NoError(t, err)
	require.NotNil(t, got)
	assert.Equal(t, username, got.Username)
}

func TestGetUserByID_NotFound(t *testing.T) {
	s := setupStore(t)
	got, err := s.GetUserByID(context.Background(), "nonexistent-user-id")
	require.NoError(t, err)
	assert.Nil(t, got)
}

func TestGetUserByNGACNodeID(t *testing.T) {
	s := setupStore(t)
	pool := getPool(t, s)
	id, _, ngacNodeID := insertTestUser(t, s, pool)

	got, err := s.GetUserByNGACNodeID(context.Background(), ngacNodeID)
	require.NoError(t, err)
	require.NotNil(t, got)
	assert.Equal(t, id, got.ID)
	assert.Equal(t, ngacNodeID, got.NGACNodeID)
}

func TestGetUserByNGACNodeID_NotFound(t *testing.T) {
	s := setupStore(t)
	got, err := s.GetUserByNGACNodeID(context.Background(), "nonexistent-ngac-node")
	require.NoError(t, err)
	assert.Nil(t, got)
}

// ---------------------------------------------------------------------------
// 5.3: TestListUsers
// ---------------------------------------------------------------------------

func TestListUsers(t *testing.T) {
	s := setupStore(t)
	pool := getPool(t, s)
	_, username, _ := insertTestUser(t, s, pool)

	users, err := s.ListUsers(context.Background())
	require.NoError(t, err)
	assert.GreaterOrEqual(t, len(users), 1)

	found := false
	for _, u := range users {
		if u.Username == username {
			found = true
		}
	}
	assert.True(t, found, "created user should appear in list")
}

// ---------------------------------------------------------------------------
// helpers
// ---------------------------------------------------------------------------

// getPool extracts the underlying pool via a round-trip. Since store doesn't
// export the pool, we create a second pool for test helpers.
func getPool(t *testing.T, _ *store.Store) *pgxpool.Pool {
	t.Helper()
	pool, err := pgxpool.New(context.Background(), testDBURL())
	if err != nil {
		t.Fatalf("connect to test DB: %v", err)
	}
	t.Cleanup(func() { pool.Close() })
	return pool
}
