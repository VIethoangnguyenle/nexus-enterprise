# Policy Clean Code — Design

## Architecture (Unchanged)

Kiến trúc CQRS + 3-layer cache giữ nguyên. Refactoring chỉ ảnh hưởng internal implementation.

```
ReadServer ──→ AccessEvaluator ──→ DecisionCache (L1/L2)
                                └→ DecisionEngine (L3 BFS/CTE + Prohibition)
WriteServer ──→ InvalidationCoordinator ──→ CacheInvalidator + MaterializedAccess + VersionTracker
```

---

## CS-01: Fix Store.CreateAssignment — Validate-first pattern

**Before** (buggy):
```go
graph.AddAssignment(a)      // validate + add
graph.RemoveAssignment(...)  // remove immediately
db.Exec(...)                // write DB
graph.AddAssignment(a)      // add again
```

**After** (correct):
```go
graph.ValidateAssignment(a)  // validate only (new method)
db.Exec(...)                 // write DB first
graph.AddAssignment(a)       // add to graph only on DB success
```

**Changes**:
- `graph.go`: Add `ValidateAssignment(a) error` — runs node existence, type check, cycle detection WITHOUT mutating state
- `store.go`: Rewrite `CreateAssignment` to use validate-first pattern

---

## CS-02: Extract prohibition affected nodes resolver

**Before**: 15 lines duplicated in `CreateProhibition` and `RemoveProhibition`

**After**: Extract to `resolveProhibitionAffectedNodes` method on WriteServer:
```go
func (s *WriteServer) resolveProhibitionAffectedNodes(subjectID string, targetOAIDs []string) []string
```

---

## CS-03: Replace `*redis.Client` with ScopeCache interface

**Before**:
```go
type ReadServer struct {
    rdb *redis.Client  // hard dependency
}
```

**After**: 
```go
// In ngac package
type ScopeCache interface {
    GetScopes(ctx context.Context, key string) (*pb.ResolveAccessibleScopesResponse, error)
    SetScopes(ctx context.Context, key string, resp *pb.ResolveAccessibleScopesResponse)
}

type ReadServer struct {
    scopeCache ScopeCache  // interface dependency
}
```

Ngoài ra, chuyển `getCachedScopes` / `setCachedScopes` vào implementation của ScopeCache.

---

## CS-05: Replace string-based deny reason with typed constant

**Before** (fragile):
```go
// access.go
Reason: "User or object node not found in graph"

// decision_engine.go
if decision.Explanation.Reason == "User or object node not found in graph" {
```

**After**:
```go
// models.go — thêm sentinel reasons
const (
    DenyReasonNodeNotFound    = "node_not_found"
    DenyReasonNoAssociation   = "no_association_path"
)

// access.go
Reason: DenyReasonNodeNotFound

// decision_engine.go  
if decision.Explanation.Reason == DenyReasonNodeNotFound {
```

---

## CS-04: Extract `isLeafOA` helper

**Before**: 3-level nested if in `resolveLeafOAs`

**After**:
```go
func hasOAChildren(graph GraphReader, nodeID string) bool {
    for _, child := range graph.GetChildren(nodeID) {
        if child.NodeType == NodeTypeObjectAttr {
            return true
        }
    }
    return false
}
```

---

## CS-07: Remove GetUserByNGACNodeID

Method queries `users` table (auth service boundary). Kiểm tra nếu không ai gọi → xóa. Nếu vẫn cần → document là cross-service query + add TODO.

---

## CS-08: Use GraphReader interface in ReadServer helpers

Replace 3 inline interface definitions với `GraphReader` đã có sẵn:
```go
func (s *ReadServer) collectUAAncestors(graph GraphReader, ...) []string
func (s *ReadServer) collectTargetOAs(graph GraphReader, ...) map[string]bool
func (s *ReadServer) resolveLeafOAs(graph GraphReader, ...) []string
```

---

## CS-10: Replace fmt.Printf with slog.Warn

```go
// store.go:121 — fmt.Printf → slog.Warn
// store.go:136 — fmt.Printf → slog.Warn
```

---

## CS-12: Remove dead code

- `ConstraintDenial` struct (models.go:64) — not referenced anywhere
- `ConstraintDenied` field in `AccessExplanation` — not used
- `ConstraintsChecked` field — not used

---

## Validation Strategy

```bash
cd backend/services/policy
go build ./...           # compile check
go test -v ./internal/ngac/  # all 51 tests pass
go vet ./...             # vet clean
```

Zero behavior change → existing tests are sufficient.
