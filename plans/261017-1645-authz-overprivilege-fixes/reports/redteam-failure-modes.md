# Red-team: failure-mode analysis — authz over-privilege plan (261017-1645)

Lens: failure modes, migration/deploy ordering, staleness, rollback holes. Every claim below was checked against code and the dev DB (5433) on 2026-10-17.

## Finding 1: The refresh membership check runs after Rotate, so the 503 "outage" path still logs everyone out
- **Severity:** Critical
- **Location:** Phase 04, Requirements item 4 ("after `Rotate` ... Policy/DB error → 503 without revoking")
- **Flaw:** `Rotate` marks the presented refresh token spent and mints its successor before the plan's check runs. On the 503 path the successor is thrown away and the cookie keeps the spent token. The client keeps its session on 5xx and tries again later. The retry presents the spent token, replay detection fires, and the whole family is revoked.
- **Failure scenario:** policy restarts during a deploy. That restart is guaranteed whenever a migration ran (`deploy.sh:105-106`), and it takes 10-20 s. Every browser whose access token expires in that window refreshes, gets 503, retries, hits "refresh token replayed", gets 401, and is logged out. The mitigation row "Policy outage logs everybody out → 503 path without revoke; test" is false, and a unit test that only asserts "session not revoked" after the 503 will pass while the next refresh kills the session.
- **Evidence:** `backend/services/auth/internal/domain/refresh.go:161-166` (a spent token burns the family), `refresh.go:170-176` (spent is marked before minting), `refresh.go:202` (`RefreshSession` calls `Rotate` first); `frontend/src/api/client.ts:43-47` (a 5xx keeps the session and retries); `backend/services/auth/internal/rest/session.go:111-118`.
- **Suggested fix:** run `requireActiveMember` before `Rotate`. Use `Lookup` (`refresh.go` `Lookup`, which does not spend the token) to get the identity, check membership, then rotate. Add a test: policy down → 503, then policy back up → the same cookie still refreshes successfully.

## Finding 2: No phase repairs the legacy workspaces, yet 058 and 060 run against them
- **Severity:** Critical
- **Location:** plan.md "Quyết định" item 4 (Q6: "sửa trong migration, không chấp nhận deny cả workspace"); Phase 03 Risk row "2 legacy workspaces ... repair names or accept"; Phase 03 `058` and Phase 07 `060`
- **Flaw:** Zane decided the legacy workspaces are repaired in a migration. No phase owns that repair, and no migration file or step contains it. Both reconcilers decide membership by the name `<ws>_Members`, which does not exist for these workspaces. 057 step e matches workspaces by name prefix, so it also misses them.
- **Failure scenario:** on prod (and dev), for each legacy workspace:
  - 058 deletes every user edge into its channels;
  - 060 sets every active row to `removed`;
  - Phase 04 refresh denies every member of it;
  - 057 leaves `Members → Channels {read,write}` in place, so F1 stays open there.

  Result: a whole tenant is locked out and still leaky. The pg_dump is the only way back, and restoring it discards every write made since.
- **Evidence:** dev DB: `SELECT ... FROM workspaces w WHERE NOT EXISTS (ngac_nodes.name = w.id||'_Members')` returns `test-ws-561b804a5443` and `test-ws-c56ed191dc29`, each with 1 active `tenant_users` row and 1 channel. `backend/ngac/ngac_ops.go:147` (`MembersUAName` is keyed by ID). `grep -i legacy` over the plan finds the repair only as an open question (`phase-03:85`, `plan.md:118`).
- **Suggested fix:** add a repair migration (or a step 0 in 057) that creates or renames ID-keyed `PC_<id>` / `<id>_Members` / `<id>_Channels` for every workspace missing them, assigned the same way as 023. Make 058 and 060 abort (`RAISE EXCEPTION`) if any workspace still lacks `<id>_Members`, instead of silently deleting.

