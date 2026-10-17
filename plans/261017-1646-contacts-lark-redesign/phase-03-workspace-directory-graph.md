---
phase: 3
title: "Workspace: directory nodes, upkeep, backfill"
status: pending
priority: P1
effort: "8h"
dependencies: [2]
---

# Phase 3: Workspace — directory nodes, upkeep, backfill

## Overview
Build and maintain the phase-02 graph from the workspace service. All writes go through `policyWrite`
(PAP → EPP invalidation). Existing workspaces are backfilled by an idempotent Go routine at workspace start-up (uses
`backend/ngac` helpers and EPP, unlike a SQL migration). Schema change only stores node FKs.

## Requirements
- New workspace: create Directory_W (under Mgmt), DirectoryOpen_W, DirectoryNoDept_W, DirectoryLimited_W (UA under PC),
  association Members→Open {read}, prohibition DirectoryLimited.
- Department create/move/delete and member department change keep Dir/DirPeer/DeptPeers mirrored.
- Backfill: every workspace and department lacking nodes gets them; `DeptPeers_<d>` children = direct U children of
  `Dept_<d>`. Re-running changes nothing.
- `guardDelegation` must not block a directory admin who lacks `read` from placing people in a hidden department
  (association `Dept_h → Dir_h {read}` sits in the dept chain).
- Workspace delete removes the directory nodes and the prohibition.

## Architecture
- Migration `data/migrations/064_directory_visibility.sql` (nullable FKs to `ngac_nodes(id)`, `ON DELETE SET NULL`):
  `workspaces.directory_oa_id`, `workspaces.directory_open_oa_id`, `workspaces.directory_nodept_oa_id`,
  `workspaces.directory_limited_ua_id`; `departments.dir_oa_id`, `departments.peer_oa_id`, `departments.peers_ua_id`,
  `departments.directory_hidden BOOLEAN NOT NULL DEFAULT false` (cache of "Dir OA parent is Directory_W"; graph is truth,
  written after the graph edit like `applyDepartment` does for `tenant_users.department_id`, `department.go:460-465`).
  Also mirror in `data/init.sql`.
- New file `backend/services/workspace/internal/domain/directory_graph.go`:
  - `ensureDirectory(ctx, ws)` → creates/reuses (`prov.EnsureNode`) the four workspace nodes + association + prohibition;
    stores IDs.
  - `ensureDeptDirectory(ctx, ws, dept, parent)` → Dir/DirPeer OAs under the parent's (or Open / Directory root), DeptPeers UA
    under PC, association DeptPeers→DirPeer {read}.
  - `syncDeptPeers(ctx, userNode, oldDept, newDept)` → U→DeptPeers edges.
  - `BackfillDirectories(ctx)` → for each workspace: ensureDirectory, departments parent-first, then peers from
    `GetChildren(Dept UA)` users. Called from `cmd/main.go` before servers listen; logs per-workspace failures and exits
    non-zero only if `ensureDirectory` fails for every workspace (so one corrupt workspace does not block start-up;
    its contacts will 503 — fail closed).
- Hook points (existing code, verified):
  - `CreateWorkspace` provisioning block `service.go:183-215` → call `ensureDirectory` before `prov.Done()`.
  - `CreateDepartment` `department.go:51` → `ensureDeptDirectory` inside the same `prov` (rollback on failure).
  - `MoveDepartment` `department.go:203` (edge swap ~`:255-266`) → re-parent DirPeer always; re-parent Dir unless hidden.
  - `DeleteDepartment` `department.go:300` → re-parent child Dir/DirPeer to the parent's, move peers of direct members to
    parent's DeptPeers (mirrors `:330-337`), delete the three nodes.
  - `applyDepartment` `department.go:401` → `syncDeptPeers` after the graph edge, before the row update; same undo pattern.
  - `workspace_delete.go` → delete directory nodes/prohibition.
  - `workspace_conversion.go` (personal → org): nothing extra (directory exists for all types).
  - `detachMember` `members.go:117` already strips every UA under PC (incl. DeptPeers) — add a test, no code.
  - `guardDelegation` `permission_areas.go:117`: when the association's OA is a directory node (prop
    `type=directory`) and op is `read`, accept `manage` on that OA (directory admin) — test both branches.

## Related Code Files
- Create: `data/migrations/064_directory_visibility.sql`, `backend/services/workspace/internal/domain/directory_graph.go`,
  `.../domain/directory_graph_test.go`
- Modify: `data/init.sql`, `backend/services/workspace/internal/domain/{service.go,department.go,permission_areas.go,workspace_delete.go}`,
  `.../internal/store/` (workspace + department structs, insert/select of new columns), `backend/services/workspace/cmd/main.go`

## Implementation Steps (TDD)
1. Failing tests (`directory_graph_test.go`, DB-backed, `TEST_DATABASE_URL` on :5433, policy via existing test fakes/real
   policy as other workspace tests do): new workspace has the nodes + association + prohibition; create A, A1 → Dir_A1
   under Dir_A under Open, DirPeer mirrors, DeptPeers_A1 under PC; move A1 to root; delete A → A1 Dir/DirPeer move up,
   direct members land in parent's peers; applyDepartment A1→A2 swaps peers; backfill twice = same graph (count nodes/edges);
   remove member → no DeptPeers edge left; guardDelegation: custom manage admin can assign into hidden dept (ALLOW) and a
   caller without manage cannot (DENY).
2. Migration + store; `make db-migrate`; `psql -p 5433 … \d departments` to observe.
3. Implement helpers and hook points until green.
4. `make test s=workspace`, `make check-layering`, `make check-ngac`, `make build-check`.
5. Run workspace against dev DB, confirm backfill log and node counts with read-only SQL.

## Success Criteria
- Tests above green; second backfill run creates 0 nodes; dev DB shows per-department triples for every department.
- Every new graph write is a `policyWrite` call (grep: no SQL on `ngac_*` in workspace).

## Risk Assessment
- Backfill on prod with many departments slows start-up (Low×Med): O(depts) policy calls once; subsequent runs are reads.
- Concurrent replicas racing `EnsureNode` (Low×Med): prod runs one workspace replica; note in spec; `EnsureNode` by name is
  idempotent enough for sequential restarts.
- Partial failure mid-department create leaves orphan nodes (Med×Low): same `prov.Fail` rollback as today.
- Dept moves/deletes now do more writes; a failure between graph and row leaves `directory_hidden` stale (Low×Low): read
  paths derive hidden from the Dir OA parent where it matters (admin list), row is display cache.
- Rollback: revert code; columns and nodes are inert.
