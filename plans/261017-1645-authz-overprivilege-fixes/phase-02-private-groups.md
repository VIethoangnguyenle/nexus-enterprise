---
phase: 2
title: "Private groups (F1): graph model, #general direct edges, calendar vectors, migration 060"
status: completed
priority: P1
effort: "8h"
dependencies: [1]
---
# Phase 2: Private groups (F1)

## Overview
Decision: groups are private — only group members read/write. Today every workspace member reads/writes every group,
its drive and its channel calendar because `Members UA → Channels OA {read,write,create_channel}`
(`workspace/internal/domain/service.go:210`) covers every content OA beneath it (`messaging/.../channels.go:153-166`),
every channel drive beneath that (`drive/internal/domain/channel_drive.go:75-82`) and every channel calendar
(`calendar/internal/domain/calendars.go:124-128`, under the content OA). Owners leak the same way
(`AllOwnerOps()` on Channels, `service.go:208`), custom roles through `AreaOps(AreaChannels)` (`ngac/areas.go:58`).

## Requirements
Target graph (workspace W):
- `<W>_Members → <W>_Channels`: `{create_channel}` (`ngac.MemberChannelOps()` changed).
- `<W>_Owners → <W>_Channels`: `{create_channel, manage}` (new `ngac.OwnerChannelOps()`); no read/write/invite.
- Custom roles: `AreaOps(AreaChannels)` = `{create_channel}` (choice 1: only Owners moderate unseen groups);
  existing role associations on Channels OAs reduced to ∩ `{create_channel}`, removed if empty.
- Group access only via `Ch_<id>_Members → Ch_<id>_Content` (`ChannelMemberOps`) and `→ Ch_<id>_Drive`
  (`ChannelDriveOps`); channel calendars inherit from content. Unchanged.
- **Workspace-wide channel (`#general`)**, `channel_type='workspace'`, at most one per workspace:
  - access + roster via **direct per-user edges** `U → Ch_general_Members` plus `channel_members` cache rows, so
    `ListMembers` (`messaging/.../members.go:113-125`), push recipients (`store/push.go:85-90`), `member_count`
    (`store/store.go:79`) and RemoveMember all work. No UA→UA edge.
  - written on join: new messaging gRPC `JoinWorkspaceChannel(workspace_id, user_node_id)` (service identity only,
    ServiceOK for `workspace` and `auth` in `messaging/cmd/main.go:190`); idempotent (IsAssigned first); requires
    `IsWorkspaceMember(user, W)`. Called by workspace `admit` (`invitations.go`) after the Members edge, and by auth
    `joinTenant` (`tenants.go`). Failure logged; 060 (re-runnable) is the backfill/repair.
  - removed on leave/removal: `detachMember` already strips every UA under the PC incl. channel UAs; the messaging
    realtime consumer on workspace `member_removed` (`realtime_consumer.go:207-216`) also deletes that user's
    `channel_members` rows for channels of W.
  - created only by the auth service (choice 3): new gRPC `ProvisionWorkspaceChannel(workspace_id, name,
    creator_user_id, creator_node_id)`, ServiceOK `auth` only; `auth/internal/domain/provisioning.go:72-74` switches to
    it (service identity). User-scoped `CreateChannel` (REST `handler.go:151-178` and gRPC `server.go:35-48`) always
    stores `private`, whatever type is sent; domain normalization `group→workspace` (`channels.go:73-75`) becomes
    `→private`.
  - Proto change (`backend/proto/messaging/*.proto`): `make proto` **and** `cd frontend && npm run proto:gen`.
  - Store helper `WorkspaceWideChannel(ctx, wsID)`: `channel_type='workspace' AND name='general' AND created_by =
    workspaces.owner_id ORDER BY created_at LIMIT 1` — same rule as 060, so it picks the right row before and after the
    migration (release A tolerance).
- Frontend branches only on `'dm'` (`frontend/src/lib/conversations.ts:117`, `SpaceView.tsx:88`) → no UI change for
  `private`.
- No policy startup flush (dropped); invalidation per plan.md rules.

## Architecture
```
before: U ─> <W>_Members ──{r,w,cc}──> <W>_Channels <─ Ch_G_Content <─ Ch_G_Drive / Ch_G_Calendar
after : U ─> <W>_Members ──{cc}─────> <W>_Channels <─ Ch_G_Content ...
        U ─> Ch_G_Members ─{r,w,invite}─> Ch_G_Content ; ─{r,w,upload,share}─> Ch_G_Drive
#general: every member U ─> Ch_gen_Members (direct edge + cache row)
join:   admit / joinTenant ──JoinWorkspaceChannel──> messaging (IsWorkspaceMember, assign, cache)
leave:  detachMember strips Ch UAs ; consumer(member_removed) deletes cache rows
```

