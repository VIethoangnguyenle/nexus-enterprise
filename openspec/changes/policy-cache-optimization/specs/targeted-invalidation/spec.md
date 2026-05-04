# Spec: Targeted Cache Invalidation

## Overview
Thay thế full-flush Redis invalidation bằng targeted deletion theo affected nodeIDs khi graph mutation xảy ra.

## Proto Mapping
- **CheckAccess** (`PolicyReadService.CheckAccess`) — không thay đổi API
- **CreateAssignment** (`PolicyWriteService.CreateAssignment`) — thay đổi invalidation logic
- **RemoveAssignment** (`PolicyWriteService.RemoveAssignment`) — thay đổi invalidation logic
- **CreateAssociation** (`PolicyWriteService.CreateAssociation`) — thay đổi invalidation logic
- **RemoveAssociation** (`PolicyWriteService.RemoveAssociation`) — thay đổi invalidation logic
- **DeleteNode** (`PolicyWriteService.DeleteNode`) — thay đổi invalidation logic
- **LoadGraph** — giữ full flush (reload toàn bộ graph)

## User Stories

### Story 1: Targeted invalidation on assignment change
**As a** system operator,
**I want** only affected users' cache entries invalidated when an assignment changes,
**so that** unrelated users' cached decisions remain intact.

**Acceptance Criteria:**
- [ ] AC1: Khi `CreateAssignment(childID=U_A, parentID=UA_X)` → chỉ xóa Redis keys có chứa `U_A` hoặc descendants của `UA_X`
- [ ] AC2: Redis keys của user B (không liên quan) vẫn tồn tại sau mutation
- [ ] AC3: Không dùng `SCAN` command — verify bằng Redis MONITOR
- [ ] AC4: `DENY` cache bị xóa đúng khi user mới được gán quyền (tránh stale DENY)

### Story 2: Targeted invalidation on association change
**As a** system operator,
**I want** only affected OA-related cache entries invalidated when an association changes,
**so that** permission changes reflect immediately cho đối tượng liên quan.

**Acceptance Criteria:**
- [ ] AC1: Khi `CreateAssociation(UA_X, OA_Y, ["read"])` → xóa keys có `OA_Y` hoặc ancestors/descendants của `OA_Y`
- [ ] AC2: Keys liên quan đến OA_Z (khác scope) vẫn tồn tại
- [ ] AC3: Stale window ≤ 30s (TTL vẫn hoạt động như safety net)

### Story 3: Full flush on LoadGraph
**As a** system operator,
**I want** LoadGraph vẫn flush toàn bộ cache,
**so that** graph reload luôn bắt đầu với clean state.

**Acceptance Criteria:**
- [ ] AC1: `LoadGraph()` vẫn xóa tất cả `ngac:access:*` và `scopes:*`
- [ ] AC2: Đây là trường hợp DUY NHẤT dùng full flush

### Story 4: Chuẩn hóa giữa PolicyServer và WriteServer
**As a** developer,
**I want** cả `PolicyServer` (server.go) và `WriteServer` (write_server.go) dùng chung targeted invalidation,
**so that** không có behavior mismatch giữa 2 implementations.

**Acceptance Criteria:**
- [ ] AC1: Cả 2 server implementations gọi cùng hàm invalidation
- [ ] AC2: Shared function nhận `nodeIDs ...string` và resolve affected keys
- [ ] AC3: Build pass: `go build ./services/policy/...`
- [ ] AC4: Existing tests pass: `go test ./services/policy/...`

## Flow: Targeted Invalidation

```
1. WriteServer.CreateAssignment(childID, parentID)
2. store.CreateAssignment() → graph updated
3. version.Increment("global") → L2 stale detection
4. materialized.InvalidateByUser(childID) → L2 targeted
5. invalidateRedisForNodes(childID, parentID):
   a. Resolve affected keys:
      - Nếu childID là U type → keys = "ngac:access:{childID}:*"
      - Nếu childID là UA type → resolve descendants(childID) → lấy U nodes → keys per user
      - Nếu parentID là OA type → keys = "ngac:access:*:{parentID}:*"
   b. Redis DEL keys (direct, không SCAN)
   c. Redis DEL "scopes:{affected_users}:*"
6. producer.PublishGraphMutated()
```

## Edge Cases
- **Node không tìm thấy trong graph**: Skip invalidation cho node đó (log warning)
- **Nhiều assignments cùng lúc**: Batch collect nodeIDs → 1 lần invalidation
- **Redis connection down**: Fail silently (TTL 30s = safety net, L2 version check = backup)
- **LoadGraph**: Full flush — đây là exception có chủ đích
