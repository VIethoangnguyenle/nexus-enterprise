# Phase 04b-admin + 06 admin redesign: report

Date 2026-10-10, branch `refactor/project-wide`. Nothing committed, staged or added to the index (the staged set is the same 158 entries as before; one slip, a `git rm --cached` on `InviteMemberForm.tsx`, was undone at once with `git reset -q HEAD -- <file>`, and the file was then deleted from the working tree only).

## What was built

**Three screens** under `routes/_workspace/admin*` (mockup `admin.html`, approved), all in `components/admin/`:

- **Tổng quan**: three figures, the organisation as the shared `TreeView` (new optional `trailing` prop for the head count; two levels open), department panel (parent chosen from a tree by name and never its own subtree, members by name with a search to add one, rename, delete after a confirmation), "Phòng ban mới" dialog.
- **Người dùng**: hairline table (name + email, department, at most two role pills then `+N`, standing in words), accent-insensitive search, filters by department (with the departments beneath it) and role, panel with removable role pills (toast with "Hoàn tác"), role picker, department picker, "Xoá khỏi workspace" after a confirmation.
- **Vai trò**: built-in roles ("Chủ sở hữu", "Thành viên", lock + "Hệ thống") and custom roles by display name, panel with who holds the role and its permissions per area in words, create dialog (a name the server refuses is explained next to the field), delete after a confirmation, and the **permission editor** (matrix on tablet/desktop, switch list on a phone, change bar that says who is affected, save one request per changed area, "Hoàn tác").
- **Invite dialog**: email chips (type, paste, separators), optional role and department, each address answers for itself.

State is in the URL (`?ws= &dept= &member= &role= &edit=1`). The key is `member`, not `user`: `stores/auth.store.ts:50` reads `?user=` as which account's session to use, and the first screenshot run showed the app logging out on `?user=<id>`.

**Data layer (04b)**: query keys only through `hooks/keys` (`keys.admin.{all,departments,members,roles,role,roleDetails,permissionAreas}`); every imperative `apiFetch` in the old screens is a hook in `hooks/useAdmin.ts` (12 mutations, each with `meta.action` or `silentError`; the invite and create-role dialogs report inline). `ConfirmDialog` is used with its real API (`open`/`onClose`/`description`): the old `admin/index.tsx` and `roles.tsx` passed `message`/`onCancel`/`variant` without `open`, so no confirmation ever opened. Dead code removed: `components/InviteMemberForm.tsx`, `useInviteMember`.

**Backend** (workspace service, plus two policy fixes the screens needed):

