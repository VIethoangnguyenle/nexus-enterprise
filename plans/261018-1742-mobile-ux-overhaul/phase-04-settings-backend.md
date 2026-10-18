---
phase: 4
title: "Settings backend"
status: pending
priority: P1
effort: "4d"
dependencies: [1]
---

# Phase 04: Settings backend

## Overview
Persist every personal option in mockups §3/§4/§8 in the service that enforces it, add the one organization lock
of this round ("Ai được tạo nhóm", implemented by removing the `create_channel` right, not by a prohibition), and give
the client one way to read and write them.

## Requirements
- Functional:
  - Each option has exactly one owner service (red team #9): messaging — push and Chat notification switches,
    quiet hours, pause until, `chat.enterToSend`, `chat.linkPreview`, `chat.openAt`, `chat.sort` (Chat's "Sắp xếp"),
    `chat.doubleTapReaction`, `chat.defaultLevel` (extends `push_preferences` / `channel_notification_levels`);
    calendar — existing `calendar_user_settings` keys **plus** default view, week start, visible calendars, default
    duration and the notification switches (extend calendar's own table); drive — `drive.view`, `drive.sort`,
    `drive.foldersFirst`, `drive.cellularPreviews`, and the one drive notification switch (documents shared with you;
    there are no document comments, so no comment switch); approval — its notification switches and view keys
    (own table in the approval service); asset — its notification switches and view keys (own table in the asset
    service); workspace — `mobile.bar` (per workspace) and the lock; auth — `account.language` (account scope). No
    settings row in mockups §3/§4/§8 is left without an owner here.
  - Quiet hours block push **except** when the person is mentioned by name (`@Tất cả` included); the push sender reads
    the mention (test both sides).
  - Notification switches are read by the code that sends the push/notification, so turning one off stops it
    (test per switch).
  - "Ai được tạo nhóm = chỉ quản trị viên": an organization-only lock implemented by **removing the right**:
    `create_channel` leaves the Members → Channels association and every custom role's Channels grant; Owners keep
    their own association. The roles screen refuses to grant `create_channel` on Channels while locked and shows the
    lock. Unlocking restores the Members association only (custom roles are not auto-restored; the UI says so). The
    lock state is read from the graph (association present or not; no separate lock row, no actor recorded).
  - Every preferences and lock route requires the caller to be a member of the workspace in the path
    (`authorizeMember`); lock changes require the workspace admin check already used by admin routes
    (`domain/authz.go`).
- Non-functional: no read-receipt option or lock this round (Zane 2026-10-19); tenant isolation from the path's
  workspace + membership check (the token carries `tenant_id`, not a workspace id).

## Architecture
Reserved migration `068_user_settings.sql` (idempotent): `user_settings(user_id, workspace_id NULL, key, value
jsonb, updated_at)` owned by the workspace service for keys that have no existing table (`mobile.bar`), with two
partial unique indexes `(user_id, key) WHERE workspace_id IS NULL` and `(user_id, workspace_id, key) WHERE
workspace_id IS NOT NULL`, FK to `workspaces` ON DELETE CASCADE; new columns on `push_preferences` (messaging),
new columns on calendar's `calendar_user_settings`, and new per-service tables (`drive_user_settings`, approval and
asset equivalents) for the rest. Routes stay under each service's existing prefix (messaging
`/api/push/...`, drive `/api/drive/settings`, workspace `/api/workspaces/:id/settings`,
`/api/workspaces/:id/locks`, approval and asset under their existing prefixes) — each added to `frontend/vite.config.js` and `deploy/nginx/default.conf` where a new prefix appears. Client:
`useServerSettings(service)` (new name; the existing device-only `usePreferences` for theme/motion stays untouched).

## Related Code Files
- Create: `data/migrations/068_user_settings.sql`, workspace `domain/store/rest` settings + locks, drive, approval and asset settings handlers, tests; `frontend/src/hooks/useServerSettings.ts`, `frontend/src/api/settings.ts`
- Modify: `backend/services/messaging/internal/store/push.go` + domain (new keys, quiet hours, pause), messaging, drive, calendar, approval and asset notification senders (read switches), calendar settings store, `backend/services/workspace/internal/domain/roles_admin.go` (show/refuse while locked; remove the right from custom roles), `backend/ngac` (lock helper over `MemberChannelOps`), `frontend/vite.config.js`, `deploy/nginx/default.conf`
- Specs: `user-settings` (new), `tenant-ngac-init` (the lock removes `create_channel`), `workspace-admin-authorization`, `notifications`

## Implementation Steps
1. Back up the dev DB; write `068_user_settings.sql` idempotently; `make db-migrate` twice (re-run safe); verify.
2. Tests first, deny first: non-member reads/writes settings of a workspace → 403; other user's settings unreachable; non-admin sets the lock → 403; with the lock on, a member **and** a custom-role holder creating a group → DENY (policy test vector), the owner → ALLOW, and the roles screen refuses to grant `create_channel` on Channels; unlocking restores Members; personal workspace lock → 400; each notification switch off → no push sent; quiet hours + mention by name or `@Tất cả` → push sent, without mention → not sent.
3. Lock write (remove the right) and unlock (restore Members) through the EPP path; lock read from the graph.
4. Per-service handlers and senders; `useServerSettings` with optimistic write + rollback toast "Chưa lưu được…".

## Success Criteria
- [ ] Migration applied twice without error on the dev DB; strict `make test` green for workspace, messaging, drive, calendar, approval, asset, policy (TEST_DATABASE_URL on `ngac_ci`)
- [ ] Deny cases tested for membership, admin-only lock, custom-role bypass, roles screen refusing the right while locked
- [ ] `make check-layering`, `make check-ngac` green; new prefixes in vite proxy and nginx

## Risk Assessment
A custom role created or edited after the lock must not regain the right → covered by the roles-screen refusal test and the custom-role deny vector. A failed graph write
leaves no false "locked" state because the graph is the only state.

## Rollback
Revert the release (`deploy.sh` keeps migrations); new tables/columns are additive and unused by the old version.
Before rollback, unlock group creation with the admin endpoint (restores Members `create_channel`; re-grant custom
roles by hand) so the old version, which has no lock UI, is not left with a lock it cannot undo.
