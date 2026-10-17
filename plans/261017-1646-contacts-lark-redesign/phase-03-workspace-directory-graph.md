---
phase: 3
title: "Workspace: directory nodes, upkeep, migration 064"
status: completed
priority: P1
effort: "9h"
dependencies: [2]
---

# Phase 3: Workspace — directory nodes, upkeep, migration 064

## Overview
Build and maintain the phase-02 graph from the workspace service (release A), and create it for existing workspaces with
migration `064_directory_visibility.sql` (release B, alone). All service writes go through `policyWrite` (PAP → EPP
invalidation). No schema columns: node ids are found by their ID-keyed names (plan decision 5).

Why a migration and not a start-up backfill: the policy service admits graph writes from a bare service identity only for
`auth` (signup) and `policy-ctl` (`InvalidateCache`) (`backend/services/policy/internal/grpc/authpolicy.go:22-37`); a
workspace start-up routine has no user and would be refused. Existing graph data is changed the way the security plan did
it (059–061): guarded, re-runnable, backed-up SQL, then policy + policy-read restart and `policy invalidate-cache`.

## Requirements
- New workspace (release A): create Directory_W (under Mgmt), DirectoryOpen_W, DirectoryNoDept_W, DirectoryOwners_W,
  DirectoryLimited_W (UA under PC), associations `<W>_Members → Open {read}` and `<W>_Members → Owners {read}`, prohibition
  DirectoryLimited. All in the existing `prov` (rollback on failure).
- Department create/move/delete and member department change keep Dir/DirPeer/DeptPeers mirrored — when the workspace has
  `Directory_W`. A workspace without it (pre-064, release A window) is skipped; 064 creates everything for it.
- Workspace delete removes the directory nodes (already covered: `deleteGraph` deletes every non-U descendant of the PC,
  `workspace_delete.go:156-185`) **and the prohibition explicitly** (`RemoveProhibition`): `ngac_prohibitions` has no FK to
  `ngac_nodes` (`data/migrations/017_ngac_prohibitions.sql:7-15`), so deleting the subject node leaves the row.
- `guardDelegation` must not block a directory admin who lacks `read` from placing people in (or moving a department under)
  a hidden department (association `Dept_h → Dir_h {read}` sits in the dept chain).
- 064: every workspace and department lacking nodes gets them; `DeptPeers_<d>` children = direct U children of `Dept_<d>`;
  existing directory nodes (made by release-A code, possibly hidden) are left as they are. Re-running changes nothing.

## Architecture
### Code (release A)
New file `backend/services/workspace/internal/domain/directory_graph.go`:
- `directoryNodes(ctx, ws) (*dirNodes, bool, error)` → finds the five workspace nodes by `ngac.Directory*Name(ws)` via
  `policyWrite.FindNodeByName` (writer: read-after-write), accepts a node only with `type=directory` and
  `workspace_id == ws` (same idea as `IsAdminFolder`, `ngac_ops.go:328`); process-wide `sync.Map` cache of hits (ids never
  change; misses and errors not cached — the pattern of `MembersUA`, `policyclient/membership.go:41-76`). `false` = workspace
  has no directory yet (NotFound); any other error = error (caller fails closed).
- `deptDirNodes(ctx, ws, deptID)` → Dir/DirPeer/DeptPeers by name, same rules and cache.
- `ensureDirectory(ctx, prov, ws ids)` → creates the workspace nodes + associations + prohibition inside `CreateWorkspace`.
- `ensureDeptDirectory(ctx, prov, ws, dept, parent)` → Dir under parent's Dir (or Open), DirPeer under parent's DirPeer
  (or Directory_W), DeptPeers UA under PC, association DeptPeers→DirPeer {read}.
- `syncDeptPeers(ctx, ws, userNode, oldDeptIDs, newDept)` → U→DeptPeers edges.
- Hidden is read from the graph, not cached in a row: dept hidden ⇔ its Dir OA's parent is Directory_W.

Hook points (verified on main 1dbbbe7):
- `CreateWorkspace` (`service.go:159`; `prov` at :169, associations :236-252, `prov.Done()` :260) → `ensureDirectory` after the
  Members/Mgmt nodes exist, before `prov.Done()`. Applies to both workspace types (conversion needs nothing:
  `workspace_conversion.go:33`).
- `CreateDepartment` (`department.go:51`; dept UA assigned at :87-95, `prov.Done()` :112) → `ensureDeptDirectory` inside the
  same `prov` when the workspace has a directory.
- `MoveDepartment` (`department.go:203`; edge swap :246-268) → re-parent DirPeer always; re-parent Dir unless hidden (a
  hidden Dir stays under Directory_W).
