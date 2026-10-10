# Phase 08 — Backend shared packages and layering: report

Date 2026-10-10 · branch `refactor/project-wide` · nothing committed, staged or otherwise written to the git index.

## Result

Every success criterion of the phase holds and is now checked by `make check-layering`
(`scripts/check-backend-layering.sh`, with a self-test, wired into the `docs` job of CI and into
`make verify`). The shard manager stays in the decision path (plan decision 4); no PDP code changed
beyond one swallowed error (below).

| Gate | Result |
|---|---|
| `make build-check` | 8/8 services build |
| `make check-ngac`, `make check-layering` (+ self-tests), `make check-docs`, `make fmt-check` | pass |
| `make test` on `ngac` | pass, zero skips (strict runner) |
| `make test` on `ngac_ci` | pass, zero skips |
| frontend `lint`, `typecheck:diff`, `test` (1113), production build | pass |
| `make lint` | fails in `auth`, `asset`, `drive`, all in test files / lines this phase did not write (ST1018, ST1023, U1000) — same before the phase |
| live stack via `make dev` | see "Live smoke" |

## What landed

**Shared packages (backend/pkg)**
- `bootstrap`: `Env`, `EnvAlias` (deprecation warning), `PolicyAddr()`, `InitLogger`, `ConnectDB`, `Shutdown{GRPC, Health, HTTP, Cancel, Cleanup}.Wait()`. `bootstrap/redisconn` holds Redis so only the three services that use it link the client.
- `grpcauth`: `ServerOptions` now always installs `Logging` then `Recovery` then the caller check (one chain, nothing parallel). `Dial(addr, service)` is the only way to open a client. `Internal(err)` is the generic-on-the-wire Internal status.
- `grpcutil.Status(err, ...Mapping)`: domain error → gRPC status; shared sentinels and per-service extras keep their message, everything else becomes `Internal "internal error"` whose cause stays in the error text for the logging interceptor.
- `policyclient`: `Check`/`CheckCaller`/`BatchCheck`/`BatchCheckCaller`, `Permissions.Has`; error or unrecognised decision is never an allow; a failed check is logged.
- `httputil`: `Internal(err)`, `ErrorHandler`, `NewEcho(service, mw...)` (request ID middleware, recovery, one error handler), `RequestID`, `LogInternal`. `MapDomainError` and `MapGRPCError` no longer return `err.Error()` for a 500; `MapGRPCError` also carries an ErrorInfo `reason`.
- One env name for the policy address: `POLICY_SERVICE_ADDR` (`bootstrap.PolicyAddr`). `POLICY_ADDR` is still read, with a deprecation warning, for one release. Updated: `docker-compose.yml`, `Procfile.dev`, `Makefile`, `.env.example`, `scripts/e2e_approval_test.sh`, `.serena/memories/backend/core.md`.
- Deviation from the brief: recovery/logging interceptors and `Dial` live in `grpcauth`, not `grpcutil`, because `grpcauth.ServerOptions` must install them and `grpcutil` imports `httputil` which imports `grpcauth` (a cycle otherwise). `grpcutil` holds the status mapper.

**500 sanitising.** A 500 is `{"message":"internal error","request_id":"…"}` (auth keeps its `code` envelope and gains `request_id`); the cause is logged under the same request ID, which is also `X-Request-ID`. Applies to the httputil mappers, approval/workspace/document/auth/asset/drive local mappers, and every gRPC→REST path; the local `mapGRPCError` ×4 are gone. Also fixed: a 404 body that carried the database error (`asset not found: getting asset: ERROR … SQLSTATE`), found in the live smoke.

**Layering (transport → domain → store)**
- drive: `internal/domain` (logic moved out of `grpc/server.go` + `sharing.go`, ~1450 lines), gRPC server is a 20-method adapter, REST calls the domain.
- asset: `domain.AssetService/AssetTypeService/AssetRequestService` (moved from three gRPC servers + `authz.go`); gRPC and REST adapters; refusals carry a `reason`.
- document: `internal/storage` (bucket naming, keys, signed-URL lifetimes, what "not uploaded" means) behind a thin gRPC server; the dead `PutObjectDirect` and the unused DB pool argument are gone. `texts` was already domain+store.
- workspace: REST calls `domain.Service`, never the gRPC server; shared converters in `internal/wire`.
- messaging: `notification_server.go` no longer runs SQL; `store/notifications.go` + `domain.NotificationService`, the gRPC server is an adapter.
- No `internal/rest` file imports its service's `internal/grpc`, and no SQL/pool remains in any `internal/grpc` or `internal/rest` (both enforced by the script).
- Domain tests: the existing adapter-level suites (drive 116 tests, asset, workspace, messaging …) were left untouched and run against the new adapters unchanged — they were the characterisation tests written before the move. New domain tests cover error classification, fail-closed authorisation and each new store operation.

