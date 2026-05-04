## Context

The NGAC platform uses a flat identity model: one `users` table with `{id, username, password, ngac_node}`, auto-provisioning a workspace on register. JWT contains `{user_id, username, ngac_node_id}` — no tenant context. All services read JWT via shared `pkg/httputil/claims.go`. The workspace service creates an NGAC Policy Class per workspace but has no formal tenant membership concept.

Target: Lark-grade multi-tenant identity where workspace = tenant, users can belong to multiple tenants, and every API request carries tenant context.

## Goals / Non-Goals

**Goals:**
- Global user identity (email + union_id) with tenant-scoped identity (open_id)
- Explicit `tenant_users` membership table with status tracking
- JWT carries `tenant_id` — every downstream service gets tenant context
- NGAC graph initialization per tenant (TenantMember/TenantOwner UAs)
- Email-domain auto-join for seamless onboarding
- Backward-compatible: old JWTs (no tenant_id) still work during migration

**Non-Goals:**
- SSO/SAML/OAuth integration (future work)
- Email invitation system (future work)
- Cross-tenant chat (separate change — depends on messaging refactor)
- Row-Level Security (RLS) in PostgreSQL (enforce via NGAC, not DB)
- Tenant billing/subscription management

## Decisions

### D1: Workspace = Tenant (1:1 mapping)
**Choice**: Reuse `workspaces` table as tenant entity. No separate `tenants` table.
**Alternatives**: (A) New `tenants` table with org hierarchy. (B) Multi-workspace per tenant.
**Rationale**: Current codebase already treats workspace as the unit of isolation (NGAC PC per workspace, channels scoped by workspace_id). Adding a separate layer creates complexity without user value today.

### D2: Email as primary login, username for display
**Choice**: Add `email` column to `users` (UNIQUE), keep `username` for display. Signin accepts email.
**Alternatives**: (A) Replace username with email everywhere. (B) Allow both email and username login.
**Rationale**: Email enables domain-based auto-join. Username remains for display/mention. Option B adds complexity to auth validation with minimal benefit.

### D3: Tenant-scoped identity via open_id
**Choice**: `tenant_users.open_id` (UUID) — unique per user-tenant pair. Included in API responses, NOT in JWT.
**Rationale**: Matches Lark's open_id model. JWT stays lightweight (tenant_id + user_id). open_id is used by external integrations to identify a user within a specific tenant context.

### D4: NGAC TenantMember/TenantOwner as UA under workspace PC
**Choice**: On tenant creation, create `TenantOwner_{tenant_id}` and `TenantMember_{tenant_id}` UAs under the workspace's existing PC. Assign users on join.
**Alternatives**: Create per-user UA per tenant.
**Rationale**: Reuses existing workspace NGAC structure. Owner/Member UAs mirror the existing `Owners/Members` pattern. Workspace service already creates PC + Owners_UA + Members_UA — we align naming.

### D5: Migration strategy — additive, backward-compatible
**Choice**: Add columns/tables, backfill from existing data. Old JWTs without `tenant_id` accepted (services fall back to empty string or first workspace).
**Rationale**: Zero-downtime deployment. Existing sessions continue working. Frontend can adopt tenant picker incrementally.

## Risks / Trade-offs

- **[Risk] Email uniqueness conflict**: Existing users have no email. → **Mitigation**: Email starts as nullable. Users set email on next profile update. Signin supports both username (legacy) and email (new).
- **[Risk] Workspace service already creates NGAC nodes**: Duplicated NGAC init logic between auth and workspace services. → **Mitigation**: Auth creates `tenant_users` record and assigns user to existing workspace UA nodes. Workspace service continues creating the PC/OA/UA structure.
- **[Risk] Token bloat**: Adding tenant_id to JWT increases token size. → **Mitigation**: tenant_id is a UUID (36 chars). Minimal impact.
- **[Trade-off] role column in tenant_users is redundant with NGAC**: We store `role` for UI display speed (avoid NGAC traversal for simple label). Authorization still goes through NGAC exclusively.
