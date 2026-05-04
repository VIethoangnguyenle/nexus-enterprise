package ngac

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
)

// VersionTracker manages graph version numbers for cache consistency.
// Each mutation increments the version, allowing caches to detect staleness
// without expensive full-flush invalidation.
type VersionTracker struct {
	db *pgxpool.Pool
}

// NewVersionTracker creates a version tracker backed by PostgreSQL.
func NewVersionTracker(db *pgxpool.Pool) *VersionTracker {
	return &VersionTracker{db: db}
}

// GetVersion returns the current graph version for a scope.
// Scope is typically "global" or "ws:{workspace_id}".
func (v *VersionTracker) GetVersion(ctx context.Context, scope string) (int64, error) {
	var version int64
	err := v.db.QueryRow(ctx,
		"SELECT version FROM ngac_graph_version WHERE scope = $1", scope,
	).Scan(&version)
	if err != nil {
		return 0, fmt.Errorf("get graph version: %w", err)
	}
	return version, nil
}

// Increment atomically increments the graph version for a scope and returns the new version.
func (v *VersionTracker) Increment(ctx context.Context, scope string) (int64, error) {
	var newVersion int64
	err := v.db.QueryRow(ctx,
		`INSERT INTO ngac_graph_version (scope, version, updated_at)
		 VALUES ($1, 1, NOW())
		 ON CONFLICT (scope) DO UPDATE SET version = ngac_graph_version.version + 1, updated_at = NOW()
		 RETURNING version`,
		scope,
	).Scan(&newVersion)
	if err != nil {
		return 0, fmt.Errorf("increment graph version: %w", err)
	}
	return newVersion, nil
}
