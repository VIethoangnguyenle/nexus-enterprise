# Signed service identity (item D) - report

## What changed

gRPC caller identity is now an HMAC-SHA256 token, one metadata key `x-nexus-identity`, minted per call by the
client interceptor and verified by the server interceptor. Raw `x-caller-*` / `x-service-name` are ignored.
No new dependencies (stdlib only). Design implemented as specified; no flaw found.

- Token: `base64url(json payload).base64url(HMAC(secret, "nexus-identity-v1." + payload-b64))`.
  Payload: `uid, nid, tid` or `svc`, `aud` = full gRPC method, `iat`, `exp` = iat+60s, `jti` random nonce.
- Verify (`backend/pkg/grpcauth/token.go`): constant-time compare against current and previous key (both always
  computed), strict JSON (unknown fields refused), `aud == info.FullMethod`, skew 5s both ways, lifetime cap 2 min,
  exactly one identity kind (user needs uid+nid; service needs svc). Failures log the reason (Warn) and answer
  `Unauthenticated "invalid caller identity"`.
- Client: unary + stream interceptors (`Dial` now installs both). Half-built caller still refuses the call.
  No caller and no service name = no token (server then refuses unless exempt). Unconfigured secret = call fails.
- Server: Exempt methods skip verification entirely. `ServiceOK` is now `map[string]ServiceRule` (reason +
  allowlist, built with `grpcauth.ServiceOnly(reason, services...)`); a valid service token is admitted only on
  a listed method and only from an allowlisted service.
  - policy `CreateNode`, `CreateAssignment`, `FindNodeByName`: `auth` only.
  - auth `IsTokenRevoked`: `messaging` only (the only service that dials auth; nothing calls the RPC today, so this
    is the tightest safe choice - widen if a new caller appears).
- Secret: `bootstrap.ConfigureInternalIdentity()` (new `backend/pkg/bootstrap/identity.go`), first call in all 9
  `main`s (8 services + policy-read). Required outside APP_ENV dev/development/local/test; refuses the committed
  placeholder `DevInternalIdentitySecret` outside dev; refuses a value equal to `JWT_SECRET`; min 16 bytes;
  `INTERNAL_IDENTITY_SECRET_PREVIOUS` verify-only. Dev falls back to the placeholder.
  `httputil.RequireJWTSecret` now shares `bootstrap.IsDevEnvironment()` (one dev-env list instead of two).
- Replay cache: not added. Nonce is only in the token (available for logging).

## Files

