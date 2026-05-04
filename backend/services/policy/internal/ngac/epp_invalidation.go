package ngac

import (
	"context"
	"log/slog"

	"ngac-platform/services/policy/internal/metrics"
)

// InvalidationCoordinator coordinates cache invalidation across all PIP layers.
// Hides L1/L2/version topology from the transport layer (WriteServer).
type InvalidationCoordinator struct {
	version      *VersionTracker
	materialized *MaterializedAccess
	cache        *CacheInvalidator
}

// NewInvalidationCoordinator creates a cache invalidation coordinator.
func NewInvalidationCoordinator(
	version *VersionTracker,
	materialized *MaterializedAccess,
	cache *CacheInvalidator,
) *InvalidationCoordinator {
	return &InvalidationCoordinator{
		version:      version,
		materialized: materialized,
		cache:        cache,
	}
}

// InvalidateForNodes increments the graph version and performs targeted invalidation
// across L2 (materialized) and L1 (Redis) for the specified node IDs.
// workspaceID enables per-workspace versioning to prevent cross-tenant invalidation storms.
func (c *InvalidationCoordinator) InvalidateForNodes(ctx context.Context, workspaceID string, nodeIDs ...string) {
	// Step 1: Bump graph version — per-workspace when possible, global as fallback
	if c.version != nil {
		scope := ScopeGlobal
		if workspaceID != "" {
			scope = WorkspaceScope(workspaceID)
		}
		if _, err := c.version.Increment(ctx, scope); err != nil {
			slog.Warn("failed to increment graph version", "scope", scope, "error", err)
		}
	}

	// Step 2: Targeted L2 (materialized) invalidation
	if c.materialized != nil {
		for _, nodeID := range nodeIDs {
			if err := c.materialized.InvalidateByUser(ctx, nodeID); err != nil {
				slog.Warn("failed to invalidate materialized user entries", "nodeID", nodeID, "error", err)
			}
			if err := c.materialized.InvalidateByObject(ctx, nodeID); err != nil {
				slog.Warn("failed to invalidate materialized object entries", "nodeID", nodeID, "error", err)
			}
		}
	}

	// Step 3: Targeted L1 (Redis) invalidation
	if c.cache != nil {
		fullFlush := c.cache.InvalidateForNodes(ctx, nodeIDs...)
		if fullFlush {
			metrics.CacheInvalidationTotal.WithLabelValues("full").Inc()
		} else {
			metrics.CacheInvalidationTotal.WithLabelValues("targeted").Inc()
		}
	}
}

// InvalidateAll flushes all cache layers (used on full graph reload).
func (c *InvalidationCoordinator) InvalidateAll(ctx context.Context) {
	if c.version != nil {
		if _, err := c.version.Increment(ctx, ScopeGlobal); err != nil {
			slog.Warn("failed to increment graph version", "error", err)
		}
	}
	if c.materialized != nil {
		if err := c.materialized.InvalidateAll(ctx); err != nil {
			slog.Warn("failed to flush materialized cache", "error", err)
		}
	}
	if c.cache != nil {
		c.cache.FlushAll(ctx)
		metrics.CacheInvalidationTotal.WithLabelValues("full").Inc()
	}
}
