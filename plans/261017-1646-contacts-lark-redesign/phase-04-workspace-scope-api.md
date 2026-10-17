---
phase: 4
title: "Workspace: scope, filtered reads, admin rules API, demo seed"
status: completed
priority: P1
effort: "8h"
dependencies: [3]
---

# Phase 4: Workspace — scope, filtered reads, admin rules API, demo seed

## Overview
One domain function decides what a caller may see (`DirectoryScope`), built only from the membership check and PDP
decisions. Every member-readable people/org read in the workspace service is filtered by it, admins get rule endpoints,
auth gets the scope over gRPC, and a dev script seeds a demo org with real graph nodes.

## Requirements
- `DirectoryScope{Ready bool; Admin bool; Limited bool; VisibleDeptIDs set; NoDeptVisible bool; OwnersVisible bool;
  OwnerNodeIDs []string; HiddenDeptIDs set (admin only); Roles []RoleHolders{ID, Name, Kind(owners|custom), HolderNodeIDs}}`.
  - Caller must be a member: `authorizeMember` (`authz.go:76`; `policyclient.IsWorkspaceMember` over the writer, :98-104).
    Not a member → `ErrAccessDenied` (403); membership unknown → denied as today.
  - `Ready=false` when the workspace has no `Directory_<ws>` (phase 03 `directoryNodes` NotFound; release-A window before
    064): no rule can exist, so the scope is "everyone": `Admin=false, Limited=false`, all departments + no-dept + owners
    visible. Any other lookup error → `ErrUnavailable` (503), never "see all".
  - `Admin` = `checkAccess(caller, Directory_<ws>, manage)` (`authz.go:25`).
  - Non-admin: one `policyclient.BatchCheck(caller, [every Dir, DirPeer, DirectoryNoDept, DirectoryOwners], [read])`
    (`backend/pkg/policyclient/policyclient.go:65`); dept visible if its Dir or DirPeer is ALLOW; a department whose nodes are
    missing in a Ready workspace is invisible to non-admins and logged at Error (fail closed).
  - `Limited` = caller has `DirectoryLimited_<ws>` among its ancestors (display only — "Chỉ phòng của bạn"; the decision is
    the PDP's). Limited and the prohibition `DirectoryLimitedProhibitionName(ws)` missing (`ListProhibitions`,
    `proto/policy/policy_read.proto:42`) → `ErrUnavailable`.
  - Owners always visible (Zane 2026-10-17): `OwnersVisible` = ALLOW read on DirectoryOwners; `OwnerNodeIDs` = direct users
    of `<ws>_Owners`.
  - Roles: Owners UA (kind owners, shown as "Quản trị viên") + custom roles (`classifyRole`, `roles_admin.go:56`); never
    Members. Holders = direct users (`roleHolders`, `roles_admin.go:84`). Visible-holder filtering happens where the people
    rows are (auth).
  - Policy connection: `s.policyRead` for decisions. In prod it is the writer (workspace sets no `POLICY_READ_SERVICE_ADDR`,
    `deploy/docker-compose.prod.yml`; default = `POLICY_SERVICE_ADDR`, `workspace/cmd/main.go:47`); if workspace ever moves to
    `policy-read`, rule-write → read tests must use the writer (same note as security phase 05).
- Filtered reads (non-admin):
  - `ListDepartments` (`department.go:127`, REST `admin_handler.go:47`): only visible departments; admins get all with
    `hidden`. Its N+1 `GetChildren` (:140) stays (not required).
  - `ListMembers` (`members.go:233`, REST `rest/handler.go:47`→:103, gRPC `internal/grpc/server.go:94`): only visible people
    (+ self, + owners). No other service calls the gRPC method today (grep on main).
  - `ListRoleSummaries` (`roles_admin.go:114`, REST `GET /roles`, `admin_people_handler.go:39`, member-readable):
    `MemberCount` counts visible holders only for non-admins.
  - Unchanged: `ListMemberDirectory` (`member_directory.go:137`) — admin only (`authorizeAdmin(OpManage)`), admins see all.
- Admin rules (require `authorizeAdmin(OpManage)`, `authz.go:53`; org workspaces only; `Ready=false` → `ErrUnavailable`):
  - `GET  /api/workspaces/:id/admin/directory-scope` → hidden departments (name, people count) + limited subjects (role or
    department, name, people count).
  - `PUT|DELETE /api/workspaces/:id/admin/directory-scope/hidden/:deptId` → move Dir OA to Directory root + associate
    `Dept UA → Dir OA {read}` / reverse (back under parent Dir or Open).
  - `PUT|DELETE /api/workspaces/:id/admin/directory-scope/limited/roles/:roleId` and
    `.../limited/departments/:deptId` → `CreateAssignment(subject UA → DirectoryLimited UA)` / remove.
    Subject checks: `requireRole` (`authz.go:154`, custom roles only, never Owners/Members), `departmentInWorkspace`
    (`department.go:467`).
  - Each write announces `realtime.KindDepartmentChanged` / `KindRoleChanged`; the client already invalidates contacts on
    every workspace event (`frontend/src/lib/realtime.ts:122-123`).