Code: `backend/pkg/grpcauth/{grpcauth.go,token.go,interceptors.go}`, `backend/pkg/bootstrap/identity.go`,
`backend/pkg/httputil/secret.go`, 9 `cmd/**/main.go`, `services/{policy,auth}/internal/grpc/authpolicy.go`,
`backend/testutil/grpcserver.go` (`ServeGRPCAs`, `Unsigned`, `ForgedIdentity`, shared test secret).
Tests: `grpcauth/token_test.go` (new), `grpcauth_test.go`, `bootstrap_test.go`, policy + auth authpolicy tests,
and a forged-identity deny test in each of approval, asset, document, drive, messaging, workspace
`caller_identity_test.go`.
Deploy/dev: `deploy/{docker-compose.prod.yml,server-bootstrap.sh,env.example,check-compose-env.sh}`,
`docker-compose.yml` (env lines only: 9 insertions after each service's `environment:`), `.env.example`
(commented entries; Procfile.dev and Makefile need nothing: they source `.env.dev` with `set -a`, and
`APP_ENV=dev` there gives the placeholder).
Docs: `docs/deployment.md`, `docs/specs/resource-pep-coverage/spec.md` (requirement rewritten, threat model,
scenarios; removed the stale auth `Register/Login/...` row, those RPCs no longer exist), `CLAUDE.md` (only the
bootstrap/grpcauth bullet), refactor `plan.md` decision 13 marked done with pointer.
Not touched: the release `plan.md` checkbox for item D (orchestrator).

## Verification (all run)

- `make build-check`, `check-ngac`, `check-layering`, `lint`, `fmt-check`, `check-docs`: pass.
- `make test` strict (zero skips): pass on `ngac` and on `ngac_ci` (both on the 5433 instance). `go test -race`
  of `backend/pkg`, `testutil`, `ngac`: 172 pass.
- `deploy/check-compose-env.sh`: ok (now also requires `INTERNAL_IDENTITY_SECRET` for every service whose main
  calls `ConfigureInternalIdentity`). Prod compose renders with `env.example`; 9 services carry the var.
  Dev compose `config` renders.
- grpcauth tests: round trip (user, service), tampered payload, tampered/garbage signature, expired (with skew
  boundaries), future-issued, wrong audience (unit + wire), wrong secret, previous-secret accepted / never signs /
  dropped after rotation, raw x-caller-* refused, service not on allowlist, method not in ServiceOK, unconfigured
  server and client, stream (raw refused, replayed token refused, own token admitted), signed-but-malformed claims.
- Live smoke on the dev stack (`make dev`, infra already up and reused; stopped with `make dev-stop`, nothing left
  listening): dev OTP login (code 999999) = signup path works (auth to policy via the `auth` service token, auth to
  workspace/messaging as the user); with that JWT, 200 on auth `/api/me`, workspace (workspaces, members, roles),
  document, messaging (channels, dms), asset (assets, asset-types), drive, approval (templates, pending,
  permissions). grpcurl with forged `x-caller-*` + `x-service-name: auth` and no token: policy
  `CreateAssignment` and `CheckAccess`, workspace `ListWorkspaces` = Unauthenticated; a garbage
  `x-nexus-identity` = Unauthenticated "invalid caller identity".

## Exact one-time server step (NOT run; orchestrator does it, before the deploy containing this change)

```bash
scp deploy/server-bootstrap.sh root@160.187.146.173:/root/
ssh root@160.187.146.173 'bash /root/server-bootstrap.sh'
ssh root@160.187.146.173 "grep -c '^INTERNAL_IDENTITY_SECRET=.' /opt/nexus/.env"   # must print 1
```

The bootstrap appends only missing keys (here `INTERNAL_IDENTITY_SECRET`, `rand_hex 48`), never rewrites existing
ones, never prints a value. `--pubkey` is not needed on re-run. Also in `docs/deployment.md` (section "Adding
INTERNAL_IDENTITY_SECRET to a server bootstrapped earlier", with the rotation recipe).
If the deploy runs first, prod compose uses `${INTERNAL_IDENTITY_SECRET:?...}` so it fails at `compose` before any
container changes.

## Concerns

- Deploy is a rolling restart of 9 containers. Between the first new container and the last, new callers talk to
  old servers (which still demand `x-caller-*`) and vice versa, so a short burst of Unauthenticated is expected
  on that one deploy. Not avoidable without a compatibility shim; I did not add one (it would reintroduce the
  unsigned path).
- Remaining risk: a stolen `INTERNAL_IDENTITY_SECRET` mints any identity (rotate via `_PREVIOUS`). Captured tokens
  replay on the same method for up to ~65 s (no replay cache). Plaintext transport, so network isolation of the
  gRPC ports still matters; this is defence in depth. All documented in the spec threat model.
- Signing happens in every service's client interceptor, not "at the REST edge" as decision 13 phrased it: the edge
  only verifies the JWT and puts the caller on the context; whichever service dials mints the token. Same trust
  model (all services share the secret), simpler.
- Latent, pre-existing, not changed: dev `docker-compose.yml` sets `APP_ENV: dev` only on auth, while others keep
  the committed `JWT_SECRET`, which `RequireJWTSecret` refuses outside dev. For the new secret I used a
  compose-only non-placeholder default (`docker-compose-dev-identity-secret-local-only`) so it does not hit the
  same trap. Worth checking whether that dev compose stack starts at all.
- The shared checkout already had unrelated uncommitted edits (CLAUDE.md, DESIGN.md, frontend/vite.config.js,
  docker-compose.yml traefik/ws lines); I left them. Index untouched, nothing committed.
- Housekeeping slip: I briefly created and dropped a scratch `ngac_ci` database in the `nexus-e2e-postgres-1`
  container (wrong instance; the real `ngac_ci` is on 5433 in `nexus-enterprise-postgres-1` and was used for the
  test run). Net state of the e2e container is unchanged.

## Review fixes

1. Rotation recipe corrected to three deploys (PREVIOUS=NEW with current=OLD; swap; drop PREVIOUS), plus the
   "suspected leak: skip PREVIOUS, accept the blip" note. Fixed in `docs/deployment.md`, the
   `deploy/server-bootstrap.sh` comment, and the spec (requirement text and the "Rotation window" scenario). No test
   asserted the old wording.
2. `verify()` rejects tokens over 1024 bytes before decoding and requires a 43-character MAC part.
   `ServerOptions` sets `grpc.MaxHeaderListSize(16<<10)`. Legitimate headers: the repo has no tracing
   (no traceparent/otel); grpc-go adds a few dozen bytes of pseudo/standard headers; the only caller metadata is the
   token (about 400 bytes, asserted under half the cap even with three 36-char ids and a long method). Tests: oversize,
   short/long MAC, 1 MiB garbage, a 32 KiB-header request refused while an ordinary signed call passes; every service's
   over-the-wire tests also run through `ServerOptions`. `testutil.ForgedIdentity` now carries a 43-char MAC so it
   still reaches the signature check.
3. Dev `docker-compose.yml`: removed the second committed secret (9 lines + comments), added `APP_ENV: dev` to every
   Go service (auth already had it); services use the guarded `DevInternalIdentitySecret`. `docker compose --profile
   app config` renders and shows `APP_ENV=dev` on all 9. Traefik pin and ws-router lines untouched.
4. Spec threat model rewritten: (1) shared symmetric secret, allowlists limit mistakes but do not authenticate
   services, compromise of any one service mints any identity; isolation is the Docker network layout (Go services only
   on `nexus`, no published ports, asserted by `check-compose-env.sh`), not the nginx edge; health is exempt even with
   no secret configured.
5. `minSecretLen` = 32 (deployment generates 96 hex chars; dev placeholder and test secrets are longer). `_PREVIOUS ==
   JWT_SECRET` now refused. Tests added in bootstrap and grpcauth.
6. Policy `ServiceOK` gained `PolicyWrite/DeleteNode` for `auth` only. Tests: admitted from auth, refused from
   workspace and with forged identity (red before the change). Spec table updated.

Verification: `check-ngac`, `check-layering`, `fmt-check`, `check-docs`, `check-compose-env.sh`, prod and dev compose
config pass. `go test -race` of `backend/pkg` + `testutil` pass (164). Strict (zero-skip) tests pass on `ngac` and
`ngac_ci` for policy, auth, approval, asset, document, drive.

Not green, and not from these changes (other agents mid-edit in shared files):
- `make build-check` / `make lint` fail on messaging: `internal/grpc/notification_server.go` and
  `internal/events/notifications_from_events_test.go` do not compile (notifications work). I could not run the
  messaging tests, including its forged-identity test; rerun once that lands.
- workspace `internal/domain`: `TestAcceptInvitation_AnnouncesTheNewMemberOnce` and
  `TestAcceptInvitation_RefusedOrUndoneAnnouncesNothing` fail on an unexpected `invitation_created` event (the
  invitation-email/notification work in `invitations.go`). Everything else in workspace passes (335).

Status: DONE_WITH_CONCERNS
Summary: All six review items fixed test-first; docs, spec, compose and bootstrap updated; my gates and tests are green on ngac and ngac_ci.
Concerns: messaging does not compile and two workspace invitation tests fail because of the concurrent notifications/invitation-mail agents' unfinished edits, so build-check, lint and those suites need a rerun after they land. Rotation now takes three deploys.
