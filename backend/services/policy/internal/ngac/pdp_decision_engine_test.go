package ngac_test

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"

	"ngac-platform/services/policy/internal/ngac"
)

// --- Mock ShardManager for testing ---

type mockShardManager struct {
	shards map[string]ngac.GraphReader
}

func newMockShardManager() *mockShardManager {
	return &mockShardManager{shards: make(map[string]ngac.GraphReader)}
}

func (m *mockShardManager) GetGraph(_ context.Context, workspaceID string) (ngac.GraphReader, error) {
	g, ok := m.shards[workspaceID]
	if !ok {
		return nil, fmt.Errorf("shard %q not found", workspaceID)
	}
	return g, nil
}

func (m *mockShardManager) InvalidateShard(workspaceID string) {
	delete(m.shards, workspaceID)
}

func (m *mockShardManager) InvalidateAll() {
	m.shards = make(map[string]ngac.GraphReader)
}

func (m *mockShardManager) Stats() ngac.ShardStats {
	return ngac.ShardStats{ActiveShards: len(m.shards)}
}

// --- Helper: build a simple graph for a workspace ---

func buildWorkspaceGraph(workspaceID string) *ngac.Graph {
	g := ngac.NewGraph()

	pcID := "pc-" + workspaceID
	g.AddNode(&ngac.NGACNode{ID: pcID, Name: "PC_" + workspaceID, NodeType: "PC",
		Properties: map[string]string{"workspace_id": workspaceID}})

	// UA hierarchy
	uaOwners := "ua-owners-" + workspaceID
	g.AddNode(&ngac.NGACNode{ID: uaOwners, Name: "Owners_" + workspaceID, NodeType: "UA"})
	g.AddAssignment(&ngac.Assignment{ID: "a-o-" + workspaceID, ChildID: uaOwners, ParentID: pcID})

	// OA hierarchy
	oaDocs := "oa-docs-" + workspaceID
	g.AddNode(&ngac.NGACNode{ID: oaDocs, Name: "Docs_" + workspaceID, NodeType: "OA"})
	g.AddAssignment(&ngac.Assignment{ID: "a-d-" + workspaceID, ChildID: oaDocs, ParentID: pcID})

	return g
}

func setShardManager(engine ngac.DecisionEngine, sm ngac.ShardManager) {
	engine.(interface{ SetShardManager(ngac.ShardManager) }).SetShardManager(sm)
}

// --- 5a. DecisionEngine tests (Tier 1 — no DB) ---

func TestDecide_ShardRouting(t *testing.T) {
	// When workspace_id is set and shard exists, engine should use the shard graph
	globalGraph := ngac.NewGraph()
	globalGraph.AddNode(&ngac.NGACNode{ID: "pc-global", Name: "PC_Global", NodeType: "PC"})

	shardGraph := buildWorkspaceGraph("ws-alpha")
	shardGraph.AddNode(&ngac.NGACNode{ID: "user-1", Name: "alice", NodeType: "U"})
	shardGraph.AddAssignment(&ngac.Assignment{ID: "a-u1", ChildID: "user-1", ParentID: "ua-owners-ws-alpha"})
	shardGraph.AddNode(&ngac.NGACNode{ID: "obj-1", Name: "doc1", NodeType: "O"})
	shardGraph.AddAssignment(&ngac.Assignment{ID: "a-o1", ChildID: "obj-1", ParentID: "oa-docs-ws-alpha"})
	shardGraph.AddAssociation(&ngac.Association{ID: "assoc-1", UAID: "ua-owners-ws-alpha", OAID: "oa-docs-ws-alpha", Operations: []string{"read"}})

	sm := newMockShardManager()
	sm.shards["ws-alpha"] = shardGraph

	engine := ngac.NewDecisionEngine(globalGraph, nil, nil)
	setShardManager(engine, sm)

	result := engine.Decide(context.Background(), ngac.AccessRequest{
		UserNodeID:   "user-1",
		ObjectNodeID: "obj-1",
		Operation:    "read",
		WorkspaceID:  "ws-alpha",
	})

	assert.Equal(t, "ALLOW", result.Decision, "shard should grant access")
}