- gRPC `GetDirectoryScope(workspace_id)` (caller from `grpcauth.CallerFrom`) in `backend/proto/workspace/workspace.proto`
  (rpcs :9-39); `make proto`. Frontend `proto:gen` only compiles `messaging/ws.proto` (security phases 02/03 notes) — not
  affected; state it in the PR.
- Dev demo data: `scripts/seed-contacts-demo.sh` (plan "Ghi chú dữ liệu demo"). Refuses to run unless the target is the
  local dev stack in fixed-OTP mode (`AUTH_FIXED_OTP_CODE`, `auth/internal/domain/otp_delivery.go:16-18`). Drives only real
  APIs through the Vite proxy: `POST /api/auth/otp/request|verify` (`auth/internal/rest/handler.go:39-40`) per person,
  `PATCH /api/me/profile` (display name, title), owner `POST /api/me/workspaces` (org), departments via
  `POST /api/workspaces/:id/departments` (tree Root→{Khối Công nghệ→{Đối soát, Nền tảng}, Khối Vận hành→{Đối soát},
  Pháp chế (to hide)→{Tuân thủ}}), invite + accept (`POST /workspaces/:id/members`, `POST /invitations/:id/accept`),
  department/role assignment, a custom role, one hidden department and one limited role via the new rule endpoints. Uses the
  old demo display names (Trần Minh Đức, Lê Quang Vinh, Nguyễn Thu Lan, Phạm Hải Yến, …) with `@example.test` addresses;
  never touches `a1000000-*` rows; writes no SQL; idempotent (skips what exists). No credential in the file.

## Architecture
- New `backend/services/workspace/internal/domain/directory_scope.go` (scope + rules), REST in new
  `internal/rest/directory_scope_handler.go`, gRPC method in `internal/grpc/server.go`. Transport → domain → store kept
  (`make check-layering`).
- Route reachability: `/api/workspaces/:id/admin/...` hits the workspace default target in `frontend/vite.config.js:55`
  (the re-dispatch block :58-87 only diverts drive, documents, channels/chat-*, contacts, asset*, calendar*), nginx and Traefik default workspace
  rules — verify, no new entry expected; if any table lists workspace paths explicitly, add it there (CLAUDE.md §5).

## Related Code Files
- Create: `backend/services/workspace/internal/domain/directory_scope.go`, `directory_scope_test.go` (fakes),
  `directory_scope_realpolicy_test.go` (`//go:build realpolicy`), `internal/rest/directory_scope_handler.go`,
  `directory_scope_handler_test.go`, `scripts/seed-contacts-demo.sh`
- Modify: `backend/proto/workspace/workspace.proto` (+ generated `*.pb.go` via `make proto`),
  `backend/services/workspace/internal/grpc/server.go`, `internal/domain/{department.go,members.go,roles_admin.go}`,
  `internal/rest/{admin_handler.go,handler.go,admin_people_handler.go}` (response field `hidden`, route registration)

## Implementation Steps (TDD, deny first, each deny paired with an allow on the same fixture)
1. Tests (real policy via `make test-realpolicy`; graph fixtures only through the harness write client; business rows SQL):
   - DENY: user of workspace V calls scope/departments/members/rules for W → 403; a person under `DeptPeers_A1` only (not
     under Members) → 403; plain member calls rule endpoints → 403; member of A1 does not get hidden H in departments/
     members/role counts; limited A1 gets only A1 subtree + owners, not A2 people; rule on Owners/Members UA → 404; dept of V
     in W's rule route → 404; Ready workspace with a broken lookup → 503; limited caller with the prohibition removed → 503.
   - ALLOW: admin sees all + `hidden`; custom manage role sees all; H1 member sees H; limited flag true for limited; owner
     placed in hidden H visible to an A1 member; pre-064 workspace (`SeedWorkspaceGraph`, no directory) → `Ready=false`,
     everyone visible, rule endpoints 503.
   - Hide → unhide restores the exact previous Dir parent (nested case: child under hidden parent).
   - Rule write then the member's `ListDepartments` in the same test excludes the department (EPP freshness).
2. Implement scope, filters, rules, gRPC; `make proto`.
3. Seed script: run against `make dev` twice (second run creates nothing); read-only SQL shows each seeded person under
   `<ws>_Members` and in `tenant_users` active; `make dev-stop` if the stack was started for this.
4. `make test s=workspace`, `make test-realpolicy`, `make check-layering`, `make build-check`, `make check-ngac`.

