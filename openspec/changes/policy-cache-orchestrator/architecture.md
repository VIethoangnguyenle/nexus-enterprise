# Architecture: Policy Cache Orchestrator

## Overview

Internal refactor of the read path in policy-service. No new services, no API changes, no DB changes.

## Component Architecture

```
                    gRPC boundary
                         │
  ┌──────────────────────┼──────────────────────────────────────┐
  │ ReadServer           │                                      │
  │ (transport)          │                                      │
  │                      ▼                                      │
  │  CheckAccess(req) ──▶ AccessEvaluator                      │
  │                       │                                     │
  │                       ├──▶ DecisionCache.Get()              │
  │                       │     ├── L1: Redis                   │
  │                       │     └── L2: Materialized + version  │
  │                       │                                     │
  │                       ├──▶ DecisionEngine.Decide()          │
  │                       │     ├── Graph.CheckAccess() (BFS)   │
  │                       │     ├── CTEEvaluator (SQL fallback) │
  │                       │     └── CheckProhibitions()         │
  │                       │                                     │
  │                       └──▶ DecisionCache.Set()              │
  │                             ├── L2: materialized.Store()    │
  │                             └── L1: redis.Set()             │
  │                                                             │
  │  Graph query RPCs ──▶ Store (unchanged)                     │
  └─────────────────────────────────────────────────────────────┘
```

## Data Flow

### Read Path (CheckAccess)
```
proto request → ReadServer → AccessEvaluator.Evaluate()
                                │
                                ├─ cache.Get() → [L1 hit] → return
                                │                [L2 hit] → promote to L1 → return
                                │                [miss]   → continue
                                │
                                ├─ engine.Decide() → BFS → CTE fallback → prohibition eval
                                │
                                └─ cache.Set() → write L2 + L1
                                
                                → return decision
```

### Write Path (unchanged)
No changes to `WriteServer`, `incrementAndInvalidate`, or cache invalidation.

## Dependency Direction (strict one-way)

```
ReadServer → AccessEvaluator → DecisionCache  → Redis, MaterializedAccess, VersionTracker
                              → DecisionEngine → Graph, CTEEvaluator, ProhibitionStore
```

No circular dependencies. Each component depends only on lower-level abstractions.

## File Mapping

| New file | Package | Contains |
|----------|---------|----------|
| `evaluator.go` | `ngac` | `AccessRequest`, `AccessEvaluator`, `NewAccessEvaluator()` |
| `decision_engine.go` | `ngac` | `DecisionEngine` interface, `decisionEngine` impl |
| `decision_cache.go` | `ngac` | `DecisionCache` interface, `layeredCache` impl |

| Modified file | Changes |
|---------------|---------|
| `read_server.go` | Remove 4 fields, simplify CheckAccess to 1-line delegation |
| `cmd/main.go` | Wire `AccessEvaluator` before passing to `NewReadServer` |
| `cmd/policy-read/main.go` | Same wiring update |

## Constraints

1. **No proto changes** — `AccessRequest` is an internal type, not exposed via gRPC
2. **No DB changes** — same tables, same queries
3. **Same metrics** — `CheckAccessTotal` and `CheckAccessDuration` labels preserved
4. **Same cache key format** — `ngac:access:{user}:{object}:{op}` unchanged
5. **Same TTL** — 30s Redis TTL unchanged
