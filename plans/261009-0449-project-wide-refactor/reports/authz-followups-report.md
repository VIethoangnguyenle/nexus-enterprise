# Authz follow-ups report (2026-10-10)

Nothing committed or staged; frontend/ untouched.

## Per defect

### 1. Drive MoveItem detaches subtree (CRITICAL)
- Cause: RemoveAssignment ran before CreateAssignment; a cycle made the second fail, leaving the folder detached. Top-level folders also kept their root edge.
- Fix (`drive/internal/grpc/server.go`): all refusals before any policy write: self/descendant (new `store.IsAncestorOrSelf`, depth-bounded) -> InvalidArgument; dest not active -> NotFound; dest not a folder / other workspace or context -> InvalidArgument. `reparent()` restores the old edge if the new one fails (also if the row update fails). Row parent + OA change in one statement (`UpdateParentAndNode`). Old parent OA now resolved for top-level folders (drive root) so the stale root edge is removed. Moving the root is refused.
- Root target: supported. Empty `NewParentId` = top level (no parent row, assigned under the drive root OA via ensureRoot).
- Tests: `authz_followups_test.go` (self, descendant, grandchild, sibling edge swap, restore on refusal, to root, top-level leaves root edge, trashed/file dest, deny without write on dest), `store/tree_guard_test.go`.
- Spec: drive-tree-navigation.

### 2. Drive CreateShare (HIGH)
- (a) Person target: `ngac.PersonalUAName(userNodeID)` (new helper). The drive finds or creates that UA, assigns the user to it, and associates from it. Policy `CreateAssociation` now validates node types/existence BEFORE the DB write (`Graph.ValidateAssociation`, `ErrInvalidAssociation` -> InvalidArgument). The drive validates the target and permission before creating any node, and deletes the share OA if a later step fails.
- (b) `ngac.ShareOps`: `read` -> [read], `write` -> [read, write]; anything else (manage, share, lists, empty) -> InvalidArgument. The gRPC `operations` field now carries exactly one permission word; REST unchanged.
- (c) Gate is now `OpShare`. Defaults checked: workspace Owners hold share (AllOwnerOps on Documents); Members do not (read, write, upload). Defaults NOT changed, to avoid widening members. Consequence: plain members can no longer create shares. Revoke-by-creator path unchanged.
- Tests: policy `pap_store_test.go` (no DB row on refusal, non-UA source, non-OA target, unknown node), `pdp_personal_share_test.go` (allow, one-hop-short, op not granted, different PC deny, U source rejected), `backend/ngac/ngac_ops_test.go`, drive tests (mapping, reuse of the UA, bad permission/target write nothing, share-gate deny, cleanup on association failure).
- Spec: drive-context-panel, tenant-ngac-init, resource-pep-coverage.

### 3. Trashed / foreign folder (MEDIUM)
- `liveFolder()` used by ListFolder, CreateFolder, CreateFile: trashed, non-folder, or other-workspace parent -> NotFound. gRPC GetItem returns NotFound for trashed items (pending uploads stay readable; an existing test depends on that). `store.GetItem` unchanged because Restore needs trashed rows.
- Tests: TestTrashedFolderIsNotFound, TestFolderOfAnotherWorkspaceIsNotFound (also asserts no writes). Spec: drive-tree-navigation.

### 4. Breadcrumb cycle (MEDIUM)
- `WHERE p.depth < 64` (`store.MaxTreeDepth`); the uncommitted `depth DESC` ordering is kept. Test builds a parent cycle and requires return within 5s.

### 5. users(ngac_node) index (LOW)
- `data/migrations/018_users_ngac_node_index.sql`: plain `CREATE INDEX IF NOT EXISTS idx_users_ngac_node`. A first partial-index version was replaced by the plain one.
- Applied with `make db-migrate` after backup `scratchpad/ngac-backup-084037.sql.gz`; `\d users` shows `idx_users_ngac_node`, and EXPLAIN uses it. After the host restart I re-checked: the index is still present.

### 6. Approval GetAuditLog (HIGH)
- `domain.GetAuditLog(ctx, callerNode, requestID)` allows: requester, any assignment row (any status or step, new `Store.HasAssignment`), or request `scope_oa_id` in `ResolveAccessibleScopes(caller, "read")` (the same scope set as GetDepartmentRequests). Otherwise ErrAccessDenied (403 / PermissionDenied). A scope-resolution error is not a grant. The caller comes from metadata (gRPC) or claims (REST). `GetRequest` now maps no-rows to ErrNotFound (before it was a 500).
- Tests: domain (each allow path, unrelated deny, cross-request deny, policy failure, unknown), gRPC over the wire (requester and assignee allowed, stranger denied), store integration (`-tags=integration`, tenant-scoped).
- Spec: resource-pep-coverage.

