package ngac_test

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"ngac-platform/services/policy/internal/ngac"
)

// Test vectors for prohibitions held in the in-memory graph. The PDP evaluates
// them without touching the database; the PAP writes the database first, then
// memory; replicas follow through the EPP graph-mutation event.

// failClosedGraphWith returns the fail-closed fixture with the given
// prohibitions loaded into it.
func failClosedGraphWith(t *testing.T, ps ...*ngac.Prohibition) *ngac.Graph {
	t.Helper()
	g := buildFailClosedGraph()
	for _, p := range ps {
		require.NoError(t, g.AddProhibition(p))
	}
	return g
}

func noSecretWrites() *ngac.Prohibition {
	return &ngac.Prohibition{
		Name: "no-secret-writes", SubjectID: "ua-staff",
		Operations: []string{"write"}, TargetOAIDs: []string{"oa-secret"},
	}
}

func batchOf(t *testing.T, e ngac.DecisionEngine, user string, objects, ops []string) map[string]map[string]bool {
	t.Helper()
	return e.(batchDecider).DecideBatch(context.Background(), ngac.BatchAccessRequest{
		UserNodeID: user, ObjectNodeIDs: objects, Operations: ops,
	})
}

// (1) A matching prohibition denies despite the association — single and batch.
func TestMemoryProhibition_MatchDeniesDespiteAssociation(t *testing.T) {
	g := failClosedGraphWith(t, noSecretWrites())
	e := ngac.NewDecisionEngine(g, nil)

	require.Equal(t, ngac.DecisionAllow, g.CheckAccess("u-alice", "oa-secret", "write").Decision,
		"precondition: the association grants write")

	d := decide(t, e, "u-alice", "oa-secret", "write")
	assert.Equal(t, ngac.DecisionDeny, d.Decision)
	require.NotNil(t, d.Explanation.ProhibitionDenied)
	assert.Equal(t, "no-secret-writes", d.Explanation.ProhibitionDenied.ProhibitionName)
	assert.Equal(t, "ua-staff", d.Explanation.ProhibitionDenied.SubjectID)
	assert.False(t, d.ErrorDerived(), "a prohibition DENY is a real policy answer and cacheable")

	res := batchOf(t, e, "u-alice", []string{"oa-docs", "oa-secret"}, []string{"read", "write"})
	assert.False(t, res["oa-secret"]["write"], "batch path agrees")
	assert.True(t, res["oa-secret"]["read"])
	assert.True(t, res["oa-docs"]["write"])
	assert.True(t, res["oa-docs"]["read"])
}

// (2) Prohibitions that do not match leave the ALLOW standing.
func TestMemoryProhibition_NonMatchingKeepsAllow(t *testing.T) {
	g := failClosedGraphWith(t,
		// different subject: bob is not in ua-staff, and alice is not bob
		&ngac.Prohibition{Name: "bob-only", SubjectID: "u-bob", Operations: []string{"write"}, TargetOAIDs: []string{"oa-secret"}},
		// different operation
		&ngac.Prohibition{Name: "read-only-ban", SubjectID: "ua-staff", Operations: []string{"read"}, TargetOAIDs: []string{"oa-nowhere"}},
		// target not among the object's containers
		&ngac.Prohibition{Name: "elsewhere", SubjectID: "ua-staff", Operations: []string{"write"}, TargetOAIDs: []string{"oa-elsewhere"}},
		// intersection with one target unmatched
		&ngac.Prohibition{Name: "needs-both", SubjectID: "u-alice", Operations: []string{"write"},
			TargetOAIDs: []string{"oa-secret", "oa-elsewhere"}, Intersection: true},
	)
	e := ngac.NewDecisionEngine(g, nil)

	for _, op := range []string{"read", "write"} {
		assert.Equal(t, ngac.DecisionAllow, decide(t, e, "u-alice", "oa-secret", op).Decision, op)
	}
	res := batchOf(t, e, "u-alice", []string{"oa-docs", "oa-secret"}, []string{"read", "write"})
	for obj, perms := range res {
		for op, ok := range perms {
			assert.Truef(t, ok, "%s/%s", obj, op)
		}
	}
}