| Change | Where |
|---|---|
| Areas and the operations valid on each, one source of truth | `backend/ngac/areas.go` (`Areas`, `AreaOps`, `AreaOAName`), tests |
| `GET /workspaces/:id/permission-areas` answers from it; an area whose OA the workspace lacks (no Assets OA yet) is omitted | `domain/permission_areas.go`, `rest/admin_people_handler.go` |
| Roles list split into `roles` (custom, what approval's picker reads) and `system_roles` (owners/members, no English name), with counts | `domain/roles_admin.go` |
| Role detail (people, grants by area), set permissions per area, assign/unassign one role, people table, invite by email | `domain/roles_admin.go`, `domain/member_directory.go`, store `directory_store.go` |
| `UpdateMemberDepartment` fixed: it wrote `tenant_users.department_id` where `user_id = <ngac node id>` (never matched), never left the old department UA, and accepted a stranger | `domain/department.go`, `store/department_store.go` |
| `RemoveMember`: an owner needs `manage`, the last owner stays, the `tenant_users` listing is dropped | `domain/service.go` |
| `DeleteDepartment`: sub-department UAs and people are re-parented in the graph before the node goes (before, a child department lost its path to the PC) | `domain/department.go` |
| Policy read RPC `GetAssociations(ua_id)` | `proto/policy/policy_read.proto`, `read_server.go` |
| Policy: a second grant on the same UA and OA replaced the DB row but **added** a second edge to the in-memory graph (new random ID), so narrowing a role did nothing until restart; the store now keeps the row's ID (`RETURNING id`) and the graph keeps one edge per pair | `pap_graph.go`, `pap_store.go` |

No migration was needed (no schema change), so no backup/`db-migrate` step ran.

## User decisions

- **Ops per resource area from the backend, labelled in Vietnamese.** The endpoint returns `{area, operations}` derived from `ngac.AreaOps`, which lists an operation on an area only where a service checks it there (management: manage, invite; documents: read, write, upload, share; channels: read, write, manage, invite, create_channel; assets: read, write, approve, manage). The client keeps only words (`lib/admin-model.ts`: Xem, Sửa, Tải lên, Duyệt, Chia sẻ, Quản lý, Mời, Tạo nhóm chat; an operation or area it has no word for reads "Quyền khác" / "Vùng khác", never a code). The matrix columns are the union of what the server offers; a cell exists only where the server says the pair applies. Test: with the server answering documents `[read, share]` and assets `[approve]`, the grid has exactly the columns Xem, Duyệt, Chia sẻ and exactly three pressable cells (`AdminScreens.test.tsx`, "draws the areas and operations the server answers with"); I confirmed it goes red when the matrix hard-codes the eight columns.
- **Members hold `share` on Documents and channel drives** (migration 019): `AreaOps(documents)` and `AreaOps(channels)` are tested to contain every operation in `MemberDocumentOps` / `MemberChannelOps`, and the Members role panel test shows "Chia sẻ" under Tài liệu and "Tạo nhóm chat" under Tin nhắn, read-only.

## Server rules

Every new or changed endpoint takes the caller from verified claims and checks on the workspace Mgmt OA (table also in `resource-pep-coverage`):

| Endpoint | Check |
|---|---|
| `GET permission-areas`, `GET roles` | membership |
| `POST/DELETE roles`, `GET roles/:id` | `manage` |
| `PUT roles/:id/permissions/:area` | `manage`; role must be an administrator-made role of this workspace; area known and present; every operation offered by the area; **each added operation must be held by the caller on the area's OA** (keep/remove need nothing more) |
| `PUT/DELETE members/:node/roles/:role` | `manage`; custom role of this workspace; target already a user of the workspace; **assigning also needs the caller to hold everything the role confers** (a grant that cannot be read is not assumed empty) |
| `PUT members/:node/department` | same: member target, department of this workspace, delegation guard, one department (old edge removed, new edge taken back if that fails) |
| `GET admin/members` | `manage` |
| `POST members` (email) | `invite`; address looked up only after that is decided; 404 no account, 409 already a member, 400 not an address; listing failure undoes the assignment |
| `DELETE members/:node` | `invite`; an owner needs `manage`; last owner stays |

Deny tests: `domain/admin_screens_test.go` (about 45 tests: non-manager callers `member`/`inviter`/`outsider`/`otherOwner`/empty for each endpoint, policy down, foreign and built-in roles, strangers as targets, escalation, unreadable grants, invalid areas/operations, rollback paths) plus `rest/handler_test.go` (caller from claims not body, 401 without claims, error mapping, wire shapes). Mutation-checked: removing the delegation guard in `AssignMemberRole`, the added-operation check in `SetRolePermissions`, the owner guard in `RemoveMember`, or the member check in assign/department each turned tests red.

Names are tenant-scoped: the people table joins `users` by node and `tenant_users` by the route's workspace; a name is never an id (a node named `U_<uuid>` without a display name reads "Thành viên").

## REST bodies, field by field

Checked against the handler structs and pinned in `api/admin.test.ts` (8 tests): create department `{name, parent_id}`; update `{name}`; move `{new_parent_id}`; member department `{department_id}` (empty to leave); invite `{email}`; create role `{name}`; set permissions `{operations}` with the area in the path; assign/unassign carry no body. The old screens' `DELETE roles/:id` and `GET members` had no matching REST route at all (404), and `POST/GET roles` went through a gRPC wrapper that returned no member counts or kinds; roles now live on the admin handler.

`vite.config.js`: checked, not edited. Every new path sits under `/api/workspaces/:id/` and none matches the re-dispatch regexes (`drive`, `documents`, `channels`, `contacts`, `asset`), so all go to the workspace service on :8181.

## Decisions where the mockup was silent (and what is not built)

1. **Activity feed not drawn.** No audit source exists (the policy service publishes graph mutations to Kafka without an actor and nothing stores them). Building one is a new table in every write path: out of scope, listed as a gap. Nothing is faked.
2. **Invite by email adds existing accounts.** A real invitation (record + accept at sign-up) lives in the auth service and is not small. The endpoint adds the account that holds the address; an address with no account is 404 and the dialog says so under its chip and keeps it. The mockup's "Lời nhắn" field is omitted (nothing is sent), as is the "Đã mời / Lời mời chờ nhận" figure.
3. **Area set follows the graph, not the mockup's rows.** The mockup lists Tài liệu, Tin nhắn, Phê duyệt, Tài sản, Thành viên. The workspace graph has four area OAs (Documents, Channels, Assets, Mgmt); approvals are not authorized on a workspace OA (templates need `manage` on Mgmt), and "Thành viên" is Mgmt. Rows are therefore Tài liệu, Tin nhắn, Tài sản, Quản trị.
4. **`manage` and `approve` are offered where a service checks them**, not on every row as in the mockup (e.g. `manage` is not offered on Documents).
5. **No lock-account action, no role rename, no "Trưởng phòng"**: nothing in the backend supports them. A person disabled in `tenant_users` shows "Đã khoá".
6. **Pickers open in place** (not as floating popovers): a popover (`z-dropdown`) would sit under a side panel or dialog, so department and role pickers render inline; the people-table filters, which are not inside a layer, do use `Popover`.
7. **Role members in the role panel** show up to three avatars and "A, B và N người khác" (the endpoint returns at most 50 people for the avatars; the count is the whole).
8. **Admin density 7** read as the same 44px rows with tighter columns, as in the mockup report; no new token.
9. **Phone switch list** (mockup §9) is a separate component chosen by a 768px media query (`hooks/usePhone`).
10. The editor drafts in memory; leaving by another route discards without asking.

## Known gaps and hazards (for the plan, not fixed here)

- `UpdateMemberRoles` (gRPC only) detaches a person from every UA under the workspace, including departments and channels, then assigns the roles, and has no delegation guard. The REST screens use the single-assignment routes; the gRPC method should be removed or narrowed.
- `GET /api/workspaces/:id/contacts` (auth service) takes no authorization and reads `users.department` text, not the departments table; Contacts group.
- `CreateWorkspace` does not insert a `tenant_users` row for the owner (auth does, elsewhere); a workspace created through the workspace service alone lists no owner in contacts.
- The people table makes one policy read per owner/role/department (roles and departments are few); it is not paged.
- The shell's floating user avatar still overlaps the bottom-left of phone sheets; my panel footers on phones carry `max-md:pl-16` to keep "Xoá …" readable. Shell issue, outside this change.
- `StatusPill`/`CountPill` are imported from `components/approval/`; they belong in `primitives` (`primitives/index.ts` was off limits this round).

## Specs

`docs/specs/workspace-admin-authorization` (areas, per-area permission editing with the delegation rule, role/department assignment as delegation, people table and invitations, owner-protecting removal, department delete keeps the graph whole, `## Status`), `tenant-ngac-init` (one association per UA/OA with the replace vector, `GetAssociations`, Members hold `share`), `resource-pep-coverage` (endpoint to op to OA table), new `admin-screens`, and `docs/specs/README.md` (re-read before editing; one line added under "Administration"). Policy-model change rule: spec + test vector in `backend/services/policy/internal/ngac/pdp_association_replace_test.go` (red on the old graph, green now).

## Evidence

- Frontend: `eslint src` exit 0; `typecheck:diff` no file above baseline; `npm test` 49 files / 563 tests pass (admin: `AdminScreens.test.tsx` 59, `admin-model.test.ts` 19, `api/admin.test.ts` 8, `TreeView.test.tsx` +1); `npm run build` exit 0 (regenerated `routeTree.gen.ts` by the plugin, not by hand). Forbidden-class grep (`transition-all`, `duration-<n>`, `animate-bounce|pulse`, `z-[`, `rgba(`) empty over `components/admin`, routes, `lib/admin-*`, `useAdmin`.
- Tests cover: no UUID rendered anywhere (text, aria-label, title, placeholder, input values, and no `Role_`/`Dept_`/`_Owners` names) with UUID fixtures on every screen, the panels, the invite dialog and the editor; the editor rendering the server's operations; deny/forbidden state; loading, empty and error states on all three screens; the confirm dialogs really open; Esc closes only the topmost layer (picker, then dialog, then panel); dialog and panel state resets per item and per opening; rows are buttons driven by arrows and Enter; the permission grid moves with arrow keys, skipping cells that do not apply; phone switches.
- Backend: `make build-check` all eight services; `make check-ngac` clean; `make test s=workspace` exit 0, 354 passes, **0 skips** on `ngac` and on `ngac_ci`; `make test s=policy` exit 0, 210 passes, 0 skips on both; `go test ./ngac` ok.
- Live smoke on the real policy service (built binaries on :50051 and :18181, throwaway users and workspace in `ngac`, all rows deleted afterwards, both processes stopped): 50 checks passed, including the new `GetAssociations` RPC, narrowing a role then reading the grants back (graph replaced, not unioned), invite by email 201/409/404/400, a plain member refused (403) on every admin route while still reading roles, a stranger refused as a target (404), last-owner removal refused, and the people table naming people, departments and roles with no identifier.
- `make proto`: I regenerated only `policy_read.proto` (same protoc 29.3 / plugin versions as the committed headers), not the whole set, because another agent had `proto/asset/*` in flight; no `.proto` consumed by the frontend changed, so `npm run proto:gen` was not needed.

## Screenshots

`plans/261009-0449-project-wide-refactor/reports/admin-screens/` (Playwright from the gstack install against the Vite already running on :5173 (not mine; left running); the API is answered in the page by `src/test/admin-fixtures.ts` with UUID fixtures, no DB writes):

- `admin-{desktop,tablet,mobile}-{light,dark}-{overview,overview-dept,users,users-panel,roles,roles-panel,permissions}.png` (42 files; the permissions shots have one change drafted so the change bar shows)
- dialogs and pickers (desktop light): `invite`, `invite-result` (one added, two refused with reasons), `role-picker`, `department-picker`, `user-filter`, `create-role-error`, `delete-role`, `new-department`, `system-role`, `permissions-saved-bar`, `permissions-areas-from-server`, `permissions-save-error`; dark and mobile `invite`
- states (desktop light): `{overview,users,roles}-{loading,empty,error,forbidden}`; dark `users-forbidden`

Compared with the mockup: header with title, workspace and primary action; tabs; three figures; tree with right-aligned counts and the open row bold; panel content order; hairline users table; matrix with `warning-wash` on changed rows and the change bar with the consequence sentence all match. Differences found and fixed from the first run: the department field and a role pill overflowed the panel (`min-w-0`, pill text truncates inside the pill); `?user=` logged the session out (renamed `?member=`); empty states flashed before the workspace id resolved (`isPending`, not `isLoading`); a delete confirmation never ran its follow-up because the panel unmounted when its subject disappeared (hooks no longer await the refetch).

## Concerns

1. The role/department delegation guard is strict: a manager who lacks, say, `write` on Documents cannot give someone a role that grants it. That is the stated rule ("grant only what you hold"), but it will surprise a delegated "HR" role; the 403 toast says they lack permission, not which operation.
2. `GetAssociations` reads the in-memory graph of whichever policy replica answers; a read replica that has not yet applied an EPP refresh can show a role's old grants for a moment.
3. The people table's department is read from the graph (the source of truth); `tenant_users.department_id` is kept in step by `UpdateMemberDepartment`, but rows written by the auth service's auto-join do not set it, so department head counts in `GET departments` (which count that column) can differ from the table until a person is moved.
4. The other agent edited `components/primitives/ChoicePicker.tsx` (used by the invite dialog) and `typecheck-baseline.txt` during this round; my tests ran green against their versions.
5. The phone matrix replacement and the permission editor were exercised in jsdom and screenshots, not on a device.

## Review fixes

Backups before any DB change: `backup-ngac-admin-review.dump`, `backup-ngac_ci-admin-review.dump` (scratchpad, `pg_dump -Fc`). Migration `data/migrations/029_workspace_invitations.sql` applied with `make db-migrate` to `ngac` and with `psql` to `ngac_ci` (idempotent). Index untouched (still 158 entries). No asset file, `AppSidebar` or `routes/_workspace.tsx` edited. Where this section contradicts the report above (invite by email adding accounts, `CreatePermission` guard, `upload` on Documents, `UpdateMemberRoles` hazard), this section wins; the specs are already rewritten.

**C1 critical: free-form permission grant removed.** `POST /api/workspaces/:id/permissions` and the gRPC `CreatePermission` (and `UpdateMemberRoles`, M1) are deleted from handler, service, interface and `workspace.proto` (regenerated only `workspace.proto`; the frontend consumes only `messaging/ws.proto`, so `npm run proto:gen` was not needed). No caller existed (checked in Go and the frontend); their tests are gone. New tests: the route is not registered on either handler and a POST/PUT to it is 404/405/401, never 200 (`rest/handler_test.go`); `SetRolePermissions` cannot reach another workspace's role, Members UA or Owners UA through any route (404 through the owner's own route for foreign attributes, 403 through a foreign workspace's route), and cannot edit Owners/Members at all (`domain/admin_screens_test.go`); live smoke repeats all of it against the real policy service.

**H1: delegation follows inheritance.** `guardDelegation` collects the UA and every UA among its `GetAncestors`, reads each one's associations, and requires the caller to hold every (OA, operation); an unreadable ancestry or grant is a denial. `MoveDepartment` runs it on the new parent's chain before any edge is touched (moving to the root grants nothing). Deny tests: put yourself in a sub-department whose parent grants more than you hold; move a department under such a parent (nothing detached or attached); unreadable ancestors.

**H2: pending invitations (migration 029).** `workspace_invitations` (workspace, normalised email, optional role/department, inviter node, status pending/accepted/declined/revoked, `created_at`, `expires_at`, one pending offer per address per workspace). `POST /workspaces/:id/members {email, role_id?, department_id?}` needs `invite`, answers `202 {"status":"invited"}` for every well-formed address, never consults accounts or memberships, never assigns, creates or refreshes the offer (7 days); attaching a role or department needs `manage` and everything it confers (with inheritance) and both must belong to the workspace. Per-caller limit 40/hour (429), counted before storing, only for callers who may invite. The old direct add and `FindUserByEmail` are gone. Admin: `GET /workspaces/:id/invitations`, `DELETE .../invitations/:id` (invite op, scoped to the workspace). Invitee: `GET /api/invitations`, `POST /api/invitations/:id/accept|decline`, matched on `users.email` of the verified token's user; another address's or unknown is 404, answered 409, expired 400. Accept refuses (403, offer stays open) when the inviter no longer holds `invite`; claims the invitation in one conditional update (concurrent accepts add once), adds the person through the Members assignment and `tenant_users`, re-checks role and department against the inviter's rights now (the person joins without them if the inviter lost `manage` or any operation, or the role was deleted), reopens the offer if adding fails. Rate-limit mechanism: in-process fixed window (the workspace service has no Redis; auth's OTP limit is the same INCR/EXPIRE shape on Redis), so the budget is per replica. Routes: `/api/invitations` added to `frontend/vite.config.js` and to the workspace Traefik rule in `docker-compose.yml`; the workspace-scoped ones need nothing (checked against the regex block). Deny/behaviour tests: `domain/invitations_test.go` (about 25 tests), store tests against Postgres for every transition (`store/invitation_store_test.go`), handler tests for wire shapes and 202/429/404/409; mutation-checked (address match, inviter re-check, role re-check, limiter). Frontend: invite dialog sends one invitation per address with the chosen role and department and says the same for every address (a refused one is named under its chip; a 429 stops the rest); pending list with "Thu hồi" under the users table; header "64 thành viên, 2 lời mời"; Tổng quan card "Lời mời chờ nhận" with how many lapse within two days; `api/invitations.ts` for the invitee side. The accept screen on workspace selection is the auth group's; endpoints and spec are ready.

