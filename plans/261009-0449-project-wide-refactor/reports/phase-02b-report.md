# Phase 02b report — gRPC caller identity

Date 2026-10-10. Branch refactor/project-wide. Nothing staged or committed.

## Design
- `backend/pkg/grpcauth` (new): `Caller{UserID,NGACNodeID,TenantID}`, `WithCaller`/`CallerFrom`/`ServiceFrom`,
  metadata keys as constants, `ClientInterceptor(service)`, `ServerInterceptor(ServerPolicy)`, `HealthExempt()`.
  - Client: strips any caller metadata already on the outgoing ctx (no smuggling), forwards the ctx caller; with no
    caller sends `x-service-name` instead.
  - Server: complete caller (user id AND node id) -> admitted, caller put on ctx. Else `Exempt` method -> admitted,
    else `ServiceOK` method + service name -> admitted, else `Unauthenticated`.
- `httputil.SetClaims` now also puts the verified caller on the request ctx, so every REST handler that passes
  `c.Request().Context()` downstream forwards it with no per-handler code (and in-process REST->server calls see it).
- Unary only; no streaming RPCs exist.
- Handlers read `grpcauth.CallerFrom(ctx)`; body caller fields are ignored (metadata wins by construction).
- Removed the in-process workarounds: `asset/internal/caller`, `drive/internal/caller` (UpdateQuota), and
  `workspace/internal/domain` `WithRequester/RequesterFrom`. These RPCs now work over the network.
- Scope was wider than "47 user_ngac_node_id": also `requester_ngac_node_id`, `inviter_ngac_node_id`,
  `current_owner_ngac_node_id`, `sender_id`/`sender_ngac_node_id`, approval `user_node_id`, notification `user_id`,
  and the paired `user_id`. 93 proto fields marked `[deprecated = true]` (no removal/renumbering).
  Policy's `user_node_id` is the subject of a question, not the caller; left alone.
- REST handlers (asset, drive, document, workspace) no longer populate the deprecated fields (staticcheck SA1019
  in `make lint` would fail otherwise). Drive `rest/handler.go`: only deleted the field lines and turned unused
  `claims, err :=` into `_, err :=`.

## Wiring (all 8 services)
Client interceptor on every `grpc.NewClient` (approval, asset, auth x3, document, drive, messaging x3, workspace x3).
Server interceptor on every server: policy (+ policy-read binary), auth, workspace, messaging, drive, document,
asset, approval.

## Exemptions (everything else needs a user caller)
| Server | Method | Mode | Reason |
|---|---|---|---|
| all 8 | grpc.health.v1.Health/Check | no identity | docker grpc_health_probe |
| auth | Register, Login, Signup, Signin | no identity | request carries the credentials; no caller exists yet |
| auth | IsTokenRevoked | service name | bare-jti validity check by services with no user session (no caller today) |
| policy | PolicyWrite/CreateNode, PolicyWrite/CreateAssignment, PolicyRead/FindNodeByName | service name | auth provisions user/tenant nodes at signup/sign-in before any token |

Policy `InitSchema/LoadGraph/RegisterOperations/InvalidateCache`: no network callers; not exempted.

## Per call site (no-user paths)
- auth signup/signin/Google/OTP -> workspace.CreateWorkspace, messaging.CreateChannel: auth has just authenticated
  the user, so it calls as that user (`actingAs` = `grpcauth.WithCaller`). Downstream servers authorize the new user
  normally. Test: `TestSignup_DownstreamRPCsCarryTheNewUserAsCaller`.
- auth -> policy CreateNode/CreateAssignment/FindNodeByName at signup: service identity (table above).
- messaging WebSocket hub subscribe check (background ctx): now builds the caller from the session's verified JWT
  identity (`authorizeSubscribe(channelID, grpcauth.Caller)`).
- Kafka consumers (policy, approval, messaging) make no gRPC calls: nothing to decide.
- messaging/workspace -> drive.CreateDriveForChannel, drive -> document, document REST -> drive: always inside a
  user request ctx, forwarded; no exemption.

## Tests
- pkg/grpcauth: 11 tests (round trip, incomplete caller, missing -> Unauthenticated, exempt, service-only, forged
  outgoing metadata overwritten/stripped). httputil: SetClaims puts caller on ctx.
