## 1. Docker Compose Profiles

- [x] 1.1 Add `profiles: [app]` to all application services in `docker-compose.yml` (policy, policy-read, auth, workspace, document, messaging, asset, drive, frontend)
- [x] 1.2 Add `profiles: [app]` to Traefik (not needed for dev mode)
- [x] 1.3 Expose infrastructure ports to host: postgres `5432:5432`, redis `6379:6379`, redpanda `19092:19092`
- [x] 1.4 Verify `docker compose up` only starts infra services (no profile = infra only)
- [x] 1.5 Verify `docker compose --profile app up` starts everything (backward compat)

## 2. Environment Configuration

- [x] 2.1 Create `.env.dev` with all service environment variables (DATABASE_URL localhost:5432, REST ports, gRPC addrs, JWT secret, MinIO, Redis, Kafka)
- [x] 2.2 Add `.env.dev` to `.gitignore` check — decide if tracked or template-only (`.env.dev.example`)

## 3. Makefile Dev Targets

- [x] 3.1 Add `dev-infra` target: start Docker infra, wait healthy, apply schema
- [x] 3.2 Add `dev` target: create `.dev-logs/` dir, truncate log files, `go run` each service with env vars redirecting stdout/stderr to `.dev-logs/<service>.log`, save PIDs to `.dev-pids`
- [x] 3.3 Add `dev-stop` target: read `.dev-pids`, kill processes, cleanup PID file
- [x] 3.4 Add `dev-logs` target: `tail -f .dev-logs/*.log` (all services), support `s=<name>` for single service
- [x] 3.5 Update `deploy` target to use `--profile app` flag
- [x] 3.6 Update `help` target with new dev commands
- [x] 3.7 Add `.dev-logs/` and `.dev-pids` to `.gitignore`

## 4. Vite Proxy Configuration

- [x] 4.1 Update `vite.config.js` with per-service proxy routes for dev mode (auth→8180, workspace→8181, document→8182, messaging→8183, asset→8184, drive→8185)
- [x] 4.2 Keep fallback `/api` → `localhost:80` for production/Docker mode
- [x] 4.3 Configure WebSocket proxy `/api/ws` → `ws://localhost:8081`

## 5. Verification

- [x] 5.1 Test `make dev-infra` — infra starts, ports accessible from host
- [x] 5.2 Test `make dev` — all services start, frontend connects, full flow works
- [x] 5.3 Test `make dev-stop` — all processes killed, PID file cleaned
- [x] 5.4 Test `make deploy` — full Docker stack works unchanged (backward compat)
