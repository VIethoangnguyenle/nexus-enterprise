package ngac

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
)

// CTEEvaluator performs NGAC access checks using SQL recursive CTEs.
// This is the L3 (fallback) layer in the 3-layer cache hierarchy,
// providing correct results without requiring an in-memory graph.
type CTEEvaluator struct {
	db *pgxpool.Pool
}

// NewCTEEvaluator creates a CTE-based evaluator that queries the database directly.
func NewCTEEvaluator(db *pgxpool.Pool) *CTEEvaluator {
	return &CTEEvaluator{db: db}
}

// CheckAccess performs an NGAC access decision via SQL recursive CTE.
// Falls back to this when both L1 (Redis) and L2 (materialized) miss.
func (e *CTEEvaluator) CheckAccess(ctx context.Context, userNodeID, objectNodeID, operation string) (bool, error) {
	var allowed bool
	err := e.db.QueryRow(ctx,
		"SELECT ngac_check_access($1, $2, $3)",
		userNodeID, objectNodeID, operation,
	).Scan(&allowed)
	if err != nil {
		return false, fmt.Errorf("cte check_access: %w", err)
	}
	return allowed, nil
}

// ancestorResult holds a node ID and its type from a CTE ancestor query.
type ancestorResult struct {
	NodeID   string
	NodeType string
}

// GetAncestors returns all ancestors of a node via SQL recursive CTE.
func (e *CTEEvaluator) GetAncestors(ctx context.Context, nodeID string) ([]ancestorResult, error) {
	rows, err := e.db.Query(ctx, "SELECT node_id, node_type FROM ngac_ancestors($1)", nodeID)
	if err != nil {
		return nil, fmt.Errorf("cte get_ancestors: %w", err)
	}
	defer rows.Close()

	var results []ancestorResult
	for rows.Next() {
		var r ancestorResult
		if err := rows.Scan(&r.NodeID, &r.NodeType); err != nil {
			return nil, fmt.Errorf("scanning ancestor: %w", err)
		}
		results = append(results, r)
	}
	return results, rows.Err()
}
