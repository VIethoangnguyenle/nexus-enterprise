# Design mockups report: Tài sản, Quản trị, Danh bạ/Văn bản/Cài đặt, Auth

Date 2026-10-10. Direction "Tín hiệu" (DESIGN.md). Phase 06 Requirement A, the remaining screen groups.

## Files (all in `design/mockups/`, self-contained, link `tokens.css`)

Every file has the following:
- Controls for theme (Theo hệ thống / Sáng / Tối), reduced motion, and showing or hiding the numbered callouts.
- Desktop frames at 1240px or wider, a tablet frame at 820px with a 64px rail, and phone frames at 360px.
- Numbered callout dots on elements that match the numbered notes under each frame.
- Demo buttons that run the DESIGN.md §7 motion presets.
- At least one loading, one empty and one error state.

The primitives are copied verbatim from `core-screens.html`. The new primitives (rail, form fields, pickers, stats, permission matrix, auth, editor) use only DESIGN.md tokens.

| File | Contents |
|---|---|
| `assets.html` | 0. Shell decision (Q5). 1. Tổng quan: stats, distribution by state and by type, waiting for you, activity with actor. 2. Danh sách + detail panel: actions follow the lifecycle, inline person picker for handover, history with actor. 3. Yêu cầu: table, detail panel, "Duyệt và giao", reason modal for "Từ chối". 4. Yêu cầu mới as a detail-panel form: type picker, urgency segmented control, validation. 5. Loại tài sản: category, custom fields, read-only lifecycle. 6. States. 7. Tablet. 8. Mobile (overview, sheet, full-screen form). |
| `admin.html` | 1. Tổng quan + organisation tree + admin audit + department panel. 2. Người dùng: table, role picker, department tree picker, lock/remove. 3. Mời thành viên modal: emails, role, department. 4. Vai trò: readable names, "Được phép" grouped by resource area. 5. Sửa quyền: matrix of resource area × 8 ops, with a save bar that states the consequence. 6. Labels for the 8 ops. 7. States, including "không có quyền". 8. Tablet. 9. Mobile (switch list per area). |
| `contacts-documents-settings.html` | 1. Danh bạ: table + profile panel, presence fade. 2. Văn bản as a group in the Tài liệu list panel, with an inline upload progress row. 3. Collaborative editor shell: person-coloured carets and wash, save state, offline band. 4. to 6. Cài đặt tabs Hồ sơ / Giao diện (theme cards that switch the page theme) / Workspace. 7. States. 8. Tablet (cards view, editor). 9. Mobile. |
| `auth.html` | 1. Đăng nhập (Google + OTP, no SSO stub). 2. Xác minh: 6 boxes, countdown, wrong-code state. 3. Tạo hồ sơ for new users. 4. Chọn workspace with invitations. 5. Tạo workspace (no slug field). 6. States: loading, empty, error, Google errors, expired code or too many attempts, no provider. 7. Tablet. 8. Mobile. **No dev OTP hint anywhere.** |

Verified as follows:
- Headless Chrome screenshots of all 4 files in both themes, reviewed visually.
- 0 uncaught JS errors.
- All 33 demo actions were clicked headlessly, with 0 errors.
- A grep for UUIDs, `999999`, `uuid_`, `*_id` finds hits only inside developer annotations, never in the mocked UI.

## Decisions

- **Assets shell (Q5): merge `/assets` into `_workspace`.** DESIGN.md §5 already says so ("Tài sản nằm trong shell này, không có layout riêng"), so there is no contradiction.
  - Sub-routes become header tabs, like Phê duyệt, with no list panel.
  - Asset detail is a detail panel. `/assets/$assetId` stays as a deep link.
  - The rationale is annotated in `assets.html` §0.
- **Asset states follow the backend `lifecycle.go`**: Chờ duyệt, Sẵn sàng, Đang giao, Bảo trì, Ngừng dùng, Đã thanh lý. The frontend currently uses two conflicting sets.
- **The Phê duyệt pattern is reused for asset requests.** "Từ chối" requires a reason (the code currently sends the fixed text "Rejected").
- **Roles are grouped by resource area (OA).** Each area lists its ops by Vietnamese label: Xem, Sửa, Tải lên, Duyệt, Chia sẻ, Quản lý, Mời, Tạo nhóm chat. This is the NGAC association expressed in user terms, and it replaces the hardcoded R/W/M grid.
- **Văn bản lives under Tài liệu** as a group in the list panel. DESIGN.md allows a list panel only in Tin nhắn and Tài liệu. The editor hides the list panel.
- **Danh bạ has no list panel**, so the department filter uses a Tree picker. The fake categories and the "Gọi" button that does nothing are removed.
- **Removed from auth:**
  - the SSO stub
  - the "Free Tier" label
  - the hand-typed workspace URL slug
  - the 2s "Welcome back" redirect page
  - the dev OTP hint

## Backend needs surfaced (marked `cần backend` in the mockups)

1. Asset holder name and avatar (`assigned_to` is shown raw today), asset history actor, and request requester name and avatar.
2. A UI for `fields_schema`, the custom fields on asset types.
3. An admin audit feed (role, permission, member and department changes, with the actor).
4. A role `display_name`, plus an endpoint that lists a role's associations with resource-area names.
5. Invite by email. `inviteMember` currently takes an `ngac_node_id`. Also: pending invitations per email for the workspace-select screen, and sending invites when a workspace is created.
6. A collaborative document editor and its sync layer. There is no editor at all today.
7. New endpoints must go into `frontend/vite.config.js`, including the regex block for routes under `/api/workspaces/:id/`.

## Open design questions

1. **List stagger is not in DESIGN.md §7.** The mockups use an opacity-only fade: 160ms, 20ms per row, at most 6 rows, first paint only. Should this be added to §7?
2. **The OTP wrong-code shake** (280ms, transform only, none under reduced motion) is not a §7 preset. Approve it, or drop it and keep only the colour change and message?
3. **Avatar 64** (profile, settings) and 56 (register preview) are outside the 20/24/32/40 scale. Should 64 be added to DESIGN.md §6?
4. **Asset request approval:** does approving also assign a specific asset (`request_server.go` assigns on fulfil), or is assignment a separate step? The mockup shows "Duyệt và giao" with an asset picker.
5. **Permission matrix "—" cells** (an op that doesn't apply to an area type): should the applicability list come from the backend or be a client constant?
6. **Appearance "Màu theo người" toggle:** DESIGN.md treats realtime wash as information and doesn't allow turning it off. Keep the toggle or drop it?
7. **Quản trị on mobile** is reached from the avatar, because the tab bar is fixed at 5 items. Confirm.
8. **Admin density 7** is read as same 44px rows with tighter column gaps (10px). Confirm, or define a density token.
9. **Theme preview thumbnails** in Cài đặt use literal OKLCH values from the other theme. The code needs theme-scoped preview tokens or a static asset.
10. **The Google "G" logo uses raw brand hex.** This needs a documented lint exemption in code.
