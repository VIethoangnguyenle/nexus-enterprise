# Code review — personal workspace phase 05 backend (convert personal → organization)

Scope: workspace domain/store/rest conversion + details, approval tenant middleware, migration 046.
Frontend ignored. Read-only review.

## Gates (run by reviewer)
- workspace: go vet clean, `go test -count=1 ./...` 535 passed
- approval: go vet clean, `go test -count=1 ./...` 267 passed
- `make check-layering check-ngac`: pass
- Dev proxy: `/api/workspaces` defaults to :8181; the regex only re-dispatches drive/documents/channels/contacts/asset, so `convert-to-organization` reaches the workspace service. OK.

## Critical
None.

## High
None.

## Medium

M1. Auto-provisioning re-activates a schema that is not `active`.
`backend/services/approval/internal/rest/handler.go:110-123` together with `pkg/httputil/schema.go:51` and `data/migrations/007_tenant_schema_approval.sql:164-166`.
`Resolve` only matches `status = 'active'`. For a tenant whose row is `disabled`, `migrating` or `provisioning`, it returns ErrNoRows, so the middleware calls `provision_tenant_schema`. That function ends with `ON CONFLICT (tenant_id) DO UPDATE SET status = 'active'`. The result is that any member's approval GET re-enables a disabled tenant, or flips a tenant to active while it is being migrated. Nothing writes those statuses today, so this is latent. But the middleware now quietly overrides any future disable or migration control.
Fix: provision only when no `tenant_schemas` row exists at all. Answer a non-active row with 503/403 and do not provision. Alternatively, add an insert-only variant (`ON CONFLICT DO NOTHING`) for the implicit path.

M2. Schema-name collisions now hit organic traffic (the bug predates this change, but this change makes it more likely).
`007_tenant_schema_approval.sql:33`: `v_schema := 'tenant_' || LEFT(tenant_id, 8)`, and `schema_name` is UNIQUE.
Before this change, schemas were only created through the explicit provision endpoint. Now every organization gets one on its first approval call, including every converted personal workspace. The number of workspaces now grows with the number of users. When two workspace IDs share their first 8 hex digits (birthday odds are about 1% at around 10k organizations), the second tenant's INSERT fails. The function is one statement, so it rolls back. That tenant then gets a 500 on every approval call, permanently.
No data leaks, because the UNIQUE constraint aborts the statement before the tenant is registered. The cost is availability.
Fix (new migration): derive the name from the full ID, for example `'tenant_' || replace(p_tenant_id,'-','')` or a hash of it. Track this as follow-up if it is out of scope here.

M3. The owner check and the irreversible write are not serialised with owner changes.
`backend/services/workspace/internal/domain/workspace_conversion.go:34-55`.
`ownersOf` + `requireOwner` run outside `store.WithOwnerLock`, even though every other owner rule (RemoveMember, TransferOwnership, last-owner) uses that lock. Scenario: owner A is being demoted or removed by owner B. A's convert reads the Owners group just before the removal commits, then runs the UPDATE after it. A workspace that A no longer owns becomes an organization, which cannot be undone.
The window is small, but the operation is one-way.
Fix: run the owner read, the type check and `store.ConvertToOrganization` inside `s.store.WithOwnerLock(ctx, ws.ID, ...)`.

## Low

L1. `workspace_conversion_test.go:94-102`: `UnreadableOwnersRefuse` asserts only `assert.Error`. Use `assert.ErrorIs(err, domain.ErrAccessDenied)`, so the test proves a 403 and fail-closed behaviour rather than any error (a 500 would also pass).

L2. Error ordering: `authorizeAdmin` returns 404 for an unknown ID before any permission check. Any authenticated user can therefore probe whether a workspace ID exists. This matches every other admin route and the IDs cannot be guessed, so it is information only. 403 is correctly returned before 409: a non-owner on an organization gets 403, and a test covers this (`AlreadyAnOrganizationIsAConflict`, second half).

L3. The middleware provisions before any NGAC check. The tenant comes from the verified JWT claims, and `ProvisionTenantSchema` re-runs `RequireOrganization`, so a caller cannot provision another tenant or a personal workspace. A member who was just removed but still holds an unexpired token could trigger creation of that organization's schema. This is harmless: the call is idempotent and only works for organizations. Acceptable.

L4. `GET /details` now costs one extra SQL query (`ConversionRecord`), two policy RPCs (`FindUAByName` and `GetChildren`) and a directory lookup on every call. That is acceptable for a settings screen. If anything polls this endpoint, watch it.

L5. Migration 046: both columns are nullable with no default, so adding them is a metadata-only change with no table rewrite. The `users(id)` TEXT FK type matches, and `IF NOT EXISTS` makes it idempotent. There is no index on `workspaces.converted_by`, so deleting a user scans `workspaces` because of the `ON DELETE SET NULL`. That is negligible at the current size.

## Verified OK
- Authorization requires manage on the Mgmt OA and membership of the Owners UA (graph, via policyWrite). It fails closed when the owners cannot be read and when there is no caller. Deny tests cover manager, member, inviter, the owner of another workspace, an outsider, no caller, and an owner without manage.
- Atomicity: a single `UPDATE ... WHERE type='personal' RETURNING`, and the loser of a race gets ErrNotPersonal (409). A real-DB concurrency test covers this. The announce happens only after the write succeeds.
- Approval `RequireOrganization` reads the DB on every call (no type cache), so a converted workspace works immediately. A DB error other than ErrNoRows returns a 500 with no detail. A personal workspace is never provisioned: the middleware checks and `ProvisionTenantSchema` checks again.
- No graph writes, so no EPP path is needed. The handler takes the caller from the claims and ignores the body.

## Recommended actions
1. M3: wrap the conversion in `WithOwnerLock` (small change, the operation cannot be undone).
2. M1: on the implicit path, do not provision over an existing non-active row.
3. M2: schedule a migration to use collision-free schema names.
4. L1: tighten the test assertion.

## Unresolved questions
- Is `tenant_schemas.status = 'disabled'` intended to become an admin control? If so, M1 becomes High.

Status: DONE_WITH_CONCERNS
Summary: Authorization, ordering, atomicity and the migration are sound, and all gates pass. The concerns are three medium issues: conversion is not under the owner lock, implicit provisioning re-activates non-active schemas, and the 8-character schema-name collision now reaches organic traffic.
