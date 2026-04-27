package ngac_test

import (
	"context"
	"fmt"
	"os"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"ngac-platform/services/policy/internal/ngac"
)

func testDBURL() string {
	if url := os.Getenv("TEST_DATABASE_URL"); url != "" {
		return url
	}
	return "postgres://ngac:ngac_secret@localhost:5432/ngac?sslmode=disable"
}

func setupStore(t *testing.T) (*ngac.Store, *pgxpool.Pool) {
	t.Helper()
	pool, err := pgxpool.New(context.Background(), testDBURL())
	if err != nil {
		t.Fatalf("connect to test DB: %v", err)
	}
	if err := pool.Ping(context.Background()); err != nil {
		t.Skipf("test DB not available: %v", err)
	}
	t.Cleanup(func() { pool.Close() })

	graph := ngac.NewGraph()
	s := ngac.NewStore(pool, graph)
	require.NoError(t, s.LoadGraph(context.Background()))
	return s, pool
}

// ---------------------------------------------------------------------------
// 6.1: TestCreateNode + TestFindNodeByName
// ---------------------------------------------------------------------------

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

func TestFindNodeByName(t *testing.T) {
	s, _ := setupStore(t)

	// PC_Global is seeded
	node := s.FindNodeByName("PC_Global", "PC")
	require.NotNil(t, node)
	assert.Equal(t, "PC_Global", node.Name)
	assert.Equal(t, "PC", node.NodeType)
}

func TestFindNodeByName_NotFound(t *testing.T) {
	s, _ := setupStore(t)
	node := s.FindNodeByName("Nonexistent", "PC")
	assert.Nil(t, node)
}

// ---------------------------------------------------------------------------
// 6.2: TestCreateAssignment + TestGetChildren
// ---------------------------------------------------------------------------

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

// ---------------------------------------------------------------------------
// 6.3: TestCreateAssociation + graph traversal
// ---------------------------------------------------------------------------

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

// ---------------------------------------------------------------------------
// 6.4: TestCheckAccess (ALLOW + DENY scenarios)
// ---------------------------------------------------------------------------

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

// ---------------------------------------------------------------------------
// 6.5: TestCheckAccess (multi-PC, inherited permissions)
// ---------------------------------------------------------------------------

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

// ---------------------------------------------------------------------------
// 6.6: TestBfsCollectAttributesAndPCs — single-pass BFS
// ---------------------------------------------------------------------------

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

// ---------------------------------------------------------------------------
// 6.7: TestFindNodeByName_IndexedLookup — O(1) index
// ---------------------------------------------------------------------------

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

// ---------------------------------------------------------------------------
// 6.8: TestCheckAccess_CrossWorkspaceShare — PC_Global via intersection
// ---------------------------------------------------------------------------

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

	// Global: PC_Global, Share_OA
	pcGlobal := &ngac.NGACNode{ID: "pc-global", Name: "PC_Global", NodeType: "PC"}
	shareOA := &ngac.NGACNode{ID: "share-oa", Name: "Share_Doc1", NodeType: "OA"}

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

	// Cross-workspace share: Doc → Share_OA → PC_Global
	require.NoError(t, g.AddAssignment(&ngac.Assignment{ID: "a6", ChildID: "doc1", ParentID: "share-oa"}))
	require.NoError(t, g.AddAssignment(&ngac.Assignment{ID: "a7", ChildID: "share-oa", ParentID: "pc-global"}))

	// Association: UA_Members_B → Share_OA [read]
	require.NoError(t, g.AddAssociation(&ngac.Association{
		ID: "assoc-share", UAID: "ua-members-b", OAID: "share-oa", Operations: []string{"read"},
	}))

	// Bob (WS_B) should be able to read Doc1 (WS_A) via PC_Global
	decision := g.CheckAccess("bob", "doc1", "read")
	assert.Equal(t, "ALLOW", decision.Decision,
		"cross-workspace sharing via PC_Global should be handled naturally by PC intersection")

	// Bob should NOT be able to write (no write in association)
	decision = g.CheckAccess("bob", "doc1", "write")
	assert.Equal(t, "DENY", decision.Decision)
}
