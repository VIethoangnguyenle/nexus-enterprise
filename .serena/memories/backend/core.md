# Backend — Core

Go module `ngac-platform`, single `go.mod` at `backend/`. Eight services under `backend/services/`, each independently deployable, each with its own gRPC port and (except `policy`) its own REST port.

## Service topology

Ports are fixed in `.env.dev` and `Procfile.dev`; the gRPC and REST numbering are parallel, which makes them easy to confuse:

| service | gRPC | REST |
|---|---|---|
| policy | 50051 | — (gRPC only) |
| auth | 50052 | 8180 |
| workspace | 50053 | 8181 |
| document | 50054 | 8182 |
| messaging | 50055 | 8183 |
| asset | 50056 | 8184 |
| drive | 50057 | 8185 |
| approval | 50058 | 8186 |

- **`policy` has no REST surface.** It is reached only over gRPC by other services. `policy-read` is a second entrypoint (`cmd/policy-read`) sharing the same port in dev but split out in compose so the read path can scale separately.
- `approval` is the only service that needs `POLICY_ADDR` explicitly; the rest resolve the policy address from `.env.dev`.
- Redis is per-service by URL (`REDIS_URL_POLICY`, `REDIS_URL_AUTH`, `REDIS_URL_MESSAGING`); only those three use it.

## Architecture rules

- **REST-per-service, no gateway.** The gateway monolith was deliberately removed (commit `500d58e`). Do not reintroduce a shared HTTP entrypoint — in Docker, Traefik routes by path; in native dev, the Vite proxy does.
- **Authorization is centralized in `policy`, naming is centralized in `backend/ngac`.** Services ask policy whether an action is allowed; they never evaluate the graph themselves. The NGAC graph model lives in `backend/services/policy/internal/ngac/`.
- Layering inside a service is `cmd/` → `internal/{grpc,rest}` → `internal/domain` → `internal/store`, with `internal/events/` for Kafka emission. See `mem:conventions` for what each layer may not do.

## Schema

`data/init.sql` is the whole schema and is applied with `IF NOT EXISTS` — it is re-runnable, not a versioned migration chain, so schema changes are edits to that file plus `make db-migrate`. `backend/services/policy/migrations/` is policy-local and separate from it.