## Finding 3: Old images keep running between `migrate` and `up`, and after an automatic rollback, so 057 creates permanent drift and broken group creation
- **Severity:** High
- **Location:** Phase 02, Implementation step 4 (057) and Risk row "Previous release + new data (rollback of images) — L×M"
- **Flaw:** `deploy.sh` migrates while the old release is still serving, and `up --wait` replaces it only afterwards (minutes: health chains with 20 s start_period and 10 retries). An automatic rollback brings old images back with no migrations. Old code does two things against the post-057 schema:
  - REST defaults every user-created group to `channel_type='workspace'`, and gRPC does too (`server.go:38`). The new partial unique index rejects these in any workspace that already has its `#general`. Group creation fails with an unmapped unique violation, which becomes a 500.
  - Old workspace and auth code builds new workspaces with `Members → Channels {read,write,create_channel}` and `Owners → Channels AllOwnerOps`. Their `#general` is created as `workspace` type without the `<W>_Members → Ch UA` edge. 057 has already run, so these workspaces stay leaky forever, and their `#general` is unreadable for later members.
- **Failure scenario:** a deploy goes out at a busy hour. Anyone creating a group during the window gets 500, and anyone signing up gets a personal workspace with F1 still open. If the health check then fails, `ROLLBACK=1` puts the whole fleet on the old code against the new index: every group creation in every workspace with a `#general` fails until someone runs the reverse SQL by hand.
- **Evidence:** `deploy/deploy.sh:90-106` (migrate → up → restart policy); `docs/deployment.md:16-18,227-229`; `backend/services/messaging/internal/rest/handler.go:163-165` (REST default `workspace`); `backend/services/messaging/internal/grpc/server.go:38`; `backend/services/messaging/internal/domain/channels.go:73-76`; `backend/services/workspace/internal/domain/service.go:208,210`.
- **Suggested fix:** split it into expand/contract.
  - Release A ships the code: it writes `private`, tolerates both types, and creates new workspaces with the new grants.
  - Release B ships 057, with the unique index built `CONCURRENTLY IF NOT EXISTS`.
  - Alternatively, make 057's association rewrite re-runnable and run it again after `up`.
  - Re-rate the rollback risk H×H and document that a rollback past B needs the reverse SQL first.

## Finding 4: `#general` through a UA→UA edge is invisible to every path that lists people
- **Severity:** High
- **Location:** Phase 02, Requirements "Workspace-wide channel (`#general`) ... Members UA is assigned under `Ch_<id>_Members`"
- **Flaw:** access works through the edge (the PDP walks UA→UA), but every roster path reads either direct U children or the `channel_members` cache. Workspace members who join after the deploy get neither.
- **Failure scenario:** a new member joins via invitation and can read and post in `#general`, but:
  - they are missing from its member list, because `ListMembers` only keeps direct `TypeU` children;
  - they get no push, mention or incoming-call notifications for it, because `ChannelPeople` joins `channel_members`;
  - `member_count` stays at the creator plus the people already in it;
  - Phase 08's `RemoveMember` requires a direct U edge, so they cannot leave it, and there is no defined product behaviour for that.

  Meanwhile 058 deletes "cache rows with no graph assignment", so the cache shrinks further. All of this passes the Phase 02 tests, which only check read and write decisions.
- **Evidence:** `backend/services/messaging/internal/domain/members.go:113-125` (`GetChildren`, skips non-U nodes); `backend/services/messaging/internal/store/push.go:85-90` and callers `domain/push.go:147,169`; `backend/services/messaging/internal/store/store.go:79` (member_count from the cache).
- **Suggested fix:** pick one model and test the roster, not just the decision.
  - Either: expand `<W>_Members` descendants in `ListMembers` / `ChannelPeople` / count when `channel_type='workspace'`;
  - or: keep direct U edges for `#general`, written by admit and detach and backfilled by 057.

  Add integration assertions that a member admitted after migration appears in `ListMembers` and receives push.

