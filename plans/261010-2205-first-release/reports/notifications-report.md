# Notifications: implementation report

Date 2026-10-10 · first-release plan item B · branch `refactor/project-wide`. Nothing staged or committed (index still the 158 staged deletions). Other agents' work in `backend/services/workspace` and `backend/pkg/grpcauth` was left alone except one line (see "Invitation scope").

## What shipped

### Backend (messaging, plus one line in workspace)
- **Migration `data/migrations/037_notifications_structured.sql`** (numbered 037: another agent took 036 for `last_emailed_at` while I worked). Renames `entity_type/entity_id` to `target_type/target_id`; adds `workspace_id` (FK, cascade), `actor_user_id`, `actor_name`, `target_name`, `params jsonb`; `title` gets a default. Idempotent (guarded rename), so `make db-migrate` replays it and the deploy ledger applies it once. `init.sql` is deliberately untouched: the prod ledger applies it once, and new-column indexes there would break the first replay on an old DB.
  - Backups taken first (`pg_dump -Fc` of `ngac` and `ngac_ci` into the session scratchpad: `ngac-pre-036.dump`, `ngac_ci-pre-036.dump`).
  - Applied with `make db-migrate` on `ngac`, by the same files in order on `ngac_ci`, then 037 replayed on both to prove idempotency.
- **Legacy rows (decision): never shown.** A row with no workspace matches no list, count, mark or push (`workspace_id IS NULL`). They carry only English text and no actor or subject names, so they cannot satisfy the "never an id, never server text" rule; they are kept, not deleted. Approval notifications were never stored before (see fix a), so only asset rows are affected. Documented in the spec.
- **Scoping:** list, count, mark one, mark all and the live push all take `(user, workspace)`. REST uses `claims.TenantID`, gRPC `Caller.TenantID`; no workspace in the token is 403 and the store is not asked.
- **a. Approval recipient fix.** `created_by` and the actor are NGAC nodes. The notification service now resolves them inside the event's workspace (`store.FindMember`, via `tenant_users`/`users`) and stores under the user id. Test with a real database: requester gets it, approver does not, requester's other workspace does not, a member of another workspace only gets nothing.
- **b. Structured fields.** `type`, `actor_user_id`, `actor_name`, `target_type/id/name`, `params` (e.g. `reason`). Names are the workspace's own (display name, else login, only for members), snapshotted at write time, absent when unknown, never an id. `title`/`body` are no longer written, served or pushed (proto fields kept, deprecated). Events without a tenant are dropped.
- **c. Self-notifications removed.** `asset_request` (pending) and `asset_lifecycle` are gone (the consumer no longer subscribes to the lifecycle topic). The service also refuses to notify a person of their own act (covers self-assignment and node-vs-user id mismatches).
- **d. Step vs completed.** New type `approval_step_approved` (status still pending after the approval) vs `approval_approved` (request complete).
- **Live push:** `Hub.SendNotification(workspace, user, n)` reaches only that user's sessions in that workspace (new Redis channel `notify:<ws>:<user>`); an empty workspace (invitation) reaches every session of the user. The frame now carries the structured fields and `created_at`.
- **Proto:** `Notification` and `NotificationEvent` changed (renames keep field numbers). `make proto` and `npm run proto:gen` both run. Regenerated `ws.ts` has 3 more generator type errors (map field); `typecheck-baseline.txt` ws.ts 2 to 5, generated code, not hand-edited.
- **Follow-ups (out of scope, listed in the spec):** document access requested/decided, telling the current approver "chờ bạn duyệt", telling asset approvers, asset rejection reason (event has none), naming who a step waits on, withdrawing an invitation notice when revoked or expired, preferences, email digest.

