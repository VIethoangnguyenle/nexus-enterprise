# Personal workspace phase 05 — backend

Status: DONE

## Files
- data/migrations/046_workspace_conversion.sql (new): converted_by (FK users, SET NULL), converted_at. Dev DB backed up to ~/backups/nexus/ngac-before-046-20261011-102111.sql.gz; applied to `ngac` (make db-migrate) and `ngac_chat_mine`; columns verified in both.
- workspace/internal/store/conversion.go (new) + conversion_test.go: `Ownership`, `ConvertToOrganization` (atomic UPDATE ... WHERE type='personal' RETURNING; 0 rows re-read -> ErrWorkspaceNotFound / ErrNotPersonal), concurrency test (1 of 2 wins).
- workspace/internal/domain: workspace_conversion.go (new), workspace_details.go (type/is_owner/owner_name/converted_*), service.go (store interface), errors.go (ErrAlreadyOrganization); tests workspace_conversion_test.go (new), authz_test.go (fakes).
- workspace/internal/rest/workspace_details_handler.go + handler_test.go: route, wire shape, 409 reason, details JSON.
- approval/internal/rest/handler.go + personal_workspace_test.go: middleware provisions on the fly.
- docs/specs/tenant-identity/spec.md, docs/specs/approval-screens/spec.md; plan.md row + phase-05 file.

## Behaviour
- Order: 404 (unknown ws) -> 403 (no manage) -> 403 (not owner) -> 409 already_organization. Non-owner on an org gets 403, not 409 (no type leak).
- Owner (revised per coordinator) = member of the workspace Owners UA, read via `ownersOf` (graph, policyWrite) - the same source TransferOwnership/RemoveMember/last-owner use (not tenant_users.role, which the workspace service never consults for this). A co-owner added later CAN convert (needs manage too); a manager who is not an owner cannot; unreadable owners -> 403. `is_owner` = caller in that group; `owner_name` = display names of current owners joined ", " via the directory (graph name fallback). Store now only has `ConversionRecord` (who/when) and `ConvertToOrganization`; workspaces.owner_id is no longer used. Announces KindUpdated(ws,ws) after write.
- PATCH /details response now also carries the new fields (same struct).
- Approval middleware: Resolve err not ErrNoRows -> 500; no schema + RequireOrganization fails -> mapped (403 personal); org -> ProvisionTenantSchema, Invalidate, Resolve again. Old "404 tenant schema not provisioned" test changed to expect 200; new test: personal 403 -> flip to organization -> POST /approval/templates 201, exactly 1 active schema (real DB, real store); 500 test with closed pool.
- NGAC: no graph change needed (verified: assets OA lazy, approvals tenant-schema). No graph writes, no EPP path touched.
- Proxy: no change. vite regex block, Traefik (PathPrefix /api/workspaces) and nginx (`location /api/workspaces`) all fall through to the workspace service for convert-to-organization.
- Audit: workspace service has no admin audit mechanism (grep "audit" empty); not invented. converted_by/converted_at are the record.

## Gates
go vet + go test ./... clean in workspace, approval, auth, asset. `make build-check check-layering check-ngac` all pass.

## Review fixes (code-reviewer-261011-1045)
- M3: `ConvertToOrganization` now reads the Owners group, re-reads the row's type and writes inside `store.WithOwnerLock`. Tests: owners read + write happen under the lock; an owner removed in the gap before the lock is refused (403) and the type is unchanged.
- M1: new `Store.EnsureSchema` (advisory lock per tenant; creates only when no `tenant_schemas` row exists; a non-active row -> `domain.ErrSchemaNotActive`, left untouched) and `Service.EnsureTenantSchema` (RequireOrganization first; added to the entry-point table). Middleware uses it; ErrSchemaNotActive -> the old 404 "tenant schema not provisioned". Tests: store (absent -> created, idempotent; disabled/migrating/provisioning untouched) and REST (disabled row -> 404, stays disabled).
- M2: migration 047 redefines `provision_tenant_schema` by reading back the live function definition and replacing only the schema-name line (no 120-line copy to drift from 007; idempotent): existing registry row keeps its `schema_name`, else `tenant_` + lower(non-alnum -> `_`) of the full id (error if >63). Dev DB backup `~/backups/nexus/ngac-before-047-20261011-103213.sql.gz`; applied to `ngac` (make db-migrate) and `ngac_chat_mine`; second apply is a no-op; verified in a rolled-back transaction on `ngac` (two ids sharing the first 8 chars -> distinct schemas, re-provision returns the same name, 0 rows left). Store tests: same-prefix ids both get schemas; a pre-registered tenant keeps its legacy name.
- Low: `UnreadableOwnersRefuse` asserts `ErrAccessDenied`.
- Specs: approval-screens (create only when no row, non-active -> 404, full-id names) and tenant-identity (owner lock) updated.
- Gates: go vet + go test -count=1 clean in workspace and approval; build-check, check-layering, check-ngac pass; dev workspace (:8181) and approval (:8186) restarted.

## Concerns
- Domain tests use fakes (no real policy); the real owner-lock race is covered by assertions on lock use, not a real two-process race.
- IDs differing only in letter case map to one schema name (UUIDs are lower-case, so none in practice).
- Serena tools were unavailable in this agent; edits done with Edit/Python on files I own.

Status: DONE
Summary: M1-M3 and the Low finding are fixed test-first-style with specs, migration 047 applied and verified; gates pass and the dev workspace and approval services are restarted.