### 7. Approval caller-identity deny gap (LOW)
- `TestBodyUserIsIgnored_DepartmentScopeDeny`: the body names a node that holds a scope, the metadata caller holds none -> empty result, the policy was asked only for the metadata node, and `ListByScopes` was never called. Allow counterpart added.

## Environment
- Mid-run the host restarted: the nexus infra containers were `Exited (255)` and `pdp_personal_share_test.go` had been truncated to 0 bytes. I recreated the test (same content) and ran `docker start` on postgres, redis, redpanda and minio (restoring prior state). The pgdata volume was intact, including migration 018.
- Dev stack (Go services) was not running, so the live smoke was NOT done. The share-to-person download, move-into-self 400 and foreign-audit 403 flows are covered by unit/integration tests only.

## Verification
- `make build-check`: all 8 services OK.
- `make test` (TEST_DATABASE_URL etc. as specified): exit 0, no FAIL, no SKIP in the log.
- Approval store integration: `go test -tags=integration` passes.

## Orphan rows
- The query for non-UA association sources returned 0. The query for `Share_*` OAs without a `drive_shares` row returned 0. Nothing was deleted from the live DB.

## Concerns
- Sharing a file assigns the file's authorizing OA (its parent folder's OA) under the share OA, so a file share exposes the whole folder. This is an existing model limit (files have no OA) and is not fixed here.
- Members can no longer share (they lack `share`). Product decision on whether members should get it.
- Concurrent first-time shares to the same person can create two personal UAs; both work, because the user is in both.
- MoveItem does not rewrite `scope_oa_id` for items moved across scopes (pre-existing).
- Live smoke not done (stack down).

Status: DONE_WITH_CONCERNS
Summary: All 7 defects are fixed with deny-case tests, specs updated and migration 018 applied and verified. build-check and full `make test` pass with zero skips. The live smoke was not run because the dev stack was down after the host restart.
Concerns: see the list above. The main ones are the file-share-exposes-folder model limit and members losing the ability to share.

---

# Review round 2

The "members can no longer share" concern from round 1 is **superseded** by the user decision below: members keep sharing.

## Findings fixed

