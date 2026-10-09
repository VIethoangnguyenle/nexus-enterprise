package ngac_test

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"ngac-platform/services/policy/internal/ngac"
)

// A DENY produced by a failure is returned, but never cached; once the store
// recovers the real answer is computed and cached.
func TestEvaluate_ErrorDerivedDecisionIsNotCached(t *testing.T) {
	store := &fakeProhibitions{err: errDBDown}
	cache := &recordingCache{}
	ev := ngac.NewAccessEvaluator(cache, ngac.NewDecisionEngine(buildFailClosedGraph(), nil, store))
	req := ngac.AccessRequest{UserNodeID: "u-alice", ObjectNodeID: "oa-docs", Operation: "read"}

	d := ev.Evaluate(context.Background(), req)
	assert.Equal(t, ngac.DecisionDeny, d.Decision, "fail closed")
	assert.True(t, d.ErrorDerived())
	assert.Empty(t, cache.stored(), "an error-derived DENY must not be cached — it would outlive the outage")

	store.setErr(nil)
	d = ev.Evaluate(context.Background(), req)
	assert.Equal(t, ngac.DecisionAllow, d.Decision)
	stored := cache.stored()
	require.Len(t, stored, 1)
	assert.Equal(t, ngac.DecisionAllow, stored[0].Decision)
}

func TestEvaluate_CTEErrorDenyIsNotCached(t *testing.T) {
	cache := &recordingCache{}
	ev := ngac.NewAccessEvaluator(cache,
		ngac.NewDecisionEngine(buildFailClosedGraph(), &fakeCTE{err: errDBDown}, &fakeProhibitions{}))

	d := ev.Evaluate(context.Background(), ngac.AccessRequest{
		UserNodeID: "u-alice", ObjectNodeID: "o-not-in-graph", Operation: "read",
	})
	assert.Equal(t, ngac.DecisionDeny, d.Decision)
	assert.True(t, d.ErrorDerived())
	assert.Empty(t, cache.stored())
}

// A policy DENY (not error-derived) is still cached as before.
func TestEvaluate_PolicyDenyIsCached(t *testing.T) {
	cache := &recordingCache{}
	ev := ngac.NewAccessEvaluator(cache, ngac.NewDecisionEngine(buildFailClosedGraph(), nil, &fakeProhibitions{}))

	d := ev.Evaluate(context.Background(), ngac.AccessRequest{UserNodeID: "u-bob", ObjectNodeID: "oa-docs", Operation: "read"})
	assert.Equal(t, ngac.DecisionDeny, d.Decision)
	stored := cache.stored()
	require.Len(t, stored, 1)
	assert.Equal(t, ngac.DecisionDeny, stored[0].Decision)
}

// blockingProhibitions holds every lookup until released, then behaves like the
// real store would: a cancelled context makes the query fail.
type blockingProhibitions struct {
	entered  chan struct{} // closed on first call
	release  chan struct{}
	calls    atomic.Int64
	sawCtxOK atomic.Bool
}

func (b *blockingProhibitions) FindForSubjects(ctx context.Context, _ []string, _ string) ([]*ngac.Prohibition, error) {
	if b.calls.Add(1) == 1 {
		close(b.entered)
	}
	<-b.release
	if err := ctx.Err(); err != nil {
		return nil, err // what pgx returns for a cancelled query
	}
	b.sawCtxOK.Store(true)
	return nil, nil
}

// The first caller's context starts the shared computation. Before the fix
// that context was used for the computation itself: cancelling the first caller
// failed the prohibition query, and the resulting decision was handed to every
// other caller collapsed onto it and written to the cache (as ALLOW while the
// engine failed open; as a sticky DENY once it failed closed).
func TestEvaluate_CancelledFirstCallerDoesNotPoisonSharedResult(t *testing.T) {
	store := &blockingProhibitions{entered: make(chan struct{}), release: make(chan struct{})}
	cache := &recordingCache{}
	ev := ngac.NewAccessEvaluator(cache, ngac.NewDecisionEngine(buildFailClosedGraph(), nil, store))
	req := ngac.AccessRequest{UserNodeID: "u-alice", ObjectNodeID: "oa-docs", Operation: "read"}

	firstCtx, cancelFirst := context.WithCancel(context.Background())
	first := make(chan *ngac.AccessDecision, 1)
	go func() { first <- ev.Evaluate(firstCtx, req) }()
	<-store.entered // the shared computation is running, started by the first caller

	second := make(chan *ngac.AccessDecision, 1)
	go func() { second <- ev.Evaluate(context.Background(), req) }()
	time.Sleep(100 * time.Millisecond) // let the second caller join the in-flight computation

	cancelFirst()

	// The cancelled caller returns promptly, without waiting for the shared work.
	select {
	case d := <-first:
		assert.Equal(t, ngac.DecisionDeny, d.Decision, "an abandoned request gets no ALLOW")
		assert.True(t, d.ErrorDerived())
	case <-time.After(2 * time.Second):
		t.Fatal("cancelled caller stayed blocked on the shared computation")
	}

	close(store.release)

	select {
	case d := <-second:
		assert.Equal(t, ngac.DecisionAllow, d.Decision, "the surviving caller gets the real answer")
		assert.False(t, d.ErrorDerived())
	case <-time.After(5 * time.Second):
		t.Fatal("second caller never got a decision")
	}

	assert.EqualValues(t, 1, store.calls.Load(), "both callers shared one computation")
	assert.True(t, store.sawCtxOK.Load(), "the shared computation ran under a context the first caller could not cancel")

	stored := cache.stored()
	require.Len(t, stored, 1, "exactly the real answer is cached")
	assert.Equal(t, ngac.DecisionAllow, stored[0].Decision)
	for _, d := range stored {
		assert.False(t, d.ErrorDerived(), "nothing error-derived may reach the cache")
	}
}

// Even when the shared computation itself fails, every collapsed caller gets
// the fail-closed DENY and nothing is cached.
func TestEvaluate_SharedComputationErrorDeniesAllCallersAndCachesNothing(t *testing.T) {
	cache := &recordingCache{}
	ev := ngac.NewAccessEvaluator(cache,
		ngac.NewDecisionEngine(buildFailClosedGraph(), nil, &fakeProhibitions{err: errDBDown}))
	req := ngac.AccessRequest{UserNodeID: "u-alice", ObjectNodeID: "oa-docs", Operation: "read"}

	results := make(chan *ngac.AccessDecision, 10)
	for range 10 {
		go func() { results <- ev.Evaluate(context.Background(), req) }()
	}
	for range 10 {
		d := <-results
		assert.Equal(t, ngac.DecisionDeny, d.Decision)
	}
	assert.Empty(t, cache.stored())
}
