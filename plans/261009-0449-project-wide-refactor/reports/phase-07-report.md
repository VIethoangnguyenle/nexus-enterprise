# Phase 07 — NGAC model conformance: report

Date 2026-10-10. Branch `refactor/project-wide`. Nothing staged, committed or added to the index.

Status: DONE_WITH_CONCERNS (see "Concerns").

## What changed

### 1. Inline operations and node names
- `scripts/check-ngac-identifiers.sh` (+ `--self-test`): scans non-test, non-generated Go outside `backend/ngac` for a quoted
  operation literal, a `Sprintf`/concatenated node-name pattern, or a well-known node-name literal. A deliberate exception ends its
  line with `// ngac-lint:allow <reason>` (one use: the asset lifecycle action name `"approve"`, a stored action label, not an op).
- Wired into the existing CI `docs` job (self-test + scan) and `make check-ngac` (also in `make verify`). Passes.
- `services/approval/**` is skipped in the script, with a comment saying why. Not touched.

### 2. Helper signatures (`backend/ngac/ngac_ops.go`, new `backend/ngac/ids.go`)
Every name helper now takes a distinct ID type, so passing a display name is a compile error and the remaining `ngac.DeptID(x)`
conversion is visible in review.

| Helper | Before | After |
|---|---|---|
| `PCName OwnersUAName MembersUAName MgmtOAName DocumentsOAName DraftDocsOAName ApprovedDocsOAName ChannelsOAName AssetsOAName DriveRootName TenantMemberUAName TenantOwnerUAName` | `string` | `WorkspaceID` |
| `DeptUAName` | `(name string)` | `(DeptID)` |
| `FolderNodeName` | `(name string)` | `(FolderID)` |
| `ShareOAName` | `(itemName, suffix string)` | `(ShareID)` |
| `ChannelContentOAName ChannelMembersUAName ChannelDriveName` | `string` | `ChannelID` |
| `PersonalUAName` | `string` | `UserNodeID` |
| `AssetTypeOAName` | `(ws, sanitized type name)` | `(WorkspaceID, AssetTypeID)` |
| `AssetCategoryOAName` | `(ws string, category)` | `(WorkspaceID, category string)` (a category is a workspace-scoped label, has no ID) |
| new `RoleUAName(RoleID)`, `UserNodeName(UserID)`, `PropDisplayName`, `PropTypeRole`, `DisplayName(name, props)` | | |
| removed `AssetNodeName`, `NodePCAssetManagement` | | |

Call sites fixed: role UA (`Role_<uuid>`, display name in `display_name`), workspace folder (`Folder_<uuid>`), drive folder
(`Folder_<drive item id>`, ID chosen before the node), share OA (`Share_<share id>`, ID chosen before the node), channel/workspace drive
(`Ch_<channel id>_Drive`, drive item shows the channel name), department (`Dept_<department id>`), user U node (`U_<users.id>`,
username in `display_name`; auth now picks the user ID before creating the node). Screens read names through `ngac.DisplayName`
(ListRoles, ListFolders, ListMembers, messaging channel members, share target label), which falls back to the node name for old nodes.
Messaging's `ChannelsOAName(ws.Name)` item was already ID-keyed by an earlier fix; only the type conversion was needed.

### 3. Drive root
`workspaces.documents_oa_id` (migration 022, written by `CreateWorkspace`) replaces the "Documents"/"Docs" substring scan with its
first-OA fallback. No recorded OA -> `DriveRoot_<ws id>`, found or created once. The unused `drive/internal/domain` package, which
carried a second copy of the heuristic and `FolderNodeName(name)`, is deleted (nothing imported it).

### 4. Assets (spec: `docs/specs/asset-authorization/spec.md`)
- Tests first, on the type OA: `asset_oa_authz_test.go` (allow on the right OA; deny for another type's OA, a different op, no op,
  no caller, policy down, type without OA, across Get/Update/Delete/History/Transition/Available transitions/Create/Return) and
  `policy/internal/ngac/pdp_asset_authorization_test.go` (owner allow; member, department member, other workspace, stray association
  deny; grants reach down only and only their ops; prohibition over ALLOW; asset ID is not a node; a second unreachable PC locks owners out).
  Seven of the new asset tests were run red against the old per-asset-node code before it was removed.