- `DeleteDepartment` (`department.go:300`; sub-depts moved up :322-327, members moved up :329-338, rows :341-349, node delete
  :352) → re-parent child Dir/DirPeer to the parent's (or Open/Directory_W), move peers of direct members to the parent's
  DeptPeers (top-level: none), delete the three nodes.
- `applyDepartment` (`department.go:401`; graph edges :426-455, row write :456-461) → `syncDeptPeers` after the graph edges,
  before the row write; on failure undo like the existing join undo (:443-452). Single site for person↔department: called by
  `UpdateMemberDepartment` (:391) and invitation `admit` (`invitations.go:488`).
- `detachMember` (`members.go:197`) already removes the user from every UA under the PC (DeptPeers and
  DirectoryLimited included) — add a test, no code.
- `DeleteWorkspace` (`workspace_delete.go:40`) → `RemoveProhibition(DirectoryLimitedProhibitionName)` before `deleteGraph`
  (NotFound tolerated, so a retry is safe).
- `guardDelegation` (`permission_areas.go:117`): when an inherited association's OA is a directory node (`type=directory`) and
  the op is `read`, accept `manage` on that OA (directory admin) — test both branches. Callers: `UpdateMemberDepartment`
  (`department.go:385-389`), `MoveDepartment` (:240-242).

### Migration (release B) `data/migrations/064_directory_visibility.sql`
Pattern: `059_ngac_workspace_id_keyed_repair.sql` (node creation in SQL, backup table), `061_*` (guard, thresholds,
`_m061_*` state), runbook in `docs/deployment.md`.
1. Header NOTE: writes `ngac_*` and `ngac_prohibitions` → deploy recreates `policy` + `policy-read`, then
   `policy invalidate-cache --all-workspaces` (dev: `make dev-restart` of policy/policy-read, then `make policy-invalidate`).
2. Guard (`DO $$ … RAISE EXCEPTION`): every workspace has `PC_<id>`, `<id>_Members` under it and `<id>_Mgmt` under it (059/
   pattern `025_*:32-43`); counts of nodes/edges/rows to create or change within stated thresholds (set from the prod dry-run).
3. Backup: `_reconcile_064_nodes` (created node id, workspace, name), `_reconcile_064_edges` (created assignment/association
   ids), `_reconcile_064_prohibitions` (created names), `_reconcile_064_tenant_users` (tenant_id, user_id, old department_id).
   Kept once; a re-run adds nothing.
4. Per workspace, by name (fixpoint `WHERE NOT EXISTS`): the five nodes with `{"type":"directory","workspace_id":…}`, their
   assignments, the two `<id>_Members` associations, the prohibition. Names identical to the phase-02 helpers.
5. Per department, parent-first (recursive CTE on `departments.parent_id`): Dir under parent Dir (or Open), DirPeer under
   parent DirPeer (or Directory), DeptPeers UA under PC, association. A department whose nodes exist is not touched.