## Finding 5: Closing the removed member's sockets is cosmetic, because they reconnect at once with the same JWT
- **Severity:** High
- **Location:** Phase 04, Requirements item 6, and the claim "Exposure after removal ... 0 for ... live sockets"
- **Flaw:** WS auth only verifies the JWT signature and expiry. It does no membership or tenant check. The frontend reconnects with backoff and resets the attempt counter on each successful connect. The removal event closes the socket exactly once.
- **Failure scenario:** a removed user's tab reconnects within about a second using the still-valid 15-minute access token. `userCameOnline` broadcasts their presence to tenant W again, they receive tenant-wide presence again, and they can re-subscribe to whatever their JWT still passes. Exposure is up to 15 min, not 0, and the "socket closes" check in manual step 5 shows a green result for a few hundred ms.
- **Evidence:** `backend/services/messaging/internal/grpc/hub_conn.go:98-136` (`handleAuth`: JWT only, then `userCameOnline`); `backend/services/messaging/internal/grpc/hub_domain.go:237-252`; `frontend/src/stores/websocket.store.ts:266,278`; `backend/services/messaging/internal/grpc/hub_broadcast.go:380-390`.
- **Suggested fix:** in `handleAuth`, when `claims.TenantID != ""`, check `IsWorkspaceMember` and reject with a terminal code the client does not retry. Alternatively, keep a short-lived Redis deny key `(tenant, node)` written by `member_removed` and checked on auth. Test: removal, then reconnect with the old token, must be rejected.

## Finding 6: Tombstoned rows break readers that do not filter on status
- **Severity:** High
- **Location:** Phase 04, Risk row "Previous release sees `removed` rows — all its readers filter `active`"; Requirements item 2
- **Flaw:** the claim is false for both the current and the new code:
  - `OtherMembers` counts every row regardless of status. Workspace delete refuses when it is above 0.
  - `CanonicalApprover` accepts any row as a `specific_user` approver.
- **Failure scenario:**
  - An owner removes everyone and tries to delete the workspace. They get "other people belong to this workspace" forever. 060 tombstoning 17+ rows makes this happen at scale.
  - An admin builds an approval template naming a removed person. Validation passes, requests route to someone who now gets 403 at the new gate, and requests stall.
- **Evidence:** `backend/services/workspace/internal/store/teardown.go:30-37` and the caller `workspace/internal/domain/workspace_delete.go:76-83`; `backend/services/approval/internal/store/names.go:113-117`; Phase 04's file list does not include either file.
- **Suggested fix:** add `AND status='active'` to `OtherMembers` and to `CanonicalApprover` (and its `role_in_dept`/`department` branches). Also grep every `FROM tenant_users` in all services as part of Phase 04, and record the audit in the phase file.

## Finding 7: The reactivating `EnsureTenantUser` undoes administrator suspension
- **Severity:** High
- **Location:** Phase 04, Requirements item 2 (`ON CONFLICT ... DO UPDATE SET status='active'`); Phase 07, "`admit`: always `EnsureTenantUser`"
- **Flaw:** today's store deliberately leaves an existing row unchanged, so that a person an administrator disabled does not come back by being invited. The plan changes this to set `active` unconditionally, and Phase 07 runs it on every invitation accept.
- **Failure scenario:** a suspended (`disabled`) user still holds a pending or reusable invitation link, or gets re-invited by an invite-only role. Accepting it flips their row to `active`, and Phase 07's `requireActiveMember` then passes. Suspension is bypassed by a weaker role.
- **Evidence:** `backend/services/workspace/internal/store/directory_store.go:71-83` (comment and `DO UPDATE SET ngac_node_id = COALESCE(...)` only).
- **Suggested fix:** `DO UPDATE SET status = CASE WHEN tenant_users.status = 'removed' THEN 'active' ELSE tenant_users.status END`. Add a test: disabled row plus invitation accept → stays `disabled`.

