---
phase: 1
title: "Foundations: legacy-workspace repair (059), real-policy harness, membership helper, cache invalidator, runbook"
status: completed
priority: P1
effort: "6h"
dependencies: []
---
# Phase 1: Foundations

## Overview
Shared pieces every later phase needs:
1. **059 repair migration** — runs before anything else. Dev has 2 workspaces (`test-ws-561b804a5443`,
   `test-ws-c56ed191dc29`) with no ID-keyed `<id>_Members` / `PC_<id>` (their PCs are named `pc-test-ws-*` and have
   no child nodes at all; each has 1 active tenant_users row and 1 channel). Every later membership check and reconcile
   keys on those names; without the repair, 061/063 would wipe those tenants and 060 would miss them.
2. A test harness running the **real policy service** (build tag `realpolicy`).
3. One membership predicate: "U is under `<W>_Members`", with a mandatory Members-UA-id cache.
4. A cache-invalidation tool calling the existing `InvalidateCache` RPC, used after graph-writing migrations.
5. Runbook: two-release rule, re-runnable migrations, dry-run in the prod migrate container.

## Requirements
- **059 `data/migrations/059_ngac_workspace_id_keyed_repair.sql`** (pattern `023_ngac_id_keyed_names.sql`):
  for each `workspaces` row, matched through `workspaces.ngac_pc_id` (FK, not name):
  - rename the PC node to `PC_<id>` if not already (record in `ngac_node_renames`);
  - find the PC's child UA/OA playing Members/Owners/Channels (by the legacy name the workspace was created with);
    rename to `<id>_Members`, `<id>_Owners`, `<id>_Channels`; create any that is missing, with today's assignments
    (Members→PC, Owners→PC, Owners→Members, Channels→PC) and associations as of release time;
  - relink `TenantMember_<id>`/`TenantOwner_<id>` under Members/Owners where they exist.
  - Re-runnable (fixpoint predicates); ends with a `DO $$ ... RAISE EXCEPTION` if any workspace still lacks
    `<id>_Members`. Backup affected node rows to `_reconcile_059_nodes`.
  - Header NOTE: writes `ngac_*` → policy + policy-read recreated by deploy, then `policy invalidate-cache`.
  - Old code tolerates it (names only become ID-keyed, as with 023) → ships alone in release R0.
- **Harness** `backend/testutil/policy_process.go` (file has `//go:build realpolicy`):
  `StartPolicy(t) (policypb.PolicyReadServiceClient, policypb.PolicyWriteServiceClient)`.
  - Binary path from env `NEXUS_POLICY_BIN`; `make test-realpolicy` builds `backend/services/policy/cmd` once into
    `.dev-logs/bin/policy` and runs every service's tagged tests with it (build stays outside the 60 s per-package
    timeout, `Makefile:426`). Missing env under the tag → `t.Fatal` (tag means "I want real policy").
  - Env passed explicitly: `DATABASE_URL=$TEST_DATABASE_URL`, `GRPC_PORT` and `METRICS_PORT` free ports,
    `REDIS_URL=redis://127.0.0.1:1/0`, `KAFKA_BROKERS=127.0.0.1:1`, `INTERNAL_IDENTITY_SECRET=<test>` — never the
    dev Redis/Redpanda (`policy/cmd/main.go:39-40` defaults to localhost:6379 / :19092).
  - One process per test binary (`TestMain`); waits on gRPC health; killed on exit. Clients via `grpcauth.Dial`.
  - Fixture rule: graph nodes/edges only through the returned write client (`testutil.CreateWorkspace` writes SQL
    the running process never loads); SQL only for business rows. Helper `SeedWorkspaceGraph(t, write)` builds the
    workspace topology with `ngac.*Ops()` helpers. Every deny assertion paired with an allow on the same fixture.
  - Why a process: the PDP is under `services/policy/internal/...` (not importable from other modules), and a
    cross-module require would break every service Dockerfile (each copies only its own `go.mod`).
  - Build tag, not `t.Skip`: `scripts/go-test-strict.sh` counts skips as failures; untagged runs don't compile these.
  - CI: `.github/workflows/ci.yml` gains a job running `make test-realpolicy` against its Postgres service.
- **Membership helper** `backend/pkg/policyclient/membership.go`:
  - `type GraphReader interface { FindNodeByName(...); GetAncestors(...) }`.
  - `MembersUA(ctx, r, ws ngac.WorkspaceID) (string, error)` — mandatory process-wide cache (`sync.Map`), ids are
    immutable for a workspace's life; errors not cached.
  - `IsWorkspaceMember(ctx, r, userNodeID, ws) (bool, error)` — Members UA id ∈ ancestors(user). Error → caller
    denies. Owners (`<ws>_Owners` under Members) and TenantMember (under Members) count; channel, event-guest,
    role-only and dept-only UAs do not.
  - `MemberWorkspaces(ctx, r, userNodeID) ([]ngac.WorkspaceID, error)` — one `GetAncestors`; for each ancestor UA whose
    name parses with `ngac.WorkspaceIDFromMembersUAName`, confirm id via `MembersUA` (anti-spoof; `_Members` is not in
    the unique-name index, `025_*.sql:45-47`). Cost bounded by the user's workspaces.
  - `SharedWorkspaces(ctx, r, a, b)` = intersection of the two `MemberWorkspaces`.
- `ngac.WorkspaceIDFromMembersUAName(name) (WorkspaceID, bool)` next to `MembersUAName` (`backend/ngac/ngac_ops.go:147`);
  round-trip test; rejects `Ch_*_Members`, `TenantMember_*`.