**H3: `upload` is not offered on Documents.** `AreaOps(documents)` is read, write, share; a test fails if any area offers `upload`; the screens, fixtures and specs follow ("listed only where a service checks it" now holds). Grants of `upload` to members stay, unshown.

**M1/M2: owners.** `UpdateMemberRoles` deleted. `TransferOwnership`, `RemoveOwner` and removing an owner through `RemoveMember` require the caller to be in the Owners UA (a role carrying `manage` is not enough); a new owner must already belong to the workspace. Each reads the owners and acts under a per-workspace Postgres advisory lock (`Store.WithOwnerLock`, `pg_advisory_xact_lock`), re-checking the last-owner rule inside it. Tests: manager-not-owner denied on all three, non-member new owner refused, a concurrency test with two owners removing each other (exactly one removal; red when the lock is removed), store tests that the lock serialises one workspace, does not block another and is released on error.

**M3: association writes ordered.** `Store.assocMu` held across the row write and the graph update in `CreateAssociation` and `RemoveAssociationByUAOA`. Honest note: my concurrency tests (60 rounds × 8 writers, and grant vs remove) pass with and without the mutex on this machine, because the window is a few microseconds; they guard the invariant (graph equals database after every round) but I could not make them fail deterministically.

**L1.** `SetRolePermissions` reads the role's current grants from the policy **writer** (new `PolicyWriteService.GetAssociations`, same answer as the read one from the writer's own graph); a test where the replica still shows `write` and the writer does not shows adding it back needs the caller to hold it.

