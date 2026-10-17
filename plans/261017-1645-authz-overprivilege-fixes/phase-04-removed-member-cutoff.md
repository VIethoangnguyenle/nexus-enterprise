---
phase: 4
title: "Removed member loses access: refresh, WS auth, approval, tombstone + tenant_users reconcile 063 (F3, F5)"
status: completed (release A code, 062 + 063 applied to dev and ngac_ci; prod dry-run still to do)
priority: P1
effort: "8h"
dependencies: [1, 3]
---
# Phase 4: Removed member loses access (F3, F5)

## Overview
Decision 3: removal marks `tenant_users.status='removed'`; token refresh checks membership; approval's named-approver
path checks membership. Today (origin/main):
- removal DELETEs the row (`workspace/internal/store/directory_store.go:88-96`, from `members.go:106`, `leave.go:48`)
  → Google `hd` sign-in re-joins (`auth/internal/domain/tenants.go:38-48`) = F5.
- `RefreshSession` (`auth/internal/domain/refresh.go:201-212`) calls `Rotate` (`:134`), which marks the token spent
  before minting (`:161-176`), and re-mints the tenant with no check.
- approval trusts `claims.TenantID` (`approval/internal/rest/handler.go:105-131`); `actingRow` skips direct rows
  (`approval/internal/domain/execution.go:175`).
- WS `handleAuth` (`messaging/internal/grpc/hub_conn.go:98-137`) checks only the JWT, then joins tenant presence;
  the SPA reconnects on any close code but 1000 (`frontend/src/stores/websocket.store.ts:266-282`).
- the SPA logs out when a refresh fails for any reason (`frontend/src/api/client.ts:156-168` returns the original 401
  to `toResult`, which calls `logout()`), although `refreshAccessToken` itself keeps the session on 5xx (`:43-47`).

## Requirements
1. **Refresh order**: `RefreshSession` = `s.refresh.Lookup(token)` (`refresh.go:235`, does not spend) →
   `requireActiveMember(ctx, id.TenantID, id.UserID, id.NGACNodeID)` (only when `id.TenantID != ""`) → `Rotate`.
   - member → `Rotate` as today.
   - not a member → rotate into the user's default remaining tenant: first tenant from `ListTenantsByUser` that passes
     `requireActiveMember` (same `selectDefaultTenant` order), else tenant-less. New `RefreshStore.RotateTo(ctx, token,
     tenantID)`; the session family stays (other tenants unaffected). Response carries `tenant_id` so the SPA can switch.
   - policy/DB error → `ErrUnavailable` → **503, cookie untouched, token not spent** (`auth/internal/rest/session.go`
     must not clear the cookie on 503).
   - `requireActiveMember` = `IsWorkspaceMember` (graph) ∧ row status ∉ {`removed`,`disabled`,`invited`}. A graph
     member with **no** row is allowed (release A tolerance; 063 inserts those rows). Reused by Phase 07.
2. **Frontend** (`frontend/src/api/client.ts`): `refreshAccessToken` returns `ok | ended | unavailable`;
   `withRefreshRetry` on `unavailable` throws `ApiError(…, 503)` instead of returning the 401 (no logout); on a token
   whose tenant differs from the current one, the auth store switches workspace and shows a notice. Vitest tests.
3. **WebSocket** (`hub_conn.go` `handleAuth`): when `claims.TenantID != ""`, `IsWorkspaceMember(claims.NGACNodeID,
   tenant)` via messaging's policy connection (writer, `messaging/cmd/main.go:49`); error or not member → auth failure
   and close with code **4403**. `websocket.store.ts`: 4403 → no reconnect loop; call `refreshAccessToken` once (gets the
   default-tenant token) and reconnect with it if it changed, else stop. `revokeWorkspaceLocal` (`hub_domain.go:237-252`)
   also closes that user's sockets for tenant W with 4403.
4. **Approval**: `tenantSchemaMiddleware` first calls `h.svc.RequireMember(ctx, claims.NGACNodeID, claims.TenantID)`
   (adapter `IsWorkspaceMember` on the existing writer connection, `approval/cmd/main.go:45,59` — keep it on
   `POLICY_SERVICE_ADDR`, never `policy-read`). Deny → 403. `actingRow` direct branch also requires membership.
   Approval gRPC exposes only reads (`approval/internal/grpc/server.go:88-164`) — out of scope.
