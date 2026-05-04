# Design: NGAC Layer Analysis — Policy Service

## Architecture Overview

```
                        policy-service/internal/
                        ┌──────────────────────────────────────────────┐
                        │  grpc/                                       │
                        │  ├── read_server.go   ← uses evaluator (✅)  │
                        │  └── write_server.go  ← uses coordinator    │
                        │                                              │
                        │  ngac/                                       │
                        │  ├── evaluator.go     ← orchestrator (✅)    │
                        │  ├── decision_engine.go  ← PDP (✅ + move)   │
                        │  ├── decision_cache.go   ← PIP cache (✅)    │
                        │  ├── graph.go         ← PIP data + PAP mut  │
                        │  ├── graph_reader.go  ← NEW: PIP read iface │
                        │  ├── invalidation.go  ← NEW: write-path PIP │
                        │  ├── store.go         ← PAP+PIP (annotated) │
                        │  ├── prohibition.go   ← PAP only (CRUD)     │
                        │  ├── materialized.go  ← PIP L2              │
                        │  ├── cache_invalidator.go ← PIP L1          │
                        │  └── version.go       ← PIP version         │
                        └──────────────────────────────────────────────┘
```

---

## Change 1: GraphReader Interface

### Problem
`Graph` has 21 methods mixing PIP reads, PAP mutations, and PDP logic. Any consumer gets full mutation access.

### Solution
Extract a `GraphReader` interface containing only read methods. PDP components depend on `GraphReader`, not `*Graph`.

### New File: `graph_reader.go`

```go
// GraphReader provides read-only access to the NGAC graph (PIP).
// PDP components should depend on this interface, not *Graph.
type GraphReader interface {
    GetNode(id string) *NGACNode
    FindNodeByName(name, nodeType string) *NGACNode
    GetAncestors(nodeID string) map[string]*NGACNode
    GetDescendants(nodeID string) map[string]*NGACNode
    GetChildren(nodeID string) []*NGACNode
    GetParents(nodeID string) []*NGACNode
    GetNodesByType(nodeType string) []*NGACNode
    GetAssociationsFromUA(uaID string) []*Association
    IsAssigned(childID, parentID string) bool
    CheckAccess(userID, objectID, operation string) *AccessDecision
}
```

### Impact
- `DecisionEngine` changes: `store.GetGraph()` → `GraphReader` dependency
- `ReadServer` scope methods: uses `GraphReader` for traversal
- Zero behavior change — `*Graph` already satisfies this interface

---

## Change 2: Move CheckProhibitions to PDP

### Problem
`CheckProhibitions()` is a pure PDP function living in `prohibition.go` alongside PAP CRUD.

### Solution
Move `CheckProhibitions()` from `prohibition.go` to `decision_engine.go` where it belongs.

### File Changes
- **prohibition.go**: Remove `CheckProhibitions()` function (lines 146-179)
- **decision_engine.go**: Already imports and uses it internally — make it a private method or keep as package function in this file

### Impact
- `prohibition.go` becomes pure PAP/PIP (CRUD only)
- `decision_engine.go` contains all PDP logic in one place
- No signature changes — just file relocation

---

## Change 3: InvalidationCoordinator (Write-Path PIP)

### Problem
`WriteServer.incrementAndInvalidate()` directly orchestrates version bump + L2 invalidation + L1 invalidation. Transport knows PIP cache topology.

### Solution
Extract `InvalidationCoordinator` to hide cache invalidation internals.

### New File: `invalidation.go`

```go
// InvalidationCoordinator coordinates cache invalidation across all PIP layers.
// Hides L1/L2/version topology from the transport layer.
type InvalidationCoordinator struct {
    version      *VersionTracker
    materialized *MaterializedAccess
    cache        *CacheInvalidator
}

func NewInvalidationCoordinator(
    version *VersionTracker,
    materialized *MaterializedAccess,
    cache *CacheInvalidator,
) *InvalidationCoordinator

// InvalidateForNodes increments version + invalidates L2 + invalidates L1 for affected nodes.
func (c *InvalidationCoordinator) InvalidateForNodes(ctx context.Context, nodeIDs ...string)

// InvalidateAll invalidates all caches (version + full L2 + full L1).
func (c *InvalidationCoordinator) InvalidateAll(ctx context.Context)
```

### WriteServer After
```go
type WriteServer struct {
    store         *Store
    producer      *events.Producer
    invalidation  *InvalidationCoordinator  // replaces version + materialized + cache
    operations    *OperationStore
    prohibitions  *ProhibitionStore
    strictOps     bool
}
```

### Impact
- `WriteServer` struct drops from 8→6 fields
- `incrementAndInvalidate()` becomes 1-line: `s.invalidation.InvalidateForNodes(ctx, nodeIDs...)`
- `invalidateAllCaches()` becomes: `s.invalidation.InvalidateAll(ctx)`

---

## Change 4: Store Responsibility Annotation

### Problem
`Store` methods mix PAP and PIP responsibilities silently.

### Solution
Add doc comments categorizing each method. No code restructuring (too risky).

```go
// --- PAP: Policy Administration ---

// CreateNode creates a graph node (PAP).
// Side effect: writes to PIP (DB + in-memory graph).

// --- PIP: Policy Information ---

// LoadGraph hydrates the in-memory graph from DB (PIP).
// GetGraph returns the in-memory graph reference (PIP).
```

### Impact
- Documentation only — zero code changes
- Future refactors can reference these annotations

---

## Dependency Direction (After All Changes)

```
ReadServer  → AccessEvaluator → DecisionCache (PIP)
                               → DecisionEngine (PDP) → GraphReader (PIP interface)
                                                       → ProhibitionStore (PIP query)

WriteServer → Store (PAP+PIP)
            → InvalidationCoordinator (PIP)
            → Events (cross-cutting)
```

No circular dependencies. Each arrow is one-directional.
