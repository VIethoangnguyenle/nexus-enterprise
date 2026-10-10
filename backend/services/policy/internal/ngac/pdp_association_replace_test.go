package ngac_test

import (
	"context"
	"sync"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"ngac-platform/services/policy/internal/ngac"
)

// roleGraph is a role with one grant on one OA, reached through one workspace PC:
//
//	PC <- role <- alice ;  PC <- docs
func roleGraph(t *testing.T) *ngac.Graph {
	t.Helper()
	g := ngac.NewGraph()
	for id, typ := range map[string]string{"pc": "PC", "role": "UA", "alice": "U", "docs": "OA"} {
		g.AddNode(&ngac.NGACNode{ID: id, Name: id, NodeType: typ})
	}
	require.NoError(t, g.AddAssignment(&ngac.Assignment{ID: "a1", ChildID: "role", ParentID: "pc"}))
	require.NoError(t, g.AddAssignment(&ngac.Assignment{ID: "a2", ChildID: "docs", ParentID: "pc"}))
	require.NoError(t, g.AddAssignment(&ngac.Assignment{ID: "a3", ChildID: "alice", ParentID: "role"}))
	return g
}

// Granting a UA new operations on an OA it is already associated with replaces
// the old operations. The database does (ON CONFLICT DO UPDATE); the graph kept
// both edges, so narrowing a role's grant changed nothing until a restart.
func TestAddAssociation_SecondGrantReplacesTheFirst(t *testing.T) {
	g := roleGraph(t)
	require.NoError(t, g.AddAssociation(&ngac.Association{ID: "x1", UAID: "role", OAID: "docs", Operations: []string{"read", "write", "share"}}))
	require.Equal(t, ngac.DecisionAllow, g.CheckAccess("alice", "docs", "share").Decision)

	require.NoError(t, g.AddAssociation(&ngac.Association{ID: "x2", UAID: "role", OAID: "docs", Operations: []string{"read"}}))

	assert.Equal(t, ngac.DecisionAllow, g.CheckAccess("alice", "docs", "read").Decision, "what is still granted stays")
	assert.Equal(t, ngac.DecisionDeny, g.CheckAccess("alice", "docs", "write").Decision, "what was narrowed away is denied")
	assert.Equal(t, ngac.DecisionDeny, g.CheckAccess("alice", "docs", "share").Decision, "what was narrowed away is denied")
	require.Len(t, g.GetAssociationsFromUA("role"), 1, "one edge per UA and OA")
	assert.Equal(t, []string{"read"}, g.GetAssociationsFromUA("role")[0].Operations)
}

// Widening works the same way.
func TestAddAssociation_SecondGrantCanWiden(t *testing.T) {
	g := roleGraph(t)
	require.NoError(t, g.AddAssociation(&ngac.Association{ID: "x1", UAID: "role", OAID: "docs", Operations: []string{"read"}}))
	require.Equal(t, ngac.DecisionDeny, g.CheckAccess("alice", "docs", "write").Decision)

	require.NoError(t, g.AddAssociation(&ngac.Association{ID: "x2", UAID: "role", OAID: "docs", Operations: []string{"read", "write"}}))

	assert.Equal(t, ngac.DecisionAllow, g.CheckAccess("alice", "docs", "write").Decision)
	require.Len(t, g.GetAssociationsFromUA("role"), 1)
}

// A different OA is a different edge; replacing is per UA and OA pair.
func TestAddAssociation_OtherOAIsUntouched(t *testing.T) {
	g := roleGraph(t)
	g.AddNode(&ngac.NGACNode{ID: "chat", Name: "chat", NodeType: "OA"})
	require.NoError(t, g.AddAssignment(&ngac.Assignment{ID: "a4", ChildID: "chat", ParentID: "pc"}))
	require.NoError(t, g.AddAssociation(&ngac.Association{ID: "x1", UAID: "role", OAID: "docs", Operations: []string{"read"}}))
	require.NoError(t, g.AddAssociation(&ngac.Association{ID: "x2", UAID: "role", OAID: "chat", Operations: []string{"write"}}))

	require.NoError(t, g.AddAssociation(&ngac.Association{ID: "x3", UAID: "role", OAID: "docs", Operations: []string{"share"}}))

	assert.Equal(t, ngac.DecisionAllow, g.CheckAccess("alice", "chat", "write").Decision)
	assert.Equal(t, ngac.DecisionDeny, g.CheckAccess("alice", "docs", "read").Decision)
	assert.Equal(t, ngac.DecisionAllow, g.CheckAccess("alice", "docs", "share").Decision)
	assert.Len(t, g.GetAssociationsFromUA("role"), 2)
}

