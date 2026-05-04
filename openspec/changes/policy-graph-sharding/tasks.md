## 1. Safety Net — CTE SQL Function + PC Intersection Fix

- [x] 1.1 Create SQL migration `ngac_check_access(user_id, object_id, operation)` with recursive CTE implementing BFS traversal + ALL-PC intersection
- [ ] 1.2 Add integration test: CTE result matches BFS result for VNPay test graph scenarios
- [x] 1.3 Replace `findCommonPC` with `allPCsSatisfied` in `access.go` — enforce `objectPCs ⊆ userPCs`
- [x] 1.4 Update `AccessExplanation.PolicyClass` from `string` to `[]string` — list all matched PCs
- [x] 1.5 Add unit tests for Multi-PC intersection: PC05 backward compat, PC06 multi-PC allow, PC07 missing-PC deny, PC08 3-PC stress
- [x] 1.6 Verify all existing tests pass (zero regression) — Fixed Share_OA pattern to standalone (no parent-child contamination)
- [x] 1.7 Update proto: add `repeated string policy_classes = 9` to AccessExplanation, deprecate field 2

## 2. PC Metadata + Authorization Guard

- [x] 2.1 Update workspace service: set `properties.scope=tenant, properties.tenant_id` when creating tenant PC
- [x] 2.2 Update asset service: set `properties.scope=global` on PC_AssetManagement
- [x] 2.3 Add PC authorization guard in `write_server.go:CreateNode` — reject `node_type=PC` without scope/tenant_id
- [x] 2.4 Add unit tests for PC guard: no-scope rejected, tenant-without-id rejected, global allowed, non-PC bypasses

## 3. ShardManager + Shard Loading

- [x] 3.1 Create `ShardManager` interface: `GetGraph(ctx, workspaceID)`, `InvalidateShard(workspaceID)`, `InvalidateAll()`, `Stats()`
- [x] 3.2 Implement LRU-based `shardManager` with configurable `max_shards` (default 1000)
- [x] 3.3 Implement `loadShard(workspaceID)` — recursive CTE to trace nodes from tenant PC + PC_Global, build `*Graph`
- [x] 3.4 Add shard invalidation hook: `WriteServer` mutations call `invalidateShards()` for affected workspace
- [x] 3.5 Add unit tests: stats, defaults, LRU eviction, promotion, invalidation, concurrent access (8 tests)
- [x] 3.6 Add observability: slog structured logging for cache hit/miss, load duration, eviction events

## 4. Decision Engine Integration

- [x] 4.1 Add `ShardManager` field to `decisionEngine` with `SetShardManager()` setter
- [x] 4.2 Update `Decide()` flow: `resolveGraph()` → shard (if workspace_id) → global graph fallback → BFS
- [x] 4.3 Add async shard promotion: after CTE serves cold request, trigger `GetGraph()` in background goroutine
- [x] 4.4 Update gRPC read server to pass workspace_id through `AccessRequest.WorkspaceID`
- [ ] 4.5 Add integration test: cold → CTE → hot promotion lifecycle (requires DB)

## 5. Proto + Workspace Routing

- [x] 5.1 Add `optional string workspace_id = 4` to `CheckAccessRequest` in policy proto
- [x] 5.2 Update policy read server to extract and route by `workspace_id`
- [ ] 5.3 Update Drive service: pass `workspace_id` in CheckAccess calls (backward compat OK — deferred to incremental rollout)
- [ ] 5.4 Update Messaging service: pass `workspace_id` in CheckAccess calls
- [ ] 5.5 Update Approval service: pass `workspace_id` in CheckAccess calls
- [ ] 5.6 Update Asset service: pass `workspace_id` in CheckAccess calls
- [x] 5.7 Verify backward compat: request without workspace_id falls back to global graph ✓

## 6. Verification + Benchmarks

- [ ] 6.1 Benchmark: LoadShard latency for workspace with 1000 users, 100 departments
- [ ] 6.2 Benchmark: BFS in shard vs full graph — verify comparable latency
- [ ] 6.3 Benchmark: CTE SQL latency for typical access check (target ≤ 10ms)
- [ ] 6.4 Memory profiling: verify 1000 shards ≤ 2GB RSS
- [x] 6.5 Run full test suite against sharded engine — zero regression (all 45+ tests pass)
