# Spec: Decouple Domain-Specific Code

## Stories

### S1: Remove hardcoded operation constants

As a platform engineer integrating policy-service into a new system,
I want the service to have ZERO domain-specific operation constants,
so that I can define my own operations without modifying policy-service source code.

**Acceptance Criteria:**
- [ ] `models.go` contains NO `Op*` constants (OpRead, OpApprove, etc.)
- [ ] Existing tests that reference `OpRead` etc. use inline strings instead
- [ ] Service compiles and all tests pass after removal

**Proto mapping:** No proto change needed — operations are already strings in proto messages.

### S2: Remove constraint engine

As a platform engineer,
I want business rule enforcement to be my responsibility (consumer-side),
so that policy-service remains a pure NGAC graph engine.

**Acceptance Criteria:**
- [ ] `constraints.go` file is DELETED
- [ ] `ConstraintEngine` field removed from `PolicyServer` struct
- [ ] `ENABLE_WEEKDAY_CONSTRAINT` env var removed from `cmd/main.go`
- [ ] `CheckAccess` in `server.go` no longer calls constraint evaluation
- [ ] Proto messages `ConstraintDenial`, `constraints_checked` remain (backward compat) but are never populated by policy-service
- [ ] Service compiles and all tests pass

**Proto mapping:** `AccessExplanation.constraints_checked` and `ConstraintDenial` — keep in proto (no breaking change), just never set by service.

### S3: Remove domain-specific seed data

As a platform engineer deploying policy-service for a new system,
I want no pre-loaded domain data,
so that I build my own NGAC graph from scratch.

**Acceptance Criteria:**
- [ ] `init.sql` seed INSERT statements for ngac_nodes/assignments/associations are removed or moved to a separate `seed-example.sql`
- [ ] Service starts with empty graph when no seed data provided
- [ ] `LoadGraph` returns 0 nodes on fresh DB

---

## State Inventory

| State | Behavior |
|---|---|
| Fresh deploy | Empty graph, 0 nodes, service operational |
| After consumer seeds graph | Graph populated via gRPC calls |

## Flow

```
1. Deploy policy-service with empty DB
2. Consumer calls CreateNode, CreateAssignment, CreateAssociation via gRPC
3. Consumer calls CheckAccess → decisions based on consumer-defined graph
4. No domain assumptions in policy-service
```
