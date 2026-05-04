## Why

The current auth system uses a flat identity model — one user, one NGAC node, one auto-provisioned workspace. This prevents multi-tenant scenarios where a user belongs to multiple organizations (workspaces), each with independent access control. For a Lark-grade collaboration platform, tenant isolation is a security foundation, not an optional feature. Without it, we cannot support cross-org collaboration, domain-based auto-join, or tenant-scoped identity (open_id).

## What Changes

- **BREAKING**: JWT claims add `tenant_id` and `session_id` fields. All services consuming JWT gain tenant context.
- Add `email` and `union_id` to `users` table as global identity fields
- Add `domain` to `workspaces` table for email-domain auto-join
- New `tenant_users` table mapping users to workspaces with tenant-scoped `open_id`, role (UI-only), and status
- New signup flow: email-based registration with domain matching (auto-join existing tenant or create new)
- New signin flow: returns list of tenants user belongs to + default tenant
- New switch-tenant API: re-issues JWT scoped to selected tenant
- NGAC graph per tenant: `TenantMember_{id}` and `TenantOwner_{id}` user attributes
- Backfill migration: existing workspace owners get `tenant_users` records automatically

## Capabilities

### New Capabilities
- `tenant-identity`: Global user identity (union_id) + tenant-scoped identity (open_id). Email-based login. Multi-tenant membership model with `tenant_users` table.
- `tenant-auth-flow`: Signup with domain auto-join, signin with tenant list, switch-tenant token re-issue. GET /me with tenant context.
- `tenant-ngac-init`: NGAC Policy Class per tenant, TenantMember/TenantOwner UA creation and user assignment on join/create.

### Modified Capabilities
_(none — no existing specs to modify)_

## Impact

- **Auth Service**: Major refactor — new store queries, domain methods, REST/gRPC handlers, JWT generation
- **All downstream services** (messaging, workspace, document, drive, asset): JWT claims struct gains `tenant_id` — backward-compatible (empty = legacy token)
- **Frontend**: Login page adds email field, post-login tenant picker for multi-tenant users
- **Database**: Migration 005 adds columns and table, backfills existing data
- **Proto**: `auth.proto` gains new RPCs and messages
- **Shared pkg**: `pkg/httputil/claims.go` updated with new fields