### C1 (critical) - share takeover through a same-named role
- Cause: `personalUA` found the UA by name and type only, a workspace administrator can create a role with any name, and the graph name index keeps the last node written.
- (a) `drive/internal/grpc/sharing.go`: personal UAs are created with `ngac.PersonalUAProperties` (`type=personal_ua`, `user_node_id`). `findPersonalUA` accepts a node only if `ngac.IsPersonalUAOf(props, user)` holds and the user is inside it (looked up through the user's ancestors; the by-name lookup only adopts a node whose properties match). A same-named role, a node without properties, or another user's personal UA is never reused; a genuine one is created.
- (b) `ngac.ValidateRoleName` rejects reserved prefixes and suffixes and the names `PC_Global` and `PublicUsers` (case-insensitive). `workspace/domain.CreateRole` calls it (InvalidArgument). I chose rejection over renaming roles to `Role_<ws>_<name>`: existing roles, approval templates that look roles up by name, and the UI display stay unchanged.
- Tests: `TestCreateShare_SameNamedNonPersonalUAIsNeverReused` (3 variants), `TestCreateShare_PersonalUAIsCreatedWithItsMarkingProperties`, `TestCreateRole_ReservedNamesAreRejected` (red before the fix), `TestValidateRoleName`, `TestPersonalUAProperties`.

### H1 - reparent detached the folder first
- `reparent` now adds the new edge first, then removes the old one. A refused new edge (a cycle) never touches the old one; if the removal fails the new edge is withdrawn; if that also fails the error says the folder is under both parents and is returned to the caller (not only logged). Rollback after a failed row update uses the same function and also returns its error.
- Tests: refused new edge leaves the old one alone, withdrawal when the old edge cannot be removed, the unrepairable state surfaced, and the edge order in the normal move.

### H2 - concurrent moves
- `store.LockItem` takes a per-item Postgres advisory lock (own pooled connection; a connection that cannot unlock is closed). MoveItem reads the item again under the lock and re-checks write access if its OA changed. `UpdateParentAndNode` is conditional on the parent the move started from and on the item not being trashed; with 0 rows the move undoes its edges and returns Aborted.
- Tests: `TestMoveItem_ConcurrentMovesOfOneItemLeaveOneParent` uses an edge-tracking fake with delays. I confirmed it fails (folder under two parents) with the lock and conditional update disabled, and passes with them. Also run with `-race`.

### M1 - personal UAs on workspace shards
- Fixed rather than documented: `loadShard` adds the personal UAs of the loaded users, and only UAs whose `user_node_id` equals the assigned user. Test `TestLoadShard_CarriesPersonalUAsOfLoadedUsersOnly`, in `policy/.../pip_shard_personal_ua_test.go`, was red first and also checks that another user's personal UA and an ordinary UA are not loaded.

### M2 - one personal UA per user
- Migration `020_personal_ua_unique.sql`: unique partial index on `properties->>'user_node_id'` where `properties->>'type'='personal_ua'`. The drive's create path resolves a lost race by looking again. Test `TestCreateNode_PersonalUAIsUniquePerUser`.

### Low items
- Foreign-workspace claim: the frontend sends no workspace to `GET /api/drive/folders/:folderId` and I may not edit it, so the REST handler now forwards an optional `?ws=` (test `TestListFolder_ForwardsOptionalWorkspace`) and the spec says exactly where the check applies.
- MoveItem and CreateShare reject trashed items (NotFound); tests added.
- GetAuditLog: a missing request and a hidden one both return PermissionDenied (403); a non-UUID id returns InvalidArgument (400). Tests updated and extended (domain and over the wire).
- Index `(request_id, user_node_id)` on `approval_assignments`: `idx_aa_request_user` added to `provision_tenant_schema` (007, re-applied by `make db-migrate`) plus migration `021_approval_assignments_request_user_index.sql` for existing tenant schemas.

### User decision - members keep `share`
- `ngac.MemberDocumentOps` and `ngac.ChannelDriveOps` now include `share`. Migration `019_member_share_op.sql` backfills Members->Documents (same-workspace match) and Ch_<id>_Members->the channel drive OA (matched through the OA's `channel_id` property, because the drive OA is named after the channel's display name). Idempotent. It bypasses EPP invalidation, so the services must be restarted after it is applied (the dev stack is down now).
- Test vectors: `TestWorkspaceMemberCanShareButShareGranteeCannotReshare` (member ALLOW share on Documents; write-share grantee gets read and write only, DENY share, manage, approve), the updated member-DENY list (share removed from it), `TestMembersKeepShareButShareGranteesDoNot`.
- Specs: tenant-ngac-init (including "member operations: read, write, upload, share on Documents", reserved role names, personal UA properties, shard behavior, unique index), drive-context-panel, drive-tree-navigation, resource-pep-coverage.

## Migrations and databases
- Backups before applying: `scratchpad/ngac-backup-r2-085545.sql.gz` and `ngac_ci-backup-r2-085546.sql.gz`.
- `ngac`: `make db-migrate` applied 019, 020, 021 and the 007 function update. Verified: Members->Documents and the 3 `Ch_*_Members`->drive associations now hold `{read,share,upload,write}`; `\di uniq_ngac_personal_ua_per_user`; `idx_aa_request_user` exists in both tenant schemas (`tenant_47d0e8e6`, `tenant_c4e727e1`).
- `ngac_ci`: applied 007, 018 (it was missing there), 019, 020, 021 with `psql -v ON_ERROR_STOP=1`; verified `idx_users_ngac_node` and `uniq_ngac_personal_ua_per_user`. It has no tenant schemas.
- Workspace-owner associations to the workspace-root channel drive OAs (`<ws>_Owners` -> `Ch_..._Root_Drive`) are Owners rows, not member associations, and were left as they are.
- No personal UAs existed in the live DB before the unique index (count 0).

## Verification
- `make build-check`: all 8 services OK.
- `make test` with `TEST_DATABASE_URL` on `ngac`: exit 0, no FAIL and no SKIP in the log. Same on `ngac_ci`: exit 0, no FAIL and no SKIP.
- Approval store integration (`-tags=integration`): pass. Drive move tests pass under `-race`.

## Concerns
- Services must be restarted after migration 019 for the in-memory graph to pick up the member `share` grant; the dev stack was not running, so no live smoke was done again.
- Reserved role names are a deny-list. The drive lookup no longer trusts names, so a missed namespace cannot hijack shares, but other name-based lookups (Owners/Members UAs) rely on the list; new platform name patterns should be added to `ngac.ValidateRoleName`. Roles that already exist with reserved names are not renamed.
- A file share still exposes the file's whole parent folder (round 1 concern, unchanged).
- A lost personal-UA creation race is resolved by retrying the lookup for about 150 ms; beyond that the share fails with Internal and can be retried.

Status: DONE_WITH_CONCERNS
Summary: All round-2 findings are fixed with deny-case tests (C1, H1, H2, M1, M2, the four low items) and the user decision on member `share` is implemented. Migrations 019-021 are applied and verified on `ngac` and `ngac_ci`. build-check and full `make test` pass with zero skips on both databases. Specs and the report are updated.
Concerns: the services need a restart after migration 019, there was no live smoke because the dev stack is down, the role-name deny-list needs maintenance, and the file-share-exposes-the-folder limit remains.