## Success Criteria
- Every member-readable workspace endpoint that names people or departments returns only scope-visible data.
- Rule writes visible to the PDP immediately (test above).
- Dev has a seeded org usable by phases 07–11 (owner, plain member, limited-role member, hidden-department member).

## Risk Assessment
- Departments filtered for non-admins narrows pickers used outside Quản trị, e.g. approval template department approver
  (`DepartmentPicker` via `StepBuilder`/`ApproverPicker`) (Med×Med): those callers need `manage` or template rights;
  limited users seeing fewer departments is intended. Listed for the phase 11 browser check.
- Batch size (Low×Low): 2×depts+2 objects per scope call; hundreds is fine for the batch PDP (`pdp_batch.go:20`).
- Department membership reads (Low×Med): a non-admin people read (`ListMembers`, `ListRoleSummaries`, `ListDepartments`, `GetDirectoryScope` roles) needs who sits directly in each department. Cost per request: one `GetChildren` per department (all of them when people with no department are visible, else only the visible ones), each read once and shared within the request, at most 8 in flight (`deptReadParallelism`), plus a constant handful (ancestors, owners, descendants of Directory, Members once). 50 departments = about 55 reads in ~7 waves, asserted by `TestDirectoryScope_ReadsStayBoundedWithManyDepartments`; an administrator or a workspace with no rules reads no department and, for role counts, the Members list once. Beyond a few hundred departments a per-workspace cache invalidated by the EPP event would be the next step; not needed now.
- Directory writes (Low×Med): hide, unhide, limit and department moves of one workspace take turns on an in-process mutex (`lockDirectory`). With several workspace replicas two writes on different replicas can still interleave; the writes are idempotent and a repeat of either converges, which is the same limit the invite rate limiter documents.
- Seed script mistaken for a prod tool (Low×Med): refuses unless fixed-OTP dev; lives under `scripts/` with a header saying so.
- Rollback: revert; graph rules remain inert.

## Carry-over from Phase 01 (Claude, 2026-10-17)
- The scope and the rules endpoints return an explicit `ready: false` (HTTP 200) for a workspace whose directory nodes do not exist yet, so the frontend can hide Quản trị › Danh bạ (decision Q4) without confusing it with a real outage (503).

## Implementation status (2026-10-18)
**Completed.** Nothing committed. No schema change. The demo org is seeded on dev (`NovaPay (demo danh bạ)`).

