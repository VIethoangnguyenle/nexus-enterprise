package ngac

import (
	"context"
	"time"

	"ngac-platform/services/policy/internal/metrics"
)

// AccessRequest is the input for access evaluation.
// Uses internal types — keeps PDP proto-free.
type AccessRequest struct {
	UserNodeID   string
	ObjectNodeID string
	Operation    string
	WorkspaceID  string // Optional: enables shard-based evaluation when set
}

// AccessEvaluator coordinates cache lookup and PDP computation.
// Single entry point for all access checks in the read path.
//
// Flow: cache.Get() → [miss] → engine.Decide() → cache.Set()
type AccessEvaluator struct {
	cache  DecisionCache
	engine DecisionEngine
}

// NewAccessEvaluator creates an evaluator with layered cache and decision engine.
func NewAccessEvaluator(cache DecisionCache, engine DecisionEngine) *AccessEvaluator {
	return &AccessEvaluator{cache: cache, engine: engine}
}

// Evaluate resolves an access decision using the 3-layer cache strategy:
//   - L1 (Redis) and L2 (Materialized) are checked by the cache
//   - L3 (BFS/CTE + prohibitions) is computed by the engine on cache miss
//   - Result is stored back into cache layers for future lookups
func (e *AccessEvaluator) Evaluate(ctx context.Context, req AccessRequest) *AccessDecision {
	start := time.Now()

	// Try cache (L1 → L2)
	if cached, layer := e.cache.Get(ctx, req); cached != nil {
		metrics.CheckAccessTotal.WithLabelValues(layer).Inc()
		metrics.CheckAccessDuration.WithLabelValues(layer).Observe(time.Since(start).Seconds())
		return cached
	}

	// Cache miss → compute via PDP engine (L3)
	decision := e.engine.Decide(ctx, req)

	// Populate caches for next lookup
	e.cache.Set(ctx, req, decision)

	metrics.CheckAccessTotal.WithLabelValues("L3").Inc()
	metrics.CheckAccessDuration.WithLabelValues("L3").Observe(time.Since(start).Seconds())

	return decision
}
