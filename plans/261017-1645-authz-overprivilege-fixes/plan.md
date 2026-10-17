---
title: "Sửa lỗi vượt quyền (authz over-privilege) F1–F10"
description: "Private groups, channel/DM/calendar membership bound to the workspace, removed members lose their session, admin guards, tenant_users derived from the graph."
status: pending
priority: P1
effort: 39h
branch: fix/authz-overprivilege
base: origin/main (43a1509)
tags: [security, ngac, authz, messaging, workspace, auth, approval, calendar]
created: 2026-10-17
blockedBy: []
blocks: [261017-1646-contacts-lark-redesign]
report: ../reports/security-261017-1630-authz-overprivilege-audit.md
redteam: [reports/redteam-security-adversary.md, reports/redteam-failure-modes.md, reports/redteam-assumptions.md, reports/redteam-scope-contracts.md]
specs: [tenant-ngac-init, resource-pep-coverage, workspace-admin-authorization, tenant-identity, tenant-auth-flow, chat-calls, calendar]
---

# Sửa lỗi vượt quyền F1–F10

Source: audit `plans/reports/security-261017-1630-authz-overprivilege-audit.md`. Every finding was re-verified
against code + dev DB on 2026-10-17. Red-team review of the same day (15 findings accepted by Zane) is applied
throughout; see "Red Team Review" below. User decisions are binding and are not reopened.

## Baseline

- Branch `fix/authz-overprivilege` cut from **`origin/main` (43a1509)**, not local `main` (97e2317, 317 files behind).
  origin/main adds `backend/services/calendar` and migrations `057_calendar.sql`, `058_calendar_occurrence_answers.sql`.
- At branch cut: `git ls-tree --name-only origin/main data/migrations | tail -1` must still be `058_*`; if not,
  renumber this plan's migrations and the contacts plan's `064`.
- File:line citations in phase files refer to origin/main unless marked "(local)"; re-check line numbers at cut.

## Outcome / acceptance

- A workspace member who is not in a group gets DENY on read, write, invite for that group, its drive, its calendar
  and its call. Owners keep only `{create_channel, manage}`; custom roles only `{create_channel}` on Channels.
- `#general` (the one workspace-wide channel) is readable by every workspace member through direct per-user edges
  (roster, push, member_count all correct), and by nobody else.
- Nobody becomes a workspace member through a channel, DM, event-guest UA, or a UA passed as a "user"
  (messaging, workspace, calendar).
- A removed member: row tombstoned `removed`; refresh re-mints a token for their default remaining tenant (or
  tenant-less); approval refused at once; WS reconnect refused; domain sign-in does not re-add. Exposure bound =
  REST access token TTL (≤ 15 min) for services that trust the JWT tenant; 0 for approval and WebSocket.
- A policy/DB outage during refresh returns 503, leaves the refresh cookie unspent, and the SPA does not log out.
- DMs and calls (incl. every DM write path) only between people sharing a workspace.
- Removing someone holding `manage` on any area needs Mgmt `manage`; only real folders are deletable as folders.
- Every tenant-token mint (sign-in, switch, refresh) agrees with the graph; tenant_users rows written graph-first and
  reconciled.
- Each phase: deny-first tests each paired with an allow test on the same fixture, spec updated in the same commit,
  `make test s=<svc>` + `make test-realpolicy` green, `make check-ngac`, `make check-layering`, `make build-check`.

## Phases