6. DeptPeers edges: for every `U` directly under `Dept_<d>` (`ngac_assignments` child U, parent the dept's `ngac_ua_id`).
7. Department-row drift: `tenant_users.department_id` set to the department whose UA the user sits directly under (graph is
   the truth, as in 063); report the count with `RAISE NOTICE`. Required because auth filters people by this column.
8. Reverse SQL (in the file's footer comment): delete prohibitions/edges/nodes listed in the backup tables, restore
   `department_id`.

## Related Code Files
- Create: `backend/services/workspace/internal/domain/directory_graph.go`, `directory_graph_test.go` (fake-based),
  `directory_graph_realpolicy_test.go` (`//go:build realpolicy`), `data/migrations/064_directory_visibility.sql`,
  `scripts/test-migration-064.sh` (pattern `scripts/test-migration-061.sh`)
- Modify: `backend/services/workspace/internal/domain/{service.go,department.go,permission_areas.go,workspace_delete.go}`
- Not modified: `data/init.sql`, store structs (no schema change); `docs/deployment.md` only if 064 needs a step beyond the
  existing runbook

## Implementation Steps (TDD, deny first, paired allows)
1. Failing tests. Real policy (`make test-realpolicy`, harness `backend/testutil/policy_process.go`, graph fixtures only via the
   harness write client, `TEST_DATABASE_URL` = `ngac_ci`): new workspace has the nodes + both associations + prohibition;
   create A, A1 → Dir_A1 under Dir_A under Open, DirPeer mirrors, DeptPeers_A1 under PC and not under Members
   (`IsWorkspaceMember` false for a user only under it); move A1 to root; delete A → A1 Dir/DirPeer move up, direct members
   land in parent's peers; applyDepartment A1→A2 swaps peers; remove member → no DeptPeers/DirectoryLimited edge left
   (DENY on `DirPeer_A1` after, ALLOW before); delete workspace → prohibition row gone; guardDelegation: custom manage admin
   can assign into hidden dept (ALLOW) and a caller without manage cannot (DENY). Pre-064 tolerance: a workspace seeded with
   `testutil.SeedWorkspaceGraph` (no directory) → create/move/delete department and member moves succeed and write no
   directory node.
2. Implement helpers and hook points until green; `make test s=workspace`, `make test-realpolicy`.
3. Write 064; `scripts/test-migration-064.sh` on a scratch copy of dev: run twice (second run 0 rows, no backup rows), a
   workspace made by release-A code (hidden dept) is unchanged, names equal the helpers, reverse SQL restores counts.
4. Dev: `make db-migrate` twice, `make dev-restart` (policy, policy-read) + `make policy-invalidate`, then read-only SQL:
   every workspace has `Directory_<id>`, every department its triple, `ngac_prohibitions` one row per workspace.
5. `make check-layering`, `make check-ngac`, `make build-check`.

## Notes from the phase-02 review (binding for this phase)
- **Hiding moves the peer OA too.** Hide/unhide in the department upkeep and in migration 064 (and its fixpoint, and the reverse SQL) must move
  both `Dir_<h>` (out of / back into Open) and `DirPeer_<h>` (between `Directory_W` and `DirPeer_<parent>`), with their subtrees. A `DirPeer_<h>`
  left under its parent's lets anyone placed directly in the parent, or a limited person whose department contains `h`, read the hidden people
  (vectors: `peer_h`, `peer_h1`, `peer_a1y` in `pdp_contacts_visibility_test.go`). Creating a department that is born hidden, and moving a hidden
  department, keep the same shape. Test on the real policy: DENY before the move is applied would be the bug; ALLOW for the hidden department's own people.
- **Removal/detach must remove the department edge.** A person removed from the workspace (`detachMember`) or moved out of a department must lose
  `U -> Dept_<d>` as well as `DeptPeers_<d>` and Members; a stale department edge keeps `read` on a hidden `Dir_h` (the association is on `Dept_h`) after
  they are gone. Test: ex-member with a stale `Dept_h` edge reads `Dir_h` (DENY once all three edges are gone, and the test asserts the department edge is removed).
- A limited person with no department sees only the owners and themself (spec `contacts-visibility`); the scope code must not widen that.

## Success Criteria
- Tests above green; second 064 run changes 0 rows; dev DB shows per-department triples for every department.
- Every new service graph write is a `policyWrite` call (grep: no SQL on `ngac_*` in workspace code).
- Release-A code runs unchanged before and after 064 (same build exercised on both, like 060's H3 review fix).

## Risk Assessment
| Risk | L×I | Mitigation |
|---|---|---|
| 064 runs while release-A code creates a department | L×M | fixpoint by name; code and SQL write the same names; the unique index makes the loser fail (rolled back, retried); a department missed by 064 is built lazily by the first operation that needs it (`deptDirNodes`), so nothing waits for a second run |
| SQL names drift from helpers | M×H | test script compares every created name with the Go helper output (`go run` a tiny printer or a table in the test) |
| Prohibition loaded late by policy-read | M×M | deploy recreates both policy services, then invalidate-cache (runbook) |
| Partial failure mid-department create leaves orphan nodes | M×L | same `prov.Fail` rollback as today |
| A failure after the graph edit leaves a department row stale | L×L | rows follow the graph (existing pattern); 064 step 7 reconciles; spec says graph wins |
| Name lookups add latency | L×L | process-wide cache of hits; one `FindNodeByName` per node per process life |

## Rollback
Code: revert (nodes made by release A stay, inert). 064: reverse SQL from `_reconcile_064_*`, then policy + policy-read
restart and `policy invalidate-cache --all-workspaces`. Rolling back past release A requires 064's reverse first only if
rules must disappear; otherwise the graph is harmless to old code.

## Implementation status (2026-10-17)
**Completed.** Nothing committed. Migration 064 applied twice to dev `ngac` and to `ngac_ci` (see below).

- [x] `directory_graph.go`: lookups by ID-keyed name (marked `type=directory` + this workspace, hits cached on the `Service`, misses/errors never cached,
  any lookup error but NotFound fails closed), `createDirectory` / `createDirectoryRule` (rule created last and removed by hand when the row cannot be
  stored), `createDeptDirectory`, `planDeptMirror` (every lookup before the first write), `moveDeptDirectory`, `reparentSubDepartments`,
  `deleteDeptDirectory`, `peersFor` / `stalePeers`.
- [x] Hooks: `CreateWorkspace` (both types), `CreateDepartment`, `MoveDepartment`, `DeleteDepartment`, `applyDepartment` (add new peers edge before leaving the old
  department, undo on failure, stale peers edges removed after), `DeleteWorkspace` (`RemoveProhibition` by name before the graph, NotFound tolerated),
  `guardDelegation` (`manage` stands in for `read` of an OA marked `directory`, nothing else). `detachMember` needed no code: test added.
- [x] Binding notes from the phase-02 review: hidden department keeps `Dir` **and** `DirPeer` under `Directory` on move; a department created under a hidden one nests
  under the hidden Dir/DirPeer; removal/detach removes the department, peers and Limited edges (real-policy test with a stale department edge on a hidden Dir);
  a limited person with no department is not widened (no peers edge is written for a person without a department).
- [x] `data/migrations/064_directory_visibility.sql` + `scripts/test-migration-064.sh` (fixture: release-A workspace with a hidden department and a department without
  nodes, legacy workspace without Mgmt, drifted rows, stale peers edge, nodes renamed by 023; guards, thresholds, dry run, apply, second run, reverse from the header,
  re-apply). Specs: `tenant-ngac-init`, `contacts-visibility`, `workspace-admin-authorization`; `docs/deployment.md` (064's own dry run).
- Tests that failed first (red for the right reason, then green): `TestRealPolicy_*` directory tests (node `Directory_<ws>` / `Dir_<d>` missing), `TestCreateWorkspace_Succeeds`
  (8 nodes, not 13), `TestCreateDepartment_*`, `TestDirectoryLookup_*`, `TestUpdateMemberDepartment_WritesAndUndoesThePeersEdge`, `TestDeleteDepartment_MovesTheDirectoryUp...`,
  `TestGuardDelegation_*`, `TestDeleteWorkspace_RemovesItsDirectoryProhibition...`; migration script red against a no-op stub, and again with fixture nodes renamed by 023.
  Mutations that each turned a test red: no rule cleanup on a failed insert, hidden check off, stale peers not removed, manage standing in for any operation, hidden
  sub-departments published on delete; in the migration: touching an existing Dir, not removing a stale edge, not syncing rows.

### Deviations from the phase text
- **Directory-node cache is a field of `Service`**, not a package-level map: same life as the process in production, and tests with fixed workspace ids do not share it.
- **`FakePolicyWrite`** (`backend/testutil/policy_fake.go`) gained `CreateProhibition`/`RemoveProhibition`/`Prohibitions()` and finds its own nodes by name; the grpc test
  mock gained the two prohibition calls and keeps node properties. Needed so the existing provisioning/rollback tests can run `CreateWorkspace`.
- **Deleting a hidden department with people placed directly in it is refused** (400, "Chuyển những người trong phòng này sang phòng khác trước khi xoá"; no frontend message map exists to extend), controller decision after review. Once empty of people it keeps its sub-departments hidden (their nodes go under `Directory`, their UA gets `read` on their Dir) instead of re-parenting them to
  the deleted one's parent, which would have published what an administrator hid. Specified in `tenant-ngac-init` / `contacts-visibility`.
- **064 additions**: a unique index on directory node names; a soft skip (reported, threshold) of a workspace with no Mgmt OA instead of aborting (dev has two such
  test leftovers; 059 does not guarantee Mgmt); stale `DeptPeers` edges are removed (and backed up) as well as added; and **step 1b, restoring names that migration 023
  takes back**: `make db-migrate` re-applies the whole chain on dev, and 023 renames the directory's nodes to `Folder_<id>` / `Role_<id>` (it takes them for folders and
  roles), after which 064 aborted and the rules were gone. Found by running `make db-migrate` twice on dev. A server applies each file once, so 023 never re-runs there and
  1b finds nothing; on dev each `make db-migrate` now renames and restores the same nodes (546 on dev), net no change. Long-term fix, outside this phase: make
  `db-migrate` use the ledger `migrate.sh` uses.
- Release-B file shipped as designed (alone, graph-writing); it was drafted before its test script, so the script was shown red against a no-op stub rather than against
  an absent file.

### Review fixes (2026-10-18)
- **H1** hidden department with people placed directly in it: delete refused, nothing changed (tests: deny, then allow on the same fixture).
- **M1** `deptDirNodes` builds a missing department's nodes lazily, parent first, per the parent's hidden state, people already in it get the peers edge; idempotent, a race loser looks the node up; the old test comment ("a child of a department without nodes gets none yet") and the risk row were wrong and are fixed.
- **M2** `Service.fresh`: a NotFound from a write drops the cached ids and the operation (create, move, delete, change department) is run once more; header and `docs/deployment.md` reverse steps say to restart `workspace`.
- **M3** the `Dept_sub -> Dir_sub {read}` grant is made before the nodes move and whether or not they already sit under Directory (retry test).
- **M4** `MoveDepartment` takes the department move (and any directory nodes already moved) back when the directory cannot follow; row untouched (tested at every write).
- **M5** the migration script drops `_reconcile_064_*` and the unique index in its scratch database first.
- **Low** header and runbook: the unique-index build blocks graph writes briefly (run off-peak); thresholds are placeholders set by the release manager from the prod dry-run.