func TestDecide_GlobalFallback(t *testing.T) {
	// When workspace_id is set but shard doesn't exist, engine falls back to global
	globalGraph := ngac.NewGraph()
	globalGraph.AddNode(&ngac.NGACNode{ID: "pc-global", Name: "PC_Global", NodeType: "PC"})
	globalGraph.AddNode(&ngac.NGACNode{ID: "ua-all", Name: "All", NodeType: "UA"})
	globalGraph.AddAssignment(&ngac.Assignment{ID: "a1", ChildID: "ua-all", ParentID: "pc-global"})
	globalGraph.AddNode(&ngac.NGACNode{ID: "oa-pub", Name: "Public", NodeType: "OA"})
	globalGraph.AddAssignment(&ngac.Assignment{ID: "a2", ChildID: "oa-pub", ParentID: "pc-global"})
	globalGraph.AddNode(&ngac.NGACNode{ID: "user-bob", Name: "bob", NodeType: "U"})
	globalGraph.AddAssignment(&ngac.Assignment{ID: "a3", ChildID: "user-bob", ParentID: "ua-all"})
	globalGraph.AddNode(&ngac.NGACNode{ID: "obj-pub", Name: "pubdoc", NodeType: "O"})
	globalGraph.AddAssignment(&ngac.Assignment{ID: "a4", ChildID: "obj-pub", ParentID: "oa-pub"})
	globalGraph.AddAssociation(&ngac.Association{ID: "assoc-pub", UAID: "ua-all", OAID: "oa-pub", Operations: []string{"read"}})

	sm := newMockShardManager() // empty — shard miss

	engine := ngac.NewDecisionEngine(globalGraph, nil, nil)
	setShardManager(engine, sm)

	result := engine.Decide(context.Background(), ngac.AccessRequest{
		UserNodeID:   "user-bob",
		ObjectNodeID: "obj-pub",
		Operation:    "read",
		WorkspaceID:  "ws-nonexistent",
	})

	assert.Equal(t, "ALLOW", result.Decision, "should fall back to global graph")
}

func TestDecide_CrossTenantIsolation(t *testing.T) {
	// Same user, same object ID, different workspaces → different decisions
	globalGraph := ngac.NewGraph()
	globalGraph.AddNode(&ngac.NGACNode{ID: "pc-global", Name: "PC_Global", NodeType: "PC"})

	// Workspace A: user has read access
	shardA := buildWorkspaceGraph("ws-a")
	shardA.AddNode(&ngac.NGACNode{ID: "user-shared", Name: "shared-user", NodeType: "U"})
	shardA.AddAssignment(&ngac.Assignment{ID: "a-u-a", ChildID: "user-shared", ParentID: "ua-owners-ws-a"})
	shardA.AddNode(&ngac.NGACNode{ID: "obj-target", Name: "target-doc", NodeType: "O"})
	shardA.AddAssignment(&ngac.Assignment{ID: "a-o-a", ChildID: "obj-target", ParentID: "oa-docs-ws-a"})
	shardA.AddAssociation(&ngac.Association{ID: "assoc-a", UAID: "ua-owners-ws-a", OAID: "oa-docs-ws-a", Operations: []string{"read"}})

	// Workspace B: same user, same object ID — but NO association → DENY
	shardB := buildWorkspaceGraph("ws-b")
	shardB.AddNode(&ngac.NGACNode{ID: "user-shared", Name: "shared-user", NodeType: "U"})
	shardB.AddAssignment(&ngac.Assignment{ID: "a-u-b", ChildID: "user-shared", ParentID: "ua-owners-ws-b"})
	shardB.AddNode(&ngac.NGACNode{ID: "obj-target", Name: "target-doc", NodeType: "O"})
	shardB.AddAssignment(&ngac.Assignment{ID: "a-o-b", ChildID: "obj-target", ParentID: "oa-docs-ws-b"})
	// NOTE: No association in workspace B

	sm := newMockShardManager()
	sm.shards["ws-a"] = shardA
	sm.shards["ws-b"] = shardB

	engine := ngac.NewDecisionEngine(globalGraph, nil, nil)
	setShardManager(engine, sm)

	resultA := engine.Decide(context.Background(), ngac.AccessRequest{
		UserNodeID: "user-shared", ObjectNodeID: "obj-target", Operation: "read", WorkspaceID: "ws-a",
	})
	resultB := engine.Decide(context.Background(), ngac.AccessRequest{
		UserNodeID: "user-shared", ObjectNodeID: "obj-target", Operation: "read", WorkspaceID: "ws-b",
	})

	assert.Equal(t, "ALLOW", resultA.Decision, "workspace A should ALLOW")
	assert.Equal(t, "DENY", resultB.Decision, "workspace B should DENY (no association)")
}