| # | Phase | Findings | Prio | Effort | Depends on |
|---|-------|----------|------|--------|-----------|
| 01 | [Foundations: legacy-workspace repair 059, real-policy harness, membership helper, cache invalidator, runbook](phase-01-foundations.md) | prereq | P1 | 6h | — |
| 02 | [Private groups + #general direct edges + calendar vectors (060)](phase-02-private-groups.md) | F1 | P1 | 8h | 01 |
| 03 | [Channel/workspace/calendar membership bound to `<W>_Members` (061)](phase-03-channel-membership-bound.md) | F2 | P1 | 5h | 01, 02 |
| 04 | [Removed member loses access: refresh, WS auth, approval, tombstone + tenant_users backfill (062, 063)](phase-04-removed-member-cutoff.md) | F3, F5 | P1 | 8h | 01, 03 |
| 05 | [DM and call gate centralised (DM, event calls)](phase-05-dm-call-relationship.md) | F4 | P2 | 4h | 01, 03 |
| 06 | [Workspace admin guards](phase-06-workspace-admin-guards.md) | F6, F10 | P2 | 2h | 04 |
| 07 | [Graph-first tenant writes + graph-gated token mints](phase-07-tenant-users-graph-reconcile.md) | F7 | P2 | 4h | 04 |
| 08 | [Messaging low items + DM listing perf](phase-08-messaging-low-items.md) | F8, F9, P1 | P3 | 2h | 03, 05 |

Order = severity, then dependency. 02→03→04 strictly sequential (shared files, migration order). 05 may run in
parallel to 04 (disjoint files). 06 and 07 serialize after 04.

## Migrations and release train (two-release rule)

| File | Phase | Content | Ships in |
|------|-------|---------|----------|
| `059_ngac_workspace_id_keyed_repair.sql` | 01 | create/rename `PC_<id>`, `<id>_Members`, `<id>_Owners`, `<id>_Channels` for legacy workspaces (pattern 023), relink TenantMember/Owner | R0, alone, before any new code |
| `060_private_groups.sql` | 02 | channel types, #general direct edges + cache, Channels associations, unique index (CONCURRENTLY, separate file statement outside txn) | after the release carrying Phase 02 code |
| `061_channel_membership_reconcile.sql` | 03 | remove injected channel edges, stale cache rows | after Phase 03 code |
| `062_tenant_users_removed_status.sql` | 04 | the `tenant_users.status` CHECK allows `removed` (nothing else; the code before it never writes `removed`) | with or before the release carrying Phase 04 code |
| `063_tenant_users_tombstone_reconcile.sql` | 04 | `ngac_node_id` backfill, insert-missing rows for graph members, tombstone active rows with no graph membership (guards: tenant UAs under Members/Owners, per-workspace half rule) | after Phase 04 code and 062 |

Contacts plan reserves `064`. All tenant_users SQL lives in 062 and 063 (Phase 04) so the security plan stays at 059–063.

Rules (red-team #6, #15):
- **Release A / Release B per migration**: release A ships code that works on both pre- and post-migration data
  (writes the new shape, reads both); release B adds the migration file. Rollback B→A is safe; rollback past A needs
  the phase's reverse SQL first. 059 ships alone first (old code tolerates it, like 023).
- **Re-runnable**: `IF NOT EXISTS` / `DROP ... IF EXISTS` / fixpoint predicates; every phase runs `make db-migrate`
  **twice** and the second run must be a no-op.
- **Guarded**: 060–063 start with `RAISE EXCEPTION` if any workspace lacks `<id>_Members` (pattern
  `025_node_and_drive_root_uniqueness.sql:32-43`) and if affected-row count exceeds a stated threshold.
- **Backed up**: each data-changing migration first copies affected rows to `_reconcile_<NNN>_<what>` tables; reverse
  SQL = `INSERT ... SELECT` / `UPDATE ... FROM` from them.
- **Dry-run on prod before approval**: run the migration's dry-run SELECT inside the prod migrate container
  (`docker compose --profile tools run --rm migrate psql ...`), paste counts in the PR; documented in
  `docs/deployment.md` by Phase 01.
- **After graph-writing migrations**: `deploy.sh` already recreates `policy` and `policy-read`
  (`deploy/deploy.sh:101-107`); then run `policy invalidate-cache --all-workspaces` (Phase 01 tool calling the
  existing `InvalidateCache` RPC, `write_server.go:324`). Dev: `make dev-restart` of policy, policy-read, messaging,
  then `make policy-invalidate`. No new startup flush.

## File ownership (no two parallel phases share a file)

- 01: `backend/testutil/policy_process.go` (new), `backend/pkg/policyclient/membership.go` (new), `backend/ngac/ngac_ops.go` (parser), `backend/services/policy/cmd/main.go` (invalidate-cache subcommand), `backend/services/policy/internal/grpc/authpolicy.go` (ServiceOK for the tool), `Makefile` (`test-realpolicy`, `policy-invalidate`), `.github/workflows/ci.yml` (real-policy job), `deploy/deploy.sh` (invalidate step), `docs/deployment.md`, `data/migrations/059_*.sql`
- 02: `workspace/internal/domain/service.go`, `messaging/internal/domain/channels.go`, `messaging/internal/domain/members.go` (ListMembers only), `messaging/internal/rest/handler.go` (CreateChannel), `messaging/internal/grpc/server.go` (CreateChannel + 2 new RPCs), `messaging/cmd/main.go` (ServerPolicy `:190` gains ServiceOK for the 2 new RPCs), `messaging/internal/store/store.go` (workspace-wide channel lookup), `messaging/internal/events/realtime_consumer.go` (cache cleanup on removal), `backend/proto/messaging/*.proto` + generated Go/TS, `auth/internal/domain/provisioning.go` (provisionChannel), `workspace/internal/domain/invitations.go` (admit → JoinWorkspaceChannel), `auth/internal/domain/tenants.go` (joinTenant → JoinWorkspaceChannel), `ngac/ngac_ops.go` (op sets), `ngac/areas.go`, `policy/internal/ngac/pdp_workspace_topology_test.go`, `policy/internal/ngac/pdp_calendar_scenarios_test.go`, `data/migrations/060_*.sql`
- 03: `messaging/internal/domain/members.go` (AddMember), `workspace/internal/domain/authz.go`, `member_directory.go`, `members.go` (ListMembers), `service.go` (ListAccessibleWorkspaces), `invitations.go` (rename call), `calendar/internal/domain/membership.go`, `data/migrations/061_*.sql`
- 04: `auth/internal/domain/refresh.go`, `tenants.go` (joinTenantByDomain), `errors.go`, `auth/internal/rest/session.go`, `frontend/src/api/client.ts`, `frontend/src/stores/websocket.store.ts`, `messaging/internal/grpc/hub_conn.go`, `hub_domain.go`, `approval/internal/rest/handler.go`, `approval/internal/domain/service.go`, `execution.go`, `approval/cmd/main.go`, `approval/internal/store/names.go`, `workspace/internal/store/directory_store.go`, `teardown.go`, `department_store.go`, `workspace/internal/domain/member_directory.go` (interface), `members.go` (RemoveMember tail), `leave.go`, `calendar/internal/store/freebusy.go`, `data/migrations/062_*.sql`
- 05: `messaging/internal/domain/authz.go`, `dms.go` (FindOrCreateDM), `calls.go`, `calls_push.go`, `calls_event.go`, `polls_tasks.go`, `reactions_pins_receipts.go`, `lifecycle.go` (call sites only), `messages.go`, `rest/handler.go` (CreateDM), `grpc/server.go` (CreateDM)
- 06: `workspace/internal/domain/members.go` (RemoveMember authorization), `folders.go`, `ngac/ngac_ops.go` (folder predicate), `ngac/areas.go` (comment)
- 07: `auth/internal/domain/tenants.go` (joinTenant, provisionTenant, SwitchTenant), `provisioning.go` (assignTenantNGAC, autoProvisionWorkspace), `google.go` (signinResultFor), `otp.go` (result builders), `auth/internal/domain/workspaces.go` (switcher filter), `workspace/internal/domain/invitations.go` (admit)
- 08: `messaging/internal/domain/members.go` (RemoveMember), `messaging/internal/domain/service.go` (memberCache), `messaging/internal/store/store.go`, `dms.go` (ListDMs), `grpc/server.go` (SendMessage)

Shared files across phases (`messaging handler.go/server.go/members.go/dms.go/store.go`, `workspace members.go/
invitations.go`, `auth tenants.go`, `ngac_ops.go`) are touched only by sequential phases.

## Cross-cutting rules applied in every phase

- Deny case first, each paired with an allow case on the same fixture (so an unknown node cannot fake a deny).
  Graph-shape vectors in `backend/services/policy/internal/ngac/` (real PDP). Service behaviour against a **real
  policy process** (Phase 01 harness, build tag `realpolicy`, `TEST_DATABASE_URL` on 5433, `REDIS_ADDR` for auth
  REST). Graph fixtures only through the harness write client; SQL fixtures only for business rows.
- NGAC strings only from `backend/ngac`; `make check-ngac`.
- Graph writes via policy write gRPC (EPP). Membership/read-after-write checks that must see a just-made removal
  (approval gate) use the writer address `POLICY_SERVICE_ADDR`, not `policy-read`.
- Verification of graph migrations runs on the **Docker stack** (with `policy-read`), incl. a non-member
  channel-drive file read → 403.

## Decisions (Zane, 2026-10-17) and recorded choices

User decisions (numbering used by phase files): 1 groups are private; 2 DMs/calls only between workspace-mates;
3 removal = tombstone + refresh check + approval named-approver check; 4 removing a `manage` holder needs `manage`;
5 AddMember checks the target is an active workspace member and a user node; 6 ngac_* is the truth, tenant_users
reconciled; 7 legacy workspaces repaired by migration (not denied).

Claude's choices (accepted with the red team):
1. Custom roles get only `{create_channel}` on the Channels area; only Owners moderate (`manage`) groups they are
   not in.
2. F6: removing someone who holds `manage` on **any** area (Mgmt, Documents, Channels, Assets) requires Mgmt `manage`.
3. A workspace-wide channel can be created only by the auth service (service identity), never by a user call.
4. Pre-flight for 060 lists drive shares of channel-drive files whose grantee is not the group's own UA.
5. DM drive uploads by an ex-colleague are **out of scope** (drive has no DM-relationship context; DM content OA
   write is gated in messaging). Follow-up item for the contacts/cross-org work.
6. All tenant_users SQL owned by Phase 04: the status constraint alone in 062 (ships before or with the code), backfill, insert-missing and drift tombstone in 063.
7. F7 single mechanism: graph-first writes + 063 reconcile + graph check at every tenant-token mint (sign-in,
   switch, refresh). No GetMe or ListContacts gate.

## Overlap

- `plans/261017-1516-search-suggestions` (frontend, in-progress): no shared files. Phase 04 edits
  `frontend/src/api/client.ts` and `frontend/src/stores/websocket.store.ts`; search-suggestions does not.
  Behavioural only: chat suggestions list fewer groups after 060.
- `261017-1646-contacts-lark-redesign`: blocked by this plan; uses `064`; its `phase-06-frontend-data.md` touches
  `websocket.store.ts` → must rebase after Phase 04. It should use the Phase 01 membership helper.

## Red Team Review

Session 2026-10-17. Four reviewers (security adversary, failure modes, assumptions, scope/contracts):
36 raw findings → 15 accepted (deduplicated), all accepted by Zane.

| # | Finding | Severity | Disposition | Applied to |
|---|---------|----------|-------------|------------|
| 1 | Plan based on stale local main; 057/058 already exist on origin/main | Critical | Accepted: base origin/main 43a1509, migrations 059–063 | plan.md, all phases |
| 2 | Refresh check after `Rotate` spends the token → 503 path still logs out; SPA logs out on refresh 5xx | Critical | Accepted: Lookup → check → Rotate; cookie untouched on 503; frontend keeps session | Phase 04 |
| 3 | Legacy workspaces without ID-keyed `<id>_Members`/`PC_<id>` wiped by reconcilers | Critical | Accepted: repair 059 first; 060–063 abort if any remain | Phase 01, 02, 03, 04 |
| 4 | Graph-first join depends on `TenantMember_<ws>` (missing in 83 personal + 8 org) | Critical | Accepted: assign under `<ws>_Members`/Owners; TenantMember optional; IsAssigned undo; strict path reordered | Phase 07 |
| 5 | Calendar keeps reaches-PC membership; no calendar spec/vector | Critical | Accepted: calendar membership via helper; vector; spec | Phase 02, 03, 05 |
| 6 | Old images run between migrate and up, and after rollback → drift, 500s | High | Accepted: release A/B split; CONCURRENTLY IF NOT EXISTS; risk H×H | plan.md, Phase 02, 03, 04 |
| 7 | `policy-read` replica not restarted/flushed; startup flush duplicates RPC | High | Accepted: restart both, InvalidateCache tool, no startup flush, approval on writer, drive 403 check | plan.md, Phase 01, 02, 04 |
| 8 | WS reconnect with valid JWT defeats socket close; "0 exposure" false | High | Accepted: membership check in handleAuth, terminal close code, claim fixed | Phase 04 |
| 9 | Tombstone readers unaudited (CanonicalApprover, OtherMembers); EnsureTenantUser reactivates `disabled` | High | Accepted: reader inventory + filters; reactivate only `removed` | Phase 04 |
| 10 | Removal event suppressed when tombstone write fails | High | Accepted: event right after detach, then return error | Phase 04 |
| 11 | DM gate misses polls, tasks, reactions, pins, edits, call join/ring | High | Accepted: centralised in authorizeChannel/authorizeMessage + mayStayInCall; DM drive declared out of scope | Phase 05 |
| 12 | #general via UA→UA invisible to roster, push, count, RemoveMember | High | Accepted: direct per-user edges written on admit/join, removed on detach | Phase 02 |
| 13 | Phase 04 gate before Phase 07 repairs rows; removal logs out of every tenant | Medium | Accepted: backfill/insert-missing in 063 (Phase 04); re-mint default tenant | Phase 04, 07 |
| 14 | Harness hits dev Redis/Kafka, builds inside 60 s timeout, t.Skip fails strict runner, SQL fixtures invisible | High | Accepted: unreachable Redis/Kafka, prebuilt binary, build tag, harness-only graph fixtures, paired tests | Phase 01 |
| 15 | Migrations not re-runnable; no backup/threshold; prod dry-run impossible as written | Medium | Accepted: idempotent, `_reconcile_NNN_*`, RAISE threshold, migrate-container dry-run doc | plan.md, Phase 01–04 |

Scope cuts applied: `ngac/topology.go` dropped (topology test uses `ngac.*Ops()`); Phase 07 single mechanism, GetMe
gate dropped; sign-in default tenant + switcher list graph-gated with soft fallback; workspace `ListMembers` added to
Phase 03; Members-UA-id cache made mandatory (no schema column, which would break the two-release rule).

### Whole-Plan Consistency Sweep

Run 2026-10-17 over plan.md + phase-01..08 (`grep -n`):

| Search | Result |
|--------|--------|
| `057_` / `058_` as this plan's migrations | none; only references to origin/main's calendar files |
| `059`–`063` | each defined once (table above) and referenced consistently by owning phase |
| `064` / `060_tenant` (old Phase 07 migration) | `064` only as contacts plan's reservation; no `060_tenant` |
| `after \`Rotate\`` / Rotate-then-check | none; Phase 04 states Lookup → check → Rotate |
| `TenantMember` as a dependency | only "optional"/"not required" (Phase 07) and the existing chain description (Phase 01 helper) |
| `startup flush` / `InvalidateAll` at start | only in Red Team table as dropped |
| `topology.go` | only in Red Team table / scope cuts as dropped |
| `GetMe` gate | only "No GetMe gate" (plan.md, Phase 07) |
| `0 exposure` / "0 for live sockets" | replaced by "0 for approval and WebSocket; ≤ access TTL elsewhere" |
| `make db-migrate` | every migration phase runs it twice |
| `policy-read` | restart + invalidate in plan.md rules, Phase 01 tool, Phase 02/03/04 steps |
| `UA→UA` for #general | only in Red Team table (#12) as replaced; Phase 02 says "No UA→UA edge" |
| Fixed during sweep | Phase 08 pointed at old `058` cache cleanup → `061`; Phase 08 refused removal of the `<W>_Members` edge → now refuses any removal from `#general`; phase files cited "decision N" for Claude's choices → "choice N"; user decisions numbered 1–7 in plan.md to match phase citations; ownership list for 05 (`calls_push.go`) and 07 (`auth/internal/domain/workspaces.go`) corrected |
| `*_integration_test.go` without tag | none; all real-policy tests named `*_realpolicy_test.go` with build tag |

Unresolved contradictions: none blocking. Two points recorded for the implementer:
- Choice 3 (auth-only workspace channel) needs a messaging proto change (two service-only RPCs) → `make proto` +
  `npm run proto:gen` in Phase 02.
- Phase 04's refresh gate deliberately treats "graph member with no row" as allowed until 063 backfills, so release A
  does not lock out the 15 NULL-node / missing-row users; the deny side is "not a graph member" or row
  `removed|disabled|invited`.

## Unresolved questions

1. Owners and private groups: Owners keep `manage` on groups they cannot list (`ListChannels` filters on read). There is
   no UI path for an owner to rename/delete an unseen group; `manage` then mostly means removing members (Phase 08).
   Acceptable?
2. Who may add to a group: any group member (current `invite` in `ChannelMemberOps`); removal = self, creator, or a
   `manage` holder. Confirm.
3. `#general` for existing data = oldest channel named "general" created by the workspace owner; 5 dev workspaces have
   none. OK?
4. Existing DMs whose two people no longer share a workspace: writes blocked, history readable. Confirm.
5. Voluntary leave also tombstones → domain auto-join will not bring a leaver back. Confirm.
6. Thresholds for 061/063 `RAISE EXCEPTION` guards: plan proposes 50 rows each; set from the prod dry-run.
