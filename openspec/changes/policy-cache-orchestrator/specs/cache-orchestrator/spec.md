# Spec: Cache Orchestrator — Read Path Refactor

## Summary

Extract 3 components from `ReadServer` to separate cache orchestration (PIP) from decision logic (PDP) and transport (gRPC).

## Type

Internal refactor. No API, proto, or DB changes.

---

## Capability 1: DecisionEngine (PDP)

### Description
Encapsulate all access decision logic into a single component that takes a request and returns a FINAL decision (including prohibition evaluation).

### Source
- `read_server.go:150-197` → `computeAccess()` 
- `read_server.go:199-234` → `checkProhibitions()`

### Contract
```go
type DecisionEngine interface {
    Decide(ctx context.Context, req AccessRequest) *AccessDecision
}
```

### Acceptance Criteria
- [ ] `DecisionEngine.Decide()` performs BFS graph traversal via `Graph.CheckAccess()`
- [ ] Falls back to CTE SQL evaluator when object node not found in graph
- [ ] Evaluates prohibitions after BFS returns ALLOW (deny override)
- [ ] Returns FINAL decision (ALLOW or DENY) — caller never needs additional checks
- [ ] Does NOT touch Redis, MaterializedAccess, or VersionTracker (no PIP cache)

---

## Capability 2: DecisionCache (PIP Cache)

### Description
Encapsulate L1 (Redis) + L2 (Materialized) + version freshness into a single cache abstraction.

### Source
- `read_server.go:63-69` → L1 Redis check
- `read_server.go:72-79` → L2 Materialized check
- `read_server.go:122-148` → `checkMaterialized()`
- `read_server.go:237-250` → `populateCaches()`
- Redis helper methods (`getRedisCache`, `setRedisCache`)

### Contract
```go
type DecisionCache interface {
    Get(ctx context.Context, req AccessRequest) *AccessDecision
    Set(ctx context.Context, req AccessRequest, decision *AccessDecision)
}
```

### Acceptance Criteria
- [ ] `Get()` checks L1 Redis first, then L2 Materialized with version freshness
- [ ] `Get()` promotes L2 hit to L1 (sets Redis on L2 hit)
- [ ] `Get()` returns nil on cache miss or stale version
- [ ] `Set()` writes to both L2 (materialized) and L1 (Redis)
- [ ] Metrics recorded per cache layer: `L1` and `L2` labels
- [ ] Graceful degradation: works when Redis is nil (no L1 cache)

---

## Capability 3: AccessEvaluator (Orchestrator)

### Description
Coordinate cache lookup and PDP computation. Single entry point for all access checks.

### Source
- `read_server.go:57-91` → `CheckAccess()` (the god method)

### Contract
```go
type AccessEvaluator struct {
    cache  DecisionCache
    engine DecisionEngine
}

func (e *AccessEvaluator) Evaluate(ctx context.Context, req AccessRequest) *AccessDecision
```

### Acceptance Criteria
- [ ] Calls `cache.Get()` first → returns on hit
- [ ] On cache miss, calls `engine.Decide()`
- [ ] Stores computed decision via `cache.Set()`
- [ ] Records `L3` metric label when engine computes
- [ ] Records total duration metric

---

## Capability 4: ReadServer Simplification

### Description
Reduce `ReadServer` dependencies from 7 to 4. `CheckAccess` becomes a single delegation.

### Source
- `read_server.go:25-55` → struct + constructor

### Acceptance Criteria
- [ ] `ReadServer` struct has exactly 4 fields: `evaluator`, `store`, `operations`, `prohibitions`
- [ ] `NewReadServer()` constructor takes `AccessEvaluator` instead of 5 separate cache/engine deps
- [ ] `CheckAccess()` is ≤5 lines: convert proto → internal, call evaluator, convert back
- [ ] `BatchCheckAccess()` delegates to evaluator per object/operation pair
- [ ] All graph query RPCs (`GetNode`, `GetAncestors`, etc.) unchanged — still use `store`
- [ ] `ListOperations` and `ListProhibitions` RPCs unchanged

---

## Non-functional

### Build Verification
- [ ] `go build ./backend/services/policy/...` passes
- [ ] `go vet ./backend/services/policy/...` passes
- [ ] No new lint warnings

### Test Verification
- [ ] All existing tests pass: `go test ./backend/services/policy/...`
- [ ] No behavior changes (same L1→L2→L3 fallback order)

### Backward Compatibility
- [ ] Legacy `PolicyServer` (`server.go`) NOT modified — remains functional
- [ ] `cmd/main.go` wiring updated to construct `AccessEvaluator`
- [ ] `cmd/policy-read/main.go` wiring updated to construct `AccessEvaluator`