// The store keeps the row the database holds: a second grant updates it, and
// the graph ends with exactly that row's ID and operations, so a restart
// (which reloads the rows) changes nothing.
func TestCreateAssociation_RegrantMatchesDatabaseAndGraph(t *testing.T) {
	s, pool := setupStore(t)
	ctx := context.Background()
	suffix := uuid.NewString()[:8]

	pc, err := s.CreateNode(ctx, "PC_regrant_"+suffix, ngac.NodeTypePolicyClass, nil)
	require.NoError(t, err)
	ua, err := s.CreateNode(ctx, "UA_regrant_"+suffix, ngac.NodeTypeUserAttribute, nil)
	require.NoError(t, err)
	oa, err := s.CreateNode(ctx, "OA_regrant_"+suffix, ngac.NodeTypeObjectAttr, nil)
	require.NoError(t, err)
	u, err := s.CreateNode(ctx, "U_regrant_"+suffix, ngac.NodeTypeUser, nil)
	require.NoError(t, err)
	t.Cleanup(func() {
		pool.Exec(context.Background(), "DELETE FROM ngac_nodes WHERE id = ANY($1)", []string{pc.ID, ua.ID, oa.ID, u.ID})
	})
	for _, a := range [][2]string{{ua.ID, pc.ID}, {oa.ID, pc.ID}, {u.ID, ua.ID}} {
		_, err := s.CreateAssignment(ctx, a[0], a[1])
		require.NoError(t, err)
	}

	first, err := s.CreateAssociation(ctx, ua.ID, oa.ID, []string{"read", "write"})
	require.NoError(t, err)
	second, err := s.CreateAssociation(ctx, ua.ID, oa.ID, []string{"read"})
	require.NoError(t, err)

	assert.Equal(t, first.ID, second.ID, "the row keeps its ID when it is updated")
	var id string
	var ops []string
	require.NoError(t, pool.QueryRow(ctx, "SELECT id, operations FROM ngac_associations WHERE ua_id = $1 AND oa_id = $2", ua.ID, oa.ID).Scan(&id, &ops))
	assert.Equal(t, second.ID, id)
	assert.Equal(t, []string{"read"}, ops)
	assert.Equal(t, ngac.DecisionAllow, s.GetGraph().CheckAccess(u.ID, oa.ID, "read").Decision)
	assert.Equal(t, ngac.DecisionDeny, s.GetGraph().CheckAccess(u.ID, oa.ID, "write").Decision)
	assert.Len(t, s.GetGraph().GetAssociationsFromUA(ua.ID), 1)
}

// Two concurrent re-grants must leave the graph holding what the database holds.
// The row is written and then the graph updated; without one lock around both,
// writer A's row can be overwritten by writer B while A's edge lands in the
// graph last, and every decision from then on disagrees with the database.
func TestCreateAssociation_ConcurrentRegrantsEndAgreeingWithTheDatabase(t *testing.T) {
	s, pool := setupStore(t)
	ctx := context.Background()
	suffix := uuid.NewString()[:8]

	pc, err := s.CreateNode(ctx, "PC_race_"+suffix, ngac.NodeTypePolicyClass, nil)
	require.NoError(t, err)
	ua, err := s.CreateNode(ctx, "UA_race_"+suffix, ngac.NodeTypeUserAttribute, nil)
	require.NoError(t, err)
	oa, err := s.CreateNode(ctx, "OA_race_"+suffix, ngac.NodeTypeObjectAttr, nil)
	require.NoError(t, err)
	t.Cleanup(func() {
		pool.Exec(context.Background(), "DELETE FROM ngac_nodes WHERE id = ANY($1)", []string{pc.ID, ua.ID, oa.ID})
	})
	for _, a := range [][2]string{{ua.ID, pc.ID}, {oa.ID, pc.ID}} {
		_, err := s.CreateAssignment(ctx, a[0], a[1])
		require.NoError(t, err)
	}

	for round := 0; round < 60; round++ {
		var wg sync.WaitGroup
		for _, ops := range [][]string{{"read"}, {"read", "write"}, {"share"}, {"write", "share"}, {"read", "share"}, {"write"}, {"read", "write", "share"}, {"share", "read", "write"}} {
			wg.Add(1)
			go func() {
				defer wg.Done()
				_, err := s.CreateAssociation(ctx, ua.ID, oa.ID, ops)
				assert.NoError(t, err)
			}()
		}
		wg.Wait()

		var dbOps []string
		require.NoError(t, pool.QueryRow(ctx, "SELECT operations FROM ngac_associations WHERE ua_id = $1 AND oa_id = $2", ua.ID, oa.ID).Scan(&dbOps))
		graph := s.GetGraph().GetAssociationsFromUA(ua.ID)
		require.Len(t, graph, 1, "round %d", round)
		assert.ElementsMatch(t, dbOps, graph[0].Operations, "round %d: graph and database disagree", round)
	}
}

