package ngac_test

import (
	"context"
	"os"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"ngac-platform/services/policy/internal/ngac"
)

// The SQL fallback against a real database: the people and OAs exist only in
// Postgres (the engine's graph is empty, as on a replica that has not loaded
// them), and the one prohibition lives in memory, as prohibitions do. The ancestors
// the PDP needs to match it come from CTEEvaluator.Ancestors over ngac_ancestors().
//
//	pc ─ members ◄ limitedUser, plainUser          members {read}→ dirOA, driveOA
//	pc ─ limited ◄ limitedUser                      prohibition: limited, read, [open]
//	pc ─ open ─ dirOA        pc ─ driveOA
func TestCTEFallbackAppliesAProhibitionFromDatabaseAncestors(t *testing.T) {
	url := os.Getenv("TEST_DATABASE_URL")
	if url == "" {
		t.Skip("TEST_DATABASE_URL not set")
	}
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, url)
	require.NoError(t, err)
	t.Cleanup(pool.Close)
	require.NoError(t, pool.Ping(ctx), "ngac_ci must be reachable")

	run := uuid.NewString()[:8]
	id := func(label string) string { return "cteprohib-" + run + "-" + label }
	nodes := []struct{ label, nodeType string }{
		{"pc", "PC"}, {"members", "UA"}, {"limited", "UA"}, {"open", "OA"}, {"dirOA", "OA"}, {"driveOA", "OA"},
		{"limitedUser", "U"}, {"plainUser", "U"},
	}
	for _, n := range nodes {
		_, err := pool.Exec(ctx, "INSERT INTO ngac_nodes (id, name, node_type) VALUES ($1, $2, $3)", id(n.label), id(n.label), n.nodeType)
		require.NoError(t, err)
	}
	// Deleting the nodes cascades to their assignments and associations.
	t.Cleanup(func() {
		for _, n := range nodes {
			_, _ = pool.Exec(context.Background(), "DELETE FROM ngac_nodes WHERE id = $1", id(n.label))
		}
	})
	for i, e := range [][2]string{
		{"members", "pc"}, {"limited", "pc"}, {"open", "pc"}, {"driveOA", "pc"},
		{"dirOA", "open"}, {"limitedUser", "members"}, {"limitedUser", "limited"}, {"plainUser", "members"},
	} {
		_, err := pool.Exec(ctx, "INSERT INTO ngac_assignments (id, child_id, parent_id) VALUES ($1, $2, $3)",
			id("as"+string(rune('a'+i))), id(e[0]), id(e[1]))
		require.NoError(t, err)
	}
	for i, oa := range []string{"dirOA", "driveOA"} {
		_, err := pool.Exec(ctx, "INSERT INTO ngac_associations (id, ua_id, oa_id, operations) VALUES ($1, $2, $3, $4)",
			id("ac"+string(rune('a'+i))), id("members"), id(oa), []string{"read"})
		require.NoError(t, err)
	}

	empty := ngac.NewGraph()
	require.NoError(t, empty.AddProhibition(&ngac.Prohibition{
		Name: "db-limited-" + run, SubjectID: id("limited"), Operations: []string{"read"}, TargetOAIDs: []string{id("open")},
	}))
	e := ngac.NewDecisionEngine(empty, ngac.NewCTEEvaluator(pool))

	single := func(user, oa string) *ngac.AccessDecision { return decide(t, e, id(user), id(oa), "read") }
	batch := func(user, oa string) bool {
		return batchOf(t, e, id(user), []string{id(oa)}, []string{"read"})[id(oa)]["read"]
	}

	// Precondition: the database alone allows both, so only the prohibition can deny.
	ok, err := ngac.NewCTEEvaluator(pool).CheckAccess(ctx, id("limitedUser"), id("dirOA"), "read")
	require.NoError(t, err)
	require.True(t, ok, "the SQL check allows the limited user the directory OA")

	// Deny: the limited user reading the directory OA, on both paths.
	d := single("limitedUser", "dirOA")
	assert.Equal(t, ngac.DecisionDeny, d.Decision)
	require.NotNil(t, d.Explanation.ProhibitionDenied)
	assert.Equal(t, "db-limited-"+run, d.Explanation.ProhibitionDenied.ProhibitionName)
	assert.False(t, d.ErrorDerived())
	assert.False(t, batch("limitedUser", "dirOA"))

	// Allow, same fixture: a non-directory OA for the same user, and the directory OA for an unlimited one.
	assert.Equal(t, ngac.DecisionAllow, single("limitedUser", "driveOA").Decision)
	assert.True(t, batch("limitedUser", "driveOA"))
	assert.Equal(t, ngac.DecisionAllow, single("plainUser", "dirOA").Decision)
	assert.True(t, batch("plainUser", "dirOA"))
}
