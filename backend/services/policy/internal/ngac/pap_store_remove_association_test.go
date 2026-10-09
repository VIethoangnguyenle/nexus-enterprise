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

// unreachablePool returns a pool whose every query fails (nothing listens on
// port 1). pgxpool connects lazily, so creating it succeeds.
func unreachablePool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	pool, err := pgxpool.New(context.Background(),
		"postgres://nobody:nothing@127.0.0.1:1/none?sslmode=disable&connect_timeout=1")
	require.NoError(t, err)
	t.Cleanup(pool.Close)
	return pool
}

// The in-memory graph must keep matching the database when the database write
// fails. Removing the edge from memory first made this process deny what the
// database (and every other process) still allowed, until restart.
func TestRemoveAssociationByUAOA_DBFailureLeavesGraphUnchanged(t *testing.T) {
	g := buildFailClosedGraph()
	s := ngac.NewStore(unreachablePool(t), g)
	require.Equal(t, ngac.DecisionAllow, g.CheckAccess("u-alice", "oa-docs", "read").Decision)

	err := s.RemoveAssociationByUAOA(context.Background(), "ua-staff", "oa-docs")

	require.Error(t, err, "the DB failure must be reported")
	found := false
	for _, a := range g.GetAssociationsFromUA("ua-staff") {
		if a.OAID == "oa-docs" {
			found = true
		}
	}
	assert.True(t, found, "association must still be in the in-memory graph")
	assert.Equal(t, ngac.DecisionAllow, g.CheckAccess("u-alice", "oa-docs", "read").Decision,
		"decisions must still reflect the (unchanged) database")
}

func TestRemoveAssociationByUAOA_RemovesFromDBAndGraph(t *testing.T) {
	s, pool := setupStore(t)
	ctx := context.Background()
	suffix := uuid.NewString()[:8]

	pc, err := s.CreateNode(ctx, "PC_rmassoc_"+suffix, ngac.NodeTypePolicyClass, nil)
	require.NoError(t, err)
	ua, err := s.CreateNode(ctx, "UA_rmassoc_"+suffix, ngac.NodeTypeUserAttribute, nil)
	require.NoError(t, err)
	oa, err := s.CreateNode(ctx, "OA_rmassoc_"+suffix, ngac.NodeTypeObjectAttr, nil)
	require.NoError(t, err)
	u, err := s.CreateNode(ctx, "U_rmassoc_"+suffix, ngac.NodeTypeUser, nil)
	require.NoError(t, err)
	t.Cleanup(func() {
		pool.Exec(context.Background(), "DELETE FROM ngac_nodes WHERE id = ANY($1)",
			[]string{pc.ID, ua.ID, oa.ID, u.ID})
	})
	for _, a := range [][2]string{{ua.ID, pc.ID}, {oa.ID, pc.ID}, {u.ID, ua.ID}} {
		_, err := s.CreateAssignment(ctx, a[0], a[1])
		require.NoError(t, err)
	}
	_, err = s.CreateAssociation(ctx, ua.ID, oa.ID, []string{"read"})
	require.NoError(t, err)
	require.Equal(t, ngac.DecisionAllow, s.GetGraph().CheckAccess(u.ID, oa.ID, "read").Decision)

	require.NoError(t, s.RemoveAssociationByUAOA(ctx, ua.ID, oa.ID))

	var n int
	require.NoError(t, pool.QueryRow(ctx,
		"SELECT COUNT(*) FROM ngac_associations WHERE ua_id = $1 AND oa_id = $2", ua.ID, oa.ID).Scan(&n))
	assert.Zero(t, n, "removed from the database")
	assert.Empty(t, s.GetGraph().GetAssociationsFromUA(ua.ID), "removed from the graph")
	assert.Equal(t, ngac.DecisionDeny, s.GetGraph().CheckAccess(u.ID, oa.ID, "read").Decision)
}
