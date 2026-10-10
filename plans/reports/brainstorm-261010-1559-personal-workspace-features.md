---
title: "Personal workspaces: no assets, no approvals"
type: brainstorm
status: accepted
created: 2026-10-10
---

# Brainstorm — Workspace cá nhân không có Tài sản và Phê duyệt

## Evidence (main @ 61616dc)
- `workspaces.type` ('personal'|'organization') chỉ được gán 'organization' khi Google `hd` nhận tên miền
  (`auth/internal/store/identity.go`); mọi đường khác mặc định 'personal' → cột hiện không tin được.
- Proto `Workspace` và `frontend/src/api/workspaces.ts` không trả loại.
- Lối vào Tài sản: `AppSidebar.tsx`, `MobileMoreSheet.tsx`, route `/assets`.
- Lối vào Phê duyệt: `AppSidebar.tsx`, `MobileNav.tsx` (1/3 tab chính + badge), route `/approval`,
  duyệt tài liệu (`DocumentScreen.tsx`, `TextsTable.tsx`), Quản trị (`UserPanel.tsx`, `UsersTable.tsx`).
- Service asset/approval không phân biệt loại workspace.

## Contract
- **Outcome:** Ở workspace cá nhân, Tài sản và Phê duyệt (gồm duyệt tài liệu) không xuất hiện ở đâu, server
  từ chối mọi thao tác của chúng; workspace tổ chức không đổi.
- **Constraints:** loại đúng cho workspace mới và cũ; TDD + test deny; migration áp + kiểm chứng (backup trước);
  `make proto` nếu sửa `workspace.proto`; không UUID trên UI; spec `tenant-identity` / `asset-authorization`.
- **Non-goals:** đổi tính năng bên trong Tài sản/Phê duyệt; xoá dữ liệu tài sản/đề nghị có sẵn ở workspace cá
  nhân (chỉ ẩn).
- **Acceptance:** workspace cá nhân — không có mục ở sidebar/mobile/màn khác; `/assets`, `/approval` chuyển về
  trang chính; API asset/approval trả 403. Workspace tổ chức như cũ. Test cả hai loại, backend + frontend.

## Decisions (người dùng chốt)
1. **Định nghĩa (A):** workspace hệ thống tự tạo khi đăng nhập lần đầu = cá nhân; "Tạo workspace" và Google
   theo tên miền = tổ chức. Gán loại ngay lúc tạo. Migration workspace cũ: tên dạng "<tên>'s Workspace" + 1
   thành viên → cá nhân, còn lại → tổ chức.
2. **Chuyển đổi:** chủ workspace chuyển cá nhân → tổ chức trong Cài đặt (một chiều).
3. **Mobile:** ở workspace cá nhân, tab Phê duyệt thay bằng **Danh bạ**.
4. **Duyệt tài liệu:** ẩn ở workspace cá nhân.

## Order
Làm sau Chat (người dùng: Chat quan trọng nhất).

## Unresolved questions
- Workspace cá nhân có được mời thêm người không? (Chưa hỏi lại; giữ nguyên hành vi hiện tại.)
