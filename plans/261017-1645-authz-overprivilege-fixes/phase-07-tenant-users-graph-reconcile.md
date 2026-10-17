---
phase: 7
title: "Graph-first tenant writes + graph-gated tenant-token mints (F7)"
status: pending
priority: P2
effort: "4h"
dependencies: [4]
---
# Phase 7: tenant_users agrees with the NGAC graph (F7)

## Overview
Decision 6: contacts and workspace switching agree with the graph; `ngac_*` is the truth. One mechanism (choice 7):
**graph-first writes** + **063 reconcile** (Phase 04) + **graph check at every tenant-token mint** (sign-in, switch,
refresh). No GetMe gate, no ListContacts gate (contacts reads the reconciled projection; email scoping belongs to
`261017-1646-contacts-lark-redesign`).

Drift sources today (origin/main):
- `joinTenant` (`auth/internal/domain/tenants.go:133-139`) inserts the row, then best-effort `assignTenantNGAC`, which
  requires `TenantMember_<ws>` (`provisioning.go:209-216`) — missing for 83/83 personal and 8/27 org workspaces in dev.
- strict `provisionTenant` inserts the row before assigning (`tenants.go:96-101`).
- `autoProvisionWorkspace` inserts the row directly (`provisioning.go:44-46`).
- `admit` skips `EnsureTenantUser` when the person is already a member (`invitations.go:424-444`).
- sign-in default tenant from `ListTenantsByUser` (row only): `google.go:198-214`, `otp.go:264-271`, `:322-328`;
  switcher list `ListWorkspaceSummaries` (`auth/internal/domain/workspaces.go:44`, store `directory.go:74-84`) row only.

## Requirements
- **Membership edge = `<ws>_Members`** (owner: `<ws>_Owners`, which is under Members). `assignTenantNGAC` assigns there
  directly; `TenantMember_<ws>`/`TenantOwner_<ws>` are additionally assigned only if they exist (optional, never an
  error). Resolve Members/Owners ids via `policyclient.MembersUA` / `FindNodeByName(ngac.OwnersUAName)`.
- **`joinTenant`** (graph first): for each target UA, `IsAssigned` first; `CreateAssignment` only where missing; then
  `InsertTenantUser`; on insert failure remove **only** edges this call created (pattern
  `messaging/internal/domain/members.go:38-46`); then `JoinWorkspaceChannel` (Phase 02). Lenient callers (domain/Google
  join, sign-in provisioning) log the error and continue with a nil tenant; sign-in never fails because of it.
- **strict `provisionTenant`** (`tenants.go:96-101`): order reversed — assign (owner edge already exists from
  `CreateWorkspace`, `workspace/.../service.go:193`; IsAssigned → no-op) then insert; failure undoes the workspace as
  today.
- **`autoProvisionWorkspace`** (`provisioning.go:28-49`): uses `joinTenant(…, "owner")`.
- **`admit`**: always `EnsureTenantUser` (reactivates only `removed`, Phase 04); undo only an edge this call created.
- **Token mints gated** with `requireActiveMember` (Phase 04):
  - sign-in: `signinResultFor` (`google.go:198`) and `otpResultFromExistingUser` / the new-user result builder
    (`otp.go:264,322`) filter tenants through it before `selectDefaultTenant`; policy error → soft fallback:
    tenant-less token, sign-in succeeds.
  - switcher: `ListWorkspaceSummaries` result filtered the same way; policy error → the current tenant (already
    validated by the caller's token) plus a logged warning, never the unfiltered list (fail-open not allowed).
  - `SwitchTenant` (`tenants.go:148-158`): row active **and** graph member; error → 503.

## Architecture
```
join: IsAssigned? ─no─> assign <ws>_Members|Owners (+TenantMember* if exists) ─> insert row ─✗─> undo only new edges
mint (sign-in / switch / refresh): ListTenantsByUser ∩ requireActiveMember ─> default tenant | tenant-less
truth: ngac_assignments ; tenant_users = projection (063 reconciles once)
```

## Related Code Files
- Modify: `auth/internal/domain/tenants.go` (joinTenant, provisionTenant, SwitchTenant), `auth/internal/domain/provisioning.go`
  (assignTenantNGAC, autoProvisionWorkspace), `auth/internal/domain/google.go` (signinResultFor), `auth/internal/domain/otp.go`
  (result builders), `auth/internal/domain/workspaces.go` (switcher filter), `workspace/internal/domain/invitations.go` (admit)
- Create: `auth/internal/domain/join_tenant_order_realpolicy_test.go`, `auth/internal/domain/signin_tenant_gate_realpolicy_test.go`,
  `workspace/internal/domain/admit_directory_realpolicy_test.go` (tag `realpolicy`)
- Spec: `docs/specs/tenant-identity/spec.md` (tenant_users is a graph projection; tokens minted only for graph members),
  `docs/specs/tenant-auth-flow/spec.md` (sign-in default tenant)

## Implementation Steps
1. Deny tests first, paired allows: OTP sign-up into a personal workspace with no TenantMember UA → row + Members edge
   exist, refresh works; Google `hd` join into an org without TenantMember → joined, sign-in succeeds; pre-existing
   Members edge + insert failure → edge survives; new edge + insert failure → edge removed; strict provisionTenant with
   failure → workspace undone, no row. Sign-in with an active row but no graph membership → that tenant not default and
   not listed; policy down at sign-in → tenant-less token, 200. SwitchTenant to a drifted tenant → 403.
   `admit` of an already-member without row → row active.
2. Implement.
3. `make test s=auth|workspace`; `make test-realpolicy` (REDIS_ADDR, TEST_DATABASE_URL); `make check-layering`.

## Success Criteria
- No path writes an active row before the graph edge (`grep -n "InsertTenantUser\|EnsureTenantUser"` reviewed in PR).
- After 063 + this phase, dev queries stay at 0 drift after a sign-up, a domain join, an invite accept and a removal.

## Risk Assessment
| Risk | L×I | Mitigation |
|------|-----|------------|
| Sign-in coupled to policy availability | M×M | soft fallback to tenant-less token |
| Undo removes a pre-existing edge | L×H | IsAssigned-before-assign; test |
| Workspaces without TenantMember | H×M (today) | no longer required |
| Switcher shows only the current workspace during policy outage | M×L | logged; next load retries; never fail-open |

## Rollback
Code-only; revert. No migration in this phase (062 and 063 belong to Phase 04).
