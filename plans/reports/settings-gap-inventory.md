# Settings gap inventory: phone vs desktop (origin/main 03ed380, 2026-11-01)

Paths relative to `frontend/src/`. "Compact" = row shown only when `useBottomBarLayout()` (below 1024). Touch-only rows
(DESIGN.md §6 "Cài đặt trên máy tính") stay phone-only.

## Missing on desktop (must be added)
| Phone page | Option | Control | Data | Visibility |
|---|---|---|---|---|
| Me | Tạm dừng thông báo | choice (`pause.ts` PAUSE_CHOICES) | `usePushPreferences().pause_until` → `/me/push-preferences` | all |
| Me / Sessions | Thiết bị và phiên: list, Đăng xuất per other device + confirm | list, action | `useSessions` GET `/auth/sessions`, `useRevokeSession` DELETE `/auth/sessions/{id}` | all |
| Account / OrgProfile | Vai trò (read-only) | read-only | `current_tenant.role` | org |
| OrgProfile | Hồ sơ trong tổ chức page (Chức danh edit, Phòng ban, Vai trò) | form + read-only | `useUpdateProfile` PATCH `/me/profile` | org |
| Notifications | Theo phân hệ: Chat, Lịch, Tài liệu (+ Phê duyệt, Tài sản org) links | link | `useSettingsNav().open` | as noted |
| Chat | Mức mặc định cho nhóm mới | choice | `useServerSettings('chat')` `default_level` → `/workspaces/{id}/chat-settings` | all |
| Chat | Quản lý mục → `/channels?tab=sections` | link | — | all |
| Chat | Ai được tạo nhóm (locked row) | read-only | `useWorkspaceLocks` | org |
| Chat | Kind switches (Tin nhắn trực tiếp, Được nhắc tên, Thread đang theo dõi, Cuộc gọi đến) | switch | `usePushPreferences.kinds` | all (desktop had them in NotificationSettings "Báo cho tôi khi"; they move here) |
| Lịch | Tuần bắt đầu vào | choice | `useServerSettings('calendar')` `weekStart` → `/workspaces/{id}/calendar/settings` | all |
| Lịch | Lịch đang hiện | multi-select | `useCalendars` + `useCalendarVisibility` | all |
| Lịch | Lời mời mới / Thay đổi sự kiện / Nhắc sự kiện | switch | `notifyInvites` / `notifyChanges` / `notifyReminders` | all |
| Lịch | Múi giờ, Nhắc trước | choice | `tzid`, `defaultReminder` | all (desktop had them only in CalendarSettingsDialog) |
| Tài liệu | Xem dạng, Sắp xếp | choice | `useServerSettings('drive')` → `/drive/settings?workspace_id=` | all |
| Tài liệu | Thư mục trước tệp | switch | `folders_first` | all |
| Tài liệu | Được chia sẻ tài liệu | switch | `notify_shared` | all |
| Phê duyệt | Đề nghị chờ bạn duyệt / Kết quả đề nghị của bạn | switch | `useServerSettings('approval')` → `/approval/settings` | org |
| Tài sản | Tab mở đầu | choice | `useServerSettings('asset')` → `/workspaces/{id}/asset-settings` | org |
| Workspace | Header: role pill | read-only | `useMyProfile` | all |
| Workspace | Quản trị viên (avatars → list) | list | `details.owner_names` | org member (not admin) |
| Workspace | Mời thành viên (+ pending count) → InviteDialog | action | `useInvitations`, `useRoles`, `useDepartments` | org admin |
| Workspace | Thành viên / Phòng ban / Vai trò và quyền / Phạm vi danh bạ (counts → `/admin/...`) | link | `useMemberDirectory`, `useDepartments`, `useRoles` | org admin |
| Workspace | Ai được tạo nhóm | choice (admin) / locked | `useWorkspaceLocks`, `useSetGroupCreation` | org |
| Workspace | Hồ sơ trong tổ chức link | link | — | org member |
| Workspace | Mẫu phê duyệt → `/approval?tab=templates`, Loại tài sản → `/assets?section=types` (counts) | link | `useApprovalTemplates`, `useAssetTypes` | org admin |
| Workspace | Dung lượng Tài liệu | read-only | `useDriveQuota` | org admin or personal |

## Touch-only (phone only, by design)
Enter để gửi, Mở Chat ở, Sắp xếp (Chat), Cảm xúc khi chạm đúp, Thanh đáy (`EditBarScreen`), Tải ảnh xem trước bằng dữ
liệu di động (`cellular_previews`), Chế độ xem mặc định và Thời lượng mặc định (Lịch), Sắp xếp (Phê duyệt).

## Desktop-only today (keep)
Thu gọn sidebar (AppearanceTab), theme cards, WorkspaceTypeCard + "Đặt quyền Phê duyệt và Tài sản", the 3-step
blocked-browser banner (NotificationSettings), Giờ làm việc (CalendarSettingsDialog: moves to Cài đặt Lịch, both layouts).

## Reuse
- Hooks: `usePushPreferences`/`isPaused`/`browserZone`, `pause.ts`, `usePushDevice`, `usePreferences`,
  `useAccountLanguage`, `useSessions`/`useRevokeSession`, `useServerSettings(service, wsId, DEFAULTS)` +
  `lib/settings-defaults`, `useCalendarVisibility`, `useCalendars`, `useWorkspaceLocks`/`useSetGroupCreation`,
  `useWorkspaceDetails`, `useInvitations`/`useDepartments`/`useRoles`/`useMemberDirectory`, `useApprovalTemplates`,
  `useAssetTypes`, `useDriveQuota`.
- `SettingsGroup`, `SettingsRow`, `SettingsSwitch` are layout-neutral pieces; `SettingsPage` and `ChoicePage` assume a
  full-screen phone page. Module choice lists (VIEWS, SORTS, LEVELS, REMINDERS…) are module-private consts.
- `LeaveWorkspace` has `section` and `row` variants. `InviteDialog` works at any width.
- Today nothing on desktop links to `/settings/<section>`; at desktop width that route renders the phone page.
