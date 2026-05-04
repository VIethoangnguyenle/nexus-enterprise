# Design — Shard Integrity Fixes

## Overview

This change fixes 10 issues where components still assume a single global graph after the shard architecture was introduced. Fixes are grouped into 4 workstreams: Cache Isolation, Engine Correctness, Invalidation Coordination, and Test Coverage.

## Workstream 1: Cache Isolation

### 1.1 Cache key includes workspace_id

**File:** `decision_cache.go`

```go
// Before
func cacheKey(req AccessRequest) string {
    return fmt.Sprintf("ngac:access:%s:%s:%s", req.UserNodeID, req.ObjectNodeID, req.Operation)
}

// After
func cacheKey(req AccessRequest) string {
    if req.WorkspaceID != "" {
        return fmt.Sprintf("ngac:access:%s:%s:%s:%s",
            req.WorkspaceID, req.UserNodeID, req.ObjectNodeID, req.Operation)
    }
    return fmt.Sprintf("ngac:access:%s:%s:%s",
        req.UserNodeID, req.ObjectNodeID, req.Operation)
}
```

Backward compatible — requests without workspace_id use existing key format.

### 1.2 Materialized cache workspace-aware

**File:** `materialized.go`

Add `workspace_id` parameter to `Store()` and `Lookup()`. DB schema change:

```sql
ALTER TABLE ngac_materialized_access ADD COLUMN workspace_id TEXT DEFAULT '';
DROP INDEX IF EXISTS ngac_materialized_access_pkey;
ALTER TABLE ngac_materialized_access
  ADD CONSTRAINT ngac_materialized_access_pkey
  UNIQUE (workspace_id, user_node_id, object_node_id, operation);
```

### 1.3 Scope cache key includes workspace_id

**File:** `read_server.go`

```go
// Before
cacheKey := fmt.Sprintf("scopes:%s:%s", req.UserNodeId, req.Operation)

// After — include workspace_id when available
wsPrefix := ""
if req.WorkspaceId != nil { wsPrefix = *req.WorkspaceId + ":" }
cacheKey := fmt.Sprintf("scopes:%s%s:%s", wsPrefix, req.UserNodeId, req.Operation)
```

### 1.4 CacheInvalidator workspace-aware patterns

**File:** `cache_invalidator.go`

When invalidating, also SCAN workspace-prefixed keys:
```go
// Additional pattern: ngac:access:{workspace_id}:{user}:*
// Additional pattern: scopes:{workspace_id}:{user}:*
```

---

## Workstream 2: Engine Correctness

### 2.1 Prohibition uses resolved graph

**File:** `decision_engine.go`

```go
// Before: checkProhibitions always uses e.graph (global)
func (e *decisionEngine) checkProhibitions(ctx context.Context, req AccessRequest) (bool, string, string) {
    ancestors := e.graph.GetAncestors(req.UserNodeID)  // ← GLOBAL
    objAncestors := e.graph.GetAncestors(req.ObjectNodeID) // ← GLOBAL
}

// After: accept resolved graph as parameter
func (e *decisionEngine) checkProhibitions(ctx context.Context, req AccessRequest, graph GraphReader) (bool, string, string) {
    ancestors := graph.GetAncestors(req.UserNodeID)    // ← RESOLVED (shard or global)
    objAncestors := graph.GetAncestors(req.ObjectNodeID)
}
```

Update `Decide()` to pass resolved graph:
```go
func (e *decisionEngine) Decide(ctx context.Context, req AccessRequest) *AccessDecision {
    graph := e.resolveGraph(ctx, req)
    decision := graph.CheckAccess(...)
    // ...
    if denied, name, subj := e.checkProhibitions(ctx, req, graph); denied { ... }
}
```

### 2.2 CreateNode adds invalidation + event

**File:** `write_server.go`

```go
func (s *WriteServer) CreateNode(ctx context.Context, req *pb.CreateNodeRequest) (*pb.NGACNode, error) {
    // ... existing PC guard + store.CreateNode ...
    
    // NEW: Invalidation (consistent with DeleteNode, CreateAssignment, etc.)
    s.invalidation.InvalidateForNodes(ctx, node.Id)
    s.invalidateShards(node.Id)
    s.publishEvent("create_node", []string{node.Id})
    
    return nodeToProto(node), nil
}
```

---

## Workstream 3: Invalidation Coordination

### 3.1 Per-workspace versioning

**File:** `invalidation.go`

```go
func (c *InvalidationCoordinator) InvalidateForNodes(ctx context.Context, workspaceID string, nodeIDs ...string) {
    if workspaceID != "" {
        // Per-workspace version bump — only this tenant's L2 goes stale
        c.version.Increment(ctx, fmt.Sprintf("ws:%s", workspaceID))
    } else {
        // Fallback: global bump (emergency / unknown workspace)
        c.version.Increment(ctx, "global")
    }
    // ... rest unchanged
}
```

Decision: `InvalidateForNodes` gains a `workspaceID` parameter. Callers in `WriteServer` pass the resolved workspace_id from `invalidateShards()`.

### 3.2 DecisionCache uses per-workspace version

**File:** `decision_cache.go`

