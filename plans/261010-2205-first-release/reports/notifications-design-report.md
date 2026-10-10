# Notifications: design report

Date 2026-10-10 · Plan item B (first release) · Design only, no app code.

Deliverables:
- Mockup: `design/mockups/notifications.html` (light/dark, desktop, tablet, phone, states, realtime demos, numbered notes, copy table)
- DESIGN.md: §5 (sidebar row, dot on Thêm, Thông báo row in the Thêm sheet), §6 (NotificationPanel, NotificationRow)

## What the screen does

- Lists the signed-in person's notifications newest first, grouped "Hôm nay" / "Trước đó", 25 per page with "Tải thêm".
- Each row says **who did what to which thing** (actor avatar + name, verb, target name), with the reason for a rejection where there is one, the domain (Phê duyệt / Tài sản) and a relative time with the full time in a tooltip.
- Unread rows: ink text, 600 names, accent dot, hidden "Chưa đọc" text. Read rows: muted text, no dot. No row background (keeps accent ≤ 10%).
- Clicking a row marks it read and opens the target (approval detail panel, asset detail panel, asset request). Mark one (hover button, desktop only) and mark all are optimistic, roll back with an error toast on failure.
- Realtime: with the panel open, a new row is inserted with the §7 person-hue wash and "vừa duyệt / vừa giao…" tag; with it closed, a toast with "Xem". ≥ 3 in 2000 ms are coalesced. Count changes fade (no spring). If the target is already open on screen, the notification is marked read silently. A polite live region announces arrivals. Reduced motion drops transforms, layout animation and the avatar flash, and keeps the wash.
- States: skeleton, empty (+ "Xem đề nghị của bạn"), error (+ "Thử lại"), target unavailable toast, mark failed toast.

Entry points (one recommendation each):
- **Desktop ≥ 1024**: NavRow "Thông báo" at the top of the sidebar's bottom group (above Quản trị), accent count pill. Opens a 380 px popover panel beside the sidebar. It's not a route.
- **Below 1024 (phone and tablet, since the 64 px rail is not built yet)**: an 8 px accent dot on Thêm (no number) and a "Thông báo" row as the first item of the Thêm sheet, with a count pill. The row swaps the sheet content to the list, reusing the workspace-switcher pattern. There's no bell in the top or bottom bar. This keeps "few icons" and "only Phê duyệt carries a number on the bar". Arrival toasts above the bar cover urgency.

## Decisions where DESIGN.md was silent

1. Notifications go in the sidebar's bottom group, not the 5 main items. They belong to the person, like Cài đặt, and aren't a place to work.
2. A panel (popover, no scrim, no focus trap), not a page, so the user keeps their current context.
3. Panel motion mirrors the Detail panel (`translateX(-16px)` instead of `+16px`) because it grows out of the sidebar.
4. Mobile entry: a dot on Thêm plus a row in the sheet, not a top-bar bell. A numbered badge on Thêm was rejected because it breaks the one-number rule.
5. Unread styling uses a dot and weight, not a wash, to stay within the accent budget.
6. Day groups reuse the chat day divider. This avoids a second uppercase label in the panel.
7. Pagination uses "Tải thêm", not infinite scroll.
8. A new notification is still unread even when seen in an open panel.
9. If the target is already open, the notification is marked read and no toast is shown.
10. Mark-all has no Undo, because the API has no mark-unread.
11. Copy is built client-side per `type`. Server `title`/`body` are never rendered.
12. Tablet sheet is capped at 560 px and centred. This is shown in the mockup but **not** written into DESIGN.md, because it would change the Thêm sheet for everything (see the questions below).

## Notification types the backend emits today

Source: `backend/services/messaging/internal/events/consumer.go`. Consumed topics: `asset.lifecycle`, `asset.request`, `asset.assignment`, `approval.events`.

