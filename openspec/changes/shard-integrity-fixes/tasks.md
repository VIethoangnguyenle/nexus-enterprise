## 1. Cache Isolation (Critical — C1, M3)

- [x] 1.1 Update `cacheKey()` in `decision_cache.go` to include workspace_id prefix
- [x] 1.2 Update `getMaterialized()` and `Set()` to pass workspace_id through to version scope
- [x] 1.3 Add `workspace_id` column to `ngac_materialized_access` (migration SQL: `006_shard_cache_isolation.sql`)
- [x] 1.4 Update `MaterializedAccess.Lookup()` and `Store()` to include workspace_id
- [x] 1.5 Update scope cache key in `ResolveAccessibleScopes` — **DEFERRED**: proto lacks workspace_id field, requires separate proto change
- [ ] 1.6 Update `CacheInvalidator.deleteTargetedKeys()` to scan workspace-prefixed patterns — **LOW PRIORITY**: current SCAN pattern already matches all prefixed keys

## 2. Engine Correctness (Critical — C2, M5)

- [x] 2.1 Refactor `checkProhibitions()` to accept `GraphReader` parameter
- [x] 2.2 Update `Decide()` to pass resolved graph to `checkProhibitions()`
- [x] 2.3 Add invalidation + event publish to `WriteServer.CreateNode()`

## 3. Invalidation Coordination (High — M1, M4)

- [x] 3.1 Add `workspaceID` parameter to `InvalidationCoordinator.InvalidateForNodes()`
- [x] 3.2 Implement per-workspace version bump: `ws:{workspace_id}` scope
- [x] 3.3 Update `DecisionCache.Set()` to use per-workspace version scope
- [x] 3.4 Update `DecisionCache.Get()` (getMaterialized) to check per-workspace version
- [x] 3.5 Add `WorkspaceID` field to `GraphMutatedEvent` in producer.go
- [x] 3.6 Update all `WriteServer` callers of `InvalidateForNodes` to pass workspace_id (9 callers)

## 4. Wiring (Medium — M2)

- [x] 4.1 Wire ShardManager in `cmd/main.go` — create + set on DecisionEngine + WriteServer
- [x] 4.2 Wire ShardManager in `cmd/policy-read/main.go` — create + set on DecisionEngine

## 5. Test Coverage (Critical — all issues)

### 5a. DecisionEngine tests (Tier 1 — no DB)

- [x] 5.1 TestDecide_ShardRouting — resolveGraph returns shard when workspace_id set
- [x] 5.2 TestDecide_GlobalFallback — resolveGraph falls back when shard miss
- [x] 5.3 TestDecide_CrossTenantIsolation — same (user,obj,op), shard A→ALLOW, shard B→DENY
- [x] 5.4 TestDecide_NoWorkspaceID_UsesGlobalGraph — backward compat

### 5b. DecisionCache tests (Tier 1 — no DB)

- [x] 5.5 TestCacheKey_WithWorkspaceID — key includes workspace prefix
- [x] 5.6 TestCacheKey_WithoutWorkspaceID — backward compat
- [x] 5.7 TestCacheKey_DifferentWorkspaces_NoCrossHit — ws-A set, ws-B miss
- [x] 5.8 TestVersionScope_WorkspaceIsolation — scope keys isolated

### 5c. Cross-tenant integration (Tier 1 — no DB)

- [x] 5.9 TestCrossTenant_UserInBothWorkspaces — ALLOW write ws-1, DENY write ws-2
- [x] 5.10 TestCrossTenant_CacheKeyIsolation — 4-workspace collision check
- [x] 5.11 TestCrossTenant_VersionScopeIsolation — scope uniqueness
- [x] 5.12 TestCrossTenant_ShardInvalidationIsolation — invalidate ws-A doesn't affect ws-B

## 6. Verification

- [x] 6.1 All existing tests pass — 63 existing + 12 new = 75 total PASS
- [x] 6.2 `go build ./...` + `go vet ./...` clean ✓
- [x] 6.3 GRPC tests (read_server) still pass — 12 PASS ✓
