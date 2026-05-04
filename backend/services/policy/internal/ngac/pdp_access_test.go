package ngac_test

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"ngac-platform/services/policy/internal/ngac"
)

// --- PDP: Access decision tests ---

func TestCheckAccess_Allow(t *testing.T) {
	g := ngac.NewGraph()

	// Build: PC -> UA -> U, PC -> OA -> O, UA --[read]--> OA
	pc := &ngac.NGACNode{ID: "pc1", Name: "PC1", NodeType: "PC"}
	ua := &ngac.NGACNode{ID: "ua1", Name: "UA1", NodeType: "UA"}
	u := &ngac.NGACNode{ID: "u1", Name: "User1", NodeType: "U"}
	oa := &ngac.NGACNode{ID: "oa1", Name: "OA1", NodeType: "OA"}
	o := &ngac.NGACNode{ID: "o1", Name: "Obj1", NodeType: "O"}

	g.AddNode(pc)
	g.AddNode(ua)
	g.AddNode(u)
	g.AddNode(oa)
	g.AddNode(o)

	require.NoError(t, g.AddAssignment(&ngac.Assignment{ID: "a1", ChildID: "ua1", ParentID: "pc1"}))
	require.NoError(t, g.AddAssignment(&ngac.Assignment{ID: "a2", ChildID: "u1", ParentID: "ua1"}))
	require.NoError(t, g.AddAssignment(&ngac.Assignment{ID: "a3", ChildID: "oa1", ParentID: "pc1"}))
	require.NoError(t, g.AddAssignment(&ngac.Assignment{ID: "a4", ChildID: "o1", ParentID: "oa1"}))
	require.NoError(t, g.AddAssociation(&ngac.Association{ID: "assoc1", UAID: "ua1", OAID: "oa1", Operations: []string{"read", "write"}}))

	decision := g.CheckAccess("u1", "o1", "read")
	assert.Equal(t, "ALLOW", decision.Decision)
}

func TestCheckAccess_DenyNoAssociation(t *testing.T) {
	g := ngac.NewGraph()

	pc := &ngac.NGACNode{ID: "pc1", Name: "PC1", NodeType: "PC"}
	ua := &ngac.NGACNode{ID: "ua1", Name: "UA1", NodeType: "UA"}
	u := &ngac.NGACNode{ID: "u1", Name: "User1", NodeType: "U"}
	oa := &ngac.NGACNode{ID: "oa1", Name: "OA1", NodeType: "OA"}
	o := &ngac.NGACNode{ID: "o1", Name: "Obj1", NodeType: "O"}

	g.AddNode(pc)
	g.AddNode(ua)
	g.AddNode(u)
	g.AddNode(oa)
	g.AddNode(o)

	require.NoError(t, g.AddAssignment(&ngac.Assignment{ID: "a1", ChildID: "ua1", ParentID: "pc1"}))
	require.NoError(t, g.AddAssignment(&ngac.Assignment{ID: "a2", ChildID: "u1", ParentID: "ua1"}))
	require.NoError(t, g.AddAssignment(&ngac.Assignment{ID: "a3", ChildID: "oa1", ParentID: "pc1"}))
	require.NoError(t, g.AddAssignment(&ngac.Assignment{ID: "a4", ChildID: "o1", ParentID: "oa1"}))
	// No association → DENY

	decision := g.CheckAccess("u1", "o1", "read")
	assert.Equal(t, "DENY", decision.Decision)
}

func TestCheckAccess_DenyWrongOperation(t *testing.T) {
	g := ngac.NewGraph()

	pc := &ngac.NGACNode{ID: "pc1", Name: "PC1", NodeType: "PC"}
	ua := &ngac.NGACNode{ID: "ua1", Name: "UA1", NodeType: "UA"}
	u := &ngac.NGACNode{ID: "u1", Name: "User1", NodeType: "U"}
	oa := &ngac.NGACNode{ID: "oa1", Name: "OA1", NodeType: "OA"}
	o := &ngac.NGACNode{ID: "o1", Name: "Obj1", NodeType: "O"}

	g.AddNode(pc)
	g.AddNode(ua)
	g.AddNode(u)
	g.AddNode(oa)
	g.AddNode(o)

	require.NoError(t, g.AddAssignment(&ngac.Assignment{ID: "a1", ChildID: "ua1", ParentID: "pc1"}))
	require.NoError(t, g.AddAssignment(&ngac.Assignment{ID: "a2", ChildID: "u1", ParentID: "ua1"}))
	require.NoError(t, g.AddAssignment(&ngac.Assignment{ID: "a3", ChildID: "oa1", ParentID: "pc1"}))
	require.NoError(t, g.AddAssignment(&ngac.Assignment{ID: "a4", ChildID: "o1", ParentID: "oa1"}))
	require.NoError(t, g.AddAssociation(&ngac.Association{ID: "assoc1", UAID: "ua1", OAID: "oa1", Operations: []string{"read"}}))

	// Has read but not write
	decision := g.CheckAccess("u1", "o1", "write")
	assert.Equal(t, "DENY", decision.Decision)
}

