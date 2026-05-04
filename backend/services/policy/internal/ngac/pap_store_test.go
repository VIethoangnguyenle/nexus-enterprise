package ngac_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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
