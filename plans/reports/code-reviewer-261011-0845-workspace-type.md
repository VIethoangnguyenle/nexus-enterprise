# Code review: workspace type decided at creation (personal-workspace, phase 02)

## Scope
- Files: proto/workspace (+pb.go), workspace svc {store, domain, wire, grpc + tests}, auth svc {domain, store, rest + tests}, data/migrations/044, docs/specs/tenant-identity, frontend api/{workspaces,auth}.ts, hooks/useWorkspaceType (+test)
- Out of scope (ignored): frontend chat/spaces, messaging, other specs
- Evidence run:
  - workspace: build, vet, `go test ./...` 507 pass (DB-backed gRPC tests do run, they are not skipped)
  - auth: build, vet, 319 pass
  - `make check-layering` and `make check-ngac` pass
  - vitest useWorkspaceType 3/3 pass, eslint clean
  - 044 run twice inside a rolled-back transaction on `ngac_chat_mine`
  - dev DB `ngac` read only: marker present, 38 personal / 23 organization, no personal row breaks the rule

## Overall
The change is correct for all four acceptance paths. Validation runs before any graph or DB write, and a test proves it. The type travels through every listed read path. The backfill is correct, and running it again keeps a later conversion. No blockers. The remaining risk is that the DB column still defaults to `'personal'`. That default undercuts the claim that the type is "decided at creation", and it starts to matter once phase 03 denies Assets and Approvals in personal workspaces.

## Creation paths (grep-verified)
| Path | File:line | Type |
|---|---|---|
| OTP legacy first sign-in | auth/domain/provisioning.go:35 (via otp.go:297) | personal |
| Google, no `hd` | auth/domain/google.go:77 | personal |
| Google company domain | auth/domain/google.go:197 | organization |
| "Tạo workspace" | auth/domain/workspaces.go:88 | organization |
| Domain claim on an existing workspace | auth/store/identity.go:71 | UPDATE → organization (unchanged) |
| Workspace REST `POST /api/workspaces` | workspace/rest/handler.go:43 | 410 Gone, cannot create |
| Raw `INSERT INTO workspaces` without type | backend/testutil/testutil.go:104 + ~14 test files | DB default `'personal'` |

No seed, script or e2e creates workspaces: `scripts/e2e_approval_test.sh` has no workspace calls, and `data/` has no INSERT. `frontend/src/generated` only generates messaging/ws.ts, so the rule that both sides must regenerate a shared proto does not apply to workspace.proto. The TS types are hand-written and match the wire format.

## Critical
None.

## High
None.

## Medium

**M1. The column default `'personal'` survives, so any insert that omits the type silently becomes personal.** `data/migrations/009_user_profile_workspace_type.sql:20` sets `DEFAULT 'personal'`. 044 does not drop it (dev DB: `column_default = 'personal'::text`).
- **Deploy window:** `deploy.sh` runs migrate *before* `up` (docs/deployment.md:16), so the old workspace binary keeps serving while 044 runs and until the containers are replaced. The old binary inserts without `type`. A "Tạo workspace" in that window is stored as `personal`, and the backfill has already run (ledger), so nothing corrects it. Once phase 03 is in, that organization loses Assets and Approvals with no visible reason.
- **Tests:** `testutil.CreateWorkspace` (backend/testutil/testutil.go:104) and the ~14 raw inserts in asset, approval, drive and document tests all produce `personal` workspaces. Phase 03's server gate will turn those allow-path integration tests into 403s. Alternatively, the tests get "fixed" by hand-setting the type, which hides the default.
- **Fix:** add `ALTER TABLE workspaces ALTER COLUMN type DROP DEFAULT;` to 044. It is idempotent, so it is safe under `make db-migrate`. Give `testutil.CreateWorkspace` an explicit `'organization'`, or a type parameter. Optionally, for the deploy window, deploy so the old service fails loudly on a NOT NULL violation instead of mislabelling. Note: the spec line (tenant-identity/spec.md, "NOT NULL, CHECK-constrained") is true, but the schema still lets the type go unstated.

**M2. The auth fake accepts any type, so a future creation path that forgets the type passes the auth unit tests.** `backend/services/auth/internal/domain/fakes_test.go:333-340` stores `req.Type` without checking it. The real server refuses `""` (workspace/domain/service.go:118). `autoProvisionWorkspace` logs and swallows that error (provisioning.go:38-41).
- **Scenario:** someone adds a new sign-in or provisioning path and leaves out `Type`. Auth tests stay green. In production every new user on that path ends up with no workspace, and the only trace is a log line.
- **Fix:** make the fake return `status.Error(codes.InvalidArgument, …)` unless the type is `personal` or `organization`, matching the server's contract.

