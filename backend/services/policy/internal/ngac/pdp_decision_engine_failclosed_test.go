package ngac_test

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"ngac-platform/services/policy/internal/ngac"
)

// Test vectors for the PDP's prohibition and CTE steps, with real fakes for the
// prohibition store and CTE evaluator (the older engine tests pass nil for
// both, which skips these steps entirely).

var errDBDown = errors.New("connection refused")

func decide(t *testing.T, e ngac.DecisionEngine, user, object, op string) *ngac.AccessDecision {
	t.Helper()
	d := e.Decide(context.Background(), ngac.AccessRequest{UserNodeID: user, ObjectNodeID: object, Operation: op})
	require.NotNil(t, d)
	return d
}

// Baseline the other vectors build on: the fixture allows exactly what it
// claims, so every DENY below is caused by the step under test.
func TestDecide_Fixture_AllowAndDenyBaseline(t *testing.T) {
	e := ngac.NewDecisionEngine(buildFailClosedGraph(), nil, &fakeProhibitions{})

	assert.Equal(t, ngac.DecisionAllow, decide(t, e, "u-alice", "oa-docs", "read").Decision)
	assert.Equal(t, ngac.DecisionAllow, decide(t, e, "u-alice", "oa-secret", "write").Decision)
	assert.Equal(t, ngac.DecisionDeny, decide(t, e, "u-carol", "oa-docs", "read").Decision,
		"no UA, no path")
	assert.Equal(t, ngac.DecisionDeny, decide(t, e, "u-bob", "oa-docs", "read").Decision,
		"association crosses policy classes — intersection principle denies")
}

// FAIL-OPEN regression: the prohibition store failing used to read as "no
// prohibition", so the ALLOW stood.
func TestDecide_ProhibitionStoreError_Denies(t *testing.T) {
	store := &fakeProhibitions{err: errDBDown}
	e := ngac.NewDecisionEngine(buildFailClosedGraph(), nil, store)

	d := decide(t, e, "u-alice", "oa-docs", "read")

	assert.Equal(t, ngac.DecisionDeny, d.Decision, "a prohibition that cannot be read may be the one that denies")
	assert.True(t, d.ErrorDerived(), "the DENY comes from the failure and must be marked so")
	assert.ErrorIs(t, d.EvaluationErr, errDBDown)
	assert.Equal(t, ngac.DenyReasonProhibitionCheckFailed, d.Explanation.Reason)
	assert.EqualValues(t, 1, store.calls.Load())

	// Control: the same question with a healthy store is an ordinary ALLOW.
	store.setErr(nil)
	d = decide(t, e, "u-alice", "oa-docs", "read")
	assert.Equal(t, ngac.DecisionAllow, d.Decision)
	assert.False(t, d.ErrorDerived())
}

// Prohibitions are deny-overrides on an ALLOW only: an existing DENY is not
// touched (and not marked error-derived) even when the store is down.
func TestDecide_ProhibitionStoreError_DoesNotTouchExistingDeny(t *testing.T) {
	store := &fakeProhibitions{err: errDBDown}
	e := ngac.NewDecisionEngine(buildFailClosedGraph(), nil, store)

	for _, user := range []string{"u-carol", "u-bob"} {
		d := decide(t, e, user, "oa-docs", "read")
		assert.Equal(t, ngac.DecisionDeny, d.Decision, user)
		assert.False(t, d.ErrorDerived(), "%s: a policy DENY is a real answer, cacheable", user)
	}
	assert.EqualValues(t, 0, store.calls.Load(), "prohibitions are not consulted for a DENY")
}

