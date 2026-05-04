# Policy Code Cleanup — Design

## Overview

Pure internal refactor — no API changes, no behavioral changes. Three pillars:
1. Constants extraction
2. Control flow flattening  
3. Data structure optimization (LRU)

---

## 1. Constants Extraction

### 1.1 Decision Constants — `models.go`

```go
// Decision outcomes — used across PDP, cache, gRPC layers
const (
    DecisionAllow = "ALLOW"
    DecisionDeny  = "DENY"
)
```

**Files affected**: `pdp_decision_engine.go`, `pdp_access.go`, `pdp_decision_cache.go`, `read_server.go`, `write_server.go`

### 1.2 Scope Helpers — `models.go`

```go
const ScopeGlobal = "global"

func WorkspaceScope(wsID string) string {
    return "ws:" + wsID
}
```

**Files affected**: `epp_invalidation.go`, `pdp_decision_cache.go`

### 1.3 Cache Key Prefix — `pdp_decision_cache.go`

```go
const (
    cacheKeyPrefix = "ngac:access:"
    scopeKeyPrefix = "scopes:"
)
```

**Files affected**: `pdp_decision_cache.go`, `epp_cache_invalidator.go`, `read_server.go`

### 1.4 Fix Raw Node Type Strings — `write_server.go`

Replace `"UA"` → `ngac.NodeTypeUserAttribute`, `"U"` → `ngac.NodeTypeUser` in `resolveProhibitionAffectedNodes`.

---

## 2. Control Flow Flattening — `pdp_decision_engine.go`

### Current (6 levels deep):

```
Decide()
  if DENY && NodeNotFound
    if cte != nil
      if err == nil && allowed
        if workspaceID != "" && shardManager != nil
          go func()
            if err != nil
```

### Proposed (max 2 levels):

Extract two methods:

```go
// tryCTEFallback promotes DENY→ALLOW when CTE succeeds for O nodes not in graph.
// Returns true if decision was changed.
func (e *decisionEngine) tryCTEFallback(ctx context.Context, req AccessRequest, decision *AccessDecision) bool {
    if decision.Decision != DecisionDeny || decision.Explanation.Reason != DenyReasonNodeNotFound {
        return false
    }
    if e.cte == nil {
        return false
    }
    allowed, err := e.cte.CheckAccess(ctx, req.UserNodeID, req.ObjectNodeID, req.Operation)
    if err != nil || !allowed {
        return false
    }
    decision.Decision = DecisionAllow
    decision.Explanation.Reason = "Resolved via CTE fallback (O node not in graph)"
    e.triggerAsyncShardPromotion(req)
    return true
}

// triggerAsyncShardPromotion loads the workspace shard in background after CTE fallback.
func (e *decisionEngine) triggerAsyncShardPromotion(req AccessRequest) {
    if req.WorkspaceID == "" || e.shardManager == nil {
        return
    }
    go func() {
        if _, err := e.shardManager.GetGraph(context.Background(), req.WorkspaceID); err != nil {
            slog.Warn("async shard promotion failed",
                "workspace_id", req.WorkspaceID, "error", err)
        }
    }()
}
```

Then `Decide()` becomes:

```go
func (e *decisionEngine) Decide(ctx context.Context, req AccessRequest) *AccessDecision {
    graph := e.resolveGraph(ctx, req)
    decision := graph.CheckAccess(req.UserNodeID, req.ObjectNodeID, req.Operation)

    // CTE fallback for O nodes not loaded into in-memory graph
    e.tryCTEFallback(ctx, req, decision)

    // Prohibition check: deny overrides on ALLOW
    if decision.Decision == DecisionAllow && e.prohibitions != nil {
        if denied, prohibName, subjectID := e.checkProhibitions(ctx, req, graph); denied {
            decision.Decision = DecisionDeny
            decision.Explanation.Reason = fmt.Sprintf("Denied by prohibition %q", prohibName)
            decision.Explanation.ProhibitionDenied = &ProhibitionDenial{
                ProhibitionName: prohibName,
                SubjectID:       subjectID,
            }
        }
    }

    return decision
}
```

**Bonus**: Eliminates the unnecessary struct copy at L75-87 — `CheckAccess` already returns `*AccessDecision`.

---

## 3. LRU Data Structure — `pip_shard_manager.go`

### Current: Slice-based (O(N) promote)

```
accessOrder []string  // linear scan on every hit
```

### Proposed: `container/list` + map (O(1) promote)

```go
import "container/list"

type shardManager struct {
    mu        sync.RWMutex
    db        *pgxpool.Pool
    shards    map[string]*shardEntry
    maxShards int

    // O(1) LRU via doubly-linked list + element map
    lruList    *list.List
    lruIndex   map[string]*list.Element  // workspaceID → list element

    // Metrics
    hits, misses, evictions, loadErrors int64
}

type lruEntry struct {
    workspaceID string
}
```

**Operations become O(1)**:

```go
func (sm *shardManager) promoteInAccessOrder(key string) {
    if elem, ok := sm.lruIndex[key]; ok {
        sm.lruList.MoveToBack(elem)
    }
}

func (sm *shardManager) removeFromAccessOrder(key string) {
    if elem, ok := sm.lruIndex[key]; ok {
        sm.lruList.Remove(elem)
        delete(sm.lruIndex, key)
    }
}

func (sm *shardManager) evictLRU() {
    front := sm.lruList.Front()
    if front == nil {
        return
    }
    entry := front.Value.(*lruEntry)
    sm.lruList.Remove(front)
    delete(sm.lruIndex, entry.workspaceID)
    delete(sm.shards, entry.workspaceID)
    sm.evictions++
    slog.Info("shard evicted (LRU)", "workspace_id", entry.workspaceID, "active_shards", len(sm.shards))
}
```

---

## 4. DRY Cache Invalidation — `epp_cache_invalidator.go`

Extract helper:

```go
func (c *CacheInvalidator) collectAndDelete(ctx context.Context, pipe redis.Pipeliner, patterns ...string) int {
    total := 0
    for _, pattern := range patterns {
        keys := c.collectKeys(ctx, pattern)
        for _, k := range keys {
            pipe.Del(ctx, k)
        }
        total += len(keys)
    }
    return total
}
```

---

## 5. Dead Code Removal — `pip_store.go`

Remove `GetUserByNGACNodeID` — already marked deprecated, queries `users` table (auth service boundary violation).

---

## Verification

- `go build ./services/policy/...` — compiles
- `go test ./services/policy/...` — all existing tests pass
- `go vet ./services/policy/...` — no warnings
- Behavioral equivalence: same decisions before/after (covered by existing test suite)