**Transactions and swallowed errors** (each has a test that fails midway and shows no half state)
- messaging: channel + its `channel_members` rows (`InsertChannelWithMembers`); message + reply count + thread participant (`InsertMessage`); poll/task + announcing message (`InsertPollWithMessage`, `InsertTaskWithMessage`); `AddMember` withdraws the graph assignment if the cache write fails.
- drive: `ActivateFile` (publish + charge quota; a second confirm is refused instead of double-charging), `DeleteItemReleasingQuota`, `GetOrCreateQuota` (one statement, was a multi-statement query that always fell back), `CreateShare` no longer skips PC_Global on a failed lookup, `GetSharedWithMe` no longer returns a silently shorter list.
- approval: `Store.InTx` (a transaction carried on the context; every store method joins it, nested methods get savepoints). Create, Approve (decision + audit + step completion + next-step assignments) and Reject (decision + skip + close + audit) are each one change; audit is part of it, so the trail has no gaps.
- auth: new accounts are created already verified in one statement (was insert then update); OTP sign-in returns the tenant-lookup error instead of issuing a tenant-less token; a session that cannot be revoked on refresh refusal is logged.
- policy: `loadShard` no longer ignores a failed global-PC lookup (only "no rows" is fine).

## Per-service checklist

| Service | main on shared parts | policy check via `policyclient` | status/error mapping | 500 test | layering | tx / swallowed |
|---|---|---|---|---|---|---|
| policy (+ policy-read) | yes | n/a | `grpcauth.Internal` | `internal_errors_test` | n/a (gRPC only) | shard global-PC error |
| auth | yes | n/a | `grpcutil.Status`; envelope + `request_id` | `internal_errors_test` | already domain | verified create, tenants, revoke logged |
| workspace | yes | `domain/authz` | `grpcutil.Status`, `MapDomainError` | `rest/errors_test` | REST→domain, `wire` | — (compensation existed) |
| document | yes | `texts` | `MapGRPCError`, `grpcutil.Status` | `rest/errors_test`, `grpc/server_test` | `storage` domain | — |
| messaging | yes | `domain/service` | `domainError`→`grpcutil.Status` | `rest/notifications_test` | notifications moved out of transport | 5 transactional units |
| asset | yes | `authz`, servers | `mapError` (+reason) | `rest/screens_test` | `domain` services | NotFound vs failure |
| drive | yes | `domain/service` | `mapError` | `rest/errors_test` | `domain` | 4 store ops + sharing |
| approval | yes (+ `POLICY_ADDR` alias) | adapter | `grpcutil.Status`, `mapDomainError` | `rest/errors_test` | already domain | `InTx`, atomic audit |

Each service's tests were run green with `make test s=<svc>`-equivalent runs while migrating; the full suite then passed on both databases.

## Behaviour changes beyond the 500 body (all deliberate, all small)

- Document's legacy proxy now maps 409/401 from drive (it returned 500 for them).
- Asset: a failed lookup is 500 (was a 404 carrying SQL text); a missing row is still 404 with `<what> not found`.
- Messaging REST `/api/notifications*` works. It was wired with a nil store (every call a nil-pointer 500); it is now backed by the same service as gRPC, and marking read is scoped to the signed-in user.
- Drive: a double `ConfirmFile` is 409 (was a second quota charge); deleting a pending upload no longer releases quota it never took; drive and approval pools now use the shared 25/5 settings.
- Approval: an audit write that fails now fails the decision (it used to be dropped silently).
- Messaging gRPC Internal messages are now the generic text (was `"<op>: <db error>"`).

## Live smoke (real stack, `make dev`, dev OTP `999999`)

Logged in, then: auth `/api/me`; workspace list/get/members/folders; document texts; messaging channels, messages, reply thread, poll, task, notifications; asset types; drive root; approval tenant provisioning. Provoked 400/404 (bad body, unknown item, empty folder name) and 500 (NUL byte in an id) on asset, document, messaging and drive: bodies were `{"message":"internal error","request_id":"…"}` and the log line for each ID carried the SQLSTATE text. Graceful shutdown logged for all 8 services. Everything I started was stopped; infra containers were already up and left running. The two smoke users/workspaces remain in the dev DB.

## Concerns / for the reviewer