**L2.** `CreateAssignment` returns the existing row's ID (`ON CONFLICT … DO UPDATE … RETURNING id`) and the graph keeps one entry per edge (a stale entry was left by `RemoveAssignment` before); tested against Postgres.

**L3.** Department head counts (`GET departments`) come from the Dept UA's user children in the graph, the same source as the people table (previous: cached `tenant_users.department_id`); test compares the count to the people the table lists for that department.

**Mobile pill.** The users panel is the same component at every width: a plain member shows the locked "Thành viên" pill and an owner without roles the locked "Chủ sở hữu" pill, no remove button; two tests run it with the phone media query (`it.each`), plus screenshots `admin-mobile-*-users-panel-plain-member.png`.

**Other fixes made on the way:** `DeleteDepartment` re-parents graph edges (reported above); the earlier "`GetAssociations`" read RPC is now also on the writer.

**Evidence.** Frontend: `eslint src` exit 0, `typecheck:diff` no file above baseline, `npm test` 50 files / 574 tests pass, `npm run build` exit 0; forbidden-class grep empty. Backend: `make build-check` all eight services, `make check-ngac` clean, `go vet` on every module clean; `make test s=workspace` exit 0, 381 passes, zero skips on `ngac` and on `ngac_ci`; `make test s=policy` exit 0, 214 passes, zero skips on both. Live smoke on the real policy service (fresh binaries, throwaway users and two workspaces, rows deleted, both processes stopped, Vite (mine this time, the other one had stopped) stopped afterwards): 43 checks, all passed: the old route 404/405; no cross-workspace grant through the per-area route either way; Members UA not editable; `upload` refused on Documents; 202 `{"status":"invited"}` identical for an account holder, a stranger address and a current member (400 for a non-address, 403 for a stranger, 404 for another workspace's role); invitee still not a member after being invited; admin list shows role and inviter by name; invitee lists the offer by workspace name without an address echoed; another user's accept/decline 404; accept 200 with the role applied and the member holding it; accept again 409; an expired offer 400 and unlisted (set in SQL); revoke 204, accept of a revoked offer 409, another workspace's owner cannot revoke (403); decline then accept 409; a manager role (`manage`+`invite`) cannot remove an owner (403) and the last owner stays (400); the 41st invitation of the hour is 429.

**Concerns.** (1) Invitations send no email; the invitee finds the offer on next sign-in, and the screen for that is the auth group's. (2) The rate limit is per process. (3) `users.email` is the match key; it is set by a verified OTP or Google login and not editable by profile update, but a deployment that adds an email-change flow must re-verify. (4) The concurrency tests for M3 do not fail without the mutex (see above). (5) `ListDepartments` now costs one policy read per department.

## Review fixes round 2

**C-A invitations matched unverified email.** Migration 030 adds `users.email_verified_at` (backfilled for Google identities), refuses to proceed if two accounts differ only by letter case (it raises and lists the addresses; nothing is merged; both databases had none) and then creates `uq_users_email_lower` on `lower(email)`; `GetUserByEmail` matches `lower(email)`. Signup, register, sign-in and OTP trim and lower-case the address (invalid ones are 400); a case-variant signup of an existing address is 409. `email_verified_at` is set only by Google sign-in with `email_verified` true and by an OTP code that a real sender delivered (`OwnershipProver`/`DeliversToOwner`); the fixed test code and the log sender never prove anything, and the first real proof on an existing account evicts its unverified credentials. Workspace `UserEmail` returns "" unless verified, so an unverified account sees an empty invitation list and 404 on accept/decline. Specs corrected (workspace-admin-authorization, tenant-auth-flow).

**H-A direct add by node ID deleted.** REST `POST /workspaces/:id/invite`, gRPC `InviteMember` (+ messages), `Service.InviteMember` and frontend `workspaceApi.inviteMember` are gone; no other caller existed (grep over all services and frontend). Test: no route contains `/invite`, POST/PUT on it is 404/405/401.

**M-A authorization reads from the writer.** `PolicyWriteService` gained `GetNode, GetChildren, GetParents, GetAncestors, GetDescendants` (same grpcauth policy as the read service: Unauthenticated without a caller, user caller required; tests for both). Every graph read in the workspace domain that feeds an authorization or write decision (owner list, membership, scope, guardDelegation ancestry and grants, directory) now goes to the writer; only `CheckAccess` stays on the read service. Stale-reader/fresh-writer tests (`stale_reader_test.go`): owner re-check, delegation grants, delegation ancestry, membership, workspace scope; mutating one read back to the replica makes the corresponding test fail.

**L-A removal revokes offers.** `RemoveMember` revokes pending invitations to the removed person's verified address in that workspace (failure is logged, removal stands); tests for revoke, denied removal revokes nothing, unverified address has nothing to revoke.

**Test hygiene found on the way.** OTP tests use unique identifiers per run (Redis rate limits persist across runs); no test skips: Redis-backed auth tests run with `REDIS_ADDR=localhost:6379`.

**Evidence.** Frontend: lint, `typecheck:diff`, `npm test` (581), `npm run build` all exit 0; `npm run proto:gen` no change (it covers only messaging/ws.proto); workspace and policy_write protos regenerated with protoc. `make build-check`, `make check-ngac` pass; `go vet` clean on every module; `make test s=workspace`, `s=policy`, `s=auth` exit 0 with zero skips on both `ngac` and `ngac_ci` (TEST_DATABASE_URL, REDIS_ADDR set). Migration 030 applied to `ngac` and `ngac_ci` (backed up first).

**Live smoke** (real policy, auth, workspace; Redis, Postgres): owner invited `Victim<N>@Smoke-B.test` (202); `POST /invite` 404; an attacker signed up with that address (201) and saw `{"invitations":[]}`, accept and decline 404; case-variant signup 409; fixed-code OTP (999999) for the address signed in but the account stayed unverified and still saw nothing (accept 404). The verified step could not use a real sender (dev has none), so I set `email_verified_at` by SQL as a stand-in for Google or a delivered code (the delivered-code path is proven in unit tests with a test-only verifying sender); then the list showed the offer and accept returned 200. Re-invite then removal: the pending invitation became `revoked`. Processes stopped and the test rows deleted; git index unchanged (158 entries).

**Concerns.** (1) No real OTP sender exists, so email-based invitation acceptance works in practice only through Google until one is added. (2) Existing OTP-created accounts are unverified until they prove the address again. (3) The earlier concerns (no invitation email, per-process rate limit, M3 guard tests) stand.

Status: DONE_WITH_CONCERNS
Summary: Round 2 defects fixed with deny tests: invitations match only verified, case-insensitive addresses; the direct add-by-node-ID route is gone; authorization reads use the policy writer; removal revokes pending offers. All gates and the live smoke pass on ngac and ngac_ci.
Concerns: no real OTP sender in dev (verified step shown with a SQL stand-in plus unit tests); accounts made through OTP before this change are unverified; no invitation email; invite limit per process.