5. **Tombstone writes**: `RemoveTenantUser` → `MarkTenantUserRemoved` (`status='removed', department_id=NULL`);
   `EnsureTenantUser` (`directory_store.go:70-80`) `ON CONFLICT DO UPDATE SET status = CASE WHEN tenant_users.status =
   'removed' THEN 'active' ELSE tenant_users.status END, ngac_node_id = COALESCE(...)` — `disabled`/`invited` never
   reactivated by an invite.
6. **Removal ordering** (`members.go:100-113`, `leave.go:44-55`): after `detachMember` succeeds, emit `member_removed`
   and `announceAccessChanged` immediately; then tombstone; a tombstone error is returned (500 with request id) after
   the events are out. Retry is idempotent.
7. **Domain join** (`tenants.go:38-48`): existing row with status ≠ `active` → return nil tenant, no join.
8. **tenant_users reader inventory** (origin/main `grep -rn tenant_users backend/services`, non-test):

   | Reader | Meaning | Action |
   |--------|---------|--------|
   | `approval/internal/store/names.go:115` CanonicalApprover | membership | add `tu.status='active'` |
   | `workspace/internal/store/teardown.go:33` OtherMembers | membership | add `status='active'` |
   | `workspace/internal/store/department_store.go:106` CountMembersByDepartment | membership | add `status='active'` |
   | `calendar/internal/store/freebusy.go:84` attendee visibility join | membership | add `tu.status='active'` |
   | `approval/internal/store/names.go:54,58,62` names | display (history) | keep |
   | `asset/internal/store/names.go:22`, `document/internal/texts/store.go:77` | display | keep |
   | `messaging/internal/store/calls.go:383,409` node lookup for ringing | lookup; DM gate is Phase 05 | keep |
   | `workspace/internal/store/directory_store.go:55` profile + status | display with status | keep |
   | `asset/handover.go:36,189`, `auth/store/directory.go:77,79,130`, `auth/store/store.go:163`, `calendar/store/people.go:66,87`, `calendar/store/calendars.go:53` | membership | already `active` |
   | `auth/store/store.go:189` GetTenantUser | returns status; callers check | keep |
   | writers `auth/store/store.go:152`, `directory_store.go:76,90`, `department_store.go:92,128`, `teardown.go:76` | write | 5 above; teardown keeps DELETE (workspace deletion) |
9. **Two migrations.** `data/migrations/062_tenant_users_removed_status.sql` only widens the status CHECK to allow `removed` (idempotent; ships before or with the code, which is the first thing that writes `removed`). Then **063 `data/migrations/063_tenant_users_tombstone_reconcile.sql`** (release B after this phase's code; dry-run
   in the prod migrate container first):
   1. Guard: any workspace lacking `<id>_Members` → `RAISE EXCEPTION`.
   2. Abort unless 062 has run; abort if a TenantMember_/TenantOwner_ UA is not under its Members/Owners UA.
   3. Backup affected rows to `_reconcile_063_tenant_users`.
   4. Backfill `ngac_node_id = users.ngac_node` where NULL (dev 15).
   5. Insert `role='member', status='active'` for graph members (reach `<ws>_Members`) without a row (dev 0).
   6. Tombstone `active` rows whose user does not reach `<tenant>_Members` (dev 17): `status='removed',
      department_id=NULL`. Threshold guard (default 50).
   Re-runnable; `tenant_users` only (no graph write → no policy restart needed).

## Exposure after removal (replaces the earlier "0" claim)
- Approval: 0 (writer-backed gate per request).
- WebSocket: 0 once the detach is in the writer's graph (sockets closed; reconnect refused).
- REST paths that trust the JWT tenant without a graph check: ≤ access TTL 15 min (`auth/internal/auth/jwt.go:34`),
  then refresh re-mints for another tenant.

## Architecture
```
RemoveMember/Leave ─> detachMember ─> emit member_removed ─> MarkTenantUserRemoved (error returned, events already out)
   consumer ─> RevokeWorkspaceSubscriptions ─> close sockets 4403
refresh ─> Lookup ─> requireActiveMember ─┬ ok ─> Rotate
                                          ├ not member ─> RotateTo(default tenant | "")
                                          └ error ─> 503, nothing spent
WS auth ─> IsWorkspaceMember(token tenant) ─✗─> 4403 (client: refresh once, no loop)
approval ─> RequireMember (writer) ─✗─> 403
```

## Related Code Files
- Create: `data/migrations/063_tenant_users_tombstone_reconcile.sql`; tests (tag `realpolicy` where policy is involved):
  `auth/internal/domain/refresh_membership_realpolicy_test.go` (Redis via `REDIS_ADDR`),
  `auth/internal/domain/domain_join_tombstone_test.go`, `approval/internal/rest/membership_gate_realpolicy_test.go`,
  `messaging/internal/grpc/hub_auth_membership_realpolicy_test.go`, `workspace/internal/store/directory_store_test.go`
  (extend), `frontend/src/api/client.test.ts` (extend), `frontend/src/stores/websocket.store.test.ts` (extend)
