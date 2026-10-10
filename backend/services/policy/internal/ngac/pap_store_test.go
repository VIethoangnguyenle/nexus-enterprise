package ngac_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"ngac-platform/services/policy/internal/ngac"
)

// --- PAP: Mutation tests ---

func TestCreateNode(t *testing.T) {
	s, pool := setupStore(t)

	node, err := s.CreateNode(context.Background(), "test-oa-create", "OA", map[string]string{"scope": "test"})
	require.NoError(t, err)
	t.Cleanup(func() {
		pool.Exec(context.Background(), "DELETE FROM ngac_nodes WHERE id = $1", node.ID)
	})

	assert.NotEmpty(t, node.ID)
	assert.Equal(t, "test-oa-create", node.Name)
	assert.Equal(t, "OA", node.NodeType)

	// Verify in graph
	found := s.GetNode(node.ID)
	require.NotNil(t, found)
	assert.Equal(t, node.ID, found.ID)
}

func TestCreateNode_InvalidType(t *testing.T) {
	s, _ := setupStore(t)
	_, err := s.CreateNode(context.Background(), "bad-node", "INVALID", nil)
	require.Error(t, err)
}

func TestCreateAssignment(t *testing.T) {
	s, pool := setupStore(t)

	ua, err := s.CreateNode(context.Background(), "test-ua-asg", "UA", nil)
	require.NoError(t, err)
	t.Cleanup(func() { pool.Exec(context.Background(), "DELETE FROM ngac_nodes WHERE id = $1", ua.ID) })

	pc := s.FindNodeByName("PC_Global", "PC")
	require.NotNil(t, pc)

	asg, err := s.CreateAssignment(context.Background(), ua.ID, pc.ID)
	require.NoError(t, err)
	t.Cleanup(func() {
		pool.Exec(context.Background(), "DELETE FROM ngac_assignments WHERE id = $1", asg.ID)
	})

	assert.NotEmpty(t, asg.ID)

	// Verify: ua should be child of PC
	children := s.GetGraph().GetChildren(pc.ID)
	found := false
	for _, c := range children {
		if c.ID == ua.ID {
			found = true
		}
	}
	assert.True(t, found, "UA should be child of PC after assignment")
}

func TestCreateAssociation(t *testing.T) {
	s, pool := setupStore(t)

	ua, err := s.CreateNode(context.Background(), "test-ua-assoc", "UA", nil)
	require.NoError(t, err)
	t.Cleanup(func() { pool.Exec(context.Background(), "DELETE FROM ngac_nodes WHERE id = $1", ua.ID) })

	oa, err := s.CreateNode(context.Background(), "test-oa-assoc", "OA", nil)
	require.NoError(t, err)
	t.Cleanup(func() { pool.Exec(context.Background(), "DELETE FROM ngac_nodes WHERE id = $1", oa.ID) })

	assoc, err := s.CreateAssociation(context.Background(), ua.ID, oa.ID, []string{"read", "write"})
	require.NoError(t, err)
	t.Cleanup(func() {
		pool.Exec(context.Background(), "DELETE FROM ngac_associations WHERE id = $1", assoc.ID)
	})

	assert.NotEmpty(t, assoc.ID)

	// Verify association exists in graph
	assocs := s.GetGraph().GetAssociationsFromUA(ua.ID)
	found := false
	for _, a := range assocs {
		if a.OAID == oa.ID {
			found = true
			assert.Contains(t, a.Operations, "read")
			assert.Contains(t, a.Operations, "write")
		}
	}
	assert.True(t, found, "association should exist in graph")
}