// A prohibition is a deny-override only: it never turns a DENY into anything
// and never grants. Users without an association stay denied and unmarked.
func TestMemoryProhibition_DoesNotTouchExistingDeny(t *testing.T) {
	g := failClosedGraphWith(t, &ngac.Prohibition{
		Name: "everyone-ish", SubjectID: "u-carol", Operations: []string{"read"}, TargetOAIDs: []string{"oa-docs"}})
	e := ngac.NewDecisionEngine(g, nil)

	for _, user := range []string{"u-carol", "u-bob"} {
		d := decide(t, e, user, "oa-docs", "read")
		assert.Equal(t, ngac.DecisionDeny, d.Decision, user)
		assert.Nil(t, d.Explanation.ProhibitionDenied, "%s: denied by the graph, not by a prohibition", user)
		assert.False(t, d.ErrorDerived())
	}
}

// Prohibitions on a UA reach users through assignment; the subject must be an
// ancestor of the user. Removing the prohibition from memory restores ALLOW.
func TestMemoryProhibition_RemoveRestoresAllow(t *testing.T) {
	g := failClosedGraphWith(t, noSecretWrites())
	e := ngac.NewDecisionEngine(g, nil)
	require.Equal(t, ngac.DecisionDeny, decide(t, e, "u-alice", "oa-secret", "write").Decision)

	g.RemoveProhibition("no-secret-writes")

	assert.Equal(t, ngac.DecisionAllow, decide(t, e, "u-alice", "oa-secret", "write").Decision)
	assert.Zero(t, g.ProhibitionCount())
}

func TestMemoryProhibition_AddRejectsInvalidAndReplacesByName(t *testing.T) {
	g := buildFailClosedGraph()
	for name, p := range map[string]*ngac.Prohibition{
		"nil":        nil,
		"no name":    {SubjectID: "u", Operations: []string{"read"}, TargetOAIDs: []string{"oa"}},
		"no subject": {Name: "n", Operations: []string{"read"}, TargetOAIDs: []string{"oa"}},
		"no ops":     {Name: "n", SubjectID: "u", TargetOAIDs: []string{"oa"}},
		"no targets": {Name: "n", SubjectID: "u", Operations: []string{"read"}},
	} {
		assert.Errorf(t, g.AddProhibition(p), name)
	}
	assert.Zero(t, g.ProhibitionCount())

	require.NoError(t, g.AddProhibition(noSecretWrites()))
	moved := noSecretWrites()
	moved.SubjectID = "u-bob"
	require.NoError(t, g.AddProhibition(moved))
	assert.Equal(t, 1, g.ProhibitionCount(), "same name replaces, never duplicates")
	assert.Empty(t, g.ProhibitionsForSubjects([]string{"ua-staff"}, "write"), "old subject index entry is gone")
	assert.Len(t, g.ProhibitionsForSubjects([]string{"u-bob"}, "write"), 1)

	// The graph keeps its own copy: mutating the caller's value changes nothing.
	moved.SubjectID = "u-alice"
	assert.Len(t, g.ProhibitionsForSubjects([]string{"u-bob"}, "write"), 1)
}