- No O node is created or deleted for an asset. `AssetServer` no longer takes a policy write client at all. The authorization OA is
  `asset_types.ngac_oa_id` via the existing join (`store.Asset.TypeOAID`); `assets.ngac_node_id` is dropped.
  `Asset.ngac_node_id` in the proto now carries that OA (unused by the frontend).
- Unplanned but required: the Assets OA now hangs under the workspace PC only and `PC_AssetManagement` is removed. The in-memory PDP
  needs the user to reach every PC the object reaches, no workspace UA reaches `PC_AssetManagement`, so checks on the type OA would
  have denied owners; the old SQL fallback used an any-PC rule and hid it. Vector included. Live smoke confirms owner 200.
- Type OA is named by the type ID (two types that sanitize to one name no longer share an OA).

### 5. Provisioning
New `backend/pkg/provision` (`Rollback`, `Creator`: `Node`, `EnsureNode`, `Assign`, `Associate`, `Fail`, `Done`) and
`backend/testutil/policy_fake.go` (in-memory policy writer with failure injection). Rollback deletes created nodes newest first on a
context detached from request cancellation, keeps going past a failed undo, and never deletes a node it found rather than created.
`EnsureNode` creates only on a positive NotFound; any other lookup error fails.

Applied to: CreateWorkspace, CreateDepartment, CreateRole, CreateFolder (workspace), CreateChannel and DMs (the second participant
now joins inside the same provisioning), user node creation at signup/register/Google/OTP (node removed if the users row fails),
initTenantNGAC (find-or-create + rollback), asset-type hierarchy (find-or-create + rollback incl. row insert failure),
drive folder, channel drive (idempotent: returns the existing drive), share OA, personal UA, drive root.
Tests inject a failure at every graph write and at the row insert for CreateWorkspace, CreateDepartment, CreateChannel, DM,
initTenantNGAC, CreateType, and assert zero live nodes plus newest-first deletion.
Two tenants creating department "Sales": `TestCreateDepartment_SameNameInTwoTenantsDoesNotCollide`.
Idempotence: flows keyed by a deterministic name (tenant UAs, asset tree, channel drive, drive root) are find-or-create; flows that
mint a fresh ID per call (workspace, department, channel) are compensating, not idempotent (no client idempotency key exists).

### 6. Data migrations (`data/migrations/`)
- `022_workspaces_documents_oa.sql`: column + backfill by `<ws id>_Documents`.
- `023_ngac_id_keyed_names.sql`: renames through the entity FK with a same-workspace check; ledger + rollback in `ngac_node_renames`.
- `024_assets_authorized_on_type_oa.sql`: delete all O nodes, drop `assets.ngac_node_id`, keep Assets OA under its workspace PC,
  delete `PC_AssetManagement`. `data/init.sql` no longer declares the column.

## Migrations: rows before and after (live `ngac`, applied with `make db-migrate`)
Backups: `.../scratchpad/ngac-backup-p07-091337.sql.gz` (after the additive 022) and `ngac-backup-p07-pre-migrate-092959.sql.gz`
(immediately before 023/024).

| | before | after |
|---|---|---|
| nodes OA / UA / U / PC | 1734 / 740 / 3 / 336 | 1734 / 740 / 3 / 335 |
| O nodes | 0 | 0 (verified again after the smoke) |
| `assets` rows | 0 | 0 |
| workspaces with `documents_oa_id` | 0 of 3 (column new) | 2 of 3 (the third has no `_Documents` OA) |
| renames recorded | | 10: user 3, channel drive 5, drive folder 1, asset type 1 |
| `drive_items` channel-drive names set to the channel name | | 5 |
| `PC_AssetManagement` | 1 | 0 |

The live DB has no department rows and no workspace-service folders/roles tied to an existing workspace: its 30 `Editor` UAs and 19
`Engineering` OAs hang under PCs whose workspace rows were deleted (test debris), so the same-workspace rule leaves them alone on purpose.
Rehearsal on a copy (`createdb -T`, since dropped) with a synthetic fixture of two tenants with colliding names: 22 checks, all ok
(roles, departments, nested folders, drive folders, channel drives, shares, users, asset types, O-node removal, column drop, PC removal;
deny cases: department/channel-drive pointing at another workspace, user node claimed by two users, role-looking node off a workspace PC,
platform nodes). The ledger rollback `UPDATE ... SET name = old_name` restored every name on the copy. Fixture and checks are in the
session scratchpad (`p07_fixture.sql`, `p07_verify.sql`), not committed. 023/024 are idempotent (second run: 0 rows) and the full
chain (init + 001..024) applies on an empty database.