// An association whose source is not a UA must be refused before anything is
// written: the row used to be inserted first and the graph check ran after, so
// a refused association left an orphan row that the next LoadGraph choked on.
func TestCreateAssociation_RejectsNonUASourceWithoutWriting(t *testing.T) {
	s, pool := setupStore(t)
	ctx := context.Background()

	user, err := s.CreateNode(ctx, "test-u-assoc-deny", "U", nil)
	require.NoError(t, err)
	t.Cleanup(func() { pool.Exec(ctx, "DELETE FROM ngac_nodes WHERE id = $1", user.ID) })
	oa, err := s.CreateNode(ctx, "test-oa-assoc-deny", "OA", nil)
	require.NoError(t, err)
	t.Cleanup(func() { pool.Exec(ctx, "DELETE FROM ngac_nodes WHERE id = $1", oa.ID) })

	_, err = s.CreateAssociation(ctx, user.ID, oa.ID, []string{"read"})
	require.Error(t, err)
	assert.ErrorIs(t, err, ngac.ErrInvalidAssociation)

	var rows int
	require.NoError(t, pool.QueryRow(ctx,
		"SELECT count(*) FROM ngac_associations WHERE ua_id = $1 OR oa_id = $2", user.ID, oa.ID).Scan(&rows))
	assert.Zero(t, rows, "a refused association must leave no database row")
	assert.Empty(t, s.GetGraph().GetAssociationsFromUA(user.ID))
}

func TestCreateAssociation_RejectsNonOATargetAndMissingNodesWithoutWriting(t *testing.T) {
	s, pool := setupStore(t)
	ctx := context.Background()

	ua, err := s.CreateNode(ctx, "test-ua-assoc-deny", "UA", nil)
	require.NoError(t, err)
	t.Cleanup(func() { pool.Exec(ctx, "DELETE FROM ngac_nodes WHERE id = $1", ua.ID) })
	otherUA, err := s.CreateNode(ctx, "test-ua-assoc-deny-target", "UA", nil)
	require.NoError(t, err)
	t.Cleanup(func() { pool.Exec(ctx, "DELETE FROM ngac_nodes WHERE id = $1", otherUA.ID) })

	_, err = s.CreateAssociation(ctx, ua.ID, otherUA.ID, []string{"read"})
	assert.ErrorIs(t, err, ngac.ErrInvalidAssociation, "target must be an OA")

	_, err = s.CreateAssociation(ctx, ua.ID, "00000000-0000-0000-0000-000000000000", []string{"read"})
	assert.ErrorIs(t, err, ngac.ErrInvalidAssociation, "unknown target")

	var rows int
	require.NoError(t, pool.QueryRow(ctx,
		"SELECT count(*) FROM ngac_associations WHERE ua_id = $1", ua.ID).Scan(&rows))
	assert.Zero(t, rows)
}

// At most one personal UA per user, enforced by the database, so two
// concurrent first shares to the same person cannot leave two of them.
func TestCreateNode_PersonalUAIsUniquePerUser(t *testing.T) {
	s, pool := setupStore(t)
	ctx := context.Background()
	user := "uniq-user-" + t.Name()
	props := map[string]string{"type": "personal_ua", "user_node_id": user}

	first, err := s.CreateNode(ctx, "User_"+user, "UA", props)
	require.NoError(t, err)
	t.Cleanup(func() { pool.Exec(ctx, "DELETE FROM ngac_nodes WHERE id = $1", first.ID) })

	dup, err := s.CreateNode(ctx, "User_"+user+"_again", "UA", props)
	assert.Error(t, err, "a second personal UA for the same user must be refused")
	if err == nil {
		pool.Exec(ctx, "DELETE FROM ngac_nodes WHERE id = $1", dup.ID)
	}
	assert.Nil(t, s.GetNode(func() string {
		if dup != nil {
			return dup.ID
		}
		return ""
	}()), "the refused node must not be in the graph")

	// Another user, and an ordinary UA carrying the same user_node_id under a
	// different type, are unaffected.
	other, err := s.CreateNode(ctx, "User_other_"+user, "UA",
		map[string]string{"type": "personal_ua", "user_node_id": user + "-2"})
	require.NoError(t, err)
	t.Cleanup(func() { pool.Exec(ctx, "DELETE FROM ngac_nodes WHERE id = $1", other.ID) })
	role, err := s.CreateNode(ctx, "role_"+user, "UA", map[string]string{"type": "role", "user_node_id": user})
	require.NoError(t, err)
	t.Cleanup(func() { pool.Exec(ctx, "DELETE FROM ngac_nodes WHERE id = $1", role.ID) })
}
