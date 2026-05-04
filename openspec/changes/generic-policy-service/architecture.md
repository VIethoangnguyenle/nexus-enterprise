# Architecture: Generic Policy Service

## Current State → Target State

```
  CURRENT                               TARGET
  ┌──────────────────────┐              ┌──────────────────────┐
  │ ngac-platform/       │              │ ngac-platform/       │
  │ services/policy/     │              │ services/policy/     │ ← same location,
  │                      │              │                      │   generic internals
  │ • Op constants ❌     │   refactor   │ • No Op constants ✅  │
  │ • ConstraintEngine ❌ │  ────────►   │ • No constraints ✅   │
  │ • Domain seed ❌      │              │ • Prohibitions ✅     │
  │ • No prohibitions ❌  │              │ • Dynamic ops ✅      │
  │ • No InvalidateCache │              │ • InvalidateCache ✅  │
  └──────────────────────┘              │ • Self-contained ✅   │
                                        │ • Integration doc ✅  │
                                        └──────────────────────┘
```

## File Change Map

### Layer 1: Proto (API contract)

```
  proto/policy/policy.proto        → ADD Prohibition messages
  proto/policy/policy_read.proto   → ADD ListOperations, ListProhibitions RPCs
  proto/policy/policy_write.proto  → ADD RegisterOperations, InvalidateCache,
                                       CreateProhibition, RemoveProhibition RPCs
```

### Layer 2: Domain (NGAC engine)

```
  internal/ngac/models.go          → REMOVE Op* constants, ADD Prohibition model
  internal/ngac/constraints.go     → DELETE entire file
  internal/ngac/graph.go           → ADD prohibition check in access path
  internal/ngac/access.go          → MODIFY CheckAccess to include prohibition step
  internal/ngac/store.go           → ADD Prohibition CRUD (DB operations)
  internal/ngac/operations.go      → NEW — dynamic operations registry
  internal/ngac/prohibition.go     → NEW — prohibition store + matching logic
```

### Layer 3: gRPC (API layer)

```
  internal/grpc/server.go          → REMOVE ConstraintEngine field + logic
  internal/grpc/read_server.go     → ADD ListOperations, ListProhibitions handlers
  internal/grpc/write_server.go    → ADD RegisterOperations, InvalidateCache,
                                       CreateProhibition, RemoveProhibition handlers
```

### Layer 4: Infrastructure

```
  internal/events/producer.go      → ADD PublishProhibitionEvent
  internal/metrics/metrics.go      → ADD prohibition cache metrics
  cmd/main.go                      → REMOVE ConstraintEngine wiring,
                                     ADD operations store + prohibition store wiring
```

### Layer 5: Schema + Docs

```
  migrations/001_core_schema.sql       → NEW (extracted from init.sql)
  migrations/002_cache_schema.sql      → NEW (extracted from 003_materialized_access.sql)
  migrations/003_operations.sql        → NEW
  migrations/004_prohibitions.sql      → NEW
  README.md                            → NEW (integration guide)
```

## Data Flow: CheckAccess with Prohibitions

```
  CheckAccess(user, object, operation)
  │
  ├── L1 Redis ── HIT? ──► return cached (already includes prohibition result)
  │                  │
  │                 MISS
  │                  │
  ├── L2 Materialized ── HIT + version fresh? ──► return cached
  │                          │
  │                        MISS/STALE
  │                          │
  ├── L3 Compute:
  │     │
  │     ├── Step 1: BFS graph traversal
  │     │           → DENY? → return DENY (skip prohibition)
  │     │
  │     ├── Step 2: Prohibition check (NEW)
  │     │           → Query ngac_prohibitions
  │     │             WHERE subject_id IN (user, user_UAs)
  │     │             AND operation = ANY(operations)
  │     │           → Match targets against object's OA ancestors
  │     │           → MATCH? → override to DENY
  │     │
  │     └── Step 3: Populate caches with FINAL decision
  │                 → L2: UPSERT (user, obj, op, FINAL_decision, version)
  │                 → L1: SET key → FINAL_decision (TTL 30s)
  │
  └── return decision
```

## Data Flow: Prohibition Mutation + Cache Invalidation

```
  CreateProhibition(name, subject, ops, targets)
  │
  ├── INSERT INTO ngac_prohibitions
  │
  ├── version.Increment("global")
  │
  ├── Identify affected users:
  │   ├── subject = U node → affected = [subject]
  │   └── subject = UA node → affected = GetDescendants(subject)
  │                            → filter type U
  │
  ├── L2: FOR EACH affected_user:
  │         DELETE FROM ngac_materialized_access
  │         WHERE user_node_id = affected_user
  │       FOR EACH target_oa:
  │         DELETE FROM ngac_materialized_access
  │         WHERE object_node_id = target_oa
  │
  ├── L1: cache.InvalidateForNodes(subject, targets...)
  │
  └── Kafka: publish "create_prohibition" event
```

## Dependency Order (build sequence)

```
  1. Proto changes (messages + services)           ← no deps
  2. models.go (remove constants, add Prohibition) ← no deps
  3. DELETE constraints.go                          ← no deps
  4. operations.go (new store)                      ← depends on models
  5. prohibition.go (new store + matching)          ← depends on models
  6. access.go (add prohibition check)              ← depends on prohibition.go
  7. store.go (add schema for ops + prohibitions)   ← depends on models
  8. server.go (remove constraint wiring)           ← depends on 3
  9. write_server.go (add new RPCs)                 ← depends on 4,5,6
  10. read_server.go (add new RPCs)                 ← depends on 4,5
  11. cmd/main.go (rewire)                          ← depends on all above
  12. migrations/ (self-contained SQL)              ← independent
  13. README.md                                     ← independent
```

## Risk Assessment

| Risk | Mitigation |
|---|---|
| Prohibition check adds latency to CheckAccess | Prohibitions loaded into memory alongside graph. In-memory check, not DB query per request |
| Breaking existing consumers | All existing RPCs unchanged. New RPCs only. Proto backward compatible |
| Cache coherence with prohibitions | Prohibition mutations use SAME invalidation flow as assignments (verified in spec) |
| Large UA prohibition → mass invalidation | GetDescendants already O(N) where N = subtree size. Same perf as RemoveAssignment on UA |