// (3) The ALLOW path reads no database: decisions run against a Store-backed
// graph and the pool's acquire counter does not move.
func TestMemoryProhibition_AllowPathMakesNoDBCalls(t *testing.T) {
	store, pool := setupStore(t)
	ctx := context.Background()
	g := store.GetGraph()

	tn := newTenantFixture(t, store, pool)
	require.NoError(t, g.AddProhibition(&ngac.Prohibition{
		Name: "unrelated-" + tn.suffix, SubjectID: "someone-else",
		Operations: []string{"read"}, TargetOAIDs: []string{"oa-nowhere"}}))

	e := ngac.NewDecisionEngine(g, nil)
	before := pool.Stat().AcquireCount()

	for range 20 {
		d := e.Decide(ctx, ngac.AccessRequest{UserNodeID: tn.user, ObjectNodeID: tn.oa, Operation: "read"})
		require.Equal(t, ngac.DecisionAllow, d.Decision)
	}
	res := e.(batchDecider).DecideBatch(ctx, ngac.BatchAccessRequest{
		UserNodeID: tn.user, ObjectNodeIDs: []string{tn.oa}, Operations: []string{"read"}})
	require.True(t, res[tn.oa]["read"])

	assert.Equal(t, before, pool.Stat().AcquireCount(),
		"deciding an ALLOW must not acquire a database connection")
}

// A prohibition-free deployment pays nothing: no entries, still ALLOW.
func TestMemoryProhibition_NoProhibitionsNoEffect(t *testing.T) {
	e := ngac.NewDecisionEngine(buildFailClosedGraph(), nil)
	assert.Equal(t, ngac.DecisionAllow, decide(t, e, "u-alice", "oa-secret", "write").Decision)
	assert.Equal(t, ngac.DecisionDeny, decide(t, e, "u-carol", "oa-secret", "write").Decision)
}

// --- Database-backed: PAP ordering and replica convergence ---

type tenantFixture struct {
	suffix, pc, ua, oa, user string
}

func newTenantFixture(t *testing.T, writer *ngac.Store, pool *pgxpool.Pool) tenantFixture {
	t.Helper()
	ctx := context.Background()
	sfx := uuid.NewString()[:8]
	mk := func(prefix, typ string) string {
		n, err := writer.CreateNode(ctx, prefix+"_prohib_"+sfx, typ, nil)
		require.NoError(t, err)
		return n.ID
	}
	f := tenantFixture{suffix: sfx,
		pc: mk("PC", ngac.NodeTypePolicyClass), ua: mk("UA", ngac.NodeTypeUserAttribute),
		oa: mk("OA", ngac.NodeTypeObjectAttr), user: mk("U", ngac.NodeTypeUser)}
	t.Cleanup(func() {
		pool.Exec(context.Background(), "DELETE FROM ngac_prohibitions WHERE name LIKE '%' || $1",
			sfx)
		pool.Exec(context.Background(), "DELETE FROM ngac_nodes WHERE id = ANY($1)",
			[]string{f.pc, f.ua, f.oa, f.user})
	})
	_, err := writer.CreateAssignment(ctx, f.ua, f.pc)
	require.NoError(t, err)
	_, err = writer.CreateAssignment(ctx, f.oa, f.pc)
	require.NoError(t, err)
	_, err = writer.CreateAssignment(ctx, f.user, f.ua)
	require.NoError(t, err)
	_, err = writer.CreateAssociation(ctx, f.ua, f.oa, []string{"read"})
	require.NoError(t, err)
	return f
}

