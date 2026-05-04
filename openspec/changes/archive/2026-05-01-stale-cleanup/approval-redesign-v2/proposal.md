# Approval Module Redesign v2 — Enterprise-Grade Approval Workflow

## Bối cảnh CEO

Tôi là CEO Samsung Vietnam. Công ty có 500+ nhân viên, 8 phòng ban (Kinh doanh, Kỹ thuật, Tài chính, Nhân sự, Marketing, R&D, Pháp lý, Hành chính). Mỗi ngày có 50-100 phiếu duyệt: mua vật phẩm, tờ trình chi tiền, đề xuất nhân sự, ký hợp đồng.

**Nhu cầu thực tế:**
- Bộ phận Kinh doanh cần tạo phiếu chi tiền mua quà khách hàng → Trưởng phòng duyệt → CFO duyệt nếu > 10 triệu
- Bộ phận Kỹ thuật cần tờ trình mua server → Trưởng phòng → IT Director → CEO nếu > 50 triệu
- HR cần phiếu nghỉ phép, phiếu tuyển dụng → Manager duyệt
- Mỗi loại phiếu có **form fields khác nhau** (đề xuất mua vật phẩm cần: tên sản phẩm, số lượng, đơn giá, tổng tiền, lý do; tờ trình cần: tiêu đề, nội dung, ngân sách, deadline)

**User thực:** viethoangnguyenle@gmail.com — member company, vai trò Sales Executive trong phòng Kinh doanh

## Evidence Summary

- **Proto**: ✅ **COMPLETE** — `approval.proto` với 13 RPCs: template CRUD (4), request lifecycle (4), query tabs (4), audit (1)
- **Backend**: ✅ **COMPLETE** — Full service: domain, store, REST handler (18 endpoints), gRPC, event consumer, tenant-scoped schema
- **DB**: ✅ **COMPLETE** — Migration 007 với 6 tables (templates, conditions, steps, requests, assignments, audit_log) + indexes
- **Frontend**: ⚠️ **EXISTS but BROKEN** — Route + components exist nhưng:
  - UI không consistent với app design system
  - Thiếu Template Management UI (CRUD)
  - Thiếu Create Request form (chỉ có list/approve/reject)
  - Thiếu dynamic form fields (user không thể nhập dữ liệu khi tạo phiếu)
  - ApprovalTable tự build, không dùng DataTable composite
  - ApprovalDetailPanel tự build, không dùng PeekPanel composite
- **Dynamic Form Fields**: ❌ **MISSING** —
  - Proto chỉ có `entity_fields_json` (raw JSON) — không có schema validation
  - Backend không có form field definition trong template
  - Không có frontend form renderer
  - **Đây là GAP lớn nhất — cần proto change + backend + frontend**

## Product Assessment

- **Size**: **XL** — Proto change + Backend extension + Complete frontend rewrite + Dynamic form system
- **Risk**: **Medium** — Backend base solid nhưng cần proto changes cho dynamic form. Frontend cần rewrite hoàn toàn.
- **Target users**:
  - **Employee** (Sales Exec, Engineer): Tạo phiếu duyệt, theo dõi trạng thái
  - **Manager** (Trưởng phòng): Duyệt/từ chối phiếu, batch approve
  - **Admin** (HR Director, CFO): Tạo/quản lý template, xem all department requests
- **Core actions**: Tạo phiếu → Duyệt/Từ chối → Theo dõi → Quản lý template

## Scope

### In scope

#### 1. Dynamic Form Field System (NEW — Proto + Backend + Frontend)
- Extend `ApprovalTemplate` proto: thêm `repeated FormFieldDefinition form_fields`
- FormFieldDefinition: `name, label, field_type (text/number/currency/date/select/textarea/file), required, options_json, validation_rules`
- Extend `ApprovalRequest` proto: store `form_data` as structured JSON matching template fields
- Backend: validate form data against template field definitions on create
- Frontend: Dynamic form renderer component

#### 2. Template Management UI (Admin CRUD)
- List templates — DataTable with name, entity_type, status, step count
- Create template — Modal/page with form builder: add fields, add steps, set conditions
- Edit template — Same form, pre-populated
- Delete/deactivate template — ConfirmDialog

#### 3. Create Approval Request UI (Employee)
- Select template type → renders dynamic form
- Fill in form fields → submit
- Shows step preview (who will approve)

#### 4. Approval List Redesign (All roles)
- Rewrite using DataTable composite (not custom ApprovalTable)
- 4 tabs: Pending, History, My Requests, Department
- Detail panel using PeekPanel composite (not custom panel)
- Batch actions for Manager/Admin
- Consistent with Drive/Assets table density

#### 5. Approval Detail & Actions
- PeekPanel with: form data display, step timeline, approve/reject with comment
- Audit log inline display
- Status badges consistent with design system

#### 6. Full CRUD Compliance
Every entity must support full CRUD:
- **Templates**: Create, Read (list + detail), Update, Delete/Deactivate
- **Requests**: Create, Read (4 tabs + detail), Update (approve/reject), Cancel (creator can cancel pending)
- **Form Fields**: Defined in template, rendered on create, displayed on detail

### Out of scope
- **Workflow designer** (drag-and-drop visual step builder) — too complex for v2, simple ordered list is sufficient
- **File attachments** in form fields — requires Drive integration, defer to v3
- **Notification system** — requires WebSocket integration, separate module
- **Request delegation** (reassign to another approver) — defer
- **SLA/escalation** (auto-approve after timeout) — backend has `timeout_hours` field, UI deferred

### Deferred
- Workflow visual builder
- File attachment fields
- Real-time notifications
- Request delegation
- SLA escalation UI
- Analytics dashboard (approval metrics)
- Mobile swipe-to-approve

## Success Criteria

1. Admin can CRUD approval templates with dynamic form fields
2. Employee can create request by selecting template, filling dynamic form, and submitting
3. Manager can view pending tab, approve/reject with comment, batch approve
4. Detail panel shows form data + step timeline + audit log
5. All views use existing DataTable, PeekPanel, Tabs, Badge components — zero custom table/panel
6. All views responsive at 375px, 768px, 1280px
7. QC tests with 4-5 users covering all CRUD actions
8. Form field validation works (required fields, number ranges, etc.)
