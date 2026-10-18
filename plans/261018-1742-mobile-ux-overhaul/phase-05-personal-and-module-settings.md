---
phase: 5
title: "Tôi, workspace and module settings"
status: pending
priority: P1
effort: "3.5d"
dependencies: [2, 4, 8]
---

# Phase 05: Tôi, workspace and module settings

## Overview
The Tôi page (avatar) replaces the four settings entry points on phones; module settings pages open from Tôi and
from each module's ⋯; Cài đặt workspace gathers the workspace's own settings (and Quản trị on phones); the
workspace-kind conversion is removed.

## Requirements
- Functional: Tôi = profile card (status) · Tạm dừng thông báo · **Tài khoản** (Tài khoản và hồ sơ, Thông báo,
  Giao diện và ngôn ngữ, Thiết bị và phiên) · **Trong workspace này** (org: Hồ sơ trong tổ chức; Chat, Lịch, Tài
  liệu, Phê duyệt*, Tài sản*, Thanh đáy) · Cài đặt workspace (one row) · Giới thiệu
  (version) · Đăng xuất (only here). Module pages: Chat, Lịch (`calendar_user_settings`), Tài liệu, Phê duyệt,
  Tài sản (notifications + view). Edit-one-field pages for profile. Locked rows (org; this round only "Ai được tạo
  nhóm") show value + "Quản trị viên <workspace> đã đặt" (no person's name: the graph records no actor), not editable. No read-receipt row (dropped this round). "Thu gọn
  sidebar" not shown on phones. Desktop Settings tabs keep working and read the same settings. Rows with their
  backend: Tạm dừng thông báo and quiet hours (messaging, phase 04), Ngôn ngữ (auth `account.language`), Thiết bị và
  phiên = new `GET /api/auth/sessions` listing only the caller's sessions + revoke through the existing `RevokeSession`, after checking the session is the caller's (it takes a bare session id).
- Cài đặt workspace (mockup §11): one page reached from the ⚙ on the Thêm workspace block, the workspace picker and Tôi;
  rows by permission using the existing admin checks — org admin: Tên, Mô tả, Thành viên, Mời thành viên (pending count),
  Phòng ban, Vai trò và quyền, Phạm vi danh bạ, Ai được tạo nhóm, Mẫu phê duyệt, Loại tài sản, Dung lượng Tài liệu (change
  limit with `manage`), Rời; org member: read-only info, Quản trị viên list, rules read-only, Hồ sơ trong tổ chức, Rời;
  personal: Tên, Dung lượng. Each admin row opens the existing admin screen in a phone layout (list + bottom sheet instead
  of the right panel). The Quản trị tile leaves the Thêm grid on phones; desktop keeps Quản trị.
- Remove: "Chuyển thành workspace tổ chức" UI (WorkspaceTab) and `POST /api/workspaces/:id/convert-to-organization`
  (route, domain method, gRPC if any, tests rewritten to assert 404/absence); `workspaces.converted_by/at` columns
  stay (history) and are no longer written.

## Architecture
Routes: `/me` (Tôi), `/me/:section`, `/settings/:module` (shared by desktop tab content where possible). Rows use a
`SettingsGroup`/`SettingsRow` pattern (value + chevron, switch, radio page). Saves through `useServerSettings`
(phase 04); theme and motion stay on the existing device-only `usePreferences`.

## Related Code Files
- Create: `frontend/src/components/settings/WorkspaceSettingsScreen.tsx` (+ phone layouts for `components/admin/{UsersScreen,RolesScreen,DirectoryScopeScreen,DepartmentPanel,InviteDialog}`), `frontend/src/components/settings/MeScreen.tsx`, `SettingsGroup.tsx`, `ModuleSettings/{Chat,Calendar,Drive,Approval,Assets}Settings.tsx`, `EditFieldScreen.tsx`, route files, tests
- Modify: `frontend/src/components/settings/SettingsScreen.tsx`, `NotificationSettings.tsx`, `AppearanceTab.tsx`, `WorkspaceTab.tsx`, `ProfileTab.tsx`, `frontend/src/components/patterns/AccountSheet.tsx` (removed on phone), `PhoneSearchBar.tsx` (avatar → Tôi)
- Modify/Delete backend: `backend/services/workspace/internal/rest/workspace_details_handler.go` (convert route), `domain/workspace_conversion.go`, `store/conversion.go` and their tests, the fake in `domain/authz_test.go`; frontend `src/api/workspaces.ts` (convert call), `src/hooks/useWorkspaces.ts` (convert mutation), `frontend/e2e/personal-workspace.mjs`; `docs/specs/tenant-identity`
- Create backend: auth `GET /api/auth/sessions` (own sessions only) + tests that another user's sessions are never listed and cannot be revoked

## Implementation Steps
1. Tests: Tôi rows per workspace kind; Đăng xuất appears once; locked row not editable and shows that an administrator set it (no person's name); module page saves and rolls back on error; convert route returns 404 and no UI offers conversion.
2. Build SettingsGroup/Row, MeScreen, module pages; wire avatar.
3. Remove conversion (backend + UI + tests) and AccountSheet on phones.

## Success Criteria
- [ ] One settings entry point on phones (avatar → Tôi); four old ones gone
- [ ] Cài đặt workspace shows only permitted rows per role and kind (e2e: org admin, org member, personal); admin screens usable at 390px
- [ ] Personal Tôi shows no Phê duyệt/Tài sản/Hồ sơ trong tổ chức; org shows them; Rời appears only in Cài đặt workspace (org)
- [ ] No conversion path in UI or API; tests assert its absence

## Risk Assessment
Removing the convert route breaks an e2e or seed script → the file list above names every caller found; grep again before deleting.

## Rollback
Revert the release; the convert route returns with the old version; sessions listing is read-only.
