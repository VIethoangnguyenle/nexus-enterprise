# Suggested Commands

System: Linux, zsh. Standard unix utils behave normally — no platform-specific forms needed.

All `make` targets run from the repo root. `make help` lists them.

## Running

- `make dev` — infra via compose, then native `go run` per service + frontend. Writes PIDs to `.dev-pids` and logs to `.dev-logs/`; **refuses to start if `.dev-pids` exists**, so `make dev-stop` (or delete a stale `.dev-pids`) first.
- `make run` — same topology in the foreground via overmind reading `Procfile.dev`. `make dev-all` is the daemon form; `make dev-connect` attaches; `make dev-stop` stops.
- `make dev-infra` — infra only. Waits up to 90s for health, then auto-runs `db-migrate`.
- `make dev-frontend` — Vite alone against already-running services.
- `make down` / `make restart` / `make ps` / `make logs` / `make health` — compose lifecycle.

## Building and testing

- `make build-check` — compiles every service's `./cmd/` to `/dev/null`; the fast "does it still build" gate.
- `make test` — Go tests across all services (truncated output per service).
- `make test s=<service>` — one service, verbose, `-count=1 -timeout 60s`.
- `cd frontend && npm test` — Vitest once; `npm run test:watch` for watch mode; `npm run lint` for ESLint.

## Codegen and schema

- `make proto` — regenerate Go from `backend/proto/`. `make proto-install` first on a fresh machine.
- `cd frontend && npm run proto:gen` — regenerate TS wire types into `src/generated/`. Separate from `make proto`; both must be run when a shared `.proto` changes.
- `make db-migrate` — re-apply `data/init.sql` to the running DB. Idempotent.
- `make clean` / `make nuke` — artifacts / full teardown including volumes.

## Code intelligence

- `codegraph explore "<question>"` and `codegraph node <symbol>` return verbatim source plus call paths — cheaper than reading files. `codegraph sync` after large refactors.
- `serena project index` refreshes the LSP symbol cache.
