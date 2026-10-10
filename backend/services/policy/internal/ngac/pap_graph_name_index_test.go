package ngac_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"ngac-platform/services/policy/internal/ngac"
)

// The name index keeps one node per name and type. Removing a node must drop
// the entry only if it still points at that node: a rollback of one provisioning
// run must not hide the node a concurrent run kept under the same name.
func TestRemoveNode_KeepsTheIndexEntryOfAnotherNodeWithTheSameName(t *testing.T) {
	g := ngac.NewGraph()
	g.AddNode(&ngac.NGACNode{ID: "a", Name: "TenantMember_t1", NodeType: "UA"})
	g.AddNode(&ngac.NGACNode{ID: "b", Name: "TenantMember_t1", NodeType: "UA"}) // b now owns the name

	g.RemoveNode("a")

	n := g.FindNodeByName("TenantMember_t1", "UA")
	require.NotNil(t, n, "removing a must not hide b")
	assert.Equal(t, "b", n.ID)
}

func TestRemoveNode_DropsItsOwnIndexEntry(t *testing.T) {
	g := ngac.NewGraph()
	g.AddNode(&ngac.NGACNode{ID: "a", Name: "X", NodeType: "OA"})

	g.RemoveNode("a")

	assert.Nil(t, g.FindNodeByName("X", "OA"))
}

// Deny side: the same name under another node type is a different entry.
func TestRemoveNode_LeavesTheSameNameOfAnotherTypeAlone(t *testing.T) {
	g := ngac.NewGraph()
	g.AddNode(&ngac.NGACNode{ID: "ua", Name: "X", NodeType: "UA"})
	g.AddNode(&ngac.NGACNode{ID: "oa", Name: "X", NodeType: "OA"})

	g.RemoveNode("ua")

	require.NotNil(t, g.FindNodeByName("X", "OA"))
	assert.Nil(t, g.FindNodeByName("X", "UA"))
}
