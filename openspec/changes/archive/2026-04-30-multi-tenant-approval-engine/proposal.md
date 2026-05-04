# Multi-Tenant Approval Workflow Engine

## Problem

Hệ thống NGAC hiện tại có 3 bottleneck nghiêm trọng khi scale tới multi-tenant enterprise (ngân hàng, doanh nghiệp lớn):

1. **Per-item CheckAccess loop**: `ListFolder` query tất cả items rồi loop gọi `policyRead.CheckAccess()` cho TỪNG item (server.go:178-187). Với hàng ngàn giao dịch/ngày → O(N × gRPC_latency) → timeout.

2. **In-memory graph không scale**: `LoadGraph()` load toàn bộ nodes vào RAM. Với chục triệu objects × chục ngàn tenants → OOM. Cần tách structural nodes (nhỏ, vào RAM) khỏi data objects (lớn, ở DB).

3. **Thiếu approval workflow**: Doanh nghiệp cần dynamic multi-step approval flows (custom theo số tiền, dịch vụ, role, department) mà NGAC graph thuần túy không model hiệu quả cho per-item dynamic assignment.

4. **Thiếu department visibility hierarchy**: Manager Miền Nam chỉ nên thấy giao dịch miền Nam. CEO thấy tất cả. Hiện tại không có cơ chế "ai nhìn data của department nào" mà không dùng per-item CheckAccess.

## Solution

Hybrid architecture gồm 3 thành phần:

### 1. NGAC Policy Engine (refactor, structural only)
Giữ nguyên vai trò PDP nhưng refactor graph chỉ chứa structural nodes (UA, OA, PC, associations, ~30-50 nodes/tenant). KHÔNG load O nodes (transactions, documents) vào graph.

OA hierarchy phản ánh cấu trúc tổ chức với **regional grouping**:
```
Root → HoiSo, MienNam_Region, MienBac_Region
MienNam_Region → CN_HCM, CN_CanTho
MienBac_Region → CN_HaNoi, CN_HaiPhong
```

Thêm RPC `ResolveAccessibleScopes`: resolve user → accessible OA IDs (bao gồm descendant resolution). Cache Redis 60s.

### 2. Approval Workflow Engine (new service)
Template-driven, per-item dynamic approval:
- Admin tạo workflow templates (conditions + steps + approver rules)
- Approver types: `specific_user`, `role_in_dept`, `department`, `creator_manager`
- Denormalized `approval_assignments` table cho fast query (zero graph traversal)
- Point-in-time snapshot + event-driven reconciliation khi policy thay đổi
- 4 query tabs: Chờ duyệt (load all), Đã duyệt (cursor), Lệnh tôi tạo (cursor), Tất cả lệnh department (scope-based cursor)

### 3. Schema-per-tenant (infrastructure)
Mỗi doanh nghiệp = 1 PostgreSQL schema. Data isolation hoàn toàn (giao dịch, tài liệu, approval flows). `SET search_path` per request.

### Cầu nối: `scope_oa_id`
Mọi data entity có `scope_oa_id` — link tới OA hierarchy. Đây là concept **universal** thay thế per-item CheckAccess bằng `WHERE scope_oa_id = ANY(user_scopes)`. Áp dụng cho transactions, documents, approval_requests, drive_items.

## Key Decisions (từ explore session)

| Decision | Kết luận | Lý do |
|----------|----------|-------|
| Reject behavior | Terminal — lệnh kết thúc | Simplicity, banking compliance |
| Delegation | Không cho phép | Strict workflow compliance |
| Batch approval | Có | UX: Kế toán trưởng duyệt 50 lệnh cùng lúc |
| Pending data paging | Load all, no paging | Dataset nhỏ per user (10-50 items) |
| History visibility | Chỉ người đã action thấy | Status IN ('approved','rejected'), skipped không thấy |
| Dept visibility | scope_oa_id + ResolveAccessibleScopes | Zero graph traversal, cached |
| Policy change | Eventual consistency (seconds) + realtime write check | Balance perf vs correctness |
| Template versioning | Snapshot at creation time | Pending requests dùng template cũ |

## Scope

### In scope
- Schema-per-tenant infrastructure (provisioning, migration, routing)
- NGAC graph refactor (structural only + regional OA hierarchy)
- `ResolveAccessibleScopes` RPC with descendant resolution + Redis cache
- `scope_oa_id` on all data entities (universal bridge NGAC → SQL)
- Approval template engine (conditions, steps, approver resolution)
- Approval execution runtime (create, approve, reject, batch approve)
- 4 query tabs with appropriate paging strategies
- Event-driven reconciliation on policy change
- Audit trail (append-only log, full traceability)

### Out of scope
- Frontend UI cho approval workflow (separate change)
- SLA tracking/reporting (optional, later)
- Delegation/proxy approval (rejected by requirement)
- Real-time WebSocket notifications (separate change)
- Migration existing drive data to tenant schemas
- Conditional branching on reject (terminal by design)

## Impact
- **Services affected**: Policy Service (refactor graph, add ResolveAccessibleScopes), Drive Service (refactor ListFolder to scope-based), new Approval Service, Database schema architecture
- **Risk**: High — fundamental architecture change, requires careful phased rollout
- **Users affected**: All enterprise tenants — enables regional department visibility, dynamic approval workflows, and high-performance data access at scale
