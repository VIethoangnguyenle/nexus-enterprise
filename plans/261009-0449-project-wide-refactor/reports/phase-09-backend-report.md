# Phase 09 (backend half) — tests, dead code, large files: report

Date 2026-10-10 · branch `refactor/project-wide` · backend only (`frontend/` untouched). Nothing committed, staged or stashed; the index still holds exactly the user's 158 staged deletions. (One `git mv` slipped in while renaming a test helper; I reset those two paths straight away and checked the index again.)

## Gates

| Gate | Result |
|---|---|
| `make build-check` | 8/8 build |
| `make check-ngac`, `make check-layering`, `make check-docs` | pass |
| `make fmt-check` | pass |
| `make lint` | **clean** (the 3 services' pre-existing test-file findings are fixed) |
| `make test` on `ngac` | exit 0, 0 skips, 0 failures |
| `make test` on `ngac_ci` | exit 0, 0 skips, 0 failures |
| policy / messaging / approval, `-count=3` | pass (the new tests are not flaky) |

The phase file's "Key insights" were stale. Re-survey found: legacy password auth, drive `domain/`, document `PutObjectDirect`/`db`, workspace `DepartmentService`, asset `ListAssetRequests` stub all already gone or now real (the asset handlers are real endpoints). What was still true is below.

## 1. Dead code removed (and the evidence)

Evidence: `deadcode ./...` (no `-test`) per service, `staticcheck -checks U1000`, then grep for every non-test reference.

| Removed | Evidence it was unused |
|---|---|
| `auth/internal/auth.CheckPassword`, `HashPassword` (+ the `bcrypt` import; `golang.org/x/crypto` moved to `// indirect` by `go mod tidy`, offline) | `deadcode`: unreachable. Only caller of `HashPassword` was one test that seeded a legacy password column; it now stores a literal. |
| `workspace/internal/domain.IsNotFound/IsAccessDenied/IsAlreadyExists/IsInvalidInput` | `deadcode`: unreachable; zero references anywhere. |
| `auth/internal/grpc` stubs `SwitchTenant`, `GetMe`, `ListUserTenants` (returned `Unimplemented` "use REST") | The embedded `UnimplementedAuthServiceServer` answers `Unimplemented` identically, so the wire result is the same; `authpolicy_test` already pins the RPC set. The RPCs remain in the proto (see concerns). |
| `messaging` hub `handleLegacyJSON` (no-op) | It only logged. Text frames are now logged inline and dropped; same behaviour, one function fewer. |
| `policyclient.CheckCaller` | Referenced only by its own test (removed with it); `BatchCheckCaller`/`Check` cover the callers. |
| `provision.Creator.OnFail` | Zero references. |
| `policy/internal/ngac/export_test_helpers.go` (non-`_test` file shipped in the production build) | Renamed to `export_test.go`; `deadcode` listed `ExportCacheKey/ExportVersionScope` as unreachable from every main. |
| duplicate `nilStr` (drive `domain` and `store`) | One exported `store.NilIfEmpty`, used by the domain. |
| test-only leftovers behind the lint findings: asset `otherWorkspace`, drive `recWrite.assocByUA/createdNode`, auth ST1018 literals | `staticcheck` U1000/ST1018. |

Kept on purpose (referenced only by tests but they are the seams the adapter suites are built on): asset `NewAssetServer/NewAssetTypeServer/NewAssetRequestServer`, `grpcauth.ServiceFrom` (the read side of the service-identity feature, tested).

## 2. Phase-08 leftovers

- **Drive**: quota exceeded → `413 {"message","reason":"quota_exceeded"}`; "item changed, retry" → `409 {"message","reason":"item_changed"}`; a lock that cannot be had stays a generic 500; an unclassified error that merely mentions the quota stays 500 (test). Spec: `drive-permission-engine`.
- **Messaging**: unknown channel / message → 404 (it was an unclassified error → 500). The cause of the 500 was a single `err != nil || ch == nil` branch that also reported a database failure as "not found"; `loadChannel` now separates them, and the same fix is applied to poll, task and message lookups. Malformed `before` cursor → 400 (was silently an empty page). Non-numeric `limit`/`offset` on notifications → 400. Specs: new `chat-channel-integrity`, `notifications`.
- **Ignored read-path errors fixed**: `ListMembers` (policy failure returned an empty room), `EnrichMessagesWithMetadata` (reactions/pins failure sent the page without them; now returns the error), `ListPins` (message load), name look-ups (now logged), `UpdateChannel` (`%w` of a nil error). Remaining `_ =` are decode-or-default idioms (`hex.DecodeString`, JSON schema type asserts) and best-effort cleanup.
- `make lint`: clean (see gates).

## 3. Real defects the new tests exposed (fixed, each with a failing-first test)

1. **Cross-channel disclosure through pins.** `PinMessage` never checked that the message belongs to the channel; `ListPins` loaded any message by id. A member with write on channel A could pin a message id from channel B and read B's text. Now the pin is refused (400) and `ListPins` skips a pin whose message is elsewhere (covers rows written before the fix). Spec `chat-channel-integrity`.
2. **Cross-poll vote.** `VotePoll` authorised on the poll but inserted any option id; tallies are counted by option, so the vote landed in another channel's poll. Now 400 (`Store.PollHasOption`).
3. **Skipped row on every page.** Approval `ListHistory`, `ListMyRequests`, `ListByScopes` set `next_cursor` to the first row of the *next* page and then filtered strictly before it, so one request vanished at every page boundary. The cursor is now the last row of the page; a test walks all three lists and checks every row appears exactly once. Spec `approval-screens`. (This changes the cursor value, which is opaque to clients.)

## 4. Open finding, not fixed

**The L2 materialized access cache cannot work against the real schema.** `MaterializedAccess.Lookup/Store` read and write `ngac_materialized_access.workspace_id`, which no migration creates (`003_materialized_access.sql` has no such column). Every L2 call fails and is logged as a warning; decisions fall through to the graph, so it fails safe, but L2 is dead weight and `pip_materialized.go` is untested for that reason. Fixing it means a schema migration plus a change to what is served from L2, so it is the owner's call. I left it and only tested the working parts (`InvalidateByUser/ByObject`).

## 5. File splits (same package, no behaviour change; gopls imports, tests green after each)

| File | Before → after | New files |
|---|---|---|
| `approval/store/store.go` | 1141 → 190 | `tx.go`, `templates.go`, `assignments.go`, `audit.go`, `request_lists.go` |
| `drive/domain/service.go` | 932 → 411 | `files.go`, `items.go` |
| `messaging/domain/service.go` | 850 → 111 | `channels.go`, `dms.go`, `messages.go`, `members.go`, `authz.go` |
| `drive/store/store.go` | 797 → 381 | `shares.go`, `quota.go`, `delete.go`, `files.go`, `lookups.go` |
| `messaging/grpc/hub.go` | 773 → 181 | `hub_conn.go`, `hub_broadcast.go` |
| `asset/store/store.go` | 768 → 272 | `requests.go`, `transitions.go`, `types.go` |
| `auth/domain/service.go` | 715 → 126 | `tenants.go`, `users.go`, `provisioning.go` |
| `workspace/domain/service.go` | 706 → 279 | `members.go`, `roles.go`, `folders.go` |
| `approval/rest/handler.go` | 639 → 180 | `templates.go`, `decisions.go`, `lists.go` |
| `asset/domain/assets.go` | 578 → 318 | `transitions.go`, `summary.go` |
| `asset/rest/handler.go` | 540 → 347 | `asset_types.go`, `asset_requests.go` |
| `approval/domain/execution.go` | 540 → 349 | `steps.go` |
| `drive/domain/sharing.go` | 523 → 374 | `quota.go`, `channel_drive.go` |
| `auth/rest/handler.go` | 498 → 346 | `workspaces.go` |
| `auth/store/store.go` | 488 → 229 | `directory.go`, `identity.go` |
| `drive/rest/handler.go` | 485 → 368 | `sharing.go` |
| `messaging/grpc/hub_domain.go` | 483 → 410 | `presence.go` |

Left between 400 and 500 lines, each one cohesive (one responsibility): `workspace/domain/department.go` 499, `invitations.go` 449, `asset/domain/requests.go` 438, `document/texts/service.go` 435, `messaging/rest/handler.go` 434, `policy/grpc/write_server.go` 424, `policy/ngac/pip_shard_manager.go` 418, `drive/domain/service.go` 411, `messaging/grpc/hub_domain.go` 410, `workspace/rest/admin_people_handler.go` 406. Splitting those would separate things that change together.

## 6. Coverage

Method: `go test -coverpkg=./internal/... -coverprofile` per service, blocks merged across test binaries (a package is credited for lines any test in the service reaches; the per-package `go test -cover` figure under-reports the domain packages, which are exercised through the adapter suites, e.g. drive `domain` shows 2.2% in-package, 78% merged). Before/after are the same measure.

| Package | Before | After |
|---|---|---|
| approval/domain | 83.5 | 85.6 |
| approval/events | 64.9 | 64.9 |
| approval/grpc | 54.0 | 54.0 |
| approval/rest | 56.7 | 74.3 |
| approval/store | 43.6 | 80.2 |
| asset/domain | 87.8 | 87.8 |
| asset/events | 43.9 | 43.9 |
| asset/grpc | 100.0 | 100.0 |
| asset/rest | 67.7 | 69.8 |
| asset/store | 81.7 | 84.1 |
| auth/auth | 78.6 | 90.0 |
| auth/domain | 82.4 | 83.1 |
| auth/googleauth | 81.0 | 81.0 |
| auth/grpc | 20.0 | 92.6 |
| auth/rest | 85.8 | 85.8 |
| auth/store | 74.8 | 74.8 |
| document/grpc | 76.0 | 76.0 |
| document/rest | 85.0 | 85.0 |
| document/storage | 100.0 | 100.0 |
| document/texts | 92.7 | 92.7 |
| drive/domain | 78.2 | 78.1 |
| drive/grpc | 100.0 | 100.0 |
| drive/rest | 26.5 | 86.7 |
| drive/store | 67.2 | 79.2 |
| messaging/domain | 54.3 | 84.0 |
| messaging/events | 28.7 | 28.7 (handlers now tested, see below) |
| messaging/grpc | 78.8 | 78.9 |
| messaging/rest | 10.1 | 53.8 |
| messaging/store | 46.0 | 76.5 |
| policy/events | 27.3 | 27.3 |
| policy/grpc | 66.5 | 84.4 |
| policy/ngac | 72.6 | 81.6 |
| workspace/domain | 86.3 | 86.6 |
| workspace/grpc | 42.5 | 97.3 |
| workspace/rest | 96.4 | 96.4 |
| workspace/store | 61.5 | 76.5 |
| workspace/wire | 50.0 | 100.0 |
| pkg/* and `ngac` | 66–100 | unchanged (already ≥ 66) |

Still under 60%, and why: `policy/events`, `messaging/events` (mid-table), `asset/events`, `approval/events`, `approval/grpc` — the uncovered code is the Redpanda client wrapper (`NewProducer/NewConsumer/Close/run/produce`), which needs a broker; the decision logic in those packages (who is notified for which event) is tested. `messaging/rest` 54%: the remaining handlers are thin parse-and-delegate (channels, DMs, members). `approval/grpc` 54%: two list adapters untested.

New test files (all write paths and deny branches first): policy `write_server_invalidation_test` (InvalidateCache, LoadGraph, prohibition create/remove deny + restore with shard/Redis invalidation, operations, optional stores absent), `read_server_decisions_test` (allow, intersection across PCs, unknown user/object, prohibition named in the answer, batch), `epp_version_invalidation_test` (version tracker, coordinator scope, Redis flushed even when Postgres is down, operations/prohibitions stores); messaging `engagement_test` (reactions, pins, receipts, search, polls, tasks — allow and deny), `not_found_test`, REST `engagement_test`, `channel_errors_test`, event notifications; approval store lists/updates/paging, REST lists; drive REST routes (every route: argument pass-through, 401, error mapping, bad bodies, batch-access fail-closed), store quota/status/shares; workspace gRPC adapter (caller from context only, status mapping, no SQL text on the wire) and department store; auth gRPC (lookups, revoke/blacklist TTL, no-Redis is Unavailable not success); asset REST delete and store existence checks.

## Concerns / for the reviewer

1. **L2 materialized cache** is unusable (section 4). Decision needed: add the column by migration, or drop L2.
2. **Placeholder RPCs remain in the protos** and are served by the generated `Unimplemented*` or a no-op: workspace `ListPermissions` (returns empty) and `DeletePermission` (authorises, deletes nothing); auth `SwitchTenant/GetMe/ListUserTenants` (Unimplemented). Removing them changes the wire contract and means `make proto` plus `npm run proto:gen` on the frontend, which is being edited concurrently, so I left them. REST `ends_at` on poll creation is parsed and dropped; poll `is_multi` / `ends_at` are not enforced on vote. Not touched (no stated behaviour).
3. `policy` `RegisterOperations/CreateProhibition/RemoveProhibition` return `Unimplemented` when their store is nil; `main` always passes them, so the branch is configuration-only. Tested, kept.
4. Behaviour changes beyond the brief's three (all defects, section 3): pin/vote integrity (400), pagination cursor. The 413 status for quota is my choice between the 409/413 the brief allowed; one line to change in `drive/internal/rest/handler.go` if the owner prefers 409.
5. `go test` leaves some `skipping assignment during graph load` warnings from rows other suites orphan in the shared database; they predate this work and do not affect results.

## Review fixes

All six items test-first (each new test was run red against the old code where it could fail). Gates: `make build-check`, `check-ngac`, `check-layering`, `check-docs`, `fmt-check`, `make lint` clean; full `make test` exit 0 with 0 skips and 0 failures on `ngac` and on `ngac_ci`. Index untouched (still the 158 staged deletions), no stash.

1. **L2 materialized cache removed.**
   - Code: `MaterializedAccess` (`pip_materialized.go`) is deleted. `VersionTracker` and `ngac_graph_version` went with it: the version was read only by L2 (`GetVersion` in the cache) and everything else just incremented it, so keeping it would have been a write-only counter. `NewLayeredCache(rdb)` and `NewInvalidationCoordinator(cache)` now take only what they use; the workspace-id argument that existed only to pick a version scope is gone from `InvalidateForNodes` (writer, replica refresher, tests). `policy` and `policy-read` mains no longer build either object. Comments and `policy-decision-freshness` no longer mention L2.
   - What stays: the Redis decision cache and `CacheInvalidator` (targeted by user/object, full flush for a policy class), and the shard invalidation. The explanation is preserved in L1 (it is the stored `AccessDecision`).
   - Schema: `data/migrations/035_drop_materialized_access.sql` (`DROP TABLE IF EXISTS` for `ngac_materialized_access` and `ngac_graph_version`). Backups taken first (`pg_dump -Fc` of `ngac` and `ngac_ci` into the scratchpad). Applied with `make db-migrate` on `ngac` and by the same files in order on `ngac_ci`; both databases now lack the two tables. Note `make db-migrate` replays every file each time, so `003` recreates the two tables and `035` drops them again on each run; the ledger-based deploy runner applies `035` once.
   - Tests (`policy/internal/ngac/decision_cache_l1_test.go`): the second identical question never reaches the PDP and still carries the prohibition explanation; after `InvalidateForNodes(user)` that user's answer is recomputed against the changed graph while another user's cached answer survives; `InvalidateAll` drops everything; with no Redis nothing is cached and nothing panics; an error-derived decision is never stored. The invalidation tests in `policy/internal/grpc` (shard + Redis, LoadGraph, prohibitions) were rewritten for the cache-only coordinator.
2. **`backend/services/policy/migrations/` deleted.** Proof, checked against the pg catalog of both databases and against the files: every table, column, index and function it defines exists via `data/init.sql` + `data/migrations/` (`ngac_nodes/assignments/associations` and their six indexes in `init.sql`; `ngac_prohibitions` + index in `017`; `ngac_ancestors`, `ngac_check_access` in `003`). Three things did not: the two cache tables (dropped by item 1; `006`'s `workspace_id` and `idx_materialized_workspace` never existed anywhere) and **`ngac_operations`**, which only the service's runtime `InitSchema` created. That one is now `data/migrations/034_ngac_operations.sql` (table plus the same back-fill from associations). CLAUDE.md §3 sentence now reads "Schema lives in two places" (re-read just before the edit, only that sentence changed). `.serena/memories/backend/core.md` had the same stale claim and is fixed. **Not mine to edit:** `docs/deployment.md:114` still says "`backend/services/policy/migrations/` is not applied by any tool today" (now just a dead reference), and `data/migrations/017` comments on the old directory; for the deploy agent.
3. **messaging gRPC `GetChannel`** uses `domainError`: unknown channel `NotFound`, no read right `PermissionDenied`, database failure `Internal` with no host or SQL text. Test `get_channel_errors_test.go` (the existing deny test passed only because it asked for any error). Spec `chat-channel-integrity` gained the denied scenario.
4. **Approval keyset cursor.** `<RFC 3339 micro>|<id>` of the last row; lists ordered `created_at DESC, id DESC` (history `acted_at DESC, aa.id DESC`); next page `WHERE (ts, id) < ($ts, $id)`. A cursor not issued by the service is `ErrInvalidInput` (400; it was a database error before). Tests: seven requests with identical `created_at` and identical `acted_at` (decided in one transaction) walked three at a time through all three lists, each row exactly once; malformed cursors; REST maps the refusal to 400. Spec `approval-screens` updated.
5. **Drive `CreateFile`**: the quota check moved after `checkAccess(write)`. Test: a caller without write gets `PermissionDenied` on a full quota; a caller with write gets `ResourceExhausted`; neither leaves a row. Spec `drive-permission-engine` updated.
6. **Workspace placeholder RPCs**: `ListPermissions` / `DeletePermission` removed from the adapter (and `DeletePermission` from the adapter's domain interface and `domain.Service`, where it was only reachable from that adapter); the embedded `Unimplemented` answers. Test over a real gRPC connection asserts `Unimplemented` for both; the old deny test of the stub is replaced. No proto change.

Open for the coordinator: nothing blocks; the two stale mentions above (`docs/deployment.md`, a comment in migration 017) are in files the deploy agent owns or that must not be edited (an applied migration).

Status: DONE
Summary: L2 and its version counter are gone (code, tables on both databases, migration 035), `policy/migrations/` is deleted after proving its schema is covered (and `ngac_operations` folded into migration 034), and the messaging GetChannel codes, approval keyset cursor, drive deny-before-quota order and workspace Unimplemented placeholders are fixed with tests; all gates are green with zero skips on `ngac` and `ngac_ci`.
Concerns: Re-running `make db-migrate` recreates and re-drops the two L2 tables because it replays every migration; `docs/deployment.md` still mentions the deleted policy migrations directory.