## Finding 8: Dev `make db-migrate` re-applies every file, but the plan never requires the migrations to be idempotent
- **Severity:** Medium
- **Location:** Phase 02 step 4f (`CREATE UNIQUE INDEX channels_one_workspace_wide ...`); Phase 03 step 4, Phase 04 step 2, Phase 07 step 3 (each runs `make db-migrate` again)
- **Flaw:** in dev, `db-migrate` loops over all of `data/migrations/*.sql` with `ON_ERROR_STOP=1` and no ledger. As written, 057f has no `IF NOT EXISTS`, so the second `make db-migrate` (Phase 03's verification) aborts at 057 and 058/059/060 are never applied in dev. Phases 03/04/07 then report "verified" against a DB that never got their migration, or the implementer starts patching migrations ad hoc. 059 ("replace the check") also has to be `DROP CONSTRAINT IF EXISTS` + `ADD`. In prod, a file run without a transaction, or one interrupted before its ledger row, also re-runs (`docs/deployment.md:293-297`).
- **Evidence:** `Makefile:561-569`; `docs/deployment.md:287-297`.
- **Suggested fix:** add a cross-cutting rule that every file 057-060 is re-runnable:
  - `CREATE UNIQUE INDEX IF NOT EXISTS`;
  - `DROP CONSTRAINT IF EXISTS`;
  - reconcilers whose predicates are fixpoints.

  Verify by running `make db-migrate` twice in each phase.

## Finding 9: The "dry-run on prod, Zane confirms" gate cannot run in this pipeline, and 058/060 have no real reverse
- **Severity:** Medium
- **Location:** plan.md "Quyết định" item 4; Phase 03 Rollback; Phase 07 Rollback ("reversible from the dry-run output")
- **Flaw:** prod migrations run automatically inside `deploy.sh` after one environment approval. The prod DB publishes no port, and the plan has no step or tool for running the commented dry-run SELECT on prod, persisting its output, or holding the migration until the counts are confirmed. Phase 07 rollback depends on dry-run output that is never stored. Phase 03 rollback says to restore from the pg_dump, which is a whole-database restore that throws away every write made after the deploy.
- **Failure scenario:** the approver clicks approve on the deploy, and 058/060 delete and tombstone rows nobody counted (the counts can also differ between whenever a manual dry-run happened and deploy time). When a real member turns out to be cut off, the only exact restore is a full DB rollback.
- **Evidence:** `deploy/deploy.sh:90-100`; `docs/deployment.md:16,227-229,287-298`; `docs/deployment.md:44-62` (no published ports).
- **Suggested fix:**
  - Inside 058/060, first `CREATE TABLE IF NOT EXISTS _reconcile_058_deleted AS SELECT ...` (and `_reconcile_060_tombstoned`) with the exact rows changed, so the reverse is `INSERT ... SELECT` / `UPDATE ... FROM`.
  - Add a guard: `DO $$ ... IF count > :threshold THEN RAISE EXCEPTION`, which fails the deploy before `up`.
  - Document running the dry-run via `docker compose run --rm migrate psql ...` on the server.

## Finding 10: Phase 07's graph-first `joinTenant` undo can delete an edge it did not create; the strict provisioning path is missing
- **Severity:** Medium
- **Location:** Phase 07, Requirements "Writers, graph first" (`joinTenant` ... "on insert failure remove the assignment(s) just made")
- **Flaw:** `CreateAssignment` is idempotent and returns the existing row's id. The caller therefore cannot tell a new edge from an old one, which is exactly why messaging calls `IsAssigned` first. The plan requires that check for `admit` but not for `joinTenant`. Separately, `provisionTenant(strict=true)` still inserts the row before assigning, and it is not in Phase 07's list of writers.
- **Failure scenario:** someone who is already a graph member but has no row (the drift class 060 step 3 backfills) signs in. `assignTenantNGAC` is a no-op, the insert hits a transient DB error, and the undo removes their real TenantMember edge. The person is now detached from the workspace and 060's own invariant is broken. Workspaces created on purpose keep the row-before-edge ordering the phase claims to remove ("No code path writes tenant_users active before the graph edge").
- **Evidence:** `backend/services/policy/internal/ngac/pap_store.go:117` (`ON CONFLICT (child_id, parent_id) DO UPDATE ... RETURNING id`); `backend/services/messaging/internal/domain/members.go:38-46` (the `IsAssigned` pattern); `backend/services/auth/internal/domain/tenants.go:92-97` (strict path inserts the row, then assigns); `tenants.go:133-139`.
- **Suggested fix:** in `joinTenant`, call `IsAssigned` per target UA before assigning and undo only the edges that were newly created. Add `provisionTenant` strict to Phase 07's file list and swap its order. Add a test: pre-existing edge plus insert failure → edge survives.

Status: DONE_WITH_CONCERNS
Summary: Two Critical holes. The refresh check placed after Rotate turns any policy blip into a mass logout. The legacy-workspace repair has no owning phase, so 058 and 060 would wipe those tenants' memberships. Deploy-window drift, the invisible #general roster, cosmetic socket closes, and tombstone readers are High.
