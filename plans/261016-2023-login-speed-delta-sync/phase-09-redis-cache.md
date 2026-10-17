---
phase: 9
title: "Cache Redis có phiên bản + danh bạ lô lớn"
status: pending
priority: P2
effort: "1d"
dependencies: [4]
---

# Phase 09: Cache Redis cho dữ liệu đọc nhiều

## Requirements
- Danh bạ workspace (`GET /workspaces/:id/contacts`): cache Redis khoá (tenant, workspace, phiên bản); phiên bản tăng
  bởi sự kiện thành viên/hồ sơ (dùng phiên bản của phase 04). Danh bạ đã lọc theo quyền thì khoá thêm user; nếu kết quả
  phụ thuộc quyền theo người xem → không cache chung giữa người dùng.
- Unread theo người dùng: cache ngắn (TTL + xoá theo sự kiện).
- `useContacts`: `limit` lớn (vd 500, server giới hạn) thay 50/trang tuần tự (`useContacts.ts:58-66`).
- Quyết định quyền **không** cache ở service (policy đã có cache + EPP invalidation).

## Implementation Steps
1. Đo trước: p95 contacts/unread trên production (log) — chỉ làm mục nào p95 > 50 ms hoặc gọi nhiều; còn lại ghi lý do bỏ.
2. Test trước: cache trúng không chạm DB; sự kiện đổi → trượt; **deny**: user không có quyền không nhận bản cache của
   người khác; tenant khác không trúng.
3. Viết + spec.

## Related Code Files
- Modify: service sở hữu contacts (kiểm `vite.config.js`: `/contacts` → 8180 auth), messaging unread, `frontend/src/hooks/useContacts.ts`

## Success Criteria
- [ ] Số đo p95 trước/sau trong report.

## Risk Assessment
- Cache sai audience → lộ dữ liệu. Khoá luôn gồm tenant; test deny bắt buộc.
