# Session Dialogue: Admin Organization Management

> Evidence-based audit trail. Write-only: Orchestrator. Read-only: Human.
> KHÔNG narrative. KHÔNG roleplay. CHỈ structured reasoning.

---

## Decisions

### D-001 — Phase 1 (CEO): Scope quyết định — mở rộng workspace service thay vì tạo service mới

Quyết định:
- Admin module sẽ mở rộng workspace service thay vì tạo service mới cho department
- Bằng chứng: file: `backend/proto/workspace/workspace.proto`, vị trí: lines 24-35, quan sát: workspace.proto đã có CreateRole, ListRoles, DeleteRole, ListMembers, UpdateMemberRoles — role + member management đã thuộc workspace service
- Bằng chứng 2: file: `backend/services/workspace/internal/rest/handler.go`, vị trí: lines 42-54, quan sát: REST handler đã mount role/member endpoints trên `/api/workspaces/:id/`
- Xác nhận: đã kiểm tra workspace service structure (domain, grpc, rest, store), kết luận: workspace service là nơi phù hợp nhất vì department thuộc organization context
- Trạng thái: VERIFIED
- Độ tin cậy: HIGH

Phương án loại bỏ: Tạo admin service riêng — rejected vì sẽ duplicate member/role logic đã có trong workspace service

### D-002 — Phase 1 (CEO): Department modeled as NGAC UA node hierarchy

Quyết định:
- Department sẽ được model bằng NGAC UA nodes (giống cách roles đã được model)
- Bằng chứng: file: `backend/ngac/ngac_ops.go`, vị trí: lines 96-103, quan sát: TypeUA = "UA" đã tồn tại, roles đã dùng UA nodes
- Bằng chứng 2: file: `data/init.sql`, vị trí: lines 16-21, quan sát: ngac_assignments table hỗ trợ child-parent relationship — perfect cho department hierarchy
- Xác nhận: đã kiểm tra NGAC model hoàn chỉnh (nodes, assignments, associations), kết luận: UA node hierarchy là cách đúng để model department tree
- Trạng thái: VERIFIED
- Độ tin cậy: HIGH

### D-003 — Phase 1 (CEO): Size L, Risk Medium

Quyết định:
- Đánh giá Size: L (backend cần department proto + handlers mới, frontend cần admin module hoàn chỉnh với 3 sections)
- Risk: Medium (backend role/member APIs tồn tại nhưng department hierarchy là logic mới)
- Bằng chứng: file: `frontend/src/routes/_workspace`, vị trí: directory listing, quan sát: không có admin route, không có admin components — frontend cần module hoàn toàn mới
- Xác nhận: đã kiểm tra cả frontend và backend structure, kết luận: effort chủ yếu ở frontend (new module) + backend department CRUD
- Trạng thái: VERIFIED
- Độ tin cậy: HIGH

---

## Handoffs

### H-001 — CEO → BA
Chuyển giao:
- CEO → BA: proposal.md đã validate — backend có sẵn role/member APIs, cần thêm department CRUD, frontend cần admin module mới

---

## Feedback Loops

## Deviations

## Red Flags

| # | Type | Description | Location |
|---|------|-------------|----------|
