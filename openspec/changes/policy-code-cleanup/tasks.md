# Policy Code Cleanup — Tasks

## Task Overview

| # | Task | Files | Effort | Risk |
|---|------|-------|--------|------|
| 1 | Add decision + scope constants | `models.go` | 5 min | None |
| 2 | Replace hard-coded strings across codebase | 7 files | 15 min | None |
| 3 | Extract cache key/scope prefix constants | `pdp_decision_cache.go`, `epp_cache_invalidator.go`, `read_server.go` | 10 min | None |
| 4 | Flatten `Decide()` nesting | `pdp_decision_engine.go` | 15 min | Low |
| 5 | Upgrade LRU to `container/list` | `pip_shard_manager.go` | 20 min | Low |
| 6 | DRY cache invalidation loops | `epp_cache_invalidator.go` | 10 min | None |
| 7 | Remove dead code | `pip_store.go` | 5 min | None |
| 8 | Fix raw node type strings in gRPC | `write_server.go` | 5 min | None |
| 9 | Verify — build + test | — | 5 min | — |

**Total estimated**: ~90 minutes

---

## Tasks

### Task 1: Add Decision + Scope Constants to `models.go`
- [x] Add `DecisionAllow = "ALLOW"` and `DecisionDeny = "DENY"` constants
- [x] Add `ScopeGlobal = "global"` constant  
- [x] Add `WorkspaceScope(wsID string) string` helper function
- **File**: `backend/services/policy/internal/ngac/models.go`

### Task 2: Replace Hard-coded Decision Strings
- [x] `pdp_decision_engine.go` — replace `"DENY"`, `"ALLOW"` with `DecisionDeny`, `DecisionAllow`
- [x] `pdp_access.go` — replace in `CheckAccess` return values
- [x] `pdp_decision_cache.go` — replace in `getMaterialized` and `Set`
- [x] `grpc/read_server.go` — replace in `BatchCheckAccess`

### Task 3: Extract Cache Key + Scope Prefix Constants
- [x] Add `cacheKeyPrefix = "ngac:access:"` and `scopeKeyPrefix = "scopes:"` to `pdp_decision_cache.go`
- [x] Update `cacheKey()` to use prefix constant
- [x] Update `epp_cache_invalidator.go` to use prefix constants
- [x] Replace `"ws:%s"` format strings with `WorkspaceScope()` in `epp_invalidation.go` and `pdp_decision_cache.go`
- [x] Replace `"global"` with `ScopeGlobal` in `epp_invalidation.go` and `pdp_decision_cache.go`
- [x] Remove unused `fmt` import from `epp_invalidation.go`

### Task 4: Flatten `Decide()` Method
- [x] Extract `tryCTEFallback(ctx, req, decision)` method
- [x] Extract `triggerAsyncShardPromotion(req)` method
- [x] Simplify `Decide()` body — remove unnecessary struct copy, use extracted methods
- [x] Verify max nesting depth ≤ 3
- **File**: `backend/services/policy/internal/ngac/pdp_decision_engine.go`

### Task 5: Upgrade LRU to O(1) via `container/list`
- [x] Add `import "container/list"` to `pip_shard_manager.go`
- [x] Replace `accessOrder []string` with `lruList *list.List` + `lruIndex map[string]*list.Element`
- [x] Add `lruKey` struct with `workspaceID string`
- [x] Update `NewShardManager` to initialize list + map
- [x] Rewrite `promoteInAccessOrder` → `MoveToBack` O(1)
- [x] Rewrite `removeFromAccessOrder` → `Remove` + map delete O(1)
- [x] Rewrite `evictLRU` → `Front` + `Remove` O(1)
- [x] Update `GetGraph` to insert new entries with `PushBack` + map set
- [x] Update `InvalidateShard` to use new removal
- [x] Update `InvalidateAll` to reinitialize list + map
- [x] Update `pip_shard_manager_test.go` to use new LRU structure
- **File**: `backend/services/policy/internal/ngac/pip_shard_manager.go`

### Task 6: DRY Cache Invalidation Loops
- [x] Extract `collectAndDelete(ctx, pipe, patterns...) int` helper method
- [x] Refactor `deleteTargetedKeys` to use helper
- **File**: `backend/services/policy/internal/ngac/epp_cache_invalidator.go`

### Task 7: Remove Dead Code
- [x] Remove `GetUserByNGACNodeID` method from `pip_store.go`
- [x] Remove `pgx` import (no longer needed)
- [x] Verify no callers exist within policy service
- **File**: `backend/services/policy/internal/ngac/pip_store.go`

### Task 8: Fix Raw Node Type Strings in gRPC
- [x] Replace `"UA"` with `ngac.NodeTypeUserAttribute` in `write_server.go`
- [x] Replace `"U"` with `ngac.NodeTypeUser` in `write_server.go`
- **File**: `backend/services/policy/internal/grpc/write_server.go`

### Task 9: Verify
- [x] `go build ./...` — ✅ passes
- [x] `go vet ./...` — ✅ passes
- [x] `go test ./... -v -count=1` — ✅ all tests pass (including updated LRU tests)
- [x] Confirm all existing tests pass with no behavioral changes