// Granting and removing at the same time must not leave an edge the database no longer has.
func TestRemoveAssociation_ConcurrentWithGrantEndsAgreeing(t *testing.T) {
	s, pool := setupStore(t)
	ctx := context.Background()
	suffix := uuid.NewString()[:8]
	pc, _ := s.CreateNode(ctx, "PC_race2_"+suffix, ngac.NodeTypePolicyClass, nil)
	ua, _ := s.CreateNode(ctx, "UA_race2_"+suffix, ngac.NodeTypeUserAttribute, nil)
	oa, _ := s.CreateNode(ctx, "OA_race2_"+suffix, ngac.NodeTypeObjectAttr, nil)
	t.Cleanup(func() {
		pool.Exec(context.Background(), "DELETE FROM ngac_nodes WHERE id = ANY($1)", []string{pc.ID, ua.ID, oa.ID})
	})
	for _, a := range [][2]string{{ua.ID, pc.ID}, {oa.ID, pc.ID}} {
		_, err := s.CreateAssignment(ctx, a[0], a[1])
		require.NoError(t, err)
	}
	for round := 0; round < 25; round++ {
		var wg sync.WaitGroup
		wg.Add(2)
		go func() { defer wg.Done(); _, _ = s.CreateAssociation(ctx, ua.ID, oa.ID, []string{"read"}) }()
		go func() { defer wg.Done(); _ = s.RemoveAssociationByUAOA(ctx, ua.ID, oa.ID) }()
		wg.Wait()
		var n int
		require.NoError(t, pool.QueryRow(ctx, "SELECT count(*) FROM ngac_associations WHERE ua_id = $1 AND oa_id = $2", ua.ID, oa.ID).Scan(&n))
		assert.Equal(t, n, len(s.GetGraph().GetAssociationsFromUA(ua.ID)), "round %d: graph and database disagree", round)
	}
}

// Assigning the same edge twice is one edge, with the ID the database holds.
func TestCreateAssignment_RepeatKeepsOneEntryAndTheRowsID(t *testing.T) {
	s, pool := setupStore(t)
	ctx := context.Background()
	suffix := uuid.NewString()[:8]
	pc, _ := s.CreateNode(ctx, "PC_dup_"+suffix, ngac.NodeTypePolicyClass, nil)
	ua, _ := s.CreateNode(ctx, "UA_dup_"+suffix, ngac.NodeTypeUserAttribute, nil)
	t.Cleanup(func() {
		pool.Exec(context.Background(), "DELETE FROM ngac_nodes WHERE id = ANY($1)", []string{pc.ID, ua.ID})
	})

	first, err := s.CreateAssignment(ctx, ua.ID, pc.ID)
	require.NoError(t, err)
	second, err := s.CreateAssignment(ctx, ua.ID, pc.ID)
	require.NoError(t, err)
	assert.Equal(t, first.ID, second.ID, "the edge keeps the row's ID")

	n := 0
	for _, a := range s.GetGraph().Assignments {
		if a.ChildID == ua.ID && a.ParentID == pc.ID {
			n++
		}
	}
	assert.Equal(t, 1, n, "one graph entry per edge")

	require.NoError(t, s.RemoveAssignment(ctx, ua.ID, pc.ID))
	assert.False(t, s.IsAssigned(ua.ID, pc.ID))
	for _, a := range s.GetGraph().Assignments {
		assert.False(t, a.ChildID == ua.ID && a.ParentID == pc.ID, "no stale entry is left behind")
	}
}
