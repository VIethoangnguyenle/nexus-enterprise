// Package testutil provides shared helpers for service unit tests.
// It contains database connection setup, seed data builders, and mock factories
// to reduce boilerplate across service test suites.
package testutil

import (
	"context"
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
