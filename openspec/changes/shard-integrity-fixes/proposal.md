# Shard Integrity Fixes

## What

Fix 10 issues (2 critical, 3 high, 5 medium) discovered during the shard architecture impact audit. The `policy-graph-sharding` change introduced per-workspace sharded graphs but left several components still assuming a single global graph, creating cross-tenant data leaks and inconsistent behavior.

## Why

- **Security (Critical)**: Cache keys lack workspace_id → cross-tenant decision collision. User in both Workspace A and B gets cached ALLOW from A applied to B.
- **Security (Critical)**: Prohibition evaluation hardcodes `e.graph` (global) while BFS uses shard → prohibition bypass when nodes exist only in shard.
- **Correctness (High)**: ReadServer graph query RPCs, scope resolution, and store mutations all assume single global graph.
- **Performance (Medium)**: VersionTracker uses global-only scope causing cache miss storms across all tenants on any single-tenant mutation.
- **Test Gap**: Zero tests exist for DecisionEngine, DecisionCache, InvalidationCoordinator, or cross-component shard interactions. All 74 existing tests only exercise single-graph BFS.

## Scope

### In scope
- Fix cache key isolation (L1 Redis + L2 Materialized + Scope cache)
- Fix prohibition evaluation to use resolved graph (not hardcoded global)
- Add per-workspace version tracking to InvalidationCoordinator
- Wire ShardManager into both `cmd/main.go` and `cmd/policy-read/main.go`
- Add workspace_id to GraphMutatedEvent
- Fix CreateNode missing invalidation call
- Add comprehensive test suite: DecisionEngine, DecisionCache, Invalidation, cross-tenant scenarios

### Out of scope
- Consumer service workspace_id pass-through (Drive, Messaging, Approval, Asset) — separate incremental rollout
- loadShard CTE integration tests — requires live database, defer to benchmark phase
- ReadServer graph RPCs workspace-aware routing — requires proto changes, separate change

## Evidence

Full audit documented in explore session. Key files affected:
- `decision_cache.go:144` — cacheKey() missing workspace_id
- `decision_engine.go:122,142` — checkProhibitions() hardcodes e.graph
- `invalidation.go:36` — version.Increment(ctx, "global") only
- `cmd/main.go:113`, `cmd/policy-read/main.go:73` — no ShardManager wired
- `write_server.go:52-83` — CreateNode skips invalidation
- `producer.go:24-29` — GraphMutatedEvent missing workspace_id

## Success Criteria

1. Cache key for `(user, obj, op)` with `workspace_id=A` MUST differ from `workspace_id=B`
2. Prohibition check MUST use same graph as BFS evaluation
3. Single-tenant mutation MUST NOT invalidate other tenants' L2 cache
4. All existing 74 tests pass (zero regression)
5. New test coverage: ≥15 tests covering DecisionEngine, DecisionCache, cross-tenant isolation
