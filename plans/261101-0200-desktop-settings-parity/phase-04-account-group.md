# Phase 04 · Account group

## Context
Phone pages: `AccountScreen.tsx` (+ `OrgProfileScreen`), `NotificationsScreen.tsx`, `AppearanceScreen.tsx`,
`SessionsScreen.tsx`. Desktop tab components with content worth keeping: `ProfileTab.tsx` (multi-field form, avatar,
Đăng xuất), `AppearanceTab.tsx` (theme cards, Giảm chuyển động, Ngôn ngữ, Thu gọn sidebar),
`NotificationSettings.tsx` (push switch, 3-step blocked banner, quiet hours, "Báo cho tôi khi").

## Requirements
1. **Tài khoản và hồ sơ** (wide): the `ProfileTab` form (avatar, Tên hiển thị, Chức danh, Nơi làm việc, Email read-only,
   Phòng ban read-only in org) plus **Vai trò** read-only (org), Lưu thay đổi/Huỷ, Đăng xuất row at the end. Rendered by
   `AccountScreen` when wide (reuse ProfileTab's body; do not duplicate validation, use `settings-model.ts`).
2. **Hồ sơ trong tổ chức** (wide, org): a small form: Chức danh (edit, Lưu thay đổi), Phòng ban and Vai trò read-only.
3. **Thông báo** (wide): Tạm dừng thông báo (Select with `PAUSE_CHOICES`; while paused the Select shows
   `pauseLabel`), Thông báo đẩy trên máy này (switch + blocked/iOS banners from NotificationSettings, keep the 3 steps),
   Giờ yên lặng (two time Selects + switch on one row, times dim when off), group Theo phân hệ (links, org-only rows as
   on phone). The "Báo cho tôi khi" kind switches are NOT on this page at wide (they live in Chat, phase 02).
4. **Giao diện và ngôn ngữ** (wide): AppearanceTab's content (theme cards, Giảm chuyển động, Ngôn ngữ, Thu gọn sidebar).
5. **Thiết bị và phiên** (wide): SessionsScreen in the wide frame; Đăng xuất a device keeps its ConfirmDialog.
6. Lead line "Dùng chung mọi workspace" on all five (org profile: "Trong <workspace>").

## Tests first
- Wide account shows Vai trò in an org and not in a personal workspace; saving sends only changed fields.
- Wide notifications: pause Select PUTs `pause_until` only; kind switches absent; Theo phân hệ Chat link → `/settings/chat`.
- Wide appearance: theme radios and sidebar switch present. Wide sessions: list + revoke with confirm.
- Phone pages unchanged.