func TestDecide_ProhibitionMatch_DeniesDespiteAssociation(t *testing.T) {
	store := &fakeProhibitions{items: []*ngac.Prohibition{{
		Name: "no-secret-writes", SubjectID: "ua-staff",
		Operations: []string{"write"}, TargetOAIDs: []string{"oa-secret"},
	}}}
	e := ngac.NewDecisionEngine(buildFailClosedGraph(), nil, store)

	d := decide(t, e, "u-alice", "oa-secret", "write")
	assert.Equal(t, ngac.DecisionDeny, d.Decision, "prohibition on alice's UA overrides the association")
	require.NotNil(t, d.Explanation.ProhibitionDenied)
	assert.Equal(t, "no-secret-writes", d.Explanation.ProhibitionDenied.ProhibitionName)
	assert.Equal(t, "ua-staff", d.Explanation.ProhibitionDenied.SubjectID)
	assert.False(t, d.ErrorDerived(), "a prohibition DENY is a real policy answer")

	// The boundary: other operation, and the parent container, are unaffected.
	assert.Equal(t, ngac.DecisionAllow, decide(t, e, "u-alice", "oa-secret", "read").Decision,
		"prohibition is for write only")
	assert.Equal(t, ngac.DecisionAllow, decide(t, e, "u-alice", "oa-docs", "write").Decision,
		"prohibition targets secret, not its parent")
}

func TestDecide_ProhibitionIntersection_RequiresAllTargets(t *testing.T) {
	g := buildFailClosedGraph()

	partial := ngac.NewDecisionEngine(g, nil, &fakeProhibitions{items: []*ngac.Prohibition{{
		Name: "both-required", SubjectID: "u-alice", Operations: []string{"read"},
		TargetOAIDs: []string{"oa-secret", "oa-elsewhere"}, Intersection: true,
	}}})
	assert.Equal(t, ngac.DecisionAllow, decide(t, partial, "u-alice", "oa-secret", "read").Decision,
		"intersection prohibition with an unmatched target does not apply")

	full := ngac.NewDecisionEngine(g, nil, &fakeProhibitions{items: []*ngac.Prohibition{{
		Name: "both-match", SubjectID: "u-alice", Operations: []string{"read"},
		TargetOAIDs: []string{"oa-secret", "oa-docs"}, Intersection: true,
	}}})
	assert.Equal(t, ngac.DecisionDeny, decide(t, full, "u-alice", "oa-secret", "read").Decision,
		"secret is inside docs, so it is in both targets")
}

// --- CTE fallback (objects not in the in-memory graph) ---

func TestDecide_CTEFallback_AllowDenyAndError(t *testing.T) {
	g := buildFailClosedGraph()
	const obj = "o-file-not-in-graph"

	t.Run("CTE allows", func(t *testing.T) {
		cte := &fakeCTE{allowed: map[string]bool{"u-alice|" + obj + "|read": true}}
		d := decide(t, ngac.NewDecisionEngine(g, cte, &fakeProhibitions{}), "u-alice", obj, "read")
		assert.Equal(t, ngac.DecisionAllow, d.Decision)
		assert.False(t, d.ErrorDerived())
	})

	t.Run("CTE denies", func(t *testing.T) {
		cte := &fakeCTE{allowed: map[string]bool{"u-alice|" + obj + "|read": true}}
		d := decide(t, ngac.NewDecisionEngine(g, cte, &fakeProhibitions{}), "u-carol", obj, "read")
		assert.Equal(t, ngac.DecisionDeny, d.Decision)
		assert.False(t, d.ErrorDerived(), "a CTE 'no' is a real answer")
	})

	t.Run("CTE errors", func(t *testing.T) {
		cte := &fakeCTE{err: errDBDown}
		d := decide(t, ngac.NewDecisionEngine(g, cte, &fakeProhibitions{}), "u-alice", obj, "read")
		assert.Equal(t, ngac.DecisionDeny, d.Decision)
		assert.True(t, d.ErrorDerived(), "a DENY from a failed CTE query must not be cached")
		assert.Equal(t, ngac.DenyReasonCTEFallbackFailed, d.Explanation.Reason)
	})

	t.Run("CTE allows but prohibition targets the object", func(t *testing.T) {
		cte := &fakeCTE{allowed: map[string]bool{"u-alice|" + obj + "|read": true}}
		store := &fakeProhibitions{items: []*ngac.Prohibition{{
			Name: "block-file", SubjectID: "u-alice", Operations: []string{"read"}, TargetOAIDs: []string{obj},
		}}}
		d := decide(t, ngac.NewDecisionEngine(g, cte, store), "u-alice", obj, "read")
		assert.Equal(t, ngac.DecisionDeny, d.Decision, "prohibition overrides a CTE ALLOW")
	})

	t.Run("CTE allows but prohibition store errors", func(t *testing.T) {
		cte := &fakeCTE{allowed: map[string]bool{"u-alice|" + obj + "|read": true}}
		d := decide(t, ngac.NewDecisionEngine(g, cte, &fakeProhibitions{err: errDBDown}), "u-alice", obj, "read")
		assert.Equal(t, ngac.DecisionDeny, d.Decision)
		assert.True(t, d.ErrorDerived())
	})
}

