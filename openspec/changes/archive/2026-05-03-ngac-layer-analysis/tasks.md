# Tasks: NGAC Layer Analysis — Policy Service

## Phase 1: GraphReader Interface (PIP read boundary)

- [x] Create `internal/ngac/graph_reader.go` with `GraphReader` interface
- [x] Verify `*Graph` satisfies `GraphReader` (compile-time check)
- [x] Update `DecisionEngine` to accept `GraphReader` instead of `*Store`
- [x] Update `ReadServer` scope/graph RPCs to use `GraphReader` where applicable
- [x] Build + test verification

## Phase 2: Move CheckProhibitions to PDP (file relocation)

- [x] Move `CheckProhibitions()` from `prohibition.go` to `decision_engine.go`
- [x] Update imports if any external consumers reference the function
- [x] Verify `prohibition.go` is now pure PAP/PIP (only CRUD + query)
- [x] Build + test verification

## Phase 3: InvalidationCoordinator (write-path PIP)

- [x] Create `internal/ngac/invalidation.go` with `InvalidationCoordinator`
- [x] Implement `InvalidateForNodes()` — version bump + L2 + L1
- [x] Implement `InvalidateAll()` — full invalidation
- [x] Refactor `WriteServer` to use `InvalidationCoordinator`
- [x] Remove `incrementAndInvalidate()` and `invalidateAllCaches()` from `write_server.go`
- [x] Update `cmd/main.go` wiring
- [x] Build + test verification

## Phase 4: Store Responsibility Annotation (documentation)

- [x] Add PAP/PIP section comments to `store.go` methods
- [x] Add layer annotations to `graph.go` method groups
- [x] Verify no code changes — documentation only