// (4) + (5) Create on the writer: the writer denies at once; the replica keeps
// allowing until the EPP event is applied, then denies without a restart.
// Remove: the same in reverse.
func TestMemoryProhibition_WriterToReplicaViaEPP(t *testing.T) {
	writer, pool := setupStore(t)
	ctx := context.Background()
	tn := newTenantFixture(t, writer, pool)
	prohibs := ngac.NewProhibitionStore(pool, writer.GetGraph())

	replica := ngac.NewStore(pool, ngac.NewGraph())
	require.NoError(t, replica.LoadGraph(ctx))
	writerEngine := ngac.NewDecisionEngine(writer.GetGraph(), nil)
	replicaEngine := ngac.NewDecisionEngine(replica.GetGraph(), nil)
	req := ngac.AccessRequest{UserNodeID: tn.user, ObjectNodeID: tn.oa, Operation: "read"}
	inv := &recordingInvalidator{}
	refresher := ngac.NewReplicaGraphRefresher(replica, &recordingShards{}, inv)

	require.Equal(t, ngac.DecisionAllow, writerEngine.Decide(ctx, req).Decision)
	require.Equal(t, ngac.DecisionAllow, replicaEngine.Decide(ctx, req).Decision)

	name := "ban-read-" + tn.suffix
	_, err := prohibs.Create(ctx, &ngac.Prohibition{
		Name: name, SubjectID: tn.ua, Operations: []string{"read"}, TargetOAIDs: []string{tn.oa}})
	require.NoError(t, err)

	d := writerEngine.Decide(ctx, req)
	assert.Equal(t, ngac.DecisionDeny, d.Decision, "the writer's memory is updated by Create")
	require.NotNil(t, d.Explanation.ProhibitionDenied)
	assert.Equal(t, ngac.DecisionAllow, replicaEngine.Decide(ctx, req).Decision,
		"precondition: until the event arrives the replica is stale")

	affected := []string{tn.ua, tn.oa, tn.user}
	require.NoError(t, refresher.Apply(ctx, []ngac.GraphMutation{
		{Type: ngac.MutationCreateProhibition, NodeIDs: affected}}))
	assert.Equal(t, ngac.DecisionDeny, replicaEngine.Decide(ctx, req).Decision,
		"after the EPP event the replica denies, no restart")
	require.Len(t, inv.calls, 1, "decision caches are invalidated for the affected nodes")
	assert.ElementsMatch(t, affected, inv.calls[0].nodes)

	// Remove.
	require.NoError(t, prohibs.Remove(ctx, name))
	assert.Equal(t, ngac.DecisionAllow, writerEngine.Decide(ctx, req).Decision, "writer allows again at once")
	assert.Equal(t, ngac.DecisionDeny, replicaEngine.Decide(ctx, req).Decision,
		"precondition: replica still holds the prohibition")
	require.NoError(t, refresher.Apply(ctx, []ngac.GraphMutation{
		{Type: ngac.MutationRemoveProhibition, NodeIDs: affected}}))
	assert.Equal(t, ngac.DecisionAllow, replicaEngine.Decide(ctx, req).Decision,
		"ALLOW returns on the replica after the EPP event")
	assert.Empty(t, replica.GetGraph().ProhibitionsForSubjects([]string{tn.ua}, "read"))
}

// Database first: a failed insert leaves memory untouched, so this process
// never denies on a prohibition that does not exist elsewhere.
func TestMemoryProhibition_CreateDBFailureLeavesMemoryUnchanged(t *testing.T) {
	g := buildFailClosedGraph()
	ps := ngac.NewProhibitionStore(unreachablePool(t), g)

	_, err := ps.Create(context.Background(), &ngac.Prohibition{
		Name: "ghost", SubjectID: "ua-staff", Operations: []string{"write"}, TargetOAIDs: []string{"oa-secret"}})

	require.Error(t, err)
	assert.Zero(t, g.ProhibitionCount())
	assert.Equal(t, ngac.DecisionAllow, g.CheckAccess("u-alice", "oa-secret", "write").Decision)
}

func TestMemoryProhibition_RemoveDBFailureKeepsProhibition(t *testing.T) {
	g := failClosedGraphWith(t, noSecretWrites())
	ps := ngac.NewProhibitionStore(unreachablePool(t), g)

	require.Error(t, ps.Remove(context.Background(), "no-secret-writes"))

	assert.Equal(t, 1, g.ProhibitionCount(), "the database still has it, so memory must too")
	e := ngac.NewDecisionEngine(g, nil)
	assert.Equal(t, ngac.DecisionDeny, decide(t, e, "u-alice", "oa-secret", "write").Decision)
}

