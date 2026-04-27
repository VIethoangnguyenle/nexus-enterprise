package ngac

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// MaterializedAccess manages the ngac_materialized_access table,
// which caches pre-computed access decisions at the database layer (L2 cache).
type MaterializedAccess struct {
	db *pgxpool.Pool
}

// NewMaterializedAccess creates a materialized access cache backed by PostgreSQL.
func NewMaterializedAccess(db *pgxpool.Pool) *MaterializedAccess {
	return &MaterializedAccess{db: db}
}

// CachedDecision represents a cached access decision with its graph version.
type CachedDecision struct {
	Decision     bool
	GraphVersion int64
}

// Lookup retrieves a cached access decision if it exists and is fresh (matching graph version).
func (m *MaterializedAccess) Lookup(ctx context.Context, userNodeID, objectNodeID, operation string, currentVersion int64) (*CachedDecision, error) {
	var decision bool
	var version int64
	err := m.db.QueryRow(ctx,
		`SELECT decision, graph_version FROM ngac_materialized_access
		 WHERE user_node_id = $1 AND object_node_id = $2 AND operation = $3`,
		userNodeID, objectNodeID, operation,
	).Scan(&decision, &version)
	if err == pgx.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("materialized lookup: %w", err)
	}

	// Stale cache: graph has been mutated since this decision was computed
	if version < currentVersion {
		return nil, nil
	}

	return &CachedDecision{Decision: decision, GraphVersion: version}, nil
}

// Store upserts a computed access decision into the materialized table.
func (m *MaterializedAccess) Store(ctx context.Context, userNodeID, objectNodeID, operation string, decision bool, graphVersion int64) error {
	_, err := m.db.Exec(ctx,
		`INSERT INTO ngac_materialized_access (user_node_id, object_node_id, operation, decision, graph_version)
		 VALUES ($1, $2, $3, $4, $5)
		 ON CONFLICT (user_node_id, object_node_id, operation)
		 DO UPDATE SET decision = $4, graph_version = $5, computed_at = NOW()`,
		userNodeID, objectNodeID, operation, decision, graphVersion,
	)
	if err != nil {
		return fmt.Errorf("materialized store: %w", err)
	}
	return nil
}

// InvalidateByUser removes all cached decisions for a user (when user's assignments change).
func (m *MaterializedAccess) InvalidateByUser(ctx context.Context, userNodeID string) error {
	_, err := m.db.Exec(ctx,
		"DELETE FROM ngac_materialized_access WHERE user_node_id = $1", userNodeID)
	if err != nil {
		return fmt.Errorf("materialized invalidate user: %w", err)
	}
	return nil
}

// InvalidateByObject removes all cached decisions for an object (when object's assignments change).
func (m *MaterializedAccess) InvalidateByObject(ctx context.Context, objectNodeID string) error {
	_, err := m.db.Exec(ctx,
		"DELETE FROM ngac_materialized_access WHERE object_node_id = $1", objectNodeID)
	if err != nil {
		return fmt.Errorf("materialized invalidate object: %w", err)
	}
	return nil
}

// InvalidateAll removes all cached decisions (emergency flush).
func (m *MaterializedAccess) InvalidateAll(ctx context.Context) error {
	_, err := m.db.Exec(ctx, "DELETE FROM ngac_materialized_access")
	if err != nil {
		return fmt.Errorf("materialized invalidate all: %w", err)
	}
	return nil
}