1. Drive and asset domains still speak the pb messages as plain data (as messaging's domain already does); only the errors are transport-free. Giving them their own types is a larger follow-up that changes every call site.
2. (Fixed in Review fixes.) `make dev-stop` killed the `go run` parent, not the compiled child, so servers survive it and the next `make dev` collides on ports. Pre-existing; not touched. The 5005x gRPC ports also sit inside Linux's ephemeral range and occasionally fail to bind. I stopped my processes by PID.
3. Read-path `_`-ignored errors remain by design (messaging reaction/pin/user-name enrichment, `strconv` query parsing); the script only polices store writes. A real "channel not found" in messaging is still an unclassified error, so it answers 500 (it did before).
4. Quota-exceeded and "item changed, retry" (drive) keep answering 500 over REST as before; they now say only "internal error". Mapping them to 413/409 is a one-line decision for the owner.
5. Mistake worth knowing: I ran `git stash` once while checking lint baselines and immediately restored with `git stash pop --index`. The index (the user's 158 staged deletions) and worktree were verified identical afterwards; no stash remains.
6. `CLAUDE.md`: edited only §2 (`make check-layering`) and §3 (shared-parts bullet, `pkg/*`). The user's AgentKit/skills edits are untouched. Spec: only `docs/specs/tenant-auth-flow` (the 500 body now carries `request_id`; there is no shared error spec).

## Review fixes

Each fix was test-first where a test could fail without it; I confirmed the approval race tests fail with the lock removed.

- **H1 approval concurrency.** `Store.LockRequest` (`SELECT status, current_step … FOR UPDATE`) is the first call in both `Approve` and `Reject` InTx blocks (`lockPending`); status is re-checked and the request and acting row are re-read under the lock, so every decision on one request runs one at a time and the Skip* statements always run under the same lock order. Real-DB tests: 12 requests × two concurrent approvals with `RequiredCount 2` all complete (stuck "pending" without the lock); 12 approve-vs-reject races finish without deadlock, each in one terminal state, a rejected one with nothing pending.
- **M1 AddMember.** `IsAssigned(target, UA)` is asked before `CreateAssignment`; only an edge this call created is withdrawn, and an unanswerable question counts as "existed". Test: re-adding an existing member while the cache write fails leaves the edge.
- **M2 `make dev-stop`.** Services and the frontend now start under `setsid` (recorded PID = process-group leader). `dev-stop` sends TERM to the group via `env kill` (the shell builtin cannot signal a group), waits up to 10 s, then KILLs; `make stop` (overmind path) also signals the group. Procfile and `make run` unchanged. Live: `make dev` → 17 service/frontend ports listening → `make dev-stop` → 0 listening, no leftover processes, all 8 services logged a graceful stop.
- **M3 script.** Rule 2 and a new rule 4b are parsed (comments and string contents blanked, call arguments matched across lines): `grpc.NewServer(opts...)` passes only if `opts` was assigned from `grpcauth.ServerOptions(`, and `rest.NewHandler(` is flagged in any `cmd/` when it receives a `New*Server`/`Serve*` result directly or through a variable. Rule 6 now also catches `500` by number in `NewHTTPError/JSON/String/…/apiError` and `status.New|Error|Errorf(codes.Internal|Unknown`. `configs()` takes a root and has a self-test. Self-test cases added for every bypass, including comment/string false positives. `make lint` now runs vet + staticcheck on `backend/pkg`, `ngac` and `testutil` (clean).
- **M4 specs.** New `notifications` spec (indexed in README); requirements added to `drive-permission-engine` (confirm idempotency, fixed 409 text), `contacts-documents-settings-screens` (document proxy passes 409/401 through), `approval-screens` (decision + audit atomic, request lock). `check-docs` passes.
- **L1** `DeleteItemReleasingQuota(ctx, id)` now reads and row-locks the subtree inside its transaction (`FOR UPDATE`, ordered by id), releases quota from that list, and returns the removed files for object cleanup. Tests: folder with an active and a pending file releases only the active one's bytes.
- **L2** `GetOrCreateQuota` is `INSERT … ON CONFLICT DO NOTHING` then `SELECT`; a test shows a read does not wait on a transaction holding the row.
- **L3** Storage errors: only `NoSuchKey`/`NotFound` becomes `ErrNotUploaded`/404 with the fixed messages `file not uploaded` / `object not found`; anything else is an unclassified error (log only, 500). Drive's `ConfirmFile` answers a fixed 409 only when the document service reports FailedPrecondition, otherwise a generic 500. Tests with host names in the errors assert nothing leaks.
- **L4** Dead `InsertChannel` removed. Drive `domain/sharing.go` and asset `domain` no longer import gRPC `status`/`codes`: "absent vs unreachable" is `policyclient.IsNotFound`, "document service refused" is `grpcutil.IsFailedPrecondition`. (Test files still use gRPC types to build stubs.)
- Also: `auth/internal/rest/errors.go` staticcheck ST1023 fixed.

Verification: `make build-check`, `make check-ngac`, `make check-layering` (+ self-test), `make fmt-check`, `make check-docs` pass; full `make test` passes with zero skips on `ngac` and `ngac_ci`; frontend lint, typecheck:diff, test (1113) and build pass. `make lint` still reports only findings that predate the phase, all in test files: auth `workspaces_test.go` (ST1018 ×2), asset `store/screens_test.go` (U1000), drive `grpc/authz_followups_test.go` (U1000 ×2). The stack was stopped by `make dev-stop` itself; nothing of mine is running. No index changes, no stash.

Status: DONE_WITH_CONCERNS
Summary: Shared packages, one policy-address env name, sanitised 500s with request IDs, domain layers for drive/asset/document-storage, REST→domain in workspace, SQL out of the messaging notification transport, and transactional multi-write paths are in, with tests and a CI-enforced script; all gates are green on both databases and the live stack behaved as specified.
Concerns: pb types still inside the drive/asset domains; read-path ignored errors and the 500s kept for parity (items 1, 3, 4 above); pre-existing `make lint` findings in three services' test files; `setsid` is Linux-only for `make dev` (item 2 above is fixed, see Review fixes).
