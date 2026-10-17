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
workspace for the caller's `DirectoryScope` and filters by it. A separate resolver returns name + avatar for people the
caller meets in content, including hidden and removed people.

## Requirements
- `GET /api/workspaces/:id/contacts` (route `auth/internal/rest/handler.go:72`, handler `workspaces.go:116`):
  - keep the membership check as left by the security plan (`261017-1645-authz-overprivilege-fixes` phase 07:
    `tenant_users.status='active'` AND graph membership under the workspace Members UA, via its shared helper; do not
    re-implement it here), then `wsClient.GetDirectoryScope` acting as caller
    (`actingAs`, as in `tenants.go:73`). gRPC error → 503 (fail closed).
  - SQL filter (`store/directory.go:128` `ListContactsByWorkspace`): `ContactFilter.Scope` → admin: none; else
    `(tu.department_id = ANY($n) OR (tu.department_id IS NULL AND $m)) OR u.id = caller`. Count uses the same predicate
    (counts never include hidden people). Search applies inside the scope.
  - Response adds `department_id` (from `tu.department_id`), `roles: [{id, name, kind}]` (from scope roles where the
    person's node is a holder; owners → kind `owners`, no name), top-level `visibility: "all" | "limited"`.
  - New optional `department_id` filter; legacy `department` (name) filter kept.
- `POST /api/workspaces/:id/contacts/resolve` body `{user_ids?: string[], node_ids?: string[]}` (≤200 total) →
  `{people: [{user_id, ngac_node_id, display_name, avatar_url}]}`:
  - caller must be an active member; returns only users with a `tenant_users` row in this workspace of any status
    (`active` or `removed` after F5) — unknown IDs are silently omitted; never email/title/department.
  - display name follows the same "empty until profile saved" rule (`directory.go:132`).
  - Same `/contacts` prefix → covered by `vite.config.js:75`, `deploy/nginx/default.conf:111`, `docker-compose.yml:246`.

## Architecture
- Domain: `ListContacts` gains scope step; new `ResolvePeople(ctx, callerID, wsID, ids)`; store gets
  `ResolvePeopleByWorkspace`. Workspace gRPC client already wired in `auth/cmd/main.go:112`.
- Threat model (resolver): IDs are random UUIDs the client only learns from content it was authorized to read; output is
  name + avatar, which the hidden-person card shows anyway. Stricter "shares content" check = plan Q3.

## Related Code Files
- Modify: `backend/services/auth/internal/domain/users.go`, `internal/store/directory.go`,
  `internal/rest/workspaces.go`, `internal/rest/handler.go` (route), domain types `ContactInfo`/`ContactQuery`
- Create: `backend/services/auth/internal/domain/contacts_visibility_test.go`, `internal/rest/contacts_resolve_test.go`

## Implementation Steps (TDD, deny first)
1. Tests (DB-backed, `TEST_DATABASE_URL` :5433, `REDIS_ADDR` for REST tests; workspace client faked with a scope):
   - DENY: hidden person absent from list, from `search=` by name/email, from `total`; limited caller gets only scope rows;
     non-member caller → 403 on both endpoints; resolve of a user of another workspace → omitted; scope gRPC error → 503
     (not the full list); resolve never returns email.
   - ALLOW: admin gets all; caller always gets own row; resolve returns hidden + removed member name/avatar;
     `department_id` and `roles` present; owners role has kind `owners`.
2. Implement; `make test s=auth`, `make check-layering`, `make build-check`.
3. Native dev: `curl` both endpoints through Vite (`:5173/api/workspaces/<id>/contacts/resolve`) to prove proxy reach.

## Success Criteria
- `ListContacts` never returns a row the scope excludes (test enumerates every hidden user).
- Resolver p95 < 50 ms for 200 IDs on dev DB (single indexed query).

## Risk Assessment
- Workspace unavailable → whole Danh bạ and every name in the app (usePeople) fail (Med×High): resolver does **not**
  depend on workspace, so names in chat keep working via resolve; Danh bạ shows its error state.
- Extra gRPC hop per contacts page (Low×Low): one call per request; pages of 500.
- Rollback: revert; old behaviour (tenant_users-only) returns — see plan rollback note.
