# NGAC Layer Organization — Tổ chức file theo chuẩn PIP/PAP/PDP/EPP

## Evidence Summary

- **Backend**: EXISTS — `backend/services/policy/internal/ngac/` (22 files, ~2000 LOC)
- **Layer logic**: EXISTS — comment annotations `// PIP:`, `// PAP:`, `// PDP:` already present
- **Interfaces**: EXISTS — `GraphReader` (PIP), `DecisionEngine` (PDP), `DecisionCache` (PDP cache)
- **CQRS**: EXISTS — `grpc/read_server.go` vs `grpc/write_server.go`
- **NIST compliance**: VERIFIED — current code follows NIST SP 800-178 functionally

### Current File → Layer Mapping

| File | NGAC Layer | Role |
|------|-----------|------|
| `graph.go` (reads) | PIP | Graph traversal (BFS, ancestors, descendants) |
| `graph.go` (mutations) | PAP | Node/edge mutations |
| `graph_reader.go` | PIP | Read-only interface contract |
| `store.go` (reads) | PIP | DB → memory hydration |
| `store.go` (writes) | PAP | DB persistence + graph sync |
| `cte.go` | PIP | SQL CTE fallback |
| `materialized.go` | PIP | L2 cache storage |
| `shard_manager.go` | PIP | Per-tenant graph loading |
| `decision_engine.go` | PDP | Core decision orchestration |
| `access.go` | PDP | BFS algorithm + association matching |
| `evaluator.go` | PDP | Cache-first evaluation orchestrator |
| `decision_cache.go` | PDP | L1/L2 cache coordination |
| `prohibition.go` | PAP | Prohibition CRUD |
| `operations.go` | PAP | Operation registry |
| `cache_invalidator.go` | EPP | Targeted Redis invalidation |
| `invalidation.go` | EPP | Invalidation coordinator |
| `version.go` | EPP | Graph version tracking |
| `models.go` | Shared | Data types |

## Product Assessment

- **Size**: XS (file rename only, zero code change)
- **Risk**: None (no functional change, no import path change)
- **Target**: Developer experience — faster codebase navigation
- **Core action**: Rename 15 files with NGAC layer prefixes

## Scope

### In scope

1. Rename files in `internal/ngac/` with NGAC layer prefixes: `pip_`, `pap_`, `pdp_`, `epp_`
2. Split `graph.go` into `pip_graph.go` (reads) + `pap_graph.go` (mutations)
3. Split `store.go` into `pip_store.go` (reads) + `pap_store.go` (writes)
4. Keep `models.go` as-is (shared types, no prefix needed)
5. Keep test files co-located with their source (`pip_graph_test.go` etc.)

### Out of scope

- Package splitting (sub-packages `pip/`, `pap/`, `pdp/`, `epp/`) — deferred until 50+ files
- Code logic changes — zero functional modifications
- Interface changes — all exports remain identical
- gRPC transport layer changes — `grpc/` package untouched
- Test logic changes — only rename test files

### Non-goals

- Enforcing compile-time layer boundaries (would require package split)
- Adding new interfaces between layers
- Refactoring `Graph` struct to separate read/write

## Success Criteria

1. All files in `internal/ngac/` have clear NGAC layer prefix (except `models.go`)
2. `go build ./...` passes — zero compilation errors
3. `go test ./internal/ngac/...` passes — all tests green
4. `go vet ./...` passes — no new warnings
5. No import path changes — package name stays `ngac`
6. Git blame preserved via `git mv` — history intact
