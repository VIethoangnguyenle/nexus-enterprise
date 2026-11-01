package ngac_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"ngac-platform/services/policy/internal/ngac"
)

// Test vector: the subject of a decision is a user. On buildFailClosedGraph,
// alice (U, in Staff) reads and writes Docs. Staff itself is a UA holding that
// grant; asked as the subject it is DENY, as is an OA or a PC, so a check like
// "does this target hold manage" cannot be passed by naming a role or a
// department instead of a person.
var nonUserSubjects = []string{"ua-staff", "oa-docs", "pc-a"}

func TestSubjectMustBeUser_GraphCheck(t *testing.T) {
	g := buildFailClosedGraph()

	// Allow first, so the deny below is about the subject and nothing else.
	require.Equal(t, ngac.DecisionAllow, g.CheckAccess("u-alice", "oa-docs", "read").Decision)

	for _, subject := range nonUserSubjects {
		d := g.CheckAccess(subject, "oa-docs", "read")
		assert.Equal(t, ngac.DecisionDeny, d.Decision, "subject %s", subject)
		assert.Equal(t, ngac.DenyReasonSubjectNotUser, d.Explanation.Reason, "subject %s", subject)
	}
}

func TestSubjectMustBeUser_GraphBatch(t *testing.T) {
	g := buildFailClosedGraph()
	ops := []string{"read", "write"}

	got := g.CheckAccessBatch("u-alice", []string{"oa-docs", "oa-secret"}, ops)
	require.True(t, got["oa-docs"]["read"] && got["oa-secret"]["write"], "alice is allowed")

	for _, subject := range nonUserSubjects {
		got := g.CheckAccessBatch(subject, []string{"oa-docs", "oa-secret"}, ops)
		for _, obj := range []string{"oa-docs", "oa-secret"} {
			for _, op := range ops {
				assert.False(t, got[obj][op], "subject %s on %s %s", subject, obj, op)
			}
		}
	}
}

// A non-user subject that is in memory never reaches the SQL fallback either,
// even for an object that is not in the graph and that the SQL would allow.
func TestSubjectMustBeUser_EngineSkipsTheSQLFallback(t *testing.T) {
	cte := &fakeCTE{allowed: map[string]bool{
		"u-alice|file-1|read":  true,
		"ua-staff|file-1|read": true,
	}}
	e := ngac.NewDecisionEngine(buildFailClosedGraph(), cte)
	ctx := context.Background()

	require.Equal(t, ngac.DecisionAllow,
		e.Decide(ctx, ngac.AccessRequest{UserNodeID: "u-alice", ObjectNodeID: "file-1", Operation: "read"}).Decision,
		"a user is still resolved through the fallback")
	calls := cte.calls.Load()

	d := e.Decide(ctx, ngac.AccessRequest{UserNodeID: "ua-staff", ObjectNodeID: "file-1", Operation: "read"})
	assert.Equal(t, ngac.DecisionDeny, d.Decision)
	assert.Equal(t, ngac.DenyReasonSubjectNotUser, d.Explanation.Reason)

	batch := ngac.NewAccessEvaluator(&recordingCache{}, e).EvaluateBatch(ctx, ngac.BatchAccessRequest{
		UserNodeID: "ua-staff", ObjectNodeIDs: []string{"file-1", "oa-docs"}, Operations: []string{"read"},
	})
	assert.False(t, batch["file-1"]["read"])
	assert.False(t, batch["oa-docs"]["read"])
	assert.Equal(t, calls, cte.calls.Load(), "the SQL fallback is not asked for a non-user subject")
}
