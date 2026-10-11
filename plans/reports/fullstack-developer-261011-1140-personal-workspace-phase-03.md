# Phase 03 report: server enforcement (personal workspace)

Working tree verified after the resume: all edits present, nothing committed, git index untouched, frontend/ and deploy/ not touched by me (their diffs belong to the PWA agent).

## Design
- Store read `WorkspaceType(ctx, id)` (pool, public `workspaces` table, no tenant schema needed) in asset, approval, messaging.
- Domain guard before any NGAC check, fail closed: unknown workspace / type != "organization" -> access denied; DB error -> unclassified -> 500 "internal error".
- Personal -> "not available in a personal workspace" (no IDs). Asset also carries reason `personal_workspace` (ErrorInfo / REST `reason`).
- No graph change, no schema change, no migration, no proto change.

## Deviations from the brief
1. **document**: no approval path exists. Its REST says approval moved to Drive; Drive has no approval RPC; text status draft/active/archived is not an approval; the frontend has no "Gửi duyệt" call. Nothing to guard, no change to document. Request creation for documents goes through approval API (guarded).
2. **approval gRPC**: nobody calls it (no service dials approval). Only 5 of 13 RPCs are implemented (queue lists, audit); the other 8 answer Unimplemented. Implemented ones are guarded.
3. **approval REST needed a second spot**: personal workspaces have no approval schema, so `tenantSchemaMiddleware` answered 404 before reaching the domain. On schema-resolve failure it now asks the guard first, so personal -> 403; organization without schema -> same 404 as before.
4. **messaging**: the card does not go through approval; it is a message with `linked_entity_type=approval`. Added guard in `SendMessage` (before the write check) for that type only; DM uses caller's tenant; unknown -> refused.

## Coverage: asset (22 REST routes, 22 gRPC methods)
Guard `requireOrganization` (domain/workspace_type.go) on the workspace in the request, or the asset/type/request row for ID-based calls.

| Route | gRPC | Guard point |
|---|---|---|
| POST/GET /workspaces/:id/asset-types | CreateType / ListTypes | req.WorkspaceId |
| GET /asset-types/:typeId, PUT /asset-types/:typeId/schema | GetType / UpdateTypeSchema | type row workspace |
| GET /workspaces/:id/assets/summary, /assets/activity | GetSummary / ListActivity | `readableTypes` (single door) |
| POST/GET /workspaces/:id/assets | CreateAsset / ListAssets | type row / req.WorkspaceId |
| GET/PUT/DELETE /assets/:assetId | GetAsset / UpdateAsset / DeleteAsset | asset row |
| POST /assets/:assetId/transition, /assign; GET /transitions, /history | TransitionAsset, HandOverAsset, GetAvailableTransitions, GetAssetHistory | asset row |
| POST/GET /workspaces/:id/asset-requests | CreateRequest / ListRequests | type row / req.WorkspaceId |
| GET /asset-requests/:reqId | GetRequest | request row |
| POST /asset-requests/:reqId/approve, /reject, /assign | ApproveRequest, RejectRequest, AssignAsset | `loadForDecision` (request row) |
| POST /assets/:assetId/return | ReturnAsset | asset row |

## Coverage: approval (16 REST routes, 13 gRPC RPCs)
Domain guard (`RequireOrganization`; caller's tenant from verified claims/signed identity) in all 17 exported Service methods except `EventAudience` (runs only after an already-guarded decision; not a route). `BatchApprove` guards once up front (it swallows per-item errors, so otherwise personal would return empty 200).

| Route | Method |
|---|---|
| POST /admin/tenants/:id/provision | ProvisionTenantSchema (guards the named tenant) |
| POST/GET /approval/templates, GET/PUT /approval/templates/:id | CreateTemplate (also guards input tenant), ListTemplates, GetTemplate, UpdateTemplate |
| GET /approval/permissions | CanManageTemplates (refuses rather than false) |
| POST /approval/requests, /approve, /reject, /batch-approve | CreateApprovalRequest, Approve, Reject, BatchApprove |
| GET /pending, /history, /my-requests, /department-requests | GetPending, GetHistory, GetMyRequests, GetDepartmentRequests |
| GET /requests/:id, /requests/:id/audit | GetRequestDetail, GetAuditLog |
| (middleware) | schema-resolve failure -> guard first -> 403 |

gRPC: GetPending/GetHistory/GetMyRequests/GetDepartmentRequests/GetAuditLog guarded via domain; other 8 Unimplemented.

## Tests added (all deny + allow)
- asset/grpc `personal_workspace_test.go`: every one of 22 methods refused in personal (PermissionDenied, reason, no ID, **zero policy checks**) with a caller holding everything; same calls in organization get past the guard (reads succeed); reflection test that the table covers every RPC; fail-closed (unknown workspace, dead DB -> Internal).
- asset/rest `personal_workspace_test.go`: walks the real router, 22 routes -> 403 `personal_workspace`; organization routes never refused as personal, listing 200.
- approval/domain `workspace_type_test.go`: all 17 methods refused with a store that panics on anything but the type read; both-tenants rule for template writes; fail-closed matrix; reflection coverage; BatchApprove once-only.
- approval/rest: walks 16 routes, personal with and without schema -> 403; organization unchanged (200 with schema, 404 without).
- approval/grpc: 13 RPCs refused or Unimplemented in personal; organization answers.
- approval/store: `WorkspaceType` (real DB, personal flip, unknown, no schema).
- messaging/domain `approval_card_workspace_test.go`: personal refused and nothing stored; other messages/cards still send; organization allowed; DM follows signed-in workspace; unknown/empty workspace refused.
- Fixture fixes: approval fakes got a `WorkspaceType`; `scratchTenant` ctx carries the tenant caller.

## Specs
`asset-authorization`, `approval-screens` (server rule; this is the only approval spec), `tenant-identity`, `chat-cards` updated with the rule and scenarios. Plan row 03 and phase file marked completed.

## Gates (from clean run)
- go vet + go test: asset, approval, document, messaging all pass (TEST_DATABASE_URL=ngac_chat_mine).
- `make build-check` OK, `make check-layering` OK, `make check-ngac` OK.

## Notes
- The ID-based asset endpoints now answer the personal-workspace 403 before the NGAC check; a caller can therefore learn that an asset ID lives in a personal workspace (UUID, low risk).
- Phase 05 (convert) must provision the approval schema after flipping type; provisioning is refused while personal.

Status: DONE
Summary: Asset, approval (REST + gRPC) and the chat approval card refuse in personal workspaces (403/PermissionDenied) with deny and allow tests and specs; document has no approval path to guard. All gates green.