- Modify: `auth/internal/domain/refresh.go`, `tenants.go` (joinTenantByDomain), `errors.go` (`ErrUnavailable` next to
  `ErrAccessDenied :19`), `auth/internal/rest/session.go`, `frontend/src/api/client.ts`,
  `frontend/src/stores/websocket.store.ts`, `messaging/internal/grpc/hub_conn.go`, `hub_domain.go`,
  `approval/internal/rest/handler.go`, `approval/internal/domain/service.go`, `execution.go`, `approval/cmd/main.go`,
  `approval/internal/store/names.go`, `workspace/internal/store/directory_store.go`, `teardown.go`,
  `department_store.go`, `workspace/internal/domain/member_directory.go` (interface), `members.go`, `leave.go`,
  `calendar/internal/store/freebusy.go`
- Spec: `docs/specs/tenant-identity/spec.md` (tombstone; refresh re-check and re-mint; 503 keeps the session),
  `docs/specs/tenant-auth-flow/spec.md` (domain join never reactivates), `docs/specs/resource-pep-coverage/spec.md`
  (approval needs current membership; WS auth checks membership), `docs/specs/calendar/spec.md` (free/busy shows active
  members only)

## Implementation Steps
1. Deny tests first, each with a paired allow:
   - refresh: removed member (row `removed` + detached) → token for default other tenant, W absent; member of no other
     tenant → tenant-less token; drift (row active, graph detached) → re-mint away from W; **policy down → 503, then
     policy up → the same cookie refreshes** (two calls); active member → same tenant.
   - frontend: refresh 503 → no `logout()`, ApiError 503; tenant change → store switches.
   - WS: removed member reconnects with a still-valid token → 4403, no reconnect loop; member of W → authenticated;
     socket for tenant V of the same user stays.
   - approval: removed named approver (direct row) → 403 `/approve`; removed member → 403 `POST /requests`,
     `GET /templates`; active member → 200.
   - removal: tombstone store error → `member_removed` still emitted, RemoveMember returns error.
   - store: removal → `removed` + dept cleared; `EnsureTenantUser` on `removed` → `active`, on `disabled` → stays
     `disabled` (invite accept test); CanonicalApprover rejects a removed user; delete-workspace allowed after everyone
     else was removed.
   - domain join: removed row + verified `hd` → no change, not assigned, default tenant ≠ W.
2. Implement 1–8 (release A).
3. 063 in the next release; dry-run; `make db-migrate` twice; verify counts: NULL node 0, graph members without row 0,
   active rows not reaching Members 0.
4. `make test s=auth|approval|workspace|messaging|calendar`; `make test-realpolicy`; `cd frontend && npm test && npm run
   lint`; `make check-layering`, `make build-check`.
5. Manual (Docker stack): remove a test member; their other browser: approval 403 at once, socket closes and stays
   closed, next refresh lands in their other workspace; Google `hd` sign-in does not re-add.

## Success Criteria
- Every F3/F5 exploit line has a failing-then-passing test; the two-call outage test passes.
- No `DELETE FROM tenant_users` outside workspace teardown.

## Risk Assessment
| Risk | L×I | Mitigation |
|------|-----|------------|
| Policy outage logs everyone out | M×H | check before spend; 503 keeps cookie; frontend no logout; two-call test |
| Release A gate locks out users with missing/NULL rows before 063 | M×H | graph member without row allowed; NULL node irrelevant (token node used) |
| WS check adds a policy call per connect | M×L | one cached-id lookup + GetAncestors |
| Re-invite blocked / suspension bypassed | M×M | reactivate only `removed`; tests both |
| Previous release sees `removed` rows | L×L | its membership readers filter `active` after this phase; don't roll back past A while re-inviting |
| 063 tombstones a real member | M×H | backup table, threshold guard, prod dry-run |

## Rollback
Code: revert to release A / previous. 063: `UPDATE tenant_users t SET status = b.status, department_id = b.department_id,
ngac_node_id = b.ngac_node_id FROM _reconcile_063_tenant_users b WHERE b.tenant_id = t.tenant_id AND b.user_id =
t.user_id;` and delete rows inserted by step 5 (listed in the backup with a marker column). Constraint stays (superset).