## Test evidence
- `make build-check`: all 8 services build.
- `make test` (strict, skips fail) with the stated env: exit 0 on `ngac`, on `ngac_ci`, and on a pristine init+migrations database.
- `scripts/check-ngac-identifiers.sh` and `--self-test`: pass. `scripts/check-docs-drift.sh`: pass.
- Mutation check: disabling the rollback makes the workspace provisioning tests fail.
- Live smoke on a throwaway stack (policy, auth, workspace, messaging, asset, drive, document built to the scratchpad, started
  on the `.env.dev` ports against the migrated DB, then stopped; the pre-existing Vite on :5173 and the stale `.dev-pids` untouched):
  signup x2 (201), department "Sales" in both tenants (201, 201: `Dept_<id>` x2), role (201), workspace folder (201), channel (201),
  asset type (201), asset (201), **GET asset as owner 200, as outsider 403**, list as owner total 1 / outsider empty,
  DELETE as outsider 403 / owner 200. Members list shows usernames, not `U_<id>`. After: 0 O nodes; Assets OA has one parent, the workspace PC.
  The smoke left two users and two workspaces in the dev DB.

## Approval follow-ups (not touched; the lint skips `services/approval`)
- `backend/services/approval/internal/domain/queries.go:68` and `:183` — `ResolveAccessibleScopes(..., "read")` -> `ngac.OpRead`
- `backend/services/approval/internal/domain/execution.go:374` — `ResolveAccessibleScopes(..., "approve")` -> `ngac.OpApprove`
- `backend/services/approval/internal/domain/execution.go:435` — `CheckAccess(..., "approve")` -> `ngac.OpApprove`
Remove the `*backend/services/approval/*` exclusion in the script once these are fixed.

## Concerns
1. **Restart required.** 022-024 write the graph tables directly, so a running policy service keeps the old names and the
   `PC_AssetManagement` node in memory until restarted (policy first, then the rest). The smoke stack started fresh and saw the new graph.
2. **Irreversible without the dump:** `assets.ngac_node_id` and the O nodes are gone; only the name renames have a ledger rollback.
3. **`initTenantNGAC` failure is still non-fatal** (as before); it now cleans up and is repeatable, but nothing re-runs it. A tenant
   whose init fails has no tenant UAs until someone does. Suggest making signup fail or self-heal on login.
4. **`ListRoles` returns every UA under the workspace PC**, so the role list shows `<id>_Owners`, `TenantMember_<id>`, `Ch_<id>_Members`
   (seen in the smoke). Pre-existing identifier leak to the UI, unrelated to naming; filter on `type = role` in a follow-up (frontend
   owner should know).
5. A department rename updates the DB row, not the node's `display_name` (the policy API has no node update); the DB is what screens read.
6. Retries after a partial failure depend on the policy read side seeing the node just created; a lagging read replica could allow a
   duplicate. Only affects the retry path.
7. The lint is a line-based awk/regex check: it will not see an operation built across lines or from a variable.
8. Pre-existing debris in the dev DB (about 315 PCs from deleted test workspaces, 30 `Editor`, 19 `Engineering`) is left as is.
9. Migration numbers 022-024 were free at the time of writing; the concurrent approval work should re-check before adding its own.

Unresolved questions: none blocking. Decision wanted on concern 3 (fail or self-heal) and on whether `PC_AssetManagement` removal is
acceptable policy-model-wise (it was the only way for owners to be allowed on the type OA in the in-memory graph).

## Review round 2

Backups before any DB change: `ngac-backup-p07-round2-094932.sql.gz`, `ngac_ci-backup-p07-round2-094933.sql.gz` (scratchpad).
022-025 applied to both `ngac` and `ngac_ci` (022-024 re-applied: idempotent); fresh init + 001..025 and a second run of 022-025 apply on an empty database.