func TestDecide_NoWorkspaceID_UsesGlobalGraph(t *testing.T) {
	// When workspace_id is empty, engine must use global graph (backward compat)
	globalGraph := ngac.NewGraph()
	globalGraph.AddNode(&ngac.NGACNode{ID: "pc-main", Name: "PC_Main", NodeType: "PC"})
	globalGraph.AddNode(&ngac.NGACNode{ID: "ua-all", Name: "All", NodeType: "UA"})
	globalGraph.AddAssignment(&ngac.Assignment{ID: "a1", ChildID: "ua-all", ParentID: "pc-main"})
	globalGraph.AddNode(&ngac.NGACNode{ID: "oa-all", Name: "AllDocs", NodeType: "OA"})
	globalGraph.AddAssignment(&ngac.Assignment{ID: "a2", ChildID: "oa-all", ParentID: "pc-main"})
	globalGraph.AddNode(&ngac.NGACNode{ID: "user-x", Name: "x", NodeType: "U"})
	globalGraph.AddAssignment(&ngac.Assignment{ID: "a3", ChildID: "user-x", ParentID: "ua-all"})
	globalGraph.AddNode(&ngac.NGACNode{ID: "obj-x", Name: "docx", NodeType: "O"})
	globalGraph.AddAssignment(&ngac.Assignment{ID: "a4", ChildID: "obj-x", ParentID: "oa-all"})
	globalGraph.AddAssociation(&ngac.Association{ID: "assoc-g", UAID: "ua-all", OAID: "oa-all", Operations: []string{"read"}})

	sm := newMockShardManager()
	engine := ngac.NewDecisionEngine(globalGraph, nil, nil)
	setShardManager(engine, sm)

	result := engine.Decide(context.Background(), ngac.AccessRequest{
		UserNodeID: "user-x", ObjectNodeID: "obj-x", Operation: "read",
		// WorkspaceID intentionally empty
	})

	assert.Equal(t, "ALLOW", result.Decision, "empty workspace should use global graph")
}

// --- 5b. DecisionCache tests (Tier 1 — no DB) ---

func TestCacheKey_WithWorkspaceID(t *testing.T) {
	req := ngac.AccessRequest{
		UserNodeID: "u1", ObjectNodeID: "o1", Operation: "read", WorkspaceID: "ws-123",
	}
	key := ngac.ExportCacheKey(req)
	assert.Contains(t, key, "ws-123", "cache key should contain workspace_id")
	assert.Equal(t, "ngac:access:ws-123:u1:o1:read", key)
}

func TestCacheKey_WithoutWorkspaceID(t *testing.T) {
	req := ngac.AccessRequest{
		UserNodeID: "u1", ObjectNodeID: "o1", Operation: "read",
	}
	key := ngac.ExportCacheKey(req)
	assert.Equal(t, "ngac:access:u1:o1:read", key, "backward compat: no workspace prefix")
}

func TestCacheKey_DifferentWorkspaces_NoCrossHit(t *testing.T) {
	reqA := ngac.AccessRequest{
		UserNodeID: "u1", ObjectNodeID: "o1", Operation: "read", WorkspaceID: "ws-A",
	}
	reqB := ngac.AccessRequest{
		UserNodeID: "u1", ObjectNodeID: "o1", Operation: "read", WorkspaceID: "ws-B",
	}
	keyA := ngac.ExportCacheKey(reqA)
	keyB := ngac.ExportCacheKey(reqB)

	assert.NotEqual(t, keyA, keyB,
		"same (user,obj,op) with different workspace_id MUST produce different cache keys")
}

func TestVersionScope_WorkspaceIsolation(t *testing.T) {
	reqA := ngac.AccessRequest{WorkspaceID: "ws-A"}
	reqB := ngac.AccessRequest{WorkspaceID: "ws-B"}
	reqGlobal := ngac.AccessRequest{}

	scopeA := ngac.ExportVersionScope(reqA)
	scopeB := ngac.ExportVersionScope(reqB)
	scopeGlobal := ngac.ExportVersionScope(reqGlobal)

	assert.Equal(t, "ws:ws-A", scopeA)
	assert.Equal(t, "ws:ws-B", scopeB)
	assert.Equal(t, "global", scopeGlobal)
	assert.NotEqual(t, scopeA, scopeB, "different workspace version scopes must differ")
}

// --- 5d. Cross-tenant integration (Tier 1 — no DB) ---