## Low

**L1. Old/new skew on the RPC contract during `up`.** A new workspace service with an old auth means `CreateWorkspaceRequest.type` is empty, so every first sign-in and "Tạo workspace" gets InvalidArgument until auth is replaced. The window is seconds, because compose recreates everything together. Still, it contradicts deployment.md:168, which asks for backward compatibility with the previous release. Accept it knowingly, or deploy auth first.

**L2. The once-only guard is keyed to a free-text column comment.** 044:18-25. If any later migration or DBA runs `COMMENT ON COLUMN workspaces.type …`, the next dev `make db-migrate` re-runs the full UPDATE and turns converted organizations back into personal ones (phase 05). Prod is safe because of the `schema_migrations` ledger. Fine for now. Add a line to the column comment itself warning not to change it, or check a dedicated marker.

**L3. The backfill heuristic edge cases follow the spec, but should be known:**
- A legacy OTP workspace whose `tenant_users` insert failed (tolerated at provisioning.go:44) has 0 active members, so it becomes `organization`. Verified: row `rv-e`.
- A user who deliberately named a workspace "X's Workspace" and is alone in it becomes `personal`.
- Dev DB has 1 organization named `%'s Workspace`. That is expected: 2+ members or a domain.

**L4. Tests lock in the default.** `auth/store/profile_test.go:153,165` expects `Type: "personal"` for workspaces inserted raw without a type, including a 2-member "shared" workspace. These assertions prove the column default, not the creation rule, and will need changing when M1 lands.

**L5. Dead field.** `store.Tenant.Type` is filled by `FindTenantByDomain` (auth/store/store.go:208-211), but no production code reads it. `TenantMembership.TenantType` is set only by `GetTenantUser`, and its comment says so. `ListTenantsByUser` and the sign-in `Tenants` list return `""`. Not a bug today, because the frontend reads the type from `/api/workspaces`. Drop the unused field or fill it everywhere.

**L6. Minor gaps in hook test coverage.** There is no case for an active workspace with no `type` field (an older server) or for a `?ws=` id that is not in the list. The code handles both (the `raw` check). The 'team' case already covers the "never personal" invariant.

## Repo rules (CLAUDE.md §5)
- **Deny test for the new branch:** present. `provisioning_test.go` "RefusesAMissingOrUnknownType" covers `"" / "team" / "Personal"`, asserts no graph write and no row. `grpc/server_test.go` "UnknownTypeIsInvalid" checks InvalidArgument.
- **Spec updated:** tenant-identity/spec.md, with scenarios. Accurate, apart from the M1 nuance.
- **Raw ID on screen:** no UI surface in this change.
- **Layering:** the type is read in the store, mapped in the domain, and serialized in wire/rest. `check-layering` passes.
- **NGAC identifiers:** the type is a business attribute, not a node or operation. `check-ngac` passes.
- **Schema applied:** dev DB carries the marker and counts. Test DB `ngac_chat_mine` has not had 044 applied (no marker, 0 workspaces). That is harmless.

## Acceptance criteria
| Criterion | Result |
|---|---|
| OTP legacy / Google without `hd` → personal | Pass: tests at auth workspace_type_test.go |
| CreateMyWorkspace / Google domain → organization | Pass |
| Missing or unknown type refused before any write | Pass: service.go:118, tested |
| Type on GetWorkspace, ListWorkspaces, `GET /api/workspaces[/:id]`, `/api/me/workspaces`, `/api/me` current_tenant.type | Pass: via authorizeMember→GetWorkspace, wire.Workspace, rest/workspaces.go:64, rest/handler.go:159 |
| useWorkspaceType never reports personal when unknown | Pass |
| Backfill rule | Pass: verified on synthetic rows |
| Re-run keeps a later conversion | Pass: verified (rv-a stayed organization on the second run) |

## Plan follow-ups (no plan edits made)
- The phase-02 success criterion is met, apart from M1. Recommend the lead resolve M1 (drop the default, fix testutil) before marking phase 02 done, because phase 03 depends on the type being correct.

## Unresolved questions
- Is the seconds-long RPC skew during `up` (L1) acceptable, or should auth deploy first?
- Should `testutil.CreateWorkspace` default to organization, or take a type parameter? Phase 03 deny tests will want both.

Status: DONE_WITH_CONCERNS
Summary: All acceptance criteria are met and verified by tests, a two-run migration exercise, and dev-DB reads. No blockers. The main concern is that the `DEFAULT 'personal'` column default is still in place (M1), so an insert that omits the type becomes personal without anyone noticing. Phase 03 needs that fixed first.