| Item | Fix | Evidence |
|---|---|---|
| H1 | 024 now deletes, before it removes `PC_AssetManagement`, every association on `<ws>_Assets` whose UA is not `<ws>_Owners` and which carries the full owner op set; a partial later grant stays. A `DO` block fails and rolls back the migration if any such association remains. Trade-off: a full-set grant made on purpose to a non-owner is indistinguishable from the legacy rows and goes with them (documented in the file and spec). | Synthetic copy with legacy rows for Members, TenantMember, department, `Ch_*_Members`, role, plus an owner row, a role `read,write` row and a row on another workspace's Assets OA: legacy ones deleted, owner and partial kept, second run no-op. Dev and CI DBs hold none (count query = 0 on both). Policy vector `TestAssetTree_LegacyBlanketGrantIsLiveOnceTheSecondPolicyClassIsGone`. |
| M1 | `RemoveNode` drops the name-index entry only if it points at that node; `EnsureNode` looks the name up again after a failed create and adopts the node found (never deletes it); migration 025 adds a partial unique index on `(name, node_type)` for `TenantMember_ TenantOwner_ DriveRoot_ *_Assets *_Drive *_Category_` (duplicates checked first: none on either DB; the file refuses to run if any). | `pap_graph_name_index_test.go` (red before), `creator_test.go` adopt / deny tests. |
| M2 | **Not done as asked, with a reason.** The proposed unique index on parent-less active folders per context is unsafe: top-level user folders are parent-less too (the drive fixtures create several per workspace), so it would reject normal use. Instead `drive_items.is_root` marks the root (backfill: oldest parent-less folder per non-empty context id), `FindRootByContext` requires it, and a partial unique index covers `is_root` rows. `InsertItem` returns `ErrRootExists`; `ensureRoot` and `CreateDriveForChannel` roll back what they created and read the winner's root. Spec made true. | `TestInsertItem_OneRootPerContextButAnyNumberOfTopLevelFolders`, `TestEnsureRoot_ConcurrentFirstRequestsCreateOneRoot` (8 parallel first requests, one root). 12 existing roots marked on the dev DB. |
| M3 | `CreateDriveForChannel` requires `ChannelId == WorkspaceId` or `GetChannelWorkspaceID(channel) == WorkspaceId` (PermissionDenied otherwise, before any write); an OA that is another workspace's drive root, or whose `workspace_id` property differs, is refused and not deleted. New drive OAs carry `workspace_id`. | deny tests: channel of another workspace, unknown channel, foreign root OA; allow tests: own channel, workspace id. |
| M5 | Lint covers backtick literals, `Sprint`/`Sprintf`/`Sprintln`, and the `%s_Type_%s` / `%s_Category_%s` middle namespaces; prefix must start the string or follow a non-word char (`MENU_%s` is fine). | Self-test: 12 bad lines, good file (comments, allow marker, `MENU_`, `drive/%s/%s_Report`) clean. |
| L1 | `requireRole` (type `role`) in `DeleteRole` and `UpdateMemberRoles`; `ListRoles` lists roles only. | `TestRolesAPI_RefusesPlatformUAs` (Owners, Members, unknown id; nothing detached), `TestListRoles_...AndNothingElse`. Roles written before 023 are marked by 023; unmigrated orphans are not listed. |
| L2 | `Role_` and `U_` added to the reserved prefixes; comment and spec reworded as display-name hygiene. | `ngac_ops_test.go`. |
| L3 | 023 and 024 end with `UPDATE ngac_graph_version SET version = version + 1` and `TRUNCATE ngac_materialized_access`. | applied on both DBs. |
| L4 | Policy `CreateNode` refuses `O` (InvalidArgument); CLAUDE.md intersection bullet now names the object's parent OA (those lines only). | `TestCreateNode_ObjectNodesAreRefused`. |
| L5 | `CreateAsset` returns InvalidArgument when the type is not in `req.WorkspaceId`, after the authorization check. | two tests (mismatch allowed-writer -> InvalidArgument; no write -> PermissionDenied). |
| L6 | 023 documents that the ledger rollback restores names only (not properties or `drive_items` names). | file header. |

Specs updated: `asset-authorization`, `tenant-ngac-init`, `drive-tree-navigation`. Verification: `make build-check` ok, `make check-ngac` ok, strict `make test` exit 0 with no skips on `ngac` and on `ngac_ci`, O nodes = 0 on both.
Migration 025 is new; 022-025 are mine.

Status: DONE_WITH_CONCERNS
Summary: ID-keyed names with typed helpers, lint in CI, drive root by stored ID, assets authorized on the type OA with no O nodes, compensating provisioning, migrations 022-024 applied and rehearsed; build, strict tests on ngac and ngac_ci, lint and live smoke (owner 200 / outsider 403) pass.
Concerns: services need a restart after the migrations; approval literals remain (4); initTenantNGAC failure still tolerated; ListRoles shows platform UAs.