func TestCrossTenant_UserInBothWorkspaces(t *testing.T) {
	// A user exists in both workspaces but with different permissions
	globalGraph := ngac.NewGraph()
	globalGraph.AddNode(&ngac.NGACNode{ID: "pc-global", Name: "PC_Global", NodeType: "PC"})

	// WS-1: user is owner → ALLOW read+write
	ws1 := buildWorkspaceGraph("ws-1")
	ws1.AddNode(&ngac.NGACNode{ID: "user-multi", Name: "multi-user", NodeType: "U"})
	ws1.AddAssignment(&ngac.Assignment{ID: "a-u-ws1", ChildID: "user-multi", ParentID: "ua-owners-ws-1"})
	ws1.AddNode(&ngac.NGACNode{ID: "obj-doc", Name: "shared-doc", NodeType: "O"})
	ws1.AddAssignment(&ngac.Assignment{ID: "a-d-ws1", ChildID: "obj-doc", ParentID: "oa-docs-ws-1"})
	ws1.AddAssociation(&ngac.Association{ID: "assoc-ws1", UAID: "ua-owners-ws-1", OAID: "oa-docs-ws-1", Operations: []string{"read", "write"}})

	// WS-2: user is member (read-only UA)
	ws2 := buildWorkspaceGraph("ws-2")
	ws2.AddNode(&ngac.NGACNode{ID: "ua-members-ws-2", Name: "Members_ws-2", NodeType: "UA"})
	ws2.AddAssignment(&ngac.Assignment{ID: "a-m-ws2", ChildID: "ua-members-ws-2", ParentID: "pc-ws-2"})
	ws2.AddNode(&ngac.NGACNode{ID: "user-multi", Name: "multi-user", NodeType: "U"})
	ws2.AddAssignment(&ngac.Assignment{ID: "a-u-ws2", ChildID: "user-multi", ParentID: "ua-members-ws-2"})
	ws2.AddNode(&ngac.NGACNode{ID: "obj-doc", Name: "shared-doc", NodeType: "O"})
	ws2.AddAssignment(&ngac.Assignment{ID: "a-d-ws2", ChildID: "obj-doc", ParentID: "oa-docs-ws-2"})
	ws2.AddAssociation(&ngac.Association{ID: "assoc-ws2", UAID: "ua-members-ws-2", OAID: "oa-docs-ws-2", Operations: []string{"read"}})

	sm := newMockShardManager()
	sm.shards["ws-1"] = ws1
	sm.shards["ws-2"] = ws2

	engine := ngac.NewDecisionEngine(globalGraph, nil, nil)
	setShardManager(engine, sm)

	// WS-1: write → ALLOW (owner)
	r1 := engine.Decide(context.Background(), ngac.AccessRequest{
		UserNodeID: "user-multi", ObjectNodeID: "obj-doc", Operation: "write", WorkspaceID: "ws-1",
	})
	assert.Equal(t, "ALLOW", r1.Decision, "ws-1 owner should write")

	// WS-2: write → DENY (member, read-only)
	r2 := engine.Decide(context.Background(), ngac.AccessRequest{
		UserNodeID: "user-multi", ObjectNodeID: "obj-doc", Operation: "write", WorkspaceID: "ws-2",
	})
	assert.Equal(t, "DENY", r2.Decision, "ws-2 member cannot write")

	// WS-2: read → ALLOW (member has read)
	r3 := engine.Decide(context.Background(), ngac.AccessRequest{
		UserNodeID: "user-multi", ObjectNodeID: "obj-doc", Operation: "read", WorkspaceID: "ws-2",
	})
	assert.Equal(t, "ALLOW", r3.Decision, "ws-2 member can read")
}

func TestCrossTenant_CacheKeyIsolation(t *testing.T) {
	// Verify that cache keys for identical (user,obj,op) across workspaces never collide
	cases := []struct {
		name        string
		workspaceID string
	}{
		{"workspace-A", "ws-A"},
		{"workspace-B", "ws-B"},
		{"workspace-C", "ws-C"},
		{"empty-workspace", ""},
	}

	keys := make(map[string]string) // key → workspace
	for _, tc := range cases {
		req := ngac.AccessRequest{
			UserNodeID: "user-1", ObjectNodeID: "obj-1", Operation: "read", WorkspaceID: tc.workspaceID,
		}
		key := ngac.ExportCacheKey(req)
		if existing, ok := keys[key]; ok {
			t.Fatalf("cache key collision: workspace %q and %q produce same key %q",
				tc.name, existing, key)
		}
		keys[key] = tc.name
	}
}

func TestCrossTenant_VersionScopeIsolation(t *testing.T) {
	// Verify version scopes are unique per workspace
	workspaces := []string{"ws-1", "ws-2", "ws-3", ""}
	scopes := make(map[string]string)
	for _, ws := range workspaces {
		req := ngac.AccessRequest{WorkspaceID: ws}
		scope := ngac.ExportVersionScope(req)
		if existing, ok := scopes[scope]; ok {
			t.Fatalf("version scope collision: workspace %q and %q produce same scope %q",
				ws, existing, scope)
		}
		scopes[scope] = ws
	}
}

func TestCrossTenant_ShardInvalidationIsolation(t *testing.T) {
	// Invalidating shard for ws-A should not affect ws-B
	sm := newMockShardManager()
	sm.shards["ws-A"] = buildWorkspaceGraph("ws-A")
	sm.shards["ws-B"] = buildWorkspaceGraph("ws-B")

	assert.Equal(t, 2, sm.Stats().ActiveShards)

	sm.InvalidateShard("ws-A")

	assert.Equal(t, 1, sm.Stats().ActiveShards)
	_, errA := sm.GetGraph(context.Background(), "ws-A")
	assert.Error(t, errA, "ws-A should be invalidated")
	_, errB := sm.GetGraph(context.Background(), "ws-B")
	assert.NoError(t, errB, "ws-B should still be available")
}