func TestCheckAccess_NodeNotFound(t *testing.T) {
	g := ngac.NewGraph()
	decision := g.CheckAccess("nonexistent", "also-nonexistent", "read")
	assert.Equal(t, "DENY", decision.Decision)
}

func TestCheckAccess_MultiPC_InheritedPermissions(t *testing.T) {
	g := ngac.NewGraph()

	// Two PCs
	pc1 := &ngac.NGACNode{ID: "pc1", Name: "PC1", NodeType: "PC"}
	pc2 := &ngac.NGACNode{ID: "pc2", Name: "PC2", NodeType: "PC"}
	// UAs and OAs in different PCs
	ua1 := &ngac.NGACNode{ID: "ua1", Name: "UA1", NodeType: "UA"}
	ua2 := &ngac.NGACNode{ID: "ua2", Name: "UA2", NodeType: "UA"}
	oa1 := &ngac.NGACNode{ID: "oa1", Name: "OA1", NodeType: "OA"}
	u := &ngac.NGACNode{ID: "u1", Name: "User1", NodeType: "U"}
	o := &ngac.NGACNode{ID: "o1", Name: "Obj1", NodeType: "O"}

	for _, n := range []*ngac.NGACNode{pc1, pc2, ua1, ua2, oa1, u, o} {
		g.AddNode(n)
	}

	// User in UA1 (under PC1) and UA2 (under PC2)
	require.NoError(t, g.AddAssignment(&ngac.Assignment{ID: "a1", ChildID: "ua1", ParentID: "pc1"}))
	require.NoError(t, g.AddAssignment(&ngac.Assignment{ID: "a2", ChildID: "ua2", ParentID: "pc2"}))
	require.NoError(t, g.AddAssignment(&ngac.Assignment{ID: "a3", ChildID: "u1", ParentID: "ua1"}))
	require.NoError(t, g.AddAssignment(&ngac.Assignment{ID: "a4", ChildID: "u1", ParentID: "ua2"}))
	// Object OA1 under PC1
	require.NoError(t, g.AddAssignment(&ngac.Assignment{ID: "a5", ChildID: "oa1", ParentID: "pc1"}))
	require.NoError(t, g.AddAssignment(&ngac.Assignment{ID: "a6", ChildID: "o1", ParentID: "oa1"}))
	// Association: UA1 --[read]--> OA1 (same PC1)
	require.NoError(t, g.AddAssociation(&ngac.Association{ID: "assoc1", UAID: "ua1", OAID: "oa1", Operations: []string{"read"}}))

	// Access through UA1 → OA1 (both under PC1) should be ALLOW
	decision := g.CheckAccess("u1", "o1", "read")
	assert.Equal(t, "ALLOW", decision.Decision)

	// UA2 has no association to OA1, so write should be DENY
	decision = g.CheckAccess("u1", "o1", "write")
	assert.Equal(t, "DENY", decision.Decision)
}

func TestBfsCollectAttributesAndPCs(t *testing.T) {
	g := ngac.NewGraph()

	pc := &ngac.NGACNode{ID: "pc1", Name: "PC1", NodeType: "PC"}
	ua1 := &ngac.NGACNode{ID: "ua1", Name: "UA1", NodeType: "UA"}
	ua2 := &ngac.NGACNode{ID: "ua2", Name: "UA2", NodeType: "UA"}
	u := &ngac.NGACNode{ID: "u1", Name: "User1", NodeType: "U"}

	for _, n := range []*ngac.NGACNode{pc, ua1, ua2, u} {
		g.AddNode(n)
	}

	// u1 → ua1 → ua2 → pc1
	require.NoError(t, g.AddAssignment(&ngac.Assignment{ID: "a1", ChildID: "u1", ParentID: "ua1"}))
	require.NoError(t, g.AddAssignment(&ngac.Assignment{ID: "a2", ChildID: "ua1", ParentID: "ua2"}))
	require.NoError(t, g.AddAssignment(&ngac.Assignment{ID: "a3", ChildID: "ua2", ParentID: "pc1"}))

	// BFS from user should find both UAs and the PC in one pass
	decision := g.CheckAccess("u1", "nonexistent", "read")
	assert.Equal(t, "DENY", decision.Decision, "should deny when object doesn't exist")

	// Verify that graph traversal through chained UAs works
	ancestors := g.GetAncestors("u1")
	assert.Contains(t, ancestors, "ua1")
	assert.Contains(t, ancestors, "ua2")
	assert.Contains(t, ancestors, "pc1")
}

