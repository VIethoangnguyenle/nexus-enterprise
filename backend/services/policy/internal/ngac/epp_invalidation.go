package ngac

import (
	"context"

	"ngac-platform/services/policy/internal/metrics"
)

// InvalidationCoordinator coordinates cache invalidation across the PIP
// layers. It hides the cache topology from the transport layer (WriteServer).
// Redis (L1) is the only shared cache; the in-memory graph and its shards are
// invalidated by the caller.
type InvalidationCoordinator struct {
	cache *CacheInvalidator
}

// NewInvalidationCoordinator creates a cache invalidation coordinator. A nil
// cache makes every call a no-op (no Redis configured).
func NewInvalidationCoordinator(cache *CacheInvalidator) *InvalidationCoordinator {
	return &InvalidationCoordinator{cache: cache}
}

// InvalidateForNodes performs targeted invalidation of the cached decisions the
// given node IDs touch.
func (c *InvalidationCoordinator) InvalidateForNodes(ctx context.Context, nodeIDs ...string) {
	if c.cache == nil {
		return
	}
	if c.cache.InvalidateForNodes(ctx, nodeIDs...) {
		metrics.CacheInvalidationTotal.WithLabelValues("full").Inc()
	} else {
		metrics.CacheInvalidationTotal.WithLabelValues("targeted").Inc()
	}
}

// InvalidateAll flushes every cached decision (used on full graph reload).
func (c *InvalidationCoordinator) InvalidateAll(ctx context.Context) {
	if c.cache == nil {
		return
	}
	c.cache.FlushAll(ctx)
	metrics.CacheInvalidationTotal.WithLabelValues("full").Inc()
}