## Release A / B
- **A (code)**: new op sets for new workspaces; private on create; auth-only workspace channel; JoinWorkspaceChannel;
  reads tolerate old types/grants. Old data still leaky until B (unchanged from today). Group creation never collides
  with the index (not present yet; REST never writes `workspace`).
- **B (060)**: data migration. Rollback B→A safe. Rollback past A: reverse SQL first (below), otherwise old code
  writes `workspace`-typed groups that the unique index rejects (500s). Risk **H×H** if ignored.

## 060 `data/migrations/060_private_groups.sql`
Statements (re-runnable; guard first):
1. Guard: `RAISE EXCEPTION` if any workspace lacks `<id>_Members` (059 must have run).
2. Backups: `_reconcile_060_associations` (all associations on `%\_Channels` OAs), `_reconcile_060_channels`
   (id, channel_type).
3. Repair: insert `Ch_*_Members → Ch_*_Content {read,write,invite}` where missing for non-DM channels (dev: 1).
4. `general` set by the store rule above; `UPDATE channels SET channel_type='private' WHERE channel_type='workspace'
   AND id NOT IN general`.
5. Direct edges: for each general, every U reaching `<ws>_Members` (recursive CTE) → `INSERT INTO ngac_assignments
   ... ON CONFLICT (child_id,parent_id) DO NOTHING`; same for `channel_members ON CONFLICT DO NOTHING`.
6. Associations on `%\_Channels`: Members → `{create_channel}`; Owners → `{create_channel,manage}`; other UAs →
   ∩ `{create_channel}`, delete if empty. Match workspace by ID-keyed names (guaranteed by step 1).
7. File manages its own `BEGIN; ... COMMIT;` for steps 1–6, then outside it `CREATE UNIQUE INDEX CONCURRENTLY IF NOT
   EXISTS channels_one_workspace_wide ON channels(workspace_id) WHERE channel_type='workspace'`.
   `deploy/migrate.sh:12-17` runs such files without `--single-transaction` and writes the ledger row after, so a crash
   re-runs the whole file → steps 1–6 must be fixpoints (they are). Dev `make db-migrate` (plain psql) handles it too.
8. Header NOTE: graph writes → recreate policy + policy-read, run `policy invalidate-cache --all-workspaces`, restart
   messaging (drops WS subscriptions).

Pre-flight (dry-run in prod migrate container, counts in PR): channels missing association; `channel_members` rows
without graph edge (dev 6); per org workspace the chosen general (name, created_at); people who posted in a group they
are not a member of in 30 days; **drive shares of channel-drive files whose grantee is not that group's UA**
(`drive/internal/domain/sharing.go:77-100` puts Share OAs under PC_Global, so they survive 060) — listed per workspace
for Zane's decision (choice 4).

## Related Code Files
- Modify: `backend/ngac/ngac_ops.go`, `backend/ngac/areas.go`, `workspace/internal/domain/service.go:208-210`,
  `messaging/internal/domain/channels.go`, `messaging/internal/domain/members.go` (ListMembers unchanged logic; doc),
  `messaging/internal/rest/handler.go` (CreateChannel), `messaging/internal/grpc/server.go` (CreateChannel + 2 RPCs),
  `messaging/cmd/main.go` (ServiceOK), `messaging/internal/store/store.go` (WorkspaceWideChannel, cache delete by
  workspace), `messaging/internal/events/realtime_consumer.go`, `backend/proto/messaging/messaging.proto` + generated,
  `auth/internal/domain/provisioning.go` (provisionChannel), `auth/internal/domain/tenants.go` (joinTenant call),
  `workspace/internal/domain/invitations.go` (admit call), `policy/internal/ngac/pdp_workspace_topology_test.go`
  (literal op lists → `ngac.AllOwnerOps()`/`OwnerChannelOps()`/`MemberChannelOps()`/`MemberDocumentOps()`),
  `policy/internal/ngac/pdp_calendar_scenarios_test.go`
- Create: `data/migrations/060_private_groups.sql`, `policy/internal/ngac/pdp_private_groups_test.go`,
  `messaging/internal/domain/private_groups_realpolicy_test.go` (tag `realpolicy`)
