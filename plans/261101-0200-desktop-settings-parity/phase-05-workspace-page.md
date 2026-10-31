# Phase 05 · Cài đặt workspace on desktop

## Context
Phone `WorkspaceSettingsScreen.tsx` has every row of DESIGN.md §6 "Cài đặt workspace" (mockup mobile-shell §11), rows
gated by permission. Desktop `WorkspaceTab.tsx` has the name/description form, AdminPointer, WorkspaceTypeCard,
LeaveWorkspace section. Mockup screen 5.

## Requirements
1. Wide `WorkspaceSettingsScreen`: header (letter tile, name, kind line, role pill "Bạn là <vai trò>"); **Thông tin** as
   a form (Tên; Mô tả for org) with Lưu thay đổi when `can_manage`, else read-only fields with "Do quản trị viên đặt";
   then the same rows and permission rules as the phone: Quản trị viên (member), Thành viên/Mời thành viên (opens
   `InviteDialog` in place)/Phòng ban/Vai trò và quyền/Phạm vi danh bạ (admin), Quy định: Ai được tạo nhóm
   (`SettingsChoice`, admin; locked row for others), Hồ sơ trong tổ chức link (member), Phân hệ: Mẫu phê duyệt, Loại tài
   sản, Dung lượng Tài liệu; **Loại workspace** (WorkspaceTypeCard), Rời <workspace> (`LeaveWorkspace` section variant,
   org only).
2. Quản trị viên at wide: the avatar cluster row expands an inline list (no subpage).
3. Personal: Tên workspace, Dung lượng Tài liệu, Loại workspace. No Rời.
4. Desktop links go to the existing desktop admin routes (`/admin/users`, `/admin`, `/admin/roles`,
   `/admin/directory`, `/approval?tab=templates`, `/assets?section=types`).

## Tests first
- Admin org at wide: all admin rows with counts; Mời opens InviteDialog; Ai được tạo nhóm Select saves via
  `useSetGroupCreation`.
- Member org at wide: read-only info, Quản trị viên list, locked rule row, Rời; no admin rows.
- Personal at wide: name, quota, type card; no Rời.
- No raw IDs rendered (use `expectNoIds`).
