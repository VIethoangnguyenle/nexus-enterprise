# Generic Policy Service — Tách policy-service thành service độc lập

## Evidence Summary

- **Backend**: EXISTS — `backend/services/policy/` (10 Go files, CQRS read/write split)
- **Proto**: EXISTS — `backend/proto/policy/` (3 proto files: policy.proto, policy_read.proto, policy_write.proto)
- **DB**: EXISTS — `data/init.sql` + `data/migrations/003_materialized_access.sql` (6 tables: ngac_nodes, ngac_assignments, ngac_associations, ngac_graph_version, ngac_materialized_access, ngac_operations)
- **Cache**: EXISTS — 3-layer cache (L1 Redis, L2 Materialized, L3 CTE) with targeted invalidation
- **Events**: EXISTS — Kafka producer (franz-go) publishing `ngac.access.checked` + `ngac.graph.mutated`
- **Dependencies**: pgx/v5, redis/go-redis/v9, prometheus, franz-go, grpc — ALL infrastructure, no domain deps

### Coupling Points Identified

| Coupling | Location | Impact |
|---|---|---|
| `go.mod` module path | `ngac-platform/services/policy` | Hardcoded platform name |
| Operation constants | `models.go:14-24` | `OpApprove`, `OpCreateChannel` — domain-specific |
| Constraint engine | `constraints.go` | `WeekdayOnlyConstraint` — business rule |
| Proto package | `policy.proto:5` | `go_package = "ngac-platform/proto/policy"` |
| Seed data | `seed.sql`, `init.sql:240-251` | VNPay-specific nodes |
| Schema co-location | `init.sql` | Policy tables mixed with workspace/channel/drive tables |

### Missing NGAC Components

| Component | Status |
|---|---|
| Prohibitions (deny overrides) | NOT IMPLEMENTED — no table, no logic |
| Obligations (event-response) | NOT IMPLEMENTED — by design, consumer owns |

## Product Assessment

- **Size**: L (major refactoring + new features + repo restructure)
- **Risk**: Medium (core engine already works, risk is in separation + new Prohibition feature)
- **Target user**: Platform engineers integrating NGAC authorization into any backend system (Go, Java, Python)
- **Core action**: Deploy a standalone policy service, connect via gRPC, manage access control graphs

## Scope

### In scope

1. **Remove domain coupling** — delete hardcoded operation constants, constraint engine, seed data
2. **Dynamic Operations** — RegisterOperations/ListOperations RPCs + `ngac_operations` table
3. **Prohibitions** — `ngac_prohibitions` table + CheckAccess integration + CRUD RPCs
4. **InvalidateCache RPC** — inbound cache invalidation endpoint for consumer services
5. **Self-contained migrations** — separate `migrations/` folder with only ngac_* tables
6. **Module path refactor** — prepare for standalone repo (`github.com/hoangnlv/ngac-policy`)
7. **Integration documentation** — README with cross-language examples

### Out of scope

- **Obligations engine** — consumer services own event-response logic via Kafka + gRPC
- **Actual repo separation** — Git operations (create new repo, push) are manual DevOps tasks
- **Consumer migration** — workspace/approval/drive services keep current import paths until manual cut-over
- **Multi-tenancy** — each system deploys its own instance
- **UI changes** — no frontend work

### Deferred

- Obligation engine (if demand arises from 3+ consumers)
- Admin dashboard for graph visualization
- GraphQL API adapter
- SDKs for Java/Python (consumers use raw gRPC stubs)

## Success Criteria

1. Policy service compiles and runs with ZERO domain-specific imports
2. `models.go` has NO hardcoded operation constants
3. `constraints.go` is DELETED
4. Prohibition CRUD + CheckAccess integration passes test cases
5. RegisterOperations + InvalidateCache RPCs work end-to-end
6. All existing tests pass (backward compatibility)
7. Self-contained `migrations/` folder with only policy tables
8. README.md documents integration for non-Go consumers