- Spec: `docs/specs/tenant-ngac-init/spec.md` (associations; "Groups are private; one workspace-wide channel whose
  members are every workspace member"), `docs/specs/resource-pep-coverage/spec.md`, `docs/specs/chat-calls/spec.md`,
  `docs/specs/calendar/spec.md` (channel calendar visible only to the group)

## Implementation Steps
1. **Deny vectors first** (each with paired allow on the same graph):
   - member not in G: DENY read/write/invite on content, read/write/upload on drive, read on G's calendar
     (`pdp_calendar_scenarios_test.go`: "member not in group G denied G's calendar"); G member ALLOW.
   - owner not in G: DENY read/write/invite; ALLOW manage. Custom role with every `AreaOps(AreaChannels)` op: DENY read,
     ALLOW create_channel on Channels.
   - G member reaching content through another workspace's PC → DENY (intersection).
   - `#general`: member with direct edge ALLOW; outsider DENY; user detached DENY.
   - prohibition on a G member overrides ALLOW.
2. Real-policy integration (messaging): non-member `ListChannels` omits G; `GetChannel`, `SendMessage`, `ListMembers`,
   `CallToken` denied; creator allowed. REST create with body `workspace` stores `private`. User-scoped gRPC cannot make
   a `workspace` channel; `ProvisionWorkspaceChannel` from `auth` can, second one → `ErrAlreadyExists`.
   A member admitted after the change: appears in `#general` `ListMembers`, is a push recipient, `member_count` +1;
   after removal: gone from list and cache.
3. Implement code; proto regen both sides.
4. Write 060; `make db-migrate` twice (second run 0 changes); recreate policy + policy-read + messaging;
   `make policy-invalidate`.
5. Verify on the Docker stack (policy-read present): 0 `*_Channels` associations with read/write/invite; non-member:
   `GET /api/channels/:id/messages` 403 **and** read of a channel-drive file 403; member 200; newly invited member sees
   `#general` with no channel add.
6. `make test s=policy|messaging|drive|workspace|calendar|auth`; `make test-realpolicy`; `make check-ngac`;
   `make build-check`; `cd frontend && npm run build`.

## Success Criteria
- All vectors pass; dev DB checks in step 5 hold; `make db-migrate` twice is clean.
- `#general` roster/push/count correct for members admitted after release A.

## Risk Assessment
| Risk | L×I | Mitigation |
|------|-----|------------|
| Deploy window / rollback with old images against 060 | H×H | release A/B split; reverse SQL documented; rollback runbook |
| Members lose groups they used without membership | H×M | intended; pre-flight list; owners re-add |
| `#general` misidentified | M×M | owner + oldest + name rule; per-workspace list in pre-flight |
| JoinWorkspaceChannel fails at join → member missing in `#general` | M×M | logged; 060 re-run repairs; reconcile query in Success |
| Old shares keep group files readable | M×M | pre-flight list; decision per workspace |
| Stale decisions after SQL | M×H | recreate both policy services + invalidate tool; L1 TTL 30 s bound |
| Ongoing calls with non-members | L×L | new tokens denied; LiveKit tokens expire |

## Rollback
Images: `ROLLBACK=1 ./deploy.sh <prev>` only to release A (safe). Past A, first run (then recreate policy services +
invalidate):
```sql
BEGIN;
DROP INDEX IF EXISTS channels_one_workspace_wide;
UPDATE ngac_associations a SET operations = b.operations FROM _reconcile_060_associations b WHERE b.id = a.id;
INSERT INTO ngac_associations SELECT * FROM _reconcile_060_associations b
  WHERE NOT EXISTS (SELECT 1 FROM ngac_associations a WHERE a.id = b.id);
UPDATE channels c SET channel_type = b.channel_type FROM _reconcile_060_channels b WHERE b.id = c.id;
COMMIT;
```
`#general` direct edges are harmless under the old model and stay.

## Implementation status (2026-10-17)

Implemented on `fix/authz-overprivilege`, not yet committed (controller reviews).

- [x] 1. Deny vectors first, each paired with an allow on the same graph: `pdp_private_groups_test.go` (member / owner /
  custom role / cross-PC / `#general` direct edges / prohibition), `pdp_calendar_scenarios_test.go` (channel calendar private
  to its group; the graph now carries the workspace's Channels grants), `pdp_workspace_topology_test.go` (op lists come from
  `ngac.*Ops()`).
- [x] 2. Real-policy tests (`private_groups_realpolicy_test.go`, tag `realpolicy`): non-member `ListChannels` omits G,
  `GetChannel`, `SendMessage`, `ListMembers`, `CallToken`, `AddMember` denied, creator allowed; owner outside G: manage yes,
  read no; `#general` roster / `member_count` / push recipient / removal; only one workspace-wide channel.
- [x] 3. Code: op sets, `OwnerChannelOps`, `AreaOps(AreaChannels)`, `CreateWorkspace`, private-only `CreateChannel` (REST + gRPC),
  `ProvisionWorkspaceChannel` / `JoinWorkspaceChannel` (proto + `make proto` + `npm run proto:gen`; no TS diff, the TS
  generator only reads `ws.proto`), auth `provisionChannel` / `joinTenant`, workspace `admit`, realtime consumer cache cleanup.
- [x] 4. `data/migrations/060_private_groups.sql`; `make db-migrate` run twice on dev, second run 0 rows; reverse SQL
  exercised on dev (restored 254 read/write/invite associations, 127 `workspace` channels, no index) and 060 re-applied.
- [x] 5. Observed on dev with the real policy process over the migrated DB: member outside a throwaway group DENY
  read/write/invite and its drive (ALLOW before 060), group member ALLOW, three real members ALLOW `#general`, person of
  another workspace DENY; 0 `*_Channels` associations with read/write/invite; every `#general` member has the direct edge.
- [x] 6. `make test s=policy|messaging|drive|workspace|calendar|auth`, `make test-realpolicy`, `make check-ngac`,
  `make check-layering`, `make check-docs`, `make build-check`, `cd frontend && npm run build`.
- [x] Specs: `tenant-ngac-init`, `resource-pep-coverage`, `chat-calls`, `calendar`, `workspace-admin-authorization`.

Deviations from the file list (all needed to make the change work or keep suites green):
- `workspace/cmd/main.go`, new `workspace/internal/chat/joiner.go`, `docker-compose.yml`, `deploy/docker-compose.prod.yml`
  (`MESSAGING_SERVICE_ADDR` for workspace): workspace had no messaging client.
- `messaging/internal/grpc/policy.go` (`ServerPolicy()`, used by `cmd/main.go`) instead of an inline policy, so tests run the
  production allow-list.
- `backend/testutil/policy_graph.go` + `policy_process_test.go` (seeded topology follows `OwnerChannelOps`),
  `scripts/run-realpolicy-tests.sh` (the shared module no longer tries to run the service module's tagged package),
  test helpers in messaging / drive that inserted several `workspace`-typed channels per workspace (the new unique index).
- Step 6 of 060 covers every `<id>_Channels` OA, including those of workspaces deleted long ago (38 such associations in dev).
- `ngac_ci` was behind the migration chain (no `messages.content_text`, no calendar tables); it was brought to HEAD with
  `data/init.sql` + migrations (dump kept in the session scratchpad) so `make test` could run.

### Review fixes (2026-10-17, coordinator review: approve with fixes)

- [x] H1 calendar: `accessLevels` holds a level only when the person can also `read` the calendar (an owner's inherited
  `manage` on a group no longer reads its calendar). `private_group_calendar_test.go`: owner outside the group denied (list
  events, get event, ensure calendar, not listed), plain member denied, group member allowed, owner still manages the
  workspace calendar. `docs/specs/calendar` updated.
- [x] H2 `#general` by type: `Store.WorkspaceWideChannel` asks (lazily, positive kept) whether `channels_one_workspace_wide`
  is valid; if so the workspace's one `workspace`-typed channel is it, else the name+owner heuristic. 060 runs the heuristic
  only while the index is absent (`_m060_state`), and its closing retype is skipped once the index exists.
  `scripts/test-migration-060-rerun.sh` (scratch copy of dev, rename a `#general`, re-run 060: unchanged). The "no
  workspace-wide channel" log is a Warn.
- [x] H3 two-release rule: nothing needs 060 at start-up; the same build was exercised on both sides (store tests force the
  index state; first-run 060 replayed from the pre-060 dev dump: 105 general, 22 retyped, index created; migration
  commit stands alone: it touches no code).
- [x] M1 `Unimplemented` from `ProvisionWorkspaceChannel` falls back to the old `CreateChannel` (type `workspace`, as the
  owner); from `JoinWorkspaceChannel` it is a Warn (auth, workspace joiner). Other failures are not swallowed by the fallback.
- [x] M2 both calls bounded by a 5 s context (auth, `workspace/internal/chat/joiner.go`); `JoinWorkspaceChannel` now carries
  `user_id` (proto regenerated both sides, no TS diff). Self-healing: `ListChannels` joins a workspace member who lacks the edge
  (membership verified first; a person of another workspace is not joined); a positive is memoised per instance.
- [x] M3 (reverted by the coordinator): joining writes no notification level; members joined via the workspace path keep the
  default. Not done: batching the per-recipient lookups in `push.go` (the policy batch API is per user, not per object).
- [x] M4 phase-03 file notes that 061 also strips `#general` edges and cache rows of non-members.
- [x] L1 `requireService(ctx, "auth", "workspace")` (provision: `auth`) in the handler, tested with a deliberately loose policy.
- [x] L2 `actingAs` uses the real `users.id` (no node id in `UserID`).

Test helper additions: `answerPolicy.IsAssigned` in the messaging REST tests (listing now asks it).

### Open follow-ups
- #general push volume: needs a 'mentions only' level (schema change) - follow-up, not in this plan.
