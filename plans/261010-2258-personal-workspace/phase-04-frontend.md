---
phase: 4
title: "Ẩn ở frontend"
status: pending
priority: P2
effort: "1d"
dependencies: [2]
---

# Phase 04: Ẩn ở frontend

## Requirements
- Theo `useWorkspaceType()`: ẩn Tài sản và Phê duyệt ở `AppSidebar.tsx`, `MobileMoreSheet.tsx`; `MobileNav.tsx`
  thay tab Phê duyệt bằng Danh bạ; ẩn duyệt ở `DocumentScreen.tsx`, `TextsTable.tsx`; Quản trị (`UserPanel.tsx`,
  `UsersTable.tsx`) bỏ phần phê duyệt; menu "+" của chat bỏ "Đề nghị phê duyệt" (plan chat phase 11).
- Route guard: `/assets`, `/approval` ở workspace cá nhân → chuyển về `/` (không trang lỗi).
- Test vitest cho cả hai loại workspace.

## Success Criteria
- [ ] Không còn lối vào Tài sản/Phê duyệt ở workspace cá nhân; workspace tổ chức như cũ; gates xanh.
