# Session Dialogue: UI Consistency Fix

> Evidence-based audit trail. Write-only: Orchestrator. Read-only: Human.
> KHÔNG narrative. KHÔNG roleplay. CHỈ structured reasoning.

---

## D-001 — Phase 1 (CEO): Scope expanded per user requirements

Quyết định:
- Cập nhật proposal.md v2 với system-level requirements từ user: Stitch as SOT, UI spec layer, QA validation, forbidden patterns
- Bằng chứng: file: `openspec/changes/ui-consistency-fix/proposal.md`, vị trí: toàn bộ file, quan sát: proposal v1 chỉ list issues, v2 thêm root cause analysis + 4 system rules (R1-R4)
- Xác nhận: đã kiểm tra user request matches proposal scope, kết luận: xác nhận — scope vẫn M (frontend-only), không backend
- Trạng thái: VERIFIED
- Độ tin cậy: HIGH

Phương án loại bỏ: Không tạo change mới — user yêu cầu tiếp tục change hiện tại với requirements mở rộng

---

## D-002 — Phase 2 (BA): Gap detected — Documents route has no Stitch UI

Quyết định:
- Phát hiện `/documents` route có full UI (upload, table, download) nhưng KHÔNG có Stitch design
- Hành động: Gen Stitch screen "Documents Management" via MCP → downloaded `13-documents-management.png`
- Bằng chứng: file: `frontend/src/routes/_workspace/documents.tsx` (136 lines), Stitch project có 40 screens nhưng không có "Documents"
- `/settings` là placeholder "coming soon" → skip
- Trạng thái: VERIFIED
- Độ tin cậy: HIGH

---

## H-001 — CEO → BA

Chuyển giao:
- CEO → BA: proposal.md v2 đã validate — scope M, risk Low, 3 modules (approval/chat/drive), frontend-only, no backend changes

---

## D-003 — Phase 2 (BA): Spec Review Complete

Quyết định:
- Tạo 3 spec files: drive-stitch-compliance, chat-stitch-compliance, cross-module-consistency
- Tổng cộng 8 user stories (US-1 to US-8)
- Approval module: ✅ đã compliant — dùng Heading, Button, IconButton, DataTable, Tabs (verified line-by-line)
- Drive module: ⚠️ nhiều raw `<button>` (28 instances across 8 files) — cần fix
- Chat module: ⚠️ section headers cần standardize
- Cross-module: cần đồng bộ sidebar labels, page headers, action buttons
- Stitch coverage: 13 screens downloaded (12 existing + 1 generated for Documents)
- Trạng thái: VERIFIED
- Độ tin cậy: HIGH

---

## SPEC-LOCK — Phase 2 Complete

Spec locked at: 2026-05-02T14:02:00+07:00
Files:
- specs/drive-stitch-compliance/spec.md
- specs/chat-stitch-compliance/spec.md
- specs/cross-module-consistency/spec.md
- stitch-designs/ (13 screenshots)

