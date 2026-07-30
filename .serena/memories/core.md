# NGAC Platform — Core

Multi-tenant workspace/collaboration platform (docs, drive, chat, approvals) whose entire authorization layer is **NGAC** (NIST Next Generation Access Control), not RBAC. Every access decision is a graph reachability query answered by the policy service.

## Source map

- `backend/` — Go 1.25 monorepo, module `ngac-platform`. 8 independent services, no API gateway. See `mem:backend/core` for service boundaries, the REST-per-service rule, and the `internal/` layering every service follows.
- `frontend/` — React 19 + TanStack SPA. See `mem:frontend/core` for routing, state split, and how requests reach the right backend port.
- `backend/proto/` — `.proto` per domain; source of truth for gRPC contracts. Generated Go lands next to the proto; generated TS lands in `frontend/src/generated/`.
- `backend/ngac/ngac_ops.go` — single source of truth for NGAC operation strings and node-naming functions.
- `data/init.sql` — full schema, applied idempotently (`IF NOT EXISTS`), not a migration chain.
- `docker-compose.yml` — infra (postgres, redis, redpanda, minio, traefik) + all services.
- `Procfile.dev` — native-Go dev topology run by overmind; the authoritative port/env map.

## Project-wide invariants

- **Never hardcode NGAC strings.** Operations and node names come from `backend/ngac`. Adding an operation or node type means adding a constant/function there first — the package exists so renames break at compile time.
- **No gateway.** Each service owns its REST surface; the frontend targets a distinct port per service. A change that adds a route must be reflected in the Vite proxy table (see `mem:frontend/core`).
- **Policy service is the only authorizer.** Other services call it over gRPC; they do not re-implement permission logic locally.
- **Tenant scoping is pervasive.** Node names embed workspace/tenant/channel IDs (`PC_<wsID>`, `TenantMember_<tenantID>`, `Ch_<chID>_Content`). Cross-tenant leakage is a naming-discipline failure, so use the helpers rather than `fmt.Sprintf`.

Commands: `mem:suggested_commands`. Definition-of-done checks: `mem:task_completion`. Language/framework versions and pins: `mem:tech_stack`. Code style and layering rules: `mem:conventions`.
