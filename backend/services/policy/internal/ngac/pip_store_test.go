package ngac_test

import (
	"context"
	"os"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"ngac-platform/services/policy/internal/ngac"
)

// --- Shared test helpers ---

func testDBURL() string {
	if url := os.Getenv("TEST_DATABASE_URL"); url != "" {
		return url
	}
	return "postgres://ngac:ngac_secret@localhost:5432/ngac?sslmode=disable"
}

func setupStore(t *testing.T) (*ngac.Store, *pgxpool.Pool) {
	t.Helper()
	pool, err := pgxpool.New(context.Background(), testDBURL())
	if err != nil {
		t.Fatalf("connect to test DB: %v", err)
	}
	if err := pool.Ping(context.Background()); err != nil {
		t.Skipf("test DB not available: %v", err)
	}
	t.Cleanup(func() { pool.Close() })

	graph := ngac.NewGraph()
	s := ngac.NewStore(pool, graph)
	require.NoError(t, s.LoadGraph(context.Background()))
	return s, pool
}

// --- PIP: Read/query tests ---

func TestFindNodeByName(t *testing.T) {
	s, _ := setupStore(t)

	// PC_Global is seeded
	node := s.FindNodeByName("PC_Global", "PC")
	require.NotNil(t, node)
	assert.Equal(t, "PC_Global", node.Name)
	assert.Equal(t, "PC", node.NodeType)
}

func TestFindNodeByName_NotFound(t *testing.T) {
	s, _ := setupStore(t)
	node := s.FindNodeByName("Nonexistent", "PC")
	assert.Nil(t, node)
}