- **Invalidator**: subcommand of the existing policy binary, `policy invalidate-cache --all-workspaces`
  (`backend/services/policy/cmd/main.go`): reads workspace PC ids from the DB, dials the running writer with service
  identity `policy-ctl` (ServiceOK on `InvalidateCache` in `policy/internal/grpc/authpolicy.go:22`), calls
  `InvalidateCache` (`write_server.go:324`) in batches. `deploy/deploy.sh` runs it after the policy/policy-read
  recreate when the migrations marker exists (`deploy.sh:101-107`); `make policy-invalidate` for dev.
- **Runbook** in `docs/deployment.md`: two-release rule; migrations re-runnable (`make db-migrate` twice); backup
  tables `_reconcile_<NNN>_*` and their reverse; dry-run command
  `docker compose --profile tools run --rm migrate psql -f /migrations/<file>.dryrun.sql` (or inline SELECT) on the
  server before approving the deploy; post-migration invalidate step.

## Architecture
```
R0: 059 repair ──> deploy recreates policy + policy-read ──> policy invalidate-cache
tests: make test-realpolicy ─build once─> NEXUS_POLICY_BIN ─> per-package TestMain starts process (no Redis/Kafka)
domain: policyclient.IsWorkspaceMember ─> MembersUA (cached) + GetAncestors
```

## Related Code Files
- Create: `data/migrations/059_ngac_workspace_id_keyed_repair.sql`, `backend/testutil/policy_process.go`,
  `backend/testutil/policy_process_test.go`, `backend/pkg/policyclient/membership.go`, `membership_test.go`
- Modify: `backend/ngac/ngac_ops.go`, `backend/services/policy/cmd/main.go`,
  `backend/services/policy/internal/grpc/authpolicy.go`, `Makefile`, `.github/workflows/ci.yml`, `deploy/deploy.sh`,
  `docs/deployment.md`
- Delete: none
- Spec: `docs/specs/tenant-ngac-init/spec.md` (every workspace has ID-keyed PC/Members/Owners/Channels; membership =
  under `<W>_Members`)

## Implementation Steps
1. Failing tests first:
   - `membership_test.go` (fake reader): DENY via channel UA only, via other workspace's Members, via event-guest UA,
     on reader error, when Members UA missing; ALLOW via Owners, via TenantMember (same fixtures). Spoofed `_Members`
     name with a different id rejected by `MemberWorkspaces`.
   - `policy_process_test.go` (tag): DENY then ALLOW after an association on one fixture; asserts the process did not
     touch Redis/Kafka (env check + no connection log).
   - 059 test on the dev DB copy: before, the 2 legacy workspaces lack `<id>_Members`; after, the guard passes; second
     run changes 0 rows.
2. Implement helper, parser, harness, invalidator, 059, runbook.
3. `make db-migrate` twice; `make dev-restart` (policy, policy-read) and `make policy-invalidate`.
4. `cd backend && go test ./pkg/... ./ngac/...`; `make test-realpolicy`; `make test s=policy`; `make check-ngac`,
   `make check-layering`, `make build-check`.

## Success Criteria
- Dev: `SELECT count(*) FROM workspaces w WHERE NOT EXISTS (SELECT 1 FROM ngac_nodes n WHERE n.name = w.id||'_Members'
  AND n.node_type='UA')` = 0, twice in a row.
- Harness tests green under the tag; untagged `make test` unchanged.
- `policy invalidate-cache --all-workspaces` exits 0 against the dev stack.

## Risk Assessment
| Risk | L×I | Mitigation |
|------|-----|------------|
| 059 picks the wrong legacy node as Members/Owners | L×H | match via `workspaces.ngac_pc_id` + direct-child + legacy name; backup table; prod dry-run lists each mapping |
| Harness pollutes dev Redis/Kafka | M×M | unreachable addresses forced; test asserts |
| CI time | M×L | binary built once; one process per package |
| Cache of Members UA id stale after a repair rename | L×M | ids do not change on rename (059 renames in place); cache keyed by ws id → id |

## Rollback
Code: revert. 059: `UPDATE ngac_nodes n SET name = r.old_name FROM ngac_node_renames r WHERE ...` for its renames;
delete nodes listed in `_reconcile_059_nodes` as created; then recreate policy services + invalidate.

## Implementation Status (2026-10-17)
- [x] 059 repair migration (`data/migrations/059_ngac_workspace_id_keyed_repair.sql`) + `scripts/test-migration-059.sh` (scratch DB, twice, ambiguous refused)
- [x] `ngac.WorkspaceIDFromMembersUAName` + round-trip/reject test
- [x] `policyclient` membership helper (`MembersUA` cached, `IsWorkspaceMember`, `MemberWorkspaces`, `SharedWorkspaces`) + deny/allow tests
- [x] Real-policy harness (`backend/testutil/policy_process*.go`, `policy_graph.go`, tag `realpolicy`), `make test-realpolicy`, CI `realpolicy` job
- [x] `policy invalidate-cache --all-workspaces` (service identity `policy-ctl`, ServiceOK on `InvalidateCache`), `make policy-invalidate`, `deploy.sh` step
- [x] Runbook in `docs/deployment.md`; spec `tenant-ngac-init` updated
- [x] Dev DB: `make db-migrate` run 3x, 2 legacy workspaces repaired, later runs add no row to `_reconcile_059_*`

Deviations: no `ngac_graph_version` / `ngac_materialized_access` exist in the current schema (023 referenced them), so 059 does not bump/truncate them; invalidation is the tool. `StartPolicy` starts the process lazily once per test binary; `RunMain` (TestMain) stops it. RPCs other than CreateNode/CreateAssignment/DeleteNode/FindNodeByName need `PolicyCaller`/`PolicyAdmin` on the ctx (policy refuses a bare service identity).