## Carry-over from Phase 01 review
Decision (controller): migration 059 does NOT grant graph membership from `tenant_users`; it only repairs topology.
So after 059 a repaired or created `<id>_Members` can have zero users under it (dev's two `test-ws-*` workspaces do:
the graph holds no user for their one active `tenant_users` row). Migration 063 therefore MUST:
- **skip and report** (RAISE NOTICE with the workspace ids and row counts; no tombstone) every workspace whose
  `<id>_Members` has zero user descendants;
- tombstone `tenant_users` drift (active row, no graph membership) only in workspaces that have at least one graph member.
Tombstoning in a workspace with no graph members would remove every member of a workspace whose graph was simply never populated.

## Implementation status (2026-10-17)

- [x] Refresh: Lookup -> `requireActiveMember` -> Rotate / `RotateTo` (default remaining tenant, else tenant-less); `ErrUnavailable` -> 503, cookie untouched; Redis outage is 503 too. `auth/internal/domain/{refresh,membership,errors,tenants}.go`, `rest/session.go`.
- [x] Frontend: `refreshAccessToken` -> `ok | ended | unavailable`; `withRefreshRetry` throws `ApiError(503)` on unavailable; `onTenantMoved` + `lib/tenant-moved.ts` (reset, notice, navigate; wired in `main.tsx`); `websocket.store.ts` 4403 -> no timer, one refresh, guard reset on accepted auth.
- [x] WebSocket: `Hub.SetMembership`, `handleAuth` check (4403 not member, 1013 unanswered or unconfigured), `revokeWorkspaceLocal` closes that user's sockets for the workspace with 4403; wired in `messaging/cmd/main.go` over the policy writer.
- [x] Approval: `memberMiddleware` before `tenantSchemaMiddleware` on both groups; `Service.RequireMember`; `actingRow` direct branch; adapter on the writer connection.
- [x] Tombstone: `MarkTenantUserRemoved`, `EnsureTenantUser` reactivates only `removed`; removal/leave announce right after detach, tombstone error returned afterwards; domain join leaves non-active rows alone.
- [x] Readers: CanonicalApprover, OtherMembers, CountMembersByDepartment, calendar guest-name search filter `status = 'active'`; others kept as inventoried.
- [x] 063 written; dry-run on dev: 11 node backfills, 0 inserts, 15 tombstones, 2 unpopulated workspaces skipped; applied twice on dev and `ngac_ci` (second run 0/0/0).
- [x] Specs: tenant-identity, tenant-auth-flow, resource-pep-coverage, calendar.
- [ ] Prod dry-run of 063 in the migrate container, then set the thresholds (100 / 50 / 50 now).
- [ ] Manual Docker-stack check (step 5): not run here.

Deviations: see the report of this phase (CI redis service for the realpolicy job; `messaging/cmd/main.go`, `frontend/src/main.tsx`, `api/auth.ts`, `api/drive.ts` touched; 1013 for an unanswered membership check; count readers use `status = 'active'` per the inventory; 063 role for graph owners without a row; the CHECK constraint can only ship with 063).

### Review fixes (2026-10-17)
- [x] H1 migrations split: 062 = CHECK allows `removed` only; 063 = reconcile (renamed, backup table `_reconcile_063_tenant_users`); plan.md, phases 01/04/07, contacts plan (now 064, with the demo-contacts note), deployment.md and the spec updated; dev and `ngac_ci` re-applied twice.
- [x] H2 cross-tab refresh: server claim (`SET NX`) per token, spent token within 10 s returns the same successor (and `moved`), later is a replay and revokes; SPA `navigator.locks` `nexus-refresh` with a jittered fallback.
- [x] H3 typing in `tenant-moved.test.ts`; `typecheck:diff` clean.
- [x] M1 `moved: true` only for a membership move; client reacts to the flag only. M2 self-leave noted (`lib/self-leave.ts`) and not announced. M3 tenant-less -> `/workspace-select`. M4 retry with backoff while unavailable; the loop guard is a 60 s window (a flag cleared on disconnect would reopen the loop, the shell disconnects on every token change) cleared at accepted auth and sign-out.
- [x] M5 063 guards: tenant UAs under Members/Owners, more than half of a workspace's rows, notice for people only under Dept_/Role_ UAs. M6 failed tombstone retried in the background on a schedule that ends (`WithRepairBackoff`). M7 lookalike Members UA is a denial; hub NotFound/lookalike closes 4403.
- [x] L1 returning person gets the invitation's role; L2 spent token judged by the store only; L3 approval policy outage is 503; L4 one `MemberWorkspaces` lookup.
