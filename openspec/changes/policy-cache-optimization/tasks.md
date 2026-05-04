# Tasks: Policy Cache Optimization

## Task 1: Create cache invalidator module
- [x] Create `policy/internal/ngac/cache_invalidator.go` with targeted invalidation logic
- [x] Implement `ResolveAffectedKeys()` — resolve nodeIDs → Redis key patterns
- [x] Handle U, UA, OA, PC node types differently
- [x] PC change → signal full flush

## Task 2: Create metrics module
- [x] Add `prometheus/client_golang` to go.mod
- [x] Create `policy/internal/metrics/metrics.go` with counter/histogram definitions
- [x] Define: check_access_total, check_access_duration, cache_invalidation_total, cache_keys_deleted, graph_node_count

## Task 3: Refactor WriteServer invalidation
- [x] Replace `invalidateRedisCache()` full flush with targeted `CacheInvalidator.InvalidateForNodes()`
- [x] Keep `invalidateAllCaches()` for LoadGraph (full flush)

## Task 4: Refactor PolicyServer (legacy) invalidation
- [x] Replace `invalidateCache()` full flush with shared `CacheInvalidator.InvalidateForNodes()`
- [x] Keep LoadGraph full flush

## Task 5: Add metrics to ReadServer
- [x] Instrument CheckAccess with L1/L2/L3 hit tracking
- [x] Add latency histogram per layer

## Task 6: Wire up in main.go
- [x] Create CacheInvalidator in main.go, pass to servers
- [x] Register Prometheus metrics
- [x] Verify build passes

## Task 7: Verify
- [x] `go build ./...` ✅
- [x] `go test ./...` ✅ (14/14 pass)
- [x] `go vet ./...` ✅
- [x] Cross-service builds ✅ (all 7 services)