| type | recipient | trigger |
|---|---|---|
| `approval_approved` | requester (`created_by`), if not the actor | every `approved` action, **including intermediate steps** |
| `approval_rejected` | requester, if not the actor | `rejected`, comment appended to body |
| `asset_request` | the requester **themselves** | request submitted (pending) |
| `asset_request_approved` | requester | request approved |
| `asset_request_rejected` | requester | request rejected |
| `asset_assigned` | `to_user_id` | assignment |
| `asset_returned` | `from_user_id` | return |
| `asset_lifecycle` | the **actor themselves** | any state transition |

The Vietnamese copy for all eight, plus four promised-but-missing types, is in mockup §6.

## Backend gaps

1. **Approval notifications are probably never stored (verify first).** `approval_*` uses `created_by`, which is an NGAC node id (`claims.NGACNodeID` in `approval/internal/rest/decisions.go`). But `notifications.user_id` has a foreign key to `users(id)`, and the list, mark-read and hub push all key on `claims.UserID`. The insert should fail the foreign key and only be logged (`notification not recorded`). Even if it succeeded, the user would never see it.
2. **No actor.** Rows carry no actor id, so the UI can't show the avatar and name it requires. The events have one (`actor_id`, `approver_id`, `actor_node_id`), but it's dropped.
3. **No target name snapshot.** There's only `entity_type`/`entity_id`. The target name exists only inside the English `title`/`body`. Approvals carry only `template_name` (e.g. "Tạm ứng"), not the request's title. Resolving names per row on the client would be an N+1 fetch and fails after deletion or loss of access.
4. **`title`/`body` are English and pre-formatted** ("Asset request approved: Laptop"). The UI needs structured fields instead, e.g. `actor_id`, `target_name`, `status`, `comment`, `from_state`/`to_state`.
5. **No workspace/tenant on the row.** A person in two workspaces gets one mixed list. The click-through can't know which workspace to open, and switching workspace doesn't filter the list.
6. **Step versus final approval can't be told apart.** `approval_approved` fires on every step with body "has been approved". The row needs the request `status` to say "đã duyệt bước của mình, đang chờ …" versus "đã hoàn tất".
7. **Asset rejection reason is missing.** `AssetRequestEvent` has no comment, but `assets.html` promises "nhận thông báo kèm lý do".
8. **Self-notifications.** `asset_request` (pending) and `asset_lifecycle` notify the actor about their own action, which is noise. The recommendation is to drop them, or store them already read. Meanwhile the **approvers** of an asset request get nothing.
9. **Missing types promised by other mockups:** document access requested (owner) and decided (requester) from `contacts-documents-settings.html`; "chờ bạn duyệt" for the current-step approver; the per-thread follow toggle in `spaces.html`.
10. **No per-type preference store** (no table and no API), so per-topic toggles and mute can't be built. **No email digest** (SMTP is used only for OTP).
11. **WS push** (`NotificationEvent`) has no `created_at`, actor or workspace. That's fine if the client just invalidates (it does today, in `websocket.store.ts`), but the toast needs the same structured fields as the row.
12. **Already present:** mark-one (`POST /api/notifications/:id/read`), mark-all (`POST /api/notifications/read-all`), `GET /api/notifications/unread-count`, and list with `total`/`unread_count`/`limit`/`offset`. There's no unread-only filter, which is fine since the design doesn't need one.

## Open questions

1. Is the desktop entry right as a sidebar bottom-group NavRow with a popover? The alternative is the top of the sidebar next to the workspace switcher.
2. Is the phone entry right as a dot on Thêm plus a sheet row? Is it acceptable that this adds a dot to the "only Phê duyệt has a number" bar?
3. Should the Thêm sheet be capped at 560 px on tablet for all its content? If yes, it should go into DESIGN.md §5.
4. Should notifications from other workspaces show in the current list, with a workspace label that switches on click, or only the current workspace's?
5. Should the self-notifications (`asset_request` pending, `asset_lifecycle`) be dropped in the backend now, before the first release?
6. Are the missing types (document access, approver "chờ bạn duyệt", asset-request approvers) in scope for the first release, or later?