func TestFindNodeByName_IndexedLookup(t *testing.T) {
	g := ngac.NewGraph()

	// Add many nodes
	for i := 0; i < 100; i++ {
		g.AddNode(&ngac.NGACNode{
			ID:       fmt.Sprintf("ua-%d", i),
			Name:     fmt.Sprintf("UA_%d", i),
			NodeType: "UA",
		})
	}
	g.AddNode(&ngac.NGACNode{ID: "target-pc", Name: "TargetPC", NodeType: "PC"})

	// Lookup should be O(1) via index, not O(N) scan
	found := g.FindNodeByName("TargetPC", "PC")
	require.NotNil(t, found)
	assert.Equal(t, "target-pc", found.ID)

	// Same name but different type should not match
	notFound := g.FindNodeByName("TargetPC", "UA")
	assert.Nil(t, notFound)

	// After removal, index should be cleaned up
	g.RemoveNode("target-pc")
	removed := g.FindNodeByName("TargetPC", "PC")
	assert.Nil(t, removed)
}

func TestCheckAccess_CrossWorkspaceShare(t *testing.T) {
	g := ngac.NewGraph()

	// Workspace A: PC_WS_A, OA_Docs_A, Doc_Node
	pcA := &ngac.NGACNode{ID: "pc-a", Name: "PC_WS_A", NodeType: "PC"}
	oaDocs := &ngac.NGACNode{ID: "oa-docs-a", Name: "OA_Docs_A", NodeType: "OA"}
	doc := &ngac.NGACNode{ID: "doc1", Name: "Document1", NodeType: "O"}

	// Workspace B: PC_WS_B, UA_Members_B, User_Bob
	pcB := &ngac.NGACNode{ID: "pc-b", Name: "PC_WS_B", NodeType: "PC"}
	uaMembers := &ngac.NGACNode{ID: "ua-members-b", Name: "UA_Members_B", NodeType: "UA"}
	bob := &ngac.NGACNode{ID: "bob", Name: "Bob", NodeType: "U"}

	// Global: PC_Global, Share_OA (standalone — NOT assigned to doc1)
	pcGlobal := &ngac.NGACNode{ID: "pc-global", Name: "PC_Global", NodeType: "PC"}
	shareOA := &ngac.NGACNode{
		ID: "share-oa", Name: "Share_Doc1", NodeType: "OA",
		Properties: map[string]string{"ref_oa_id": "doc1"},
	}

	for _, n := range []*ngac.NGACNode{pcA, oaDocs, doc, pcB, uaMembers, bob, pcGlobal, shareOA} {
		g.AddNode(n)
	}

	// WS_A structure
	require.NoError(t, g.AddAssignment(&ngac.Assignment{ID: "a1", ChildID: "oa-docs-a", ParentID: "pc-a"}))
	require.NoError(t, g.AddAssignment(&ngac.Assignment{ID: "a2", ChildID: "doc1", ParentID: "oa-docs-a"}))

	// WS_B structure
	require.NoError(t, g.AddAssignment(&ngac.Assignment{ID: "a3", ChildID: "ua-members-b", ParentID: "pc-b"}))
	require.NoError(t, g.AddAssignment(&ngac.Assignment{ID: "a4", ChildID: "bob", ParentID: "ua-members-b"}))

	// Bob in WS_B also has global scope
	require.NoError(t, g.AddAssignment(&ngac.Assignment{ID: "a5", ChildID: "ua-members-b", ParentID: "pc-global"}))

	// Cross-workspace share: Share_OA → PC_Global (standalone, no link to doc1)
	require.NoError(t, g.AddAssignment(&ngac.Assignment{ID: "a7", ChildID: "share-oa", ParentID: "pc-global"}))

	// Association: UA_Members_B → Share_OA [read]
	require.NoError(t, g.AddAssociation(&ngac.Association{
		ID: "assoc-share", UAID: "ua-members-b", OAID: "share-oa", Operations: []string{"read"},
	}))

	// Bob (WS_B) can read via Share_OA (PC_Global only — ALL-PC satisfied)
	decision := g.CheckAccess("bob", "share-oa", "read")
	assert.Equal(t, "ALLOW", decision.Decision,
		"cross-workspace sharing via standalone Share_OA → PC_Global")

	// Bob CANNOT access the original doc1 directly (PC_A required)
	decision = g.CheckAccess("bob", "doc1", "read")
	assert.Equal(t, "DENY", decision.Decision,
		"bob cannot access original doc1 — requires PC_A")

	// Bob should NOT be able to write Share_OA (no write in association)
	decision = g.CheckAccess("bob", "share-oa", "write")
	assert.Equal(t, "DENY", decision.Decision)
}