- `testutil.ServeGRPC`: real loopback server with the production interceptors for over-the-wire tests.
- Per server, missing metadata -> Unauthenticated and body-A/metadata-B -> decision for B (deny AND allow cases):
  asset (4), drive (4, incl. UpdateQuota over network), workspace (3), messaging (4), approval (2, own tests, no
  testify dep), policy (3, exemptions), auth (4, exemptions), document (missing -> Unauthenticated; no decision to
  split since it does no authz).
- Existing server tests moved from body fields to ctx callers.
- `make build-check` OK. `make test` (strict script, all services + shared) exit 0, no skips. `go vet` and
  `staticcheck` clean in all 8 services; docs-drift script passes.
- `make proto` run. `cd frontend && npm run proto:gen` run: TS output is generated only from `messaging/ws.proto`,
  which I did not change, so zero diff under `frontend/src/generated`.

## Frontend verification (not mine)
`npm run typecheck:diff` and `npm run build` currently FAIL, from the concurrent drive UI work
(`useDeleteItem` not exported by `src/hooks/useDrive.ts`; new type errors only in drive/* files). proto:gen changed
no frontend file, so these cannot come from this phase. Re-run once the drive work lands.

## Live smoke (stack was up; reused)
Restarted only the 8 backend services (new code), frontend untouched; `.dev-pids` rewritten with the new PIDs.
Gotcha: approval's gRPC port 50058 was held by workspace's own outbound Postgres connection (ephemeral port range);
workspace had to be restarted to free it. Pre-existing hazard of the dev setup, not related to this change.
Fresh signup via REST, then with that JWT:
- signup 200 (auth -> policy via service identity; workspace + messaging as the new user)
- signin 200
- workspace GET 200; drive root/quota/create-folder 200/200/201; documents list (document REST -> drive gRPC) 200
- channels list 200, create channel 201 (messaging -> policy + drive gRPC), send message 201, members 200, dms 200
- asset types list 200 (previously denied over network), create type 201, assets/requests 200
- drive file create 201 (drive -> document gRPC)
- raw grpcurl without metadata (asset, drive, workspace, policy write+read, approval, messaging, document): all
  `Unauthenticated: caller identity required`, even with a spoofed `user_ngac_node_id` in the body.
- Health/Check on all 8 gRPC ports: SERVING.
Smoke user `smoke1791570521@example.org` and its workspace/channel remain in the dev DB.
GET /api/notifications on messaging returns 500 (nil `notifSt` in `rest/handler.go:350`): pre-existing, file not touched.

## Files
New: backend/pkg/grpcauth/{grpcauth.go,grpcauth_test.go}, backend/pkg/httputil/claims_test.go,
backend/testutil/grpcserver.go, policy/internal/grpc/authpolicy{,_test}.go, auth/internal/grpc/authpolicy{,_test}.go,
caller_identity_test.go in asset/drive/workspace/messaging/approval/document grpc.
Modified: pkg/httputil/claims.go; 6 proto + generated .pb.go; all 8 cmd/main.go (policy-read too); asset/drive/
workspace/messaging/approval grpc servers; messaging hub.go; auth domain service.go; REST handlers (asset, drive,
document, workspace); affected tests; docs/specs/resource-pep-coverage/spec.md.
Deleted: asset/internal/caller, drive/internal/caller, workspace domain WithRequester/RequesterFrom.

## Spec
docs/specs/resource-pep-coverage/spec.md: new requirement "Caller identity on gRPC comes from metadata" with the
exemption table and 4 scenarios.

## Not done / follow-ups
- Service-to-service authentication (mTLS/internal token) out of scope: metadata is unsigned, so anyone reaching a
  gRPC port can claim any identity; `x-service-name` attributes, it does not authenticate.
- Messaging RPCs with `user_id` request fields that the gRPC server does not implement (reactions, pins, polls,
  tasks, receipts) were not deprecated; REST serves them in-process. Do it when/if implemented.
- `approval.GetAuditLog` takes only a request id and has no per-caller authorization (it now at least requires a
  caller). Separate gap.
- Policy does not check the caller against the subject of a CheckAccess/mutation; any authenticated service call is
  admitted. Out of this phase's scope.