### Invitation scope (added mid-task)
- **New type `workspace_invitation`.** The workspace service emits `invitation_created` (ids: the invitation id, never the address) from `InviteByEmail` for every address alike. **One line in `backend/services/workspace/internal/domain/invitations.go`** (`s.announce(...)` before the email call), re-read right before editing (the mailer agent's changes were already there); plus the constant in `backend/pkg/realtime/event.go`.
- Messaging's notification consumer now also reads `workspace.events`; for `invitation_created` it reads the stored pending, unexpired invitation and notifies each account whose `lower(email)` matches **and has a verified email** (same rule as `GET /api/invitations`). Unverified or nonexistent: nothing is created, nothing reveals which. The hub's realtime consumer drops that kind, so workspace members are never told who was invited. Inviter response is unchanged (test).
- **Personal exception:** this one type is listed, counted, marked and pushed regardless of the open workspace (the invitee is not a member). A re-invite replaces the earlier notice. Documented in the spec.
- Click-through goes to `/workspace-select`. **Accepting or declining marks it read** (`POST /api/notifications/read-about {type,id}`, called by `useAcceptInvitation`/`useDeclineInvitation`; best effort).
- **Switchers:** the sidebar's workspace dropdown and the Thêm sheet's "Đổi workspace" view get a "Lời mời đang chờ (N)" row linking to `/workspace-select` (from `GET /api/invitations`, asked only while the switcher is open). DESIGN.md §5 has a sentence for it.
- Tests: created only for existing verified account; none for unverified or nonexistent; none for revoked; workspace announces for every address alike and nothing when refused; switcher row with N>0 and absent at 0; click-through; accept/decline marks read; a failing mark does not undo the answer.

### Frontend
- Entry points as mocked: "Thông báo" row with count pill, first in the sidebar's bottom group, opening a 380px panel; below 1024 an 8px dot on Thêm (no number, label "Thêm, N thông báo chưa đọc") and a "Thông báo" row first in the Thêm sheet that swaps the sheet to the list (back / title / close, sheet stays 85% high). No new icon on either bar.
- `NotificationRow`: real `Link`, actor avatar or icon tile, sentence built client-side from `type` + names (`lib/notification-model.ts`), reason block, domain + relative time (full time as tooltip), unread dot + hidden "Chưa đọc", mark-one button on hover/focus (hidden on touch). Groups "Hôm nay"/"Trước đó", "Tải thêm" and "Đã hiện X trong Y", skeleton, empty (with "Xem đề nghị của bạn"), error (with "Thử lại").
- Mark one / mark all are optimistic and roll back with the failure toast ("Thử lại" action); mark all toasts the count.
- Click-through: marks read, checks the subject can be opened (403/404 = cannot), then navigates (approval → Của bạn + request, asset request → Tài sản › Yêu cầu, asset → Tài sản list). Cannot open: stays put, still read, mockup toast.
- Realtime (`lib/notification-arrivals.ts`, called from the websocket store's `notification` case): list open = row inserted with the person-hue wash, "vừa …" tag, polite live region; list closed = toast with avatar and "Xem"; 3+ within 2000 ms = one summary toast with "Mở" (or one summary line in the open list, tags dropped); subject already open on screen = read silently, no toast. Frames for another workspace are ignored except invitations. Reduced motion uses the existing presets (new `panelLeft` preset: fade only when reduced).
- `keys.notifications.*` used for every query and invalidation; "kept for the coming UI" comments removed (`websocket.store.ts`, `lib/realtime.ts`).
- Focus: panel and sheet both use `useModalFocus` (Esc closes only the topmost layer, focus returns to the row or to Thêm). Toast got an optional `icon` slot.
- No `transition-all`, raw durations, `z-[`, `rgba` or raw palette classes (lint clean).

## Verification

| Gate | Result |
|---|---|
| frontend `npm run lint` | clean |
| frontend `typecheck:diff` | no file above baseline (ws.ts baseline 2 to 5, generated) |
| frontend `npm test` | 91 files, 1216 tests pass (full run before the last small edit; 1123 in components/hooks/lib/stores re-run after it) |
| frontend `npm run build` | ok |
| `make build-check`, `check-ngac`, `check-layering`, `make lint`, `check-docs`, `fmt-check` | all pass |
| `make test s=messaging` and `s=workspace` | exit 0, 0 skips, 0 failures on `ngac` **and** `ngac_ci` |

New tests: store (workspace scoping, legacy row, membership resolution, cross-workspace mark), domain (scope refusal, resolution, self-act, not-a-member, push order, invitation), events with a real database (requester receives the approval decision under their user id, step vs complete, outside-workspace recipient gets nothing, self-assignment, invitation rules), REST (shape without title/body, workspace from token, read-about), gRPC (cross-workspace mark/list/count), hub (push reaches only the recipient in the workspace; personal reaches all of the user's sessions), workspace (announces every address alike). Frontend: model, arrivals (coalescing, silent read, other workspace, open/unavailable), panel states, mark one/all with failure, paging, no-UUID with UUID fixtures, realtime insert, keyboard, desktop and phone placement, switcher rows, invitation hooks.

## Screenshots

`plans/261010-2205-first-release/reports/notifications-screens/` (44 files; desktop 1440, tablet 820, phone 390; light and dark): `*-panel-list`, `*-panel-arrival-wash`, `*-panel-empty|error|loading`, `*-toast-arrival`, `*-switcher-offers` (desktop), `*-bar-dot`, `*-sheet-menu`, `*-sheet-list`, `*-sheet-empty|error`, `*-sheet-switcher-offers`. Method: Playwright from the gstack install against a Vite I started on :5173 (port checked free first, stopped afterwards; the :5174 server belongs to someone else and was not touched). The API is answered in the page with UUID fixtures (no database, no backend); arrivals are injected by importing the app's own module. The round-bottom-left palm icon in the shots is the React Query devtools button, dev only. Compared with the mockup: layout, grouping, unread styling, wash and tag, toast and dot match. Tablet sheet stays full width like the existing Thêm sheet (the mockup's 560px cap was never put into DESIGN.md).

## Decisions and deviations worth a look
1. **`useModalFocus` on the desktop panel.** As instructed, but it makes `#root` inert while open, so the panel is effectively modal (the mockup said non-modal, no trap). A click outside closes it; the first click does not reach the page behind.
2. **Migration number 037**, not 036 (collision). Tell the other agent not to reuse 037.
3. Routes: `/api/notifications/read-about` is covered by the existing `/api/notifications` prefix in Vite, Traefik and nginx; no proxy change needed.
4. `title`/`body` removed from REST and WS payloads (spec updated). gRPC fields remain, deprecated and empty.
5. Notification text names come from a snapshot at write time; a renamed person keeps the old name on old rows.

Status: DONE_WITH_CONCERNS

Summary: Notifications ship end to end: workspace-scoped structured rows with migration 037 (applied on ngac and ngac_ci), the approval recipient fix, no self-notifications, step vs completed, scoped list/mark/push, the full mockup UI on desktop and below 1024, the realtime behaviours, and the invitation notification plus "Lời mời đang chờ (N)" switcher rows. All gates pass with zero skips on both databases; 44 screenshots saved.

Concerns:
- The desktop panel is modal in effect because `useModalFocus` inerts the app behind it (deviates from the mockup's non-modal popover).
- Legacy rows are hidden permanently, not migrated or deleted.
- An invitation notice stays (unread) after the invitation is revoked or expires until the person opens it; the picker then shows nothing. Listed as a follow-up.
- Invitation event is consumed by messaging only; if Kafka is down at invite time the notice is not created (the email and `/workspace-select` still work). Same best-effort behaviour as the other notifications.
- Migration 037 and `docs/specs` edits touch shared files (DESIGN.md already had other uncommitted edits; mine is one sentence in §5).
