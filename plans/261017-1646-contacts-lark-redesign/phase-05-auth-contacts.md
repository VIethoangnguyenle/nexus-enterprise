---
phase: 5
title: "Auth: NGAC-decided contacts + name resolver"
status: pending
priority: P1
effort: "5h"
dependencies: [4]
---

# Phase 5: Auth — NGAC-decided contacts + name resolver

## Overview
`ListContacts` (`backend/services/auth/internal/domain/users.go:110`) stops trusting `tenant_users` alone: it asks
workspace for the caller's `DirectoryScope` (which also enforces graph membership) and filters by it. A separate resolver
returns name + avatar for people the caller meets in content, including hidden and removed people.

Current behaviour on main (verified): `ListContacts` checks only the caller's row `status='active'` (`users.go:114-120`);
the store lists `tenant_users` rows with `status='active'` (`store/directory.go:128-131`). The security plan deliberately
added no ListContacts graph gate (its choice 7); graph membership for auth lives in `requireActiveMember` /
`activeMemberRow` (`auth/internal/domain/membership.go:25,33`: row not `removed|disabled|invited`, `IsWorkspaceMember` over
the writer, NotFound/`IsNotMembersUA` → denied, other errors → `ErrUnavailable`).

## Requirements
- `GET /api/workspaces/:id/contacts` (route `auth/internal/rest/handler.go:72`, handler `workspaces.go:116`):
  - keep the cheap active-row check (`users.go:114-120`), then `wsClient.GetDirectoryScope` acting as the caller
    (`actingAs`, `auth/internal/domain/provisioning.go:28`, as `tenants.go:85` does). Workspace's `authorizeMember` makes the
    call graph-gated: gRPC `PermissionDenied` → 403; any other gRPC error → 503 (fail closed, never the unfiltered list).
    Do not re-implement membership in auth.
  - SQL filter (`store/directory.go:128` `ListContactsByWorkspace`): `ContactFilter.Scope` → admin or `Ready=false`: none;
    else `(tu.department_id = ANY($n) OR (tu.department_id IS NULL AND $m) OR (u.ngac_node = ANY($o) AND $p) OR u.id = caller)`
    with `$o` = `OwnerNodeIDs`, `$p` = `OwnersVisible` (owners always visible). Count uses the same predicate (counts never
    include hidden people). Search applies inside the scope. Rows stay `status='active'` (existing reader rule).
  - Response adds `department_id` (from `tu.department_id`), `roles: [{id, name, kind}]` (from scope roles where the
    person's node is a holder; owners → kind `owners`, no name), top-level `visibility: "all" | "limited"`.
  - New optional `department_id` filter; legacy `department` (name) filter kept.
- `POST /api/workspaces/:id/contacts/resolve` body `{user_ids?: string[], node_ids?: string[]}` (≤200 total) →
  `{people: [{user_id, ngac_node_id, display_name, avatar_url}]}`:
  - caller check: `s.requireActiveMember(ctx, wsID, callerID, callerNode)` (graph via writer; DRY with refresh/sign-in);
    `ErrUnavailable` → 503. Does **not** call workspace, so names keep working when workspace is down.
  - returns only users with a `tenant_users` row in this workspace of any status (`active`, `invited`, `disabled`,
    `removed` — CHECK from `062`); unknown IDs are silently omitted; never email/title/department. This is a deliberate
    exception to "readers filter active" (security phase 04 reader inventory) — say so in the store comment and the
    `contacts-visibility` spec.
  - node ids resolved through `users.ngac_node` (indexed, `data/migrations/018_users_ngac_node_index.sql:5`), tenant
    through `tenant_users (tenant_id, user_id)`.
  - display name follows the same "empty until profile saved" rule (`directory.go:132`).
  - Same `/contacts` prefix → covered by `frontend/vite.config.js:75`, `deploy/nginx/default.conf:116`,
    `docker-compose.yml:246` (Traefik).

## Architecture
- Domain: `ListContacts` gains the scope step; new `ResolvePeople(ctx, callerID, callerNode, wsID, ids)`; store gets
  `ResolvePeopleByWorkspace`. Workspace gRPC client already wired (`auth/cmd/main.go:82-88`, passed at :112).
- Threat model (resolver): IDs are random UUIDs the client only learns from content it was authorized to read; output is
  name + avatar, which the hidden-person card shows anyway. Scope fixed by Zane: any member of the workspace, any status.

## Related Code Files
- Modify: `backend/services/auth/internal/domain/users.go`, `internal/store/directory.go`,
  `internal/rest/workspaces.go`, `internal/rest/handler.go` (route), domain types `ContactInfo`/`ContactQuery` (`users.go:14,61`)
- Create: `backend/services/auth/internal/domain/contacts_visibility_test.go` (workspace client faked with a scope),
  `contacts_visibility_realpolicy_test.go` (`//go:build realpolicy`, resolver membership via the real writer),
  `internal/rest/contacts_resolve_test.go`

## Implementation Steps (TDD, deny first, each deny paired with an allow on the same fixture)
1. Tests (`TEST_DATABASE_URL` = `ngac_ci` on :5433, `REDIS_ADDR`/`TEST_REDIS_ADDR` for REST tests):
   - DENY: hidden person absent from list, from `search=` by name/email, from `total`; limited caller gets only scope rows
     and owners, not another department's person; non-member caller → 403 on both endpoints; caller with active row but no
     graph membership → 403 on resolve (real policy) and on contacts (scope `PermissionDenied`); `removed` caller → 403;
     resolve of a user of another workspace → omitted; scope gRPC `Unavailable` → 503 (not the full list); resolve never
     returns email.
   - ALLOW: admin gets all; `Ready=false` scope gets all; caller always gets own row; limited caller gets an owner placed in
     another department; resolve returns hidden + removed member name/avatar; `department_id` and `roles` present; owners
     role has kind `owners`.
2. Implement; `make test s=auth`, `make test-realpolicy`, `make check-layering`, `make build-check`.
3. Native dev with the seeded org (phase 04): `curl` both endpoints through Vite (`:5173/api/workspaces/<id>/contacts/resolve`)
   to prove proxy reach.

## Success Criteria
- `ListContacts` never returns a row the scope excludes (test enumerates every hidden user).
- Resolver p95 < 50 ms for 200 IDs on dev DB (single indexed query + one membership check).

## Risk Assessment
- Workspace unavailable → Danh bạ fails (Med×High): resolver does **not** depend on workspace, so names in chat keep working
  via resolve; Danh bạ shows its error state.
- Policy unavailable → resolver 503 (Low×Med): same dependency every authenticated path already has; client keeps cached names.
- Extra gRPC hop per contacts page (Low×Low): one call per request; pages of 500.
- `tenant_users.department_id` drift from the graph would mis-file a person (Low×High): rows follow the graph
  (`applyDepartment`), 064 reconciles existing drift (phase 03 step 7).
- Rollback: revert; old behaviour (tenant_users-only) returns — see plan rollback note.
