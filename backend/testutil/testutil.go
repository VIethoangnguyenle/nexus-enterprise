// Package testutil provides shared helpers for service unit tests.
// It contains database connection setup, seed data builders, and mock factories
// to reduce boilerplate across service test suites.
package testutil

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"os"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
)

// DatabaseURL returns the test database URL, defaulting to the Docker-internal address.
func DatabaseURL() string {
	if url := os.Getenv("TEST_DATABASE_URL"); url != "" {
		return url
	}
	return "postgres://ngac:ngac_secret@localhost:5432/ngac?sslmode=disable"
}

// SetupTestDB creates a pgxpool.Pool connected to the test database.
// It registers a cleanup function to close the pool when the test finishes.
func SetupTestDB(t *testing.T) *pgxpool.Pool {
	t.Helper()
	pool, err := pgxpool.New(context.Background(), DatabaseURL())
	if err != nil {
		t.Fatalf("connect to test DB: %v", err)
	}
	t.Cleanup(func() { pool.Close() })

	if err := pool.Ping(context.Background()); err != nil {
		t.Skipf("test DB not available: %v", err)
	}
	return pool
}

// CleanTable truncates the given table with CASCADE. Only for use in tests.
func CleanTable(t *testing.T, pool *pgxpool.Pool, tables ...string) {
	t.Helper()
	for _, table := range tables {
		_, err := pool.Exec(context.Background(), fmt.Sprintf("TRUNCATE %s CASCADE", table))
		if err != nil {
			t.Fatalf("truncating %s: %v", table, err)
		}
	}
}

// uniqueID returns prefix plus a random suffix, so parallel test packages
// sharing one database never collide on a primary or unique key.
func uniqueID(prefix string) string {
	b := make([]byte, 6)
	if _, err := rand.Read(b); err != nil {
		panic(err)
	}
	return prefix + "-" + hex.EncodeToString(b)
}

// exec runs one fixture statement and fails the test on error.
func exec(t *testing.T, pool *pgxpool.Pool, sql string, args ...any) {
	t.Helper()
	if _, err := pool.Exec(context.Background(), sql, args...); err != nil {
		t.Fatalf("fixture %q: %v", sql, err)
	}
}

// CreateUser inserts a user of its own, backed by its own NGAC user node, and
// removes both when the test ends. Tests create the rows they need instead of
// borrowing whatever happens to exist, which an empty CI database does not
// have and which another package may be deleting concurrently.
//
// pool must outlive the test's cleanups: register pool.Close before calling.
func CreateUser(t *testing.T, pool *pgxpool.Pool) (userID, ngacNodeID string) {
	t.Helper()
	userID = uniqueID("test-user")
	ngacNodeID = "u-" + userID
	exec(t, pool, `INSERT INTO ngac_nodes (id, name, node_type, properties) VALUES ($1, $1, 'U', '{}')`, ngacNodeID)
	exec(t, pool, `INSERT INTO users (id, username, password, ngac_node, display_name) VALUES ($1, $1, '', $2, 'testuser')`, userID, ngacNodeID)
	t.Cleanup(func() {
		ctx := context.Background()
		if _, err := pool.Exec(ctx, `DELETE FROM users WHERE id = $1`, userID); err != nil {
			t.Errorf("cleanup user %s: %v", userID, err)
		}
		if _, err := pool.Exec(ctx, `DELETE FROM ngac_nodes WHERE id = $1`, ngacNodeID); err != nil {
			t.Errorf("cleanup node %s: %v", ngacNodeID, err)
		}
	})
	return userID, ngacNodeID
}

// CreateWorkspace inserts a workspace owned by ownerID, with its own policy
// class node, and removes both when the test ends. Register any rows that
// reference the workspace after this call so their cleanup runs first.
func CreateWorkspace(t *testing.T, pool *pgxpool.Pool, ownerID string) (wsID, pcID string) {
	t.Helper()
	wsID = uniqueID("test-ws")
	pcID = "pc-" + wsID
	exec(t, pool, `INSERT INTO ngac_nodes (id, name, node_type, properties) VALUES ($1, $1, 'PC', '{}')`, pcID)
	// The name differs from the ID on purpose: name-keyed and ID-keyed NGAC
	// node names must not coincide, or a test cannot tell them apart.
	exec(t, pool, `INSERT INTO workspaces (id, name, owner_id, ngac_pc_id) VALUES ($1, $2, $3, $4)`, wsID, "Workspace "+wsID, ownerID, pcID)
	t.Cleanup(func() {
		ctx := context.Background()
		if _, err := pool.Exec(ctx, `DELETE FROM workspaces WHERE id = $1`, wsID); err != nil {
			t.Errorf("cleanup workspace %s: %v", wsID, err)
		}
		if _, err := pool.Exec(ctx, `DELETE FROM ngac_nodes WHERE id = $1`, pcID); err != nil {
			t.Errorf("cleanup node %s: %v", pcID, err)
		}
	})
	return wsID, pcID
}