func TestNewDecisionEngine_TypedNilDependenciesAreDisabled(t *testing.T) {
	e := ngac.NewDecisionEngine(buildFailClosedGraph(), (*ngac.CTEEvaluator)(nil), (*ngac.ProhibitionStore)(nil))
	assert.NotPanics(t, func() {
		assert.Equal(t, ngac.DecisionAllow, decide(t, e, "u-alice", "oa-docs", "read").Decision)
		assert.Equal(t, ngac.DecisionDeny, decide(t, e, "u-alice", "o-missing", "read").Decision)
	})
}

// --- Batch path ---

type batchDecider interface {
	DecideBatch(context.Context, ngac.BatchAccessRequest) map[string]map[string]bool
}

func TestDecideBatch_ProhibitionDenies(t *testing.T) {
	store := &fakeProhibitions{items: []*ngac.Prohibition{{
		Name: "no-secret-writes", SubjectID: "ua-staff",
		Operations: []string{"write"}, TargetOAIDs: []string{"oa-secret"},
	}}}
	e := ngac.NewDecisionEngine(buildFailClosedGraph(), nil, store).(batchDecider)

	res := e.DecideBatch(context.Background(), ngac.BatchAccessRequest{
		UserNodeID:    "u-alice",
		ObjectNodeIDs: []string{"oa-docs", "oa-secret"},
		Operations:    []string{"read", "write"},
	})

	assert.False(t, res["oa-secret"]["write"], "prohibition overrides the association in the batch path too")
	assert.True(t, res["oa-secret"]["read"], "other operation unaffected")
	assert.True(t, res["oa-docs"]["write"], "parent container unaffected")
	assert.True(t, res["oa-docs"]["read"])
}

func TestDecideBatch_ProhibitionStoreError_DeniesEverything(t *testing.T) {
	e := ngac.NewDecisionEngine(buildFailClosedGraph(), nil, &fakeProhibitions{err: errDBDown}).(batchDecider)

	res := e.DecideBatch(context.Background(), ngac.BatchAccessRequest{
		UserNodeID:    "u-alice",
		ObjectNodeIDs: []string{"oa-docs", "oa-secret"},
		Operations:    []string{"read", "write"},
	})

	for obj, perms := range res {
		for op, allowed := range perms {
			assert.Falsef(t, allowed, "%s/%s must fail closed", obj, op)
		}
	}
}

// Single and batch paths must agree, including on the fail-closed case.
func TestDecideBatch_AgreesWithDecide_OnProhibitionAndError(t *testing.T) {
	for name, store := range map[string]*fakeProhibitions{
		"prohibition": {items: []*ngac.Prohibition{{
			Name: "p", SubjectID: "ua-staff", Operations: []string{"write"}, TargetOAIDs: []string{"oa-secret"},
		}}},
		"store error": {err: errDBDown},
	} {
		t.Run(name, func(t *testing.T) {
			e := ngac.NewDecisionEngine(buildFailClosedGraph(), nil, store)
			objects := []string{"oa-docs", "oa-secret"}
			ops := []string{"read", "write"}
			for _, user := range []string{"u-alice", "u-bob", "u-carol"} {
				batch := e.(batchDecider).DecideBatch(context.Background(), ngac.BatchAccessRequest{
					UserNodeID: user, ObjectNodeIDs: objects, Operations: ops,
				})
				for _, obj := range objects {
					for _, op := range ops {
						single := decide(t, e, user, obj, op).Decision == ngac.DecisionAllow
						assert.Equalf(t, single, batch[obj][op], "%s %s %s", user, obj, op)
					}
				}
			}
		})
	}
}
