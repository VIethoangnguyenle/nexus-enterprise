package ngac

import (
	"context"
	"time"

	"golang.org/x/sync/singleflight"

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

	// inflight collapses concurrent identical questions into one traversal.
	// See Evaluate for why this is separate from the cache.
	inflight singleflight.Group
}

// NewAccessEvaluator creates an evaluator with layered cache and decision engine.
func NewAccessEvaluator(cache DecisionCache, engine DecisionEngine) *AccessEvaluator {
	return &AccessEvaluator{cache: cache, engine: engine}
}

// inflightKey identifies one distinct access question.
//
// It carries the workspace because that selects which graph answers the
// question — collapsing two shards' answers together would hand one workspace
// the other's decision.
func inflightKey(req AccessRequest) string {
	return req.WorkspaceID + "\x00" + req.UserNodeID + "\x00" + req.ObjectNodeID + "\x00" + req.Operation
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

	// Cache miss → compute via PDP engine (L3).
	//
	// Collapsed per distinct question. A cache miss does not arrive alone: a
	// page load fires many checks at once and, on a cold or just-invalidated
	// cache, many of them are the same question. Without this the cost scales
	// with how many callers happen to ask simultaneously instead of with how
	// many distinct questions there are, and every graph invalidation turns
	// into a thundering herd across the graph.
	//
	// This is not a second cache: it only merges callers that overlap in time.
	// The moment one traversal finishes, the next request is a fresh question
	// and goes to the cache as normal.
	computed, _, _ := e.inflight.Do(inflightKey(req), func() (any, error) {
		d := e.engine.Decide(ctx, req)
		e.cache.Set(ctx, req, d)
		return d, nil
	})

	decision, _ := computed.(*AccessDecision)

	metrics.CheckAccessTotal.WithLabelValues("L3").Inc()
	metrics.CheckAccessDuration.WithLabelValues("L3").Observe(time.Since(start).Seconds())

	return decision
}

// EvaluateBatch resolves many objects for one user in a single pass.
//
// It goes straight to the engine's batch path rather than consulting the
// decision cache per pair. The cache is a per-(user, object, operation) lookup,
// so using it here would trade one in-memory traversal for one network
// round-trip per item — the opposite of the point. The batch path is already
// the cheap one: it walks the user's side of the graph once for the whole page.
func (e *AccessEvaluator) EvaluateBatch(ctx context.Context, req BatchAccessRequest) map[string]map[string]bool {
	start := time.Now()

	batcher, ok := e.engine.(interface {
		DecideBatch(context.Context, BatchAccessRequest) map[string]map[string]bool
	})
	if !ok {
		// No batch path on this engine — fall back to the per-pair evaluation
		// so behaviour stays correct even if the engine is swapped in a test.
		results := make(map[string]map[string]bool, len(req.ObjectNodeIDs))
		for _, objID := range req.ObjectNodeIDs {
			perms := make(map[string]bool, len(req.Operations))
			for _, op := range req.Operations {
				d := e.Evaluate(ctx, AccessRequest{
					UserNodeID:   req.UserNodeID,
					ObjectNodeID: objID,
					Operation:    op,
					WorkspaceID:  req.WorkspaceID,
				})
				perms[op] = d != nil && d.Decision == DecisionAllow
			}
			results[objID] = perms
		}
		return results
	}

	results := batcher.DecideBatch(ctx, req)

	metrics.CheckAccessTotal.WithLabelValues("batch").Inc()
	metrics.CheckAccessDuration.WithLabelValues("batch").Observe(time.Since(start).Seconds())

	return results
}
