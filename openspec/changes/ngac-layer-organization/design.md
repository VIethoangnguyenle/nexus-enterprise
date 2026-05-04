# Design: NGAC Layer File Organization

## Approach

Rename files với NGAC layer prefix. Tách 2 file lớn (`graph.go`, `store.go`) chứa cả PIP+PAP thành file riêng theo layer. Zero code change — chỉ `git mv`.

## File Rename Map

### Before → After

```
internal/ngac/
├── models.go                 →  models.go                    (unchanged — shared)
│
│ ── PIP (Policy Information Point) ──
├── graph.go (read methods)   →  pip_graph.go                 (SPLIT)
├── graph_reader.go           →  pip_graph_reader.go
├── store.go (read methods)   →  pip_store.go                 (SPLIT)
├── cte.go                    →  pip_cte.go
├── materialized.go           →  pip_materialized.go
├── shard_manager.go          →  pip_shard_manager.go
│
│ ── PAP (Policy Administration Point) ──
├── graph.go (mutation methods)→ pap_graph.go                 (SPLIT)
├── store.go (write methods)  →  pap_store.go                 (SPLIT)
├── prohibition.go            →  pap_prohibition.go
├── operations.go             →  pap_operations.go
│
│ ── PDP (Policy Decision Point) ──
├── access.go                 →  pdp_access.go
├── decision_engine.go        →  pdp_decision_engine.go
├── evaluator.go              →  pdp_evaluator.go
├── decision_cache.go         →  pdp_decision_cache.go
│
│ ── EPP (Event Processing Point) ──
├── cache_invalidator.go      →  epp_cache_invalidator.go
├── invalidation.go           →  epp_invalidation.go
├── version.go                →  epp_version.go
│
│ ── Tests (co-located) ──
├── decision_engine_test.go   →  pdp_decision_engine_test.go
├── shard_manager_test.go     →  pip_shard_manager_test.go
├── store_test.go             →  pip_store_test.go  + pap_store_test.go  (SPLIT)
├── vnpay_graph_test.go       →  pdp_vnpay_graph_test.go
├── vnpay_scenarios_test.go   →  pdp_vnpay_scenarios_test.go
├── export_test_helpers.go    →  export_test_helpers.go       (unchanged)
```

## Split Strategy

### graph.go → pip_graph.go + pap_graph.go

**pip_graph.go** chứa:
- `Graph` struct definition + `NewGraph()`
- Read methods: `GetNode`, `FindNodeByName`, `GetAncestors`, `GetAncestorsWithPaths`, `GetDescendants`, `GetChildren`, `GetParents`, `GetNodesByType`, `GetAssociationsFromUA`, `IsAssigned`
- Internal BFS helpers: `bfsCollectAttributesAndPCs`

**pap_graph.go** chứa:
- Mutation methods: `AddNode`, `RemoveNode`, `ValidateAssignment`, `AddAssignment`, `RemoveAssignment`, `AddAssociation`, `RemoveAssociationByID`
- Internal helpers: `wouldCreateCycle`, `canReach`, `removeAssignmentIndexes`, `removeAssociationIndexes`

### store.go → pip_store.go + pap_store.go

**pip_store.go** chứa:
- `Store` struct definition + `NewStore()`
- `LoadGraph` (DB → memory hydration)
- Read methods: `GetGraph`, `FindNodeByName`, `GetNodesByType`, `GetNode`, `IsAssigned`, `HasSeedData`
- `GetUserByNGACNodeID` (deprecated, but PIP role)

**pap_store.go** chứa:
- `InitSchema`
- Write methods: `CreateNode`, `DeleteNode`, `CreateAssignment`, `RemoveAssignment`, `CreateAssociation`, `RemoveAssociationByUAOA`

### store_test.go split

**pip_store_test.go** — tests for LoadGraph, read methods
**pap_store_test.go** — tests for CreateNode, CreateAssignment etc.

> Cần đọc `store_test.go` khi implement để xác định test nào thuộc PIP vs PAP.

## Invariants (MUST preserve)

1. Package name stays `ngac` — tất cả file vẫn `package ngac`
2. Zero export changes — tất cả public types/functions giữ nguyên signature
3. `Graph` struct chỉ định nghĩa 1 lần (trong `pip_graph.go`) — `pap_graph.go` dùng receiver methods
4. `Store` struct chỉ định nghĩa 1 lần (trong `pip_store.go`) — `pap_store.go` dùng receiver methods
5. `go build`, `go test`, `go vet` phải pass
6. Dùng `git mv` để preserve blame history

## Risks

| Risk | Mitigation |
|------|-----------|
| Git blame bị mất | Dùng `git mv` thay vì delete+create |
| IDE confusion trong quá trình rename | Rename tất cả trong 1 commit |
| Split test file sai method | Đọc kỹ test file trước khi split |
