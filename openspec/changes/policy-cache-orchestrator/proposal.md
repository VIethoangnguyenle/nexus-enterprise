# Policy Service — Cache Orchestrator Refactor

## Evidence Summary
- Backend: EXISTS — `backend/services/policy/internal/grpc/read_server.go` (542 lines, 7 dependencies)
- Frontend: NOT AFFECTED — backend-only internal refactor
- Proto: NOT AFFECTED — no API contract changes
- DB: NOT AFFECTED — no schema changes
- Dependencies: SELF-CONTAINED — all changes within `backend/services/policy/internal/`

## Codebase Evidence

### Problem (verified)
- `ReadServer` struct has 7 fields: `store`, `rdb`, `cte`, `materialized`, `version`, `operations`, `prohibitions`
- `CheckAccess()` method (lines 57-91) mixes L1 Redis, L2 Materialized, L3 BFS, and prohibition eval in one function
- `computeAccess()` (lines 150-197) contains PDP logic (BFS + CTE fallback + prohibition) inside transport layer
- `checkProhibitions()` (lines 199-234) also in transport layer — should be PDP
- `populateCaches()` (lines 237-250) writes to L1+L2 — transport knows cache internals
- `checkMaterialized()` (lines 122-148) checks version freshness — transport knows PIP internals

### Existing Patterns
- `ngac/` package already has clean separation: `graph.go`, `store.go`, `materialized.go`, `cache_invalidator.go`
- Prior refactor (generic-policy-service) successfully decoupled ConstraintEngine and operations

## Product Assessment
- Size: **S** — 3 new files, 1 modified file, zero API changes
- Risk: **Low** — internal refactor, no behavior change, no proto/DB changes
- Target user: **Policy service maintainer** — better testability and modularity
- Core action: Separate cache orchestration from transport and decision logic

## Scope

### In scope
1. Extract `DecisionEngine` interface + impl (PDP: BFS + CTE + prohibition evaluation)
2. Extract `DecisionCache` interface + impl (PIP: L1 Redis + L2 Materialized + version check)
3. Create `AccessEvaluator` orchestrator (cache → engine → cache write-back)
4. Simplify `ReadServer` to delegate to `AccessEvaluator`
5. Move metrics recording into cache/engine (where layer info is available)

### Out of scope
- Write path refactor (`write_server.go` + `incrementAndInvalidate`) — separate change
- `Graph` struct interface extraction (PIP read-only boundary) — separate change
- `Store` struct PAP/PIP separation — separate change
- Scope resolution refactor (`ResolveAccessibleScopes`) — different query pattern
- Legacy `PolicyServer` (`server.go`) refactor — backward compat maintained

### Deferred
- Interface extraction for `Graph` (read-only view vs mutation)
- Write path cache orchestrator
- Unit tests for `DecisionEngine` in isolation (possible after this refactor)

## Success Criteria
- `ReadServer.CheckAccess()` becomes a single delegation call
- `ReadServer` struct drops from 7 fields to 4
- Zero behavior change: same L1→L2→L3 fallback, same prohibition check, same metrics
- Build passes (`go build ./...`)
- Existing tests pass (`go test ./...`)