- [x] `directory_scope.go`: `DirectoryScope` (membership first; admin = `manage` on Directory; one batch of `read` on `DirectoryNoDept`, `DirectoryOwners` and each department's Dir/DirPeer; limited from the caller's ancestors with the prohibition required; owners and roles with direct holders; `Ready=false` only for a plain NotFound of `Directory_<ws>`), `DirectoryRules`, `Hide/UnhideDepartment`, `Limit/UnlimitRole`, `Limit/UnlimitDepartment` (manage, org workspaces only, subject checks, idempotent, EPP through the policy writes, realtime announce).
- [x] Filtered reads: `ListDepartments` (`Hidden` for admins only; a department whose parent is not visible is a root), `ListMembers` (REST and gRPC), `ListRoleSummaries` counts. `ListMemberDirectory` unchanged (manage only).
- [x] REST `internal/rest/directory_scope_handler.go` (`GET|PUT|DELETE /api/workspaces/:id/admin/directory-scope…`); gRPC `GetDirectoryScope` + `wire.DirectoryScope`; `make proto` run; `npm run proto:gen` run (messaging `ws.proto` only: no change). No new route entry in the Vite proxy / nginx / Traefik: `/api/workspaces/:id/admin/…` falls to the workspace default, checked by running the dev regexes over the four new paths.
- [x] `scripts/seed-contacts-demo.sh`: run twice against `make dev` (second run: every count of workspaces, departments, `tenant_users`, invitations, `ngac_nodes` of the workspace, all `ngac_assignments`, `ngac_associations`, prohibitions unchanged); each seeded person is a `tenant_users` row (active) with a direct edge under `<ws>_Members` (owner under `<ws>_Owners`, which sits under Members). Checked live through the Vite proxy: owner sees 7 people and every department, Pháp chế flagged hidden; Lê Quang Vinh 6 people, no Pháp chế; Nguyễn Thu Lan (limited role) 3 people (herself, Võ Thanh Tùng in her department, the owner) and only her `Đối soát`; Phạm Hải Yến (hidden department) 7 people; rule read 200 for the owner and 403 for everyone else.
- [x] Specs: `contacts-visibility` (scope, filtered reads, rules; status), `workspace-admin-authorization`, `admin-screens` (status), `docs/specs/README.md`.
- Tests that failed first (red for the right reason, then green): `TestRealPolicy_Scope_*` and `TestRealPolicy_Rules_*` (domain stubs returned "directory scope not built" for every call, so each assertion on scope, filtered reads and rules failed there), `TestDirectoryScope_*` (fakes: fail-closed, department visibility, limited needs its prohibition), REST `TestDirectoryScopeRoutes_*` / `TestListDepartments_HiddenFlag…` (404: routes absent), gRPC `TestGetDirectoryScope_*` (Unimplemented). Mutations that each turned a test red (15 + 2): limited never detected, members/role counts/departments unfiltered, parent not blanked, admin never admin, hide skips the peer OA, hide grants the wrong OA, unhide ignores the parent, unhide keeps the grant, limit skips the role check, rules skip the org check, missing prohibition tolerated, batch error tolerated, lookup error read as "no directory", owners not always visible, rule writes without an administrator check. One mutant survived the first fixture (hide granting the Peer OA instead of the Dir) and the test was strengthened (the department's own people read the hidden `Dir`).

### Deviations from the phase text
- **`ready:false` is a 200 on the read, a 503 on a write.** The phase said "rule endpoints 503" and its carry-over said "`ready:false` with 200": the carry-over wins for `GET …/directory-scope` (so the tab can be hidden), while PUT/DELETE on a workspace with no directory answer 503 (400 for a personal workspace, which also reads `ready:false`).
- **Wire shape of the rules read**: `{ready, hidden_departments:[{id,name,parent_id,member_count}], limited:[{kind:"role"|"department", id, name, parent_id, member_count}]}`; people are counted beneath a department (sub-departments included) and directly in a role. `hidden` on `GET /departments` is the explicit state (the department's Dir sits directly under Directory); a child of a hidden department is hidden by inheritance and the screen derives "Ẩn theo <phòng cha>" from the tree.
- **`GetDirectoryScope` and "only the auth service"**: it is in no `ServiceOK` list, so every service on its own identity is refused (Unauthenticated, tested for auth, messaging, workspace, calendar); the auth service gets answers by forwarding the person's identity. The identity token does not name the forwarding service, so "only auth may forward" cannot be enforced at the interface; it holds by the network boundary like every other internal method.
- **A department whose parent is not visible is a root in the caller's list** (`parent_id` blanked) instead of naming a department the caller may not see.
- **Seed script**: the seeded accounts cannot be verified by any API in fixed-one-time-code mode (by design the code proves no address), yet creating an organization and accepting an invitation both need a verified address. The script therefore runs one statement, `UPDATE users SET email_verified_at` for its own `@example.test` addresses, through `docker exec` into the dev Postgres container (documented in its header; no `ngac_*` write, no other table). The department tree has no artificial root department: the three groups are top-level.
- A `make dev` from a worktree starts its own compose project; the first attempt created and failed on empty containers and volumes of `nexus-enterprise-contacts_*`, which were removed again (the real infra was never touched); the stack was then started with `make -o dev-infra dev` and `.env.dev` linked to the main checkout's (read only, link removed afterwards), and stopped with `make dev-stop`.

### Review fixes (2026-10-18)
- **M1** department membership is read once per request, concurrently (at most 8 in flight), and shared by `ListDepartments` and the visible set (`loadDeptUsers`); only the visible departments are read unless people with no department are visible; `ListRoleSummaries` reads the Members list lazily and once, and not for an administrator's counts beyond that single read. Test: `TestDirectoryScope_ReadsStayBoundedWithManyDepartments` (50 departments: reads ≤ departments + a few, overlapping, ≤ 8 in flight; admin role counts read Members once and no department).
- **L1** directory writes of a workspace are serialised (`lockDirectory`: hide, unhide, limit, unlimit, department move); the move plans its directory mirror inside the lock, so it never works from a picture taken before a hide. Test: `TestRealPolicy_Rules_AMoveRacingAHideLeavesTheDepartmentHidden` (a hide launched at the move's first write; without the lock the move put the Dir back under Open).
- **L2** `Roles[].HolderNodeIDs` of `GetDirectoryScope` are filtered by the visible set inside the workspace service. Test: `TestDirectoryScope_RoleHoldersFollowTheVisibleSet`.
- **L3** hide/unhide with nothing to change write and announce nothing (`alreadyAsAsked`). Test: `TestHideDepartment_NothingToDoWritesAndAnnouncesNothing`, each no-op paired with the opposite request on the same fixture. The fake writer gained `RemoveAssociation`.
- **L5** seed script: the address is a psql variable (`-v email=…`, `:'email'`), never part of the SQL text (checked with a hostile address: `UPDATE 0`); it refuses unless `APP_ENV` (or the dev env file's `APP_ENV`) is `dev`/`development` (checked unset and `production`). Re-run against the live dev stack: no count changed, the same people see the same lists.
- Tests that failed first: the four new tests above (M1: no overlap and 2 Members reads; L1: Dir under `DirectoryOpen` instead of `Directory`; L2: both holders listed; L3: one write and one announcement each).
