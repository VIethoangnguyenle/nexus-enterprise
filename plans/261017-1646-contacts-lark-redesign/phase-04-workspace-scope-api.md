---
phase: 4
title: "Workspace: scope, filtered reads, admin rules API, demo seed"
status: pending
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
- Seed script mistaken for a prod tool (Low×Med): refuses unless fixed-OTP dev; lives under `scripts/` with a header saying so.
- Rollback: revert; graph rules remain inert.
