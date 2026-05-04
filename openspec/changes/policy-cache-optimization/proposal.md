# Policy Cache Optimization — Targeted Invalidation

## Evidence Summary
- Backend: **EXISTS** — `backend/services/policy/internal/grpc/` 
  - `server.go`: Legacy PolicyServer with L1 Redis-only cache
  - `read_server.go`: ReadServer with 3-layer cache (L1 Redis → L2 Materialized → L3 BFS)
  - `write_server.go`: WriteServer with targeted L2 + full-flush L1
- Proto: **EXISTS** — `backend/proto/policy/` (CheckAccess, BatchCheckAccess)
- DB: **EXISTS** — `ngac_graph_version` table, `ngac_materialized_access` table
- Redis: **EXISTS** — key format: `ngac:access:{user}:{obj}:{op}`, TTL 30s
- Frontend: **NOT APPLICABLE** — backend-only change, no UI

## Problem Statement

### Current Behavior
Mỗi graph mutation (CreateAssignment, RemoveAssignment, CreateAssociation, etc.) → **flush TOÀN BỘ Redis L1 cache** bằng `SCAN "ngac:access:*" → DEL`. 

**Evidence**:
- `server.go:271-287` — `invalidateCache()` uses wildcard SCAN + DEL
- `write_server.go:178-197` — `invalidateRedisCache()` same pattern

### Impact
- 1 user thay đổi quyền → 200 users mất cache → cache stampede  
- SCAN trên production Redis là O(N) blocking command
- Mọi service (Drive ~16 checkAccess calls, Messaging ~4, Approval ~1) đều phải recompute

### Why Now
- Graph mutations hiện rất ít (vài lần/ngày), nhưng sharing flow mới sẽ tăng tần suất
- Hiện có 2 server implementations (legacy + split) cùng pattern flush — cần chuẩn hóa

## Product Assessment
- **Size**: M (backend refactor, 4-6 files, no proto change, no frontend)
- **Risk**: Medium (cache invalidation là critical correctness — stale cache = security issue)
- **Target user**: Tất cả user — performance tăng, không có thay đổi visible
- **Core action**: CheckAccess nhanh hơn sau graph mutation, không bị cache stampede

## Scope

### In scope
1. **Targeted Redis L1 invalidation**: Thay SCAN wildcard bằng xóa key cụ thể theo affected nodeIDs
   - Resolve affected users/objects từ graph (ancestors/descendants)
   - Chỉ xóa keys `ngac:access:{affected}:*`
2. **Chuẩn hóa invalidation logic**: Cả `PolicyServer` và `WriteServer` dùng chung targeted strategy
3. **Prometheus metrics**: Thêm metrics cho cache hit/miss, invalidation count, checkAccess latency
4. **SCAN elimination**: Dùng Redis Hash hoặc prefix-based UNLINK thay vì SCAN

### Out of scope
- Multi-region / distributed cache (Phase 3 — chưa cần)
- Per-workspace version tracking (Phase 2b — chưa cần)  
- Frontend changes (không có UI)
- Proto changes (API không đổi)

### Deferred
- Event-driven cache invalidation qua Redpanda (khi scale > 5000 users)
- Per-service local LRU cache (khi cần giảm gRPC calls)

## Success Criteria
1. Graph mutation KHÔNG còn SCAN Redis — verify bằng `MONITOR` command
2. Cache invalidation chỉ xóa keys liên quan — verify bằng test scenario: 
   - User A thay đổi → User B cache vẫn còn
3. Build PASS: `go build ./...`
4. Existing tests PASS: `go test ./services/policy/...`
5. Metrics endpoint expose cache hit/miss ratio
