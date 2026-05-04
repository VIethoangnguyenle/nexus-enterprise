## Why

Policy service hiện tại load **toàn bộ NGAC graph** (tất cả tenants) vào RAM khi khởi động. Với target scale 50K workspaces × 1000 users × 100 phòng ban, tổng graph đạt ~70M nodes + 120M edges → **~62GB RAM/instance**. Hệ thống chạy 3 instances (2 Read + 1 Write) → ~186GB RAM chỉ cho graph — hoàn toàn không khả thi.

Ngoài ra, thuật toán PC intersection hiện tại (`findCommonPC`) chỉ check "ít nhất 1 PC chung", vi phạm NIST NGAC spec yêu cầu "TẤT CẢ PC của object phải nằm trong PC set của user". Khi policy_service là common service hỗ trợ Multi-PC (Confidential, PCI, Project), bug này trở thành critical.

## What Changes

- **Sharded Graph**: Thay `LoadGraph()` (load tất cả) bằng `LoadShard(tenantPCID)` — mỗi shard chứa đúng 1 tenant + secondary PCs + PC_Global. LRU eviction giữ top ~1000 active shards trong RAM (~1.5GB thay vì 62GB).
- **CTE SQL Fallback**: Tạo `ngac_check_access` SQL function (hiện **chưa tồn tại** dù code gọi nó). Đây là safety net cho cold workspaces khi shard chưa load.
- **ALL-PC Intersection**: Fix `findCommonPC` → `allPCsSatisfied` — enforce `objectPCs ⊆ userPCs` đúng NIST spec.
- **PC Metadata**: Thêm `properties.tenant_id` để biết secondary PCs (Confidential, PCI) thuộc tenant nào. PC chỉ system/tenant-admin tạo.
- **Workspace Routing**: Thêm `workspace_id` vào `CheckAccessRequest` proto để route request tới đúng shard.
- **PC Authorization Guard**: Chặn CreateNode type=PC từ non-admin callers.

## Capabilities

### New Capabilities
- `sharded-graph`: Per-tenant graph sharding with LRU eviction, lazy-load, và shard lifecycle management
- `cte-fallback`: SQL function `ngac_check_access` cho cold-path evaluation khi shard chưa load
- `multi-pc-intersection`: ALL-intersection algorithm đúng NIST NGAC spec cho Multi-PC scenarios
- `pc-authorization`: Authorization guard và metadata cho PC lifecycle (system-created vs tenant-admin-created)

### Modified Capabilities
- `batch-access-check`: Thêm `workspace_id` field vào CheckAccessRequest proto cho shard routing

## Impact

- **Proto**: `CheckAccessRequest` thêm field `workspace_id` — tất cả callers (messaging, drive, approval, asset) cần update
- **Policy service**: Refactor core graph loading, access algorithm, decision engine
- **Database**: Tạo CTE SQL function, có thể thêm index cho shard loading performance
- **All gRPC callers**: Truyền thêm `workspace_id` khi gọi CheckAccess — breaking change nhưng backward-compatible (field optional, fallback CTE nếu không có)
- **Memory**: Giảm từ ~62GB/instance → ~1.5GB/instance
- **Startup**: Giảm từ ~30s → instant (lazy-load)