func TestMemoryProhibition_DuplicateNameLeavesFirstIntact(t *testing.T) {
	writer, pool := setupStore(t)
	ctx := context.Background()
	tn := newTenantFixture(t, writer, pool)
	ps := ngac.NewProhibitionStore(pool, writer.GetGraph())
	name := "dup-" + tn.suffix

	first := &ngac.Prohibition{Name: name, SubjectID: tn.ua, Operations: []string{"read"}, TargetOAIDs: []string{tn.oa}}
	_, err := ps.Create(ctx, first)
	require.NoError(t, err)
	_, err = ps.Create(ctx, &ngac.Prohibition{Name: name, SubjectID: tn.user, Operations: []string{"write"}, TargetOAIDs: []string{tn.pc}})
	require.Error(t, err, "name is unique")

	got := writer.GetGraph().ProhibitionsForSubjects([]string{tn.ua}, "read")
	require.Len(t, got, 1, "the first prohibition is unchanged")
	assert.Empty(t, writer.GetGraph().ProhibitionsForSubjects([]string{tn.user}, "write"))
}

// ReloadGraph swaps prohibitions together with the graph: a prohibition
// deleted behind the replica's back disappears, a new one appears, and a failed
// reload leaves the previous set in place.
func TestMemoryProhibition_ReloadGraphSwapsAtomically(t *testing.T) {
	writer, pool := setupStore(t)
	ctx := context.Background()
	tn := newTenantFixture(t, writer, pool)
	replica := ngac.NewStore(pool, ngac.NewGraph())
	require.NoError(t, replica.LoadGraph(ctx))
	base := replica.GetGraph().ProhibitionCount()

	name := "reload-" + tn.suffix
	_, err := ngac.NewProhibitionStore(pool, nil).Create(ctx, &ngac.Prohibition{
		Name: name, SubjectID: tn.ua, Operations: []string{"read"}, TargetOAIDs: []string{tn.oa}})
	require.NoError(t, err)
	require.Equal(t, base, replica.GetGraph().ProhibitionCount(), "replica has not seen it yet")

	require.NoError(t, replica.ReloadGraph(ctx))
	assert.Len(t, replica.GetGraph().ProhibitionsForSubjects([]string{tn.ua}, "read"), 1)

	require.NoError(t, ngac.NewProhibitionStore(pool, nil).Remove(ctx, name))
	require.NoError(t, replica.ReloadGraph(ctx))
	assert.Empty(t, replica.GetGraph().ProhibitionsForSubjects([]string{tn.ua}, "read"),
		"reload reflects deletions of prohibitions too")
}

func TestMemoryProhibition_FailedReloadKeepsPreviousProhibitions(t *testing.T) {
	g := failClosedGraphWith(t, noSecretWrites())
	s := ngac.NewStore(unreachablePool(t), g)

	require.Error(t, s.ReloadGraph(context.Background()))

	assert.Equal(t, 1, g.ProhibitionCount())
	assert.Equal(t, ngac.DecisionDeny,
		decide(t, ngac.NewDecisionEngine(g, nil), "u-alice", "oa-secret", "write").Decision,
		"still fail closed on the previous prohibition set")
}

// LoadGraph populates the prohibitions that exist in the database.
func TestMemoryProhibition_LoadGraphLoadsThem(t *testing.T) {
	writer, pool := setupStore(t)
	ctx := context.Background()
	tn := newTenantFixture(t, writer, pool)
	name := "load-" + tn.suffix
	_, err := ngac.NewProhibitionStore(pool, nil).Create(ctx, &ngac.Prohibition{
		Name: name, SubjectID: tn.ua, Operations: []string{"read"}, TargetOAIDs: []string{tn.oa}})
	require.NoError(t, err)

	fresh := ngac.NewStore(pool, ngac.NewGraph())
	require.NoError(t, fresh.LoadGraph(ctx))

	got := fresh.GetGraph().ProhibitionsForSubjects([]string{tn.ua}, "read")
	require.Len(t, got, 1)
	assert.Equal(t, name, got[0].Name)
	assert.Equal(t, ngac.DecisionDeny,
		ngac.NewDecisionEngine(fresh.GetGraph(), nil).Decide(ctx,
			ngac.AccessRequest{UserNodeID: tn.user, ObjectNodeID: tn.oa, Operation: "read"}).Decision)
}
