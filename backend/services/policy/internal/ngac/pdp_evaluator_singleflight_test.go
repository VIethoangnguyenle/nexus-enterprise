package ngac_test

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"ngac-platform/services/policy/internal/ngac"
)

// countingEngine records how many times the PDP actually computed a decision.
type countingEngine struct {
	computations atomic.Int64
	release      chan struct{} // held closed until the test lets computations finish
}

func (e *countingEngine) Decide(_ context.Context, req ngac.AccessRequest) *ngac.AccessDecision {
	e.computations.Add(1)
	if e.release != nil {
		<-e.release
	}
	return &ngac.AccessDecision{
		Decision: ngac.DecisionAllow,
		User:     req.UserNodeID,
		Object:   req.ObjectNodeID,
	}
}

// nullCache always misses and never stores, so every request reaches the engine
// unless something else collapses them.
type nullCache struct{}

func (nullCache) Get(_ context.Context, _ ngac.AccessRequest) (*ngac.AccessDecision, string) {
	return nil, ""
}
func (nullCache) Set(_ context.Context, _ ngac.AccessRequest, _ *ngac.AccessDecision) {}

// A burst of identical checks must cost one traversal, not one per caller.
//
// This is the shape the read path actually sees: a page load fires many
// requests at once, and on a cold cache they are frequently the *same* question
// — the same user against the same folder. Without collapsing, each one walks
// the graph independently, so the work scales with concurrency rather than with
// distinct questions.
func TestEvaluate_CollapsesConcurrentIdenticalRequests(t *testing.T) {
	engine := &countingEngine{release: make(chan struct{})}
	evaluator := ngac.NewAccessEvaluator(nullCache{}, engine)

	req := ngac.AccessRequest{UserNodeID: "u1", ObjectNodeID: "oa1", Operation: "read"}

	const callers = 50
	var wg sync.WaitGroup
	results := make([]*ngac.AccessDecision, callers)

	wg.Add(callers)
	for i := range callers {
		go func() {
			defer wg.Done()
			results[i] = evaluator.Evaluate(context.Background(), req)
		}()
	}

	// Let every caller reach the evaluator before the single in-flight
	// computation is allowed to finish. Releasing earlier would let the first
	// flight complete while stragglers are still arriving, and a caller that
	// arrives after a flight ends legitimately starts a new one — that would be
	// correct behaviour measured as a failure.
	time.Sleep(100 * time.Millisecond)
	close(engine.release)
	wg.Wait()

	require.Equal(t, int64(1), engine.computations.Load(),
		"identical concurrent checks must be collapsed into one traversal")
	for i, d := range results {
		require.NotNilf(t, d, "caller %d got no decision", i)
		require.Equal(t, ngac.DecisionAllow, d.Decision, "every caller must get the real answer")
	}
}

// Different questions must not be collapsed into each other.
func TestEvaluate_DoesNotCollapseDifferentRequests(t *testing.T) {
	engine := &countingEngine{}
	evaluator := ngac.NewAccessEvaluator(nullCache{}, engine)

	distinct := []ngac.AccessRequest{
		{UserNodeID: "u1", ObjectNodeID: "oa1", Operation: "read"},
		{UserNodeID: "u1", ObjectNodeID: "oa1", Operation: "write"}, // different operation
		{UserNodeID: "u1", ObjectNodeID: "oa2", Operation: "read"},  // different object
		{UserNodeID: "u2", ObjectNodeID: "oa1", Operation: "read"},  // different user
		{UserNodeID: "u1", ObjectNodeID: "oa1", Operation: "read", WorkspaceID: "ws1"}, // different shard
	}

	for _, req := range distinct {
		evaluator.Evaluate(context.Background(), req)
	}

	require.Equal(t, int64(len(distinct)), engine.computations.Load(),
		"each distinct question must be evaluated on its own")
}

// Once a burst has settled, a later request is a fresh question again — the
// collapse must not behave like a cache with no expiry.
func TestEvaluate_SequentialRequestsAreNotCollapsed(t *testing.T) {
	engine := &countingEngine{}
	evaluator := ngac.NewAccessEvaluator(nullCache{}, engine)
	req := ngac.AccessRequest{UserNodeID: "u1", ObjectNodeID: "oa1", Operation: "read"}

	evaluator.Evaluate(context.Background(), req)
	evaluator.Evaluate(context.Background(), req)

	require.Equal(t, int64(2), engine.computations.Load(),
		"collapsing is for concurrent callers, not a substitute for the cache")
}
