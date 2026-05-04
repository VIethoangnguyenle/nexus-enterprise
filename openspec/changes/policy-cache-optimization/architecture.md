# Architecture: Policy Cache Optimization

## 1. Affected Services

| Service | Impact | Files |
|---|---|---|
| `policy` | Primary — refactor invalidation + add metrics | 4-6 files |

No other services affected. No proto changes. No frontend changes.

## 2. Data Flow — Targeted Invalidation

### Current (full flush):
```
WriteServer.CreateAssignment(child, parent)
  → store.CreateAssignment()
  → version.Increment("global")
  → materialized.InvalidateByUser(child)
  → SCAN "ngac:access:*" → DEL all          ← PROBLEM
  → SCAN "scopes:*" → DEL all               ← PROBLEM
```

### Proposed (targeted):
```
WriteServer.CreateAssignment(child, parent)
  → store.CreateAssignment()
  → version.Increment("global")
  → materialized.InvalidateByUser(child)
  → invalidateTargeted(child, parent):       ← NEW
      ① Resolve node types from graph
      ② Collect affected user IDs:
         - If child is U → {child}
         - If child is UA → GetDescendants(child) → filter U nodes
      ③ Collect affected object IDs:
         - If parent is OA → {parent}  
         - If parent is UA → skip (user-side, no object cache)
      ④ Build key patterns per user: "ngac:access:{userID}:*"
      ⑤ Build key patterns per object: "ngac:access:*:{objectID}:*"
      ⑥ Redis DEL (batch, no SCAN)
      ⑦ Scopes: "scopes:{userID}:*" per affected user
```

### Key Insight — Why NOT use Redis SCAN

`SCAN` is O(N) where N = total keys in Redis. Even with COUNT hint, it iterates 
the keyspace. With structured keys, we can construct exact key patterns and use 
`DEL` directly or `KEYS` with prefix (O(matched) not O(total)).

**Trade-off**: We use Redis Hash structure to group keys by user, enabling
O(1) deletion per user rather than O(N) scan.

## 3. Redis Key Strategy

### Option A: Keep flat keys + prefix delete (CHOSEN)
```
Key: ngac:access:{userID}:{objectID}:{op}
Delete: pipeline.Del("ngac:access:{userID}:*") — per affected user
```

**Why chosen**: Simplest change. Key format already supports prefix matching.
`KEYS` with prefix is acceptable at small scale (keys per user < 100).

### Option B: Redis Hash per user (DEFERRED)
```
HSET ngac:access:{userID} "{objectID}:{op}" "{decision_json}"
HDEL ngac:access:{userID} — delete all for user
```

**Why deferred**: Requires changing read path + TTL strategy (Hash fields don't have individual TTL). Over-engineering for current scale.

## 4. Affected Node Resolution Algorithm

```go
func resolveAffectedKeys(graph *Graph, nodeIDs ...string) (userKeys, objectKeys []string) {
    affectedUsers := map[string]bool{}
    affectedObjects := map[string]bool{}
    
    for _, nodeID := range nodeIDs {
        node := graph.GetNode(nodeID)
        if node == nil { continue }
        
        switch node.NodeType {
        case "U":
            affectedUsers[nodeID] = true
        case "UA":
            // UA change affects all user descendants
            for id, n := range graph.GetDescendants(nodeID) {
                if n.NodeType == "U" {
                    affectedUsers[id] = true
                }
            }
        case "OA":
            affectedObjects[nodeID] = true
            // OA change affects descendant OAs too
            for id, n := range graph.GetDescendants(nodeID) {
                if n.NodeType == "OA" {
                    affectedObjects[id] = true
                }
            }
        case "PC":
            // PC change = broad impact → full flush (rare operation)
            return nil, nil // signal full flush
        }
    }
    
    for uid := range affectedUsers {
        userKeys = append(userKeys, fmt.Sprintf("ngac:access:%s:*", uid))
        userKeys = append(userKeys, fmt.Sprintf("scopes:%s:*", uid))
    }
    for oid := range affectedObjects {
        objectKeys = append(objectKeys, fmt.Sprintf("ngac:access:*:%s:*", oid))
    }
    return userKeys, objectKeys
}
```

**Edge cases**:
- **PC change**: Falls back to full flush (PC changes are extremely rare — workspace creation only)
- **Node not found**: Skip silently, log warning
- **Empty affected set**: No Redis operations needed

## 5. Metrics Architecture

```
policy/internal/metrics/
  └── metrics.go    — Prometheus counter/histogram definitions

Metrics registered in main.go, passed to ReadServer/WriteServer.
```

**Decision**: Metrics are NOT a separate package with heavy abstraction. 
A single `metrics.go` file with package-level vars is sufficient.

**Note**: `prometheus/client_golang` is NOT currently in go.mod. 
Must add as new dependency.

## 6. File Change Summary

| File | Change | Type |
|---|---|---|
| `policy/internal/ngac/cache_invalidator.go` | **NEW** — Targeted invalidation logic | New file |
| `policy/internal/metrics/metrics.go` | **NEW** — Prometheus metrics definitions | New file |
| `policy/internal/grpc/write_server.go` | MODIFY — Use targeted invalidation | Refactor |
| `policy/internal/grpc/server.go` | MODIFY — Use targeted invalidation | Refactor |
| `policy/internal/grpc/read_server.go` | MODIFY — Add metrics to CheckAccess | Enhancement |
| `policy/cmd/main.go` | MODIFY — Register metrics, init invalidator | Wiring |
| `policy/cmd/policy-read/main.go` | MODIFY — Register metrics | Wiring |
| `go.mod` | MODIFY — Add prometheus/client_golang | Dependency |

## 7. NGAC Permission Model
No changes. This is internal infrastructure — no permission checks needed.

## 8. Integration Points
- **Redis**: Same client, different deletion strategy
- **Prometheus**: New HTTP endpoint for metrics scraping
- **No new gRPC/REST APIs**
- **No Redpanda changes** (existing events unchanged)
