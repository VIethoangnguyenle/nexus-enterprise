# Repo-side CI/CD report (2026-10-10)

Files only: nothing committed, staged or pushed; no server, GitHub API or `gh` calls.

## Files
New: `deploy/docker-compose.prod.yml`, `deploy/migrate.sh`, `deploy/deploy.sh`,
`deploy/server-bootstrap.sh`, `deploy/env.example`, `.github/workflows/deploy.yml`,
`docs/deployment.md`.
Edited: `.github/workflows/ci.yml` (docs job: compose validation, no-`ports:` guard, shellcheck
-S warning), `CLAUDE.md` (one added line after the §2 command block, pointing to docs/deployment.md).

## Design decisions
- `deploy/deploy.sh` (not requested) holds the server-side logic (login via stdin token, pull,
  migrate, `up --wait`, write `.deployed-tag`, scoped old-image cleanup, flock). The workflow
  stays thin and the same script serves manual redeploy/rollback.
- Migrations are rsynced to `/opt/nexus/db/` each deploy (not baked into an image): they always
  match the deployed commit and need no extra build.
- Router/service/middleware names are `nexus-*` (the shared Traefik's names are global).
- No `env_file`: every container gets only the variables it needs, interpolated from
  `/opt/nexus/.env`. Deviation from the brief, for least privilege (Go services do not see
  MinIO/Postgres root secrets they do not use).
- `AUTH_FIXED_OTP_CODE: ""` hard-coded in compose (set-but-empty = off; unset would mean 999999),
  so a stray `.env` line cannot re-enable it. `APP_ENV=production` for all services (Secure
  cookie, rejects placeholder JWT secret).
- MinIO pinned to the release dev actually runs (`RELEASE.2025-09-07T16-13-09Z`, `:latest` was
  not pinnable). policy-read: 1 replica. Memory limits total about 3.1 GB.
- `restrict` on the authorized_keys line is compatible with rsync+ssh (only drops pty/forwarding);
  not skipped.
- Actions pinned: checkout v7.0.1, setup-buildx v4.4.1, login v4.6.0, build-push v7.4.0 (SHAs
  resolved with `git ls-remote`).

## Verification
- `docker compose -f deploy/docker-compose.prod.yml --env-file deploy/env.example --profile tools config`: pass; 0 `ports:`; 13 images resolved.
- Migration runner against scratch DB `nexus_deploy_test` (created, then dropped): run 1 applied init.sql + 33 migrations (8 with their own BEGIN/COMMIT handled outside the wrapper); run 2 "up to date". Extra: new migration triggers a pg_dump (`.partial` then mv), CONCURRENTLY file runs outside a transaction, failing file rolled back and not in ledger (exit 3), edited file refused with exit 3 before any change, prune kept 14.
- Images built locally with Dockerfiles: auth (57.9 MB, has grpc_health_probe) and frontend (65.4 MB; the compose wget healthcheck works). Test images removed.
- shellcheck -S warning (docker image) on the 3 scripts: clean. actionlint: only info-level SC2029/SC2012 (intentional client-side expansion of validated values; one pre-existing in ci.yml). Workflow YAML parses.
- Bootstrap tested in a sandbox dir with root/network checks stubbed: creates, is idempotent (.env byte-identical on rerun), mode 600, key appended once with `restrict`, no secret printed; the generated .env passes `compose config`.
- `make build-check` OK; frontend `npm run build` OK; `scripts/check-docs-drift.sh` OK.
- NOT verified (needs the real server/GitHub): deploy.yml execution, the rsync/ssh path, ghcr pulls, Traefik routing and TLS, `up --wait` on the real host, `docker compose config` with the external `traefik` network actually present.

## What the user must do (outward-facing)
1. DNS: A `nexus.zaneng.xyz` and A `storage.nexus.zaneng.xyz` -> 160.187.146.173.
2. `ssh-keygen -t ed25519 -N '' -C nexus-deploy -f ./nexus-deploy`
3. `ssh-keyscan -t ed25519 160.187.146.173 > known_hosts.txt`; compare `ssh-keygen -lf known_hosts.txt` with `ssh-keygen -lf /etc/ssh/ssh_host_ed25519_key.pub` on the server.
4. GitHub: environment `production` with required reviewer; repo secrets `DEPLOY_SSH_KEY` (private key file content) and `DEPLOY_KNOWN_HOSTS` (known_hosts.txt content).
5. Orchestrator, after approval (touches the server):
   `scp deploy/server-bootstrap.sh nexus-deploy.pub root@160.187.146.173:/root/`
   `ssh root@160.187.146.173 'bash /root/server-bootstrap.sh --pubkey-file /root/nexus-deploy.pub'`
6. Edit `/opt/nexus/.env`: set `GOOGLE_CLIENT_ID`, `GOOGLE_CLIENT_SECRET` (redirect URI `https://nexus.zaneng.xyz/api/auth/google/callback` registered in Google).
7. Commit and merge `deploy/`, `deploy.yml`, `docs/deployment.md` to main; approve the first run. If the pull is denied, grant the repo read access on each `nexus-*` package.

## Concerns
- **Production login**: with the fixed OTP off there is no OTP sender in the code (LogSender is dev-only), so Google OAuth is the only sign-in. No "email sender" env exists to fill; the .env has only Google placeholders.
- **File upload/download breaks over https**: `document/cmd/main.go` hard-codes `Secure: false` for the presign client, so presigned URLs are `http://storage.nexus.zaneng.xyz/...` (mixed content). The dev `/storage` path-prefix route cannot work with signed URLs, so I routed MinIO by host instead. Needs a small Go change (public-endpoint TLS flag) outside this brief.
- First deploy has no previous tag, so a failed health check cannot auto-roll back (stack is left as is).
- Rollback is image-only; migrations are forward-only (documented).
- DNS-alias collision risk: services share the `traefik` network with other apps; a same-named service (e.g. `frontend`, `drive`) there could shadow ours for DNS. Low likelihood (Odysseus is stopped).
- The deploy key is root-equivalent (restrict does not limit commands).
- `backend/services/policy/migrations/` is applied by nothing in the repo today; the runner does not apply it either.
- No `docs/specs` entry added: infra only, no system behaviour change.

Status: DONE_WITH_CONCERNS
Summary: All six deliverables written and verified locally (compose config, migration runner on scratch DB incl. failure/tamper/prune cases, two image builds, shellcheck, build-check, frontend build); server/GitHub paths are untested by design.
Concerns: Google-only sign-in in prod; presigned URLs are http (needs a code change); first deploy cannot auto-roll back.

# Review fixes (2026-10-10, second pass)

## Fixed
- **C1** policy `Dockerfile` and `Dockerfile.read`: added `COPY pkg/` and `COPY ngac/`. All 10 images were created locally from their Dockerfiles. Sizes: approval 57.2 MB, asset 57.5, auth 58.1, document 60.3, drive 57.2, frontend 65.4, messaging 64.9, policy 62.8, policy-read 62.7, workspace 60.6. Images removed afterwards. CI: new `images` job in `ci.yml` runs `docker build` for all 10 Dockerfiles (no push).
- **C2** messaging had no `JWT_SECRET` in the prod compose: added. New `deploy/check-compose-env.sh` (run in the CI docs job) renders the compose JSON and derives each service's required env from its `cmd/main.go` (`bootstrap.Env/EnvAlias`, `os.Getenv/LookupEnv`, `PolicyAddr()`, `RequireJWTSecret`, `realtime`), minus an explicit optional list. It also checks that every `*_ADDR` and `MINIO_ENDPOINT` names a real compose service. Negative test (JWT removed from messaging) fails as intended.
- **H1** single edge. nginx (`deploy/nginx/default.conf`, `proxy.inc`, mounted into the unchanged frontend image) is the only app container on `traefik` besides MinIO; every Go service is on `nexus` only, with `nexus-<svc>` aliases. The two shared-network containers are renamed `nexus-frontend` / `nexus-minio`. nginx has the fixed IP `10.77.231.10` (subnet `10.77.231.0/24`; dynamic pool restricted to `.128/25`, because the smoke test showed dynamic allocation otherwise takes `.10` first). `AUTH_TRUSTED_PROXIES=10.77.231.10/32`. nginx `set_real_ip_from` comes from `nginx/real-ip.conf`, generated by `deploy.sh` from `docker network inspect traefik`; `X-Forwarded-For` / `X-Real-IP` are set to the resolved client address (replace, never append). CORS middleware dropped. Routing mirrors Traefik precedence (ws > 120 group > auth > workspace > SPA). Deviation: `/api/ws` is passed to the hub without stripping `/api`, because the hub only serves `/api/ws` (finding 2 below).
- **M1** rsync `-rlptz --delete --chmod=D750,Fgo-w` (no -o/-g), into `releases/<tag>/`.
- **M2** `deploy.sh` uses a private `DOCKER_CONFIG` (mktemp, removed by trap; root's cli-plugins linked in). The remote `docker logout` is gone from the workflow.
- **M3** per-release dirs `/opt/nexus/releases/<tag>/` (compose, nginx, db, scripts; newest 5 kept). Rollback runs the previous release's own `deploy.sh` with `ROLLBACK=1`: no migrations. A tag-input redeploy reuses an existing release dir untouched. State (`.env`, `backups/`, `.deployed-tag`, lock) stays in `/opt/nexus`.
- **M4** `/api/polls` and `/api/tasks` added to messaging in nginx, the dev `docker-compose.yml` Traefik rule and `frontend/vite.config.js`. Every path in `frontend/src/api/*.ts` was checked against the prod routing and is covered. Gap: `/api/admin` (approval tenant provisioning) is unrouted in dev Traefik and prod nginx and not called by the frontend (vite proxies it); left unrouted publicly on purpose, documented.
- **M5** docs: "If the first deploy fails" runbook (logs, `compose down` without `-v`, rerun, DNS, Let's Encrypt, package access).
- **Lows** 1 previous-tag regex in the workflow. 2 CORS middleware removed. 3 bootstrap `ensure_key` adds a missing trailing newline. 4 migrate.sh no-transaction regex is `create (unique )?index concurrently` (plus ADD VALUE, VACUUM, CREATE DATABASE, marker); idempotence requirement documented. 5 `migrate.sh` creates `/backups/.migrations-applied`; `deploy.sh` then force-recreates `policy` and `policy-read`. 6 CI guard checks the rendered JSON for `ports` and `network_mode`. 8 restore runbook takes a safety dump first and flushes Redis. 9 host-key docs rewritten (fingerprint verified over an already-trusted SSH session; provider-console alternative).
- Docs: step 4 now describes environment secrets of `production`, owner as sole reviewer, Selected branches `main`, the self-review note; `policy/migrations` mention removed; Traefik's global web -> websecure redirect noted. SMTP_* and MINIO_PUBLIC_SECURE wiring kept.
- Extra: `SKIP_PULL=1` in `deploy.sh` for images already on the host.

## New findings
1. **The MinIO image is gone.** `minio/minio` (Docker Hub) and `quay.io/minio/minio` tags are no longer pullable (pull denied; Hub repo returns 404). The tag pinned in the first pass would have failed `docker compose pull` on the server. Compose now defaults to `pgsty/minio:RELEASE.2026-08-04T00-00-00Z` (has `mc`; healthcheck verified), overridable with `MINIO_IMAGE`. This is a third-party distribution: needs your approval, or supply your own image/mirror.
2. **Dev Traefik WebSocket is probably broken**: `ws-strip` removes `/api`, but the hub only handles `/api/ws`. Prod nginx does not strip and the upgrade returned 101 in the smoke test. Dev compose left as is (out of scope); worth fixing separately.
3. The `traefik` network is shared, so MinIO's S3 API (:9000, credentials required) and console (:9001) are reachable from other apps on it.

## Verification
- `check-compose-env.sh`: ok (no `ports`, no `network_mode`, only `nexus-frontend` and `nexus-minio` on `traefik`, OTP code empty, trusted proxies = edge /32).
- `nginx -t` in the frontend image with the prod config: ok.
- Local smoke of the prod compose through `deploy.sh` (project `nexus-smoke`, throwaway env, temporary `traefik` network, local images). Fresh DB migrated, policy restarted, re-run = up to date, `ROLLBACK=1` skipped migrations; 9 app containers plus infra healthy. Via nginx at 10.77.231.10: `/api/auth/providers` 200 JSON, SPA and deep link 200, `/api/me` 401; each of workspace, document, messaging (`/channels`, `/polls/p1`, `PATCH /tasks/t1`), asset, drive (`/drive/folders`, `/drive/files/a.png`), contacts (auth) and approval confirmed in that service's own access log; `/api/admin/x` JSON 404; `/api/ws` upgrade 101. Client IP: a request from the traefik network with `X-Forwarded-For: 203.0.113.9` (and a spoofed chain `6.6.6.6, 203.0.113.9`) reached auth as `203.0.113.9`; the same header from the `nexus` network was ignored. From a container on `traefik` only: `nexus-auth` / `auth` DNS, auth IP :8080 and :50052 all unreachable. Torn down with `down -v` on that project only; temporary network, images and files removed; dev stack untouched.
- shellcheck -S warning clean; actionlint info-level SC2029/SC2012 only (intentional); `make build-check` ok; docs drift ok.

## Orchestrator / user actions added
- Decide the MinIO image (pgsty vs own image/mirror).
- The first deploy after this change has no `releases/` dir on the server: bootstrap now creates it (rerun is safe) and the workflow creates each release dir itself.

Status: DONE_WITH_CONCERNS
Summary: All blockers, highs, mediums and lows addressed and exercised locally, including a full smoke run of the prod compose through deploy.sh.
Concerns: MinIO image source needs a decision; dev Traefik WS strip looks broken; the workflow itself has still not run against GitHub or the server.
