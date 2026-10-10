package ngac_test

import (
	"context"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"ngac-platform/services/policy/internal/ngac"
)

// pdpCallCounter counts how often the PDP is really asked.
type pdpCallCounter struct {
	ngac.DecisionEngine
	calls atomic.Int32
}

func (c *pdpCallCounter) Decide(ctx context.Context, req ngac.AccessRequest) *ngac.AccessDecision {
	c.calls.Add(1)
	return c.DecisionEngine.Decide(ctx, req)
}

// Decisions are cached in Redis only; a cached answer is the whole answer,
// explanation included, and the database is not involved.
func TestDecisionCache_L1ServesTheFullDecisionWithItsExplanation(t *testing.T) {
	rdb := testRedis(t, 13)
	g := failClosedGraphWith(t, noSecretWrites())
	engine := &pdpCallCounter{DecisionEngine: ngac.NewDecisionEngine(g, nil)}
	ev := ngac.NewAccessEvaluator(ngac.NewLayeredCache(rdb), engine)
	ctx := context.Background()
	req := ngac.AccessRequest{UserNodeID: "u-alice", ObjectNodeID: "oa-secret", Operation: "write", WorkspaceID: "ws-1"}

	first := ev.Evaluate(ctx, req)
	require.Equal(t, ngac.DecisionDeny, first.Decision)
	require.NotNil(t, first.Explanation.ProhibitionDenied)

	second := ev.Evaluate(ctx, req)
	assert.Equal(t, int32(1), engine.calls.Load(), "the second answer came from the cache, not the PDP")
	assert.Equal(t, ngac.DecisionDeny, second.Decision)
	require.NotNil(t, second.Explanation.ProhibitionDenied, "the cache keeps the explanation")
	assert.Equal(t, "no-secret-writes", second.Explanation.ProhibitionDenied.ProhibitionName)

	_, layer := ngac.NewLayeredCache(rdb).Get(ctx, req)
	assert.Equal(t, "L1", layer)
}

// What is cached is dropped when the graph around it changes, and the next
// question is answered afresh.
func TestDecisionCache_InvalidationMakesTheNextAnswerFresh(t *testing.T) {
	rdb := testRedis(t, 13)
	g := buildFailClosedGraph()
	engine := &pdpCallCounter{DecisionEngine: ngac.NewDecisionEngine(g, nil)}
	cache := ngac.NewLayeredCache(rdb)
	ev := ngac.NewAccessEvaluator(cache, engine)
	coord := ngac.NewInvalidationCoordinator(ngac.NewCacheInvalidator(rdb, func() *ngac.Graph { return g }))
	ctx := context.Background()
	alice := ngac.AccessRequest{UserNodeID: "u-alice", ObjectNodeID: "oa-docs", Operation: "write", WorkspaceID: "ws-1"}
	bob := ngac.AccessRequest{UserNodeID: "u-bob", ObjectNodeID: "oa-docs", Operation: "read", WorkspaceID: "ws-1"}

	require.Equal(t, ngac.DecisionAllow, ev.Evaluate(ctx, alice).Decision)
	require.Equal(t, ngac.DecisionDeny, ev.Evaluate(ctx, bob).Decision)
	require.Equal(t, int32(2), engine.calls.Load())

	// A prohibition lands in the graph; until the cache is invalidated the old
	// answer is what is served.
	require.NoError(t, g.AddProhibition(&ngac.Prohibition{
		Name: "no-docs-writes", SubjectID: "ua-staff", Operations: []string{"write"}, TargetOAIDs: []string{"oa-docs"},
	}))
	assert.Equal(t, ngac.DecisionAllow, ev.Evaluate(ctx, alice).Decision, "still the cached answer")

	coord.InvalidateForNodes(ctx, "u-alice")

	assert.Equal(t, ngac.DecisionDeny, ev.Evaluate(ctx, alice).Decision, "invalidated, then recomputed against the new graph")
	assert.Equal(t, int32(3), engine.calls.Load())
	ev.Evaluate(ctx, bob)
	assert.Equal(t, int32(3), engine.calls.Load(), "bob's cached answer survived alice's invalidation")

	coord.InvalidateAll(ctx)
	ev.Evaluate(ctx, bob)
	assert.Equal(t, int32(4), engine.calls.Load(), "a full flush drops every cached answer")
}

func TestDecisionCache_NoRedisMeansNoCachingNotAnError(t *testing.T) {
	g := buildFailClosedGraph()
	engine := &pdpCallCounter{DecisionEngine: ngac.NewDecisionEngine(g, nil)}
	ev := ngac.NewAccessEvaluator(ngac.NewLayeredCache(nil), engine)
	req := ngac.AccessRequest{UserNodeID: "u-alice", ObjectNodeID: "oa-docs", Operation: "read"}

	assert.Equal(t, ngac.DecisionAllow, ev.Evaluate(context.Background(), req).Decision)
	assert.Equal(t, ngac.DecisionAllow, ev.Evaluate(context.Background(), req).Decision)
	assert.Equal(t, int32(2), engine.calls.Load())

	// Nothing to invalidate and nothing to panic on.
	coord := ngac.NewInvalidationCoordinator(nil)
	coord.InvalidateForNodes(context.Background(), "n1")
	coord.InvalidateAll(context.Background())
}

// Only decisions a failure did not produce are cached.
func TestDecisionCache_NeverStoresAnErrorDerivedDecision(t *testing.T) {
	rdb := testRedis(t, 13)
	cache := ngac.NewLayeredCache(rdb)
	req := ngac.AccessRequest{UserNodeID: "u", ObjectNodeID: "o", Operation: "read"}
	cache.Set(context.Background(), req, nil)
	got, _ := cache.Get(context.Background(), req)
	assert.Nil(t, got)
}
