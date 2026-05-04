# Policy Clean Code — Tasks

## Task 1: Fix Store.CreateAssignment (CS-01) 🔴 HIGH
- [x] Add `ValidateAssignment(a *Assignment) error` to `graph.go` — validate without mutating
- [x] Rewrite `Store.CreateAssignment` in `store.go` — validate → DB → graph (no add-remove-add)
- [x] Run tests: `go test ./internal/ngac/ -run TestCreateAssignment`

## Task 2: Extract prohibition invalidation helper (CS-02) 🔴 HIGH
- [x] Add `resolveProhibitionAffectedNodes(subjectID string, targetOAIDs []string) []string` to `write_server.go`
- [x] Refactor `CreateProhibition` to use the helper
- [x] Refactor `RemoveProhibition` to use the helper
- [x] Verify no behavior change: `go build ./...`

## Task 3: Add typed deny reasons (CS-05) 🟡 MEDIUM
- [x] Add `DenyReasonNodeNotFound` and `DenyReasonNoAssociation` constants to `models.go`
- [x] Update `access.go CheckAccess` to use constants
- [x] Update `decision_engine.go Decide` to match on constants
- [x] Run tests: `go test ./internal/ngac/ -run TestCA`

## Task 4: Use GraphReader in ReadServer helpers (CS-08) 🟡 MEDIUM
- [x] Change `collectUAAncestors` signature to use `GraphReader`
- [x] Change `collectTargetOAs` signature to use `GraphReader`
- [x] Change `resolveLeafOAs` signature to use `GraphReader`
- [x] Extract `hasOAChildren` helper (CS-04)
- [x] Verify: `go build ./internal/grpc/`

## Task 5: Replace fmt.Printf with slog (CS-10) 🟢 LOW
- [x] `store.go:121` — `fmt.Printf` → `slog.Warn`
- [x] `store.go:136` — `fmt.Printf` → `slog.Warn`

## Task 6: Remove dead code (CS-12) 🟢 LOW  
- [x] Remove `ConstraintDenial` struct from `models.go`
- [x] Remove `ConstraintDenied` field from `AccessExplanation`
- [x] Remove `ConstraintsChecked` field from `AccessExplanation`
- [x] Verify proto conversion in `read_server.go` doesn't reference removed fields
- [x] Run full test suite

## Task 7: Handle GetUserByNGACNodeID boundary (CS-07) 🟡 MEDIUM
- [x] Check if `GetUserByNGACNodeID` is called anywhere
- [x] If unused → remove
- [x] If used → add `// TODO: cross-service query, migrate to auth gRPC call` comment

## Task 8: Fix misleading comment (CS-06) 🟢 LOW
- [x] `cache_invalidator.go:119` — fix comment "no SCAN" → accurately describe SCAN usage

## Task 9: Final validation
- [x] `go build ./...`
- [x] `go test -v ./internal/ngac/` — all 51 tests pass
- [x] `go vet ./...`
- [x] Verify no cross-service imports remain clean
