---
phase: 4
title: "Workspace: scope, filtered reads, admin rules API"
status: pending
priority: P1
effort: "6h"
dependencies: [3]
---

# Phase 4: Workspace — scope, filtered reads, admin rules API

## Overview
One domain function decides what a caller may see (`DirectoryScope`), built only from PDP decisions. Every
member-readable people/org read in the workspace service is filtered by it, admins get rule endpoints, and auth gets
the scope over gRPC.

## Requirements
- `DirectoryScope{Admin bool; Limited bool; VisibleDeptIDs set; NoDeptVisible bool; HiddenDeptIDs set (admin only);
  Roles []RoleHolders{ID, Name, Kind(owners|custom), HolderNodeIDs}}`.
  - `Admin` = `Check(caller, directory_oa_id, manage)`.
  - Non-admin: one `policyclient.BatchCheck(caller, [all dir_oa_id, peer_oa_id, directory_nodept_oa_id], [read])`
    (`backend/pkg/policyclient/policyclient.go:65`); dept visible if either its Dir or DirPeer is ALLOW.
  - `Limited` = caller reaches `directory_limited_ua_id` (ancestors; display only — drives "Chỉ phòng của bạn"; the
    decision itself is the PDP's).
  - Roles: Owners UA (kind owners, shown as "Quản trị viên") + custom roles (`classifyRole`, `roles_admin.go:56`); never
    Members. Holders = direct users (`roleHolders`, `roles_admin.go:84`).
  - Caller must be a member (`authorizeMember`, after F2 fix). Missing directory nodes → `ErrUnavailable` (503), never "see all".
- Filtered reads (non-admin):
  - `ListDepartments` (`department.go:127`): only visible departments; admins get all with `hidden`.
    Also fix its N+1 (one `GetChildren` per dept) only if trivial — not required.
  - `ListMembers` (`members.go`, REST `handler.go:47` + gRPC `grpc/server.go:94`): only visible people (+ self).
  - `ListRoleSummaries` (`roles_admin.go:114`, REST `GET /roles`): `MemberCount` counts visible holders only for non-admins.
- Admin rules (require `authorizeAdmin(OpManage)`, `authz.go:53`; org workspaces only):
  - `GET  /api/workspaces/:id/admin/directory-scope` → hidden departments (name, people count) + limited subjects (role or
    department, name, people count).
  - `PUT|DELETE /api/workspaces/:id/admin/directory-scope/hidden/:deptId` → move Dir OA to Directory root + associate
    `Dept UA → Dir OA {read}` / reverse (back under parent Dir or Open), update `directory_hidden`.
  - `PUT|DELETE /api/workspaces/:id/admin/directory-scope/limited/roles/:roleId` and
    `.../limited/departments/:deptId` → `CreateAssignment(subject UA → DirectoryLimited UA)` / remove.
    Subject checks: `requireRole` (`authz.go:131`, custom roles only, never Owners/Members), `departmentInWorkspace`
    (`department.go:467`).
  - Each write announces `realtime.KindDepartmentChanged` / `KindRoleChanged` so open clients refetch.
- gRPC `GetDirectoryScope(workspace_id)` (caller from `grpcauth.CallerFrom`) in `backend/proto/workspace/workspace.proto`;
  `make proto`. (Frontend `proto:gen` only compiles `messaging/ws.proto` — not affected; state it in the PR.)

## Architecture
- New `backend/services/workspace/internal/domain/directory_scope.go` (scope + rules), REST in new
  `internal/rest/directory_scope_handler.go`, gRPC method in `internal/grpc/server.go`. Transport → domain → store kept
  (`make check-layering`).
- Route reachability: `/api/workspaces/:id/admin/...` hits the workspace default target in `frontend/vite.config.js:55`
  (no regex re-dispatch matches `admin`), nginx and Traefik default workspace rules — verify, no new entry expected;
  if any table lists workspace paths explicitly, add it there.

## Related Code Files
- Create: `backend/services/workspace/internal/domain/directory_scope.go`, `directory_scope_test.go`,
  `internal/rest/directory_scope_handler.go`, `directory_scope_handler_test.go`
- Modify: `backend/proto/workspace/workspace.proto` (+ generated `*.pb.go` via `make proto`),
  `backend/services/workspace/internal/grpc/server.go`, `internal/domain/{department.go,members.go,roles_admin.go}`,
  `internal/rest/{admin_handler.go,handler.go,admin_people_handler.go}` (response fields `hidden`), domain errors mapping

## Implementation Steps (TDD, deny first)
1. Tests:
   - DENY: user of workspace V calls scope/departments/members/rules for W → 403; plain member calls rule endpoints → 403;
     member of A1 does not get hidden H in departments/members/role counts; limited A1 gets only A1 subtree; rule on
     Owners/Members UA → 404; dept of V in W's rule route → 404; directory nodes missing → 503.
   - ALLOW: admin sees all + `hidden`; custom manage role sees all; H1 member sees H; limited flag true for limited.
   - Hide → unhide restores the exact previous Dir parent (nested case: child under hidden parent).
2. Implement scope, filters, rules, gRPC; `make proto`.
3. `make test s=workspace`, `make check-layering`, `make build-check`, `make check-ngac`.

## Success Criteria
- Every member-readable workspace endpoint that names people or departments returns only scope-visible data.
- Rule writes visible to PDP immediately (test: hide then a member's `ListDepartments` excludes it in the same test).

## Risk Assessment
- Departments filtered for non-admins breaks pickers used outside Quản trị, e.g. approval template department approver
  (`DepartmentPicker` used by `StepBuilder`/`ApproverPicker`) (Med×Med): those callers need `manage` or template rights;
  limited users seeing fewer departments is intended. List affected screens in phase 11 browser check.
- Batch size (Low×Low): 2×depts+1 objects per scope call; hundreds is fine for the batch PDP (`pdp_batch.go`).
- Rollback: revert; graph rules remain inert.