```go
func (c *layeredCache) Set(ctx context.Context, req AccessRequest, decision *AccessDecision) {
    scope := "global"
    if req.WorkspaceID != "" {
        scope = fmt.Sprintf("ws:%s", req.WorkspaceID)
    }
    currentVersion, err := c.version.GetVersion(ctx, scope)
    // ...
}
```

### 3.3 GraphMutatedEvent workspace_id

**File:** `producer.go`

```go
type GraphMutatedEvent struct {
    MutationType string   `json:"mutation_type"`
    NodeIDs      []string `json:"node_ids"`
    WorkspaceID  string   `json:"workspace_id,omitempty"` // NEW
    ChildType    string   `json:"child_type,omitempty"`
    ParentType   string   `json:"parent_type,omitempty"`
    Timestamp    int64    `json:"timestamp"`
}
```

### 3.4 Wire ShardManager in both entry points

**Files:** `cmd/main.go`, `cmd/policy-read/main.go`

```go
shardManager := ngac.NewShardManager(pool, ngac.ShardManagerConfig{})
decisionEngine := ngac.NewDecisionEngine(store.GetGraph(), cte, prohibitionStore)
decisionEngine.(*ngac.DecisionEngine).SetShardManager(shardManager) // type assert needed
writeServer.SetShardManager(shardManager)
```

---

## Workstream 4: Test Coverage

### 4.1 decision_engine_test.go (Tier 1 — no DB)

Tests use manually-constructed Graph objects to verify:

| Test | Verifies |
|---|---|
| TestDecide_ShardRouting | resolveGraph returns shard when workspace_id set |
| TestDecide_GlobalFallback | resolveGraph falls back to global on shard miss |
| TestDecide_ProhibitionUsesResolvedGraph | C2 fix — prohibition evaluated on same graph as BFS |
| TestDecide_CrossTenantIsolation | Same (user,obj,op), shard A→ALLOW, shard B→DENY |
| TestDecide_ProhibitionOnShardNode | Prohibition on shard-only node triggers DENY |

### 4.2 decision_cache_test.go (Tier 1 — no DB)

Uses mock Redis (miniredis) or interface mock:

| Test | Verifies |
|---|---|
| TestCacheKey_WithWorkspaceID | Key includes workspace_id prefix |
| TestCacheKey_WithoutWorkspaceID | Backward compat — no workspace prefix |
| TestCacheKey_DifferentWorkspaces_NoCrossHit | ws-A set → ws-B get → MISS |
| TestScopeCacheKey_WorkspaceIsolation | scopes:ws-A:user ≠ scopes:ws-B:user |

### 4.3 invalidation_coordination_test.go (Tier 2 — mock deps)

| Test | Verifies |
|---|---|
| TestInvalidation_PerWorkspaceVersion | Mutation ws-A → only ws-A version bumped |
| TestInvalidation_GlobalFallback | No workspace → global version bumped |
| TestInvalidation_ShardInvalidated | After mutation → shard removed from ShardManager |
| TestCreateNode_TriggersInvalidation | M5 fix — CreateNode calls invalidation |

### 4.4 cross_tenant_test.go (Tier 1 — no DB)

End-to-end scenarios with 2 manually-built shard graphs:

| Test | Verifies |
|---|---|
| TestCrossTenant_UserInBothWorkspaces | Same user, workspace A→ALLOW, workspace B→DENY |
| TestCrossTenant_CacheIsolation | Cached result from ws-A does NOT serve ws-B |
| TestCrossTenant_ProhibitionIsolation | Prohibition in ws-A does NOT affect ws-B |

---

## Migration

### Database schema change

```sql
-- Add workspace_id to materialized cache for per-workspace isolation
ALTER TABLE ngac_materialized_access ADD COLUMN IF NOT EXISTS workspace_id TEXT DEFAULT '';

-- Recreate unique constraint with workspace_id
ALTER TABLE ngac_materialized_access DROP CONSTRAINT IF EXISTS ngac_materialized_access_user_node_id_object_node_id_operat_key;
ALTER TABLE ngac_materialized_access ADD CONSTRAINT ngac_materialized_access_ws_key
  UNIQUE (workspace_id, user_node_id, object_node_id, operation);

-- Seed per-workspace version rows (existing tenants)
INSERT INTO ngac_graph_version (scope, version, updated_at)
SELECT CONCAT('ws:', properties->>'workspace_id'), 1, NOW()
FROM ngac_nodes
WHERE node_type = 'PC' AND properties->>'workspace_id' IS NOT NULL
ON CONFLICT (scope) DO NOTHING;
```

### Backward compatibility

All changes are backward compatible:
- Cache keys without workspace_id use existing format
- Version scope "global" still works as fallback
- GraphMutatedEvent.workspace_id is `omitempty`
- No proto changes required (workspace_id already in CheckAccessRequest)

## Risks

| Risk | Mitigation |
|---|---|
| Materialized cache schema migration on live data | Add column with DEFAULT, non-blocking |
| CacheInvalidator SCAN patterns now 2x (global + workspace-prefixed) | Only additional patterns when workspace_id known |
| InvalidateForNodes signature change | Only internal, not exposed via proto |
