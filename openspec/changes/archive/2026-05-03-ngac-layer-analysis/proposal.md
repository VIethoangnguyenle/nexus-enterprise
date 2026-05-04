# NGAC Layer Analysis — Policy Service

## What

Formalize NGAC layer boundaries (PDP/PIP/PAP) within `policy-service` by refactoring remaining cross-cutting concerns. This continues the work started with the Cache Orchestrator refactor (Problem 1 — SOLVED).

## Why

The layer analysis identified **5 structural problems** in the policy service. Problem 1 (ReadServer god method) is now resolved — `AccessEvaluator`, `DecisionEngine`, and `DecisionCache` cleanly separate read-path concerns. The remaining 4 problems still cause:

- **Testability friction**: `Graph` struct can't be provided as read-only dependency
- **Mutation blast radius**: Any `*Graph` consumer can accidentally mutate
- **File-level coupling**: PDP decision logic (`CheckProhibitions`) lives in the same file as PAP storage (`ProhibitionStore`)
- **Cache topology leaking**: `WriteServer.incrementAndInvalidate()` knows PIP internals

## Evidence (current state)

| # | Problem | File | Status |
|---|---------|------|--------|
| 1 | ReadServer god method | `read_server.go` | ✅ **SOLVED** — delegated to `AccessEvaluator` |
| 2 | Graph mixes PIP + PAP + PDP | `graph.go` | ❌ Open — 21 methods, no role separation |
| 3 | Store merges PAP + PIP | `store.go` | ❌ Open — every mutation writes to DB + memory silently |
| 4 | prohibition.go mixes PAP + PDP | `prohibition.go` | ❌ Open — CRUD + decision logic in one file |
| 5 | WriteServer coordinates PIP internals | `write_server.go` | ❌ Open — `incrementAndInvalidate` knows cache topology |

## Scope

### In scope (4 remaining problems)
1. **Graph read-only interface** — Extract `GraphReader` interface (PIP read) from `Graph` struct. PDP consumes `GraphReader`, PAP consumes `*Graph`.
2. **prohibition.go file split** — Move `CheckProhibitions()` (PDP) to `decision_engine.go`. Keep `ProhibitionStore` (PAP/PIP) in `prohibition.go`.
3. **Write-path cache abstraction** — Extract `InvalidationCoordinator` from `WriteServer.incrementAndInvalidate()`. Same pattern as read-path: hide PIP topology from transport.
4. **Store responsibility annotation** — Document PAP vs PIP on Store methods. No code split (too risky for marginal gain).

### Out of scope
- Full PAP/PIP split of `Store` struct — too risky, high coupling with all consumers
- `Graph` struct splitting into separate types — shared mutex makes this unsafe
- Read-path changes — already done in cache orchestrator refactor
- Proto or DB schema changes — none needed

### Deferred
- Unit tests for `DecisionEngine` in isolation (enabled by GraphReader interface)
- Write-path `CacheOrchestrator` (parallel to read-path `AccessEvaluator`)
- Event sourcing for PAP mutations

## Success Criteria
- `Graph` methods accessible via `GraphReader` interface for PDP consumers
- `CheckProhibitions()` lives in `decision_engine.go`, not `prohibition.go`
- `WriteServer` delegates cache invalidation to a coordinator, not inline
- All existing tests pass
- Build passes with zero new warnings

## Risk
- **Size**: M (4 file modifications, 1 new interface, 1 new coordinator)
- **Risk**: Low (all internal, zero API changes, existing test coverage)
