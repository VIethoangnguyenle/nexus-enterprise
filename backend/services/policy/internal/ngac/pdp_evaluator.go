package ngac

import (
	"context"
	"errors"
	"log/slog"
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
//   - L1 (Redis) is checked by the cache
//   - L3 (BFS/CTE + prohibitions) is computed by the engine on cache miss
//   - Result is stored back into cache layers for future lookups
func (e *AccessEvaluator) Evaluate(ctx context.Context, req AccessRequest) *AccessDecision {
	start := time.Now()

	// Try the cache (L1)
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
	flight := e.inflight.DoChan(inflightKey(req), func() (any, error) {
		return e.computeShared(ctx, req), nil
	})

	var decision *AccessDecision
	select {
	case res := <-flight:
		decision, _ = res.Val.(*AccessDecision)
	case <-ctx.Done():
		// This caller gave up. The shared computation carries on for everyone
		// else collapsed onto it; this caller just gets a DENY it will most
		// likely never read. It is error-derived, and is not cached.
		d := &AccessDecision{
			Decision:  DecisionDeny,
			User:      req.UserNodeID,
			Object:    req.ObjectNodeID,
			Operation: req.Operation,
		}
		d.failClosed(DenyReasonEvaluationAborted, ctx.Err())
		return d
	}

	metrics.CheckAccessTotal.WithLabelValues("L3").Inc()
	metrics.CheckAccessDuration.WithLabelValues("L3").Observe(time.Since(start).Seconds())

	return decision
}

// sharedDecisionTimeout bounds one shared (singleflight) computation. It is the
// computation's own deadline, independent of any caller's.
const sharedDecisionTimeout = 10 * time.Second

// computeShared runs one PDP evaluation on behalf of every caller collapsed
// onto the same question, and caches the result if — and only if — it is a
// real policy answer.
//
// The context is detached from the caller that happened to start the flight:
// the result is handed to every collapsed caller and written to the shared
// caches, so it must not depend on one caller's cancellation. Under the
// caller's context, a cancelled first caller made the CTE fallback query fail,
// and the resulting decision was served to the other callers and cached.
// Values (trace IDs etc.) are kept; only cancellation and deadline are dropped,
// and replaced by sharedDecisionTimeout.
func (e *AccessEvaluator) computeShared(callerCtx context.Context, req AccessRequest) *AccessDecision {
	ctx, cancel := context.WithTimeout(context.WithoutCancel(callerCtx), sharedDecisionTimeout)
	defer cancel()

	d := e.engine.Decide(ctx, req)
	if d == nil {
		d = &AccessDecision{
			Decision:  DecisionDeny,
			User:      req.UserNodeID,
			Object:    req.ObjectNodeID,
			Operation: req.Operation,
		}
		d.failClosed(DenyReasonEvaluationAborted, errNoDecision)
	}

	if d.ErrorDerived() {
		// Correct to return for this request, wrong to remember: it describes
		// a failure at this instant, not the policy. Caching it would make a
		// transient database blip a sticky denial.
		slog.Warn("not caching error-derived access decision",
			"user_node_id", req.UserNodeID, "object_node_id", req.ObjectNodeID,
			"operation", req.Operation, "workspace_id", req.WorkspaceID,
			"reason", d.Explanation.Reason, "error", d.EvaluationErr)
		return d
	}

	e.cache.Set(ctx, req, d)
	return d
}

// errNoDecision marks the (defensive) case of an engine returning no decision.
var errNoDecision = errors.New("decision engine returned no decision")

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
