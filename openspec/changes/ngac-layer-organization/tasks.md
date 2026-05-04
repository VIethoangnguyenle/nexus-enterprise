# Tasks: NGAC Layer Organization

## Phase A: Simple renames (git mv, no split)
- [x] A1: `git mv access.go pdp_access.go`
- [x] A2: `git mv decision_engine.go pdp_decision_engine.go`
- [x] A3: `git mv decision_engine_test.go pdp_decision_engine_test.go`
- [x] A4: `git mv evaluator.go pdp_evaluator.go`
- [x] A5: `git mv decision_cache.go pdp_decision_cache.go`
- [x] A6: `git mv graph_reader.go pip_graph_reader.go`
- [x] A7: `git mv cte.go pip_cte.go`
- [x] A8: `git mv materialized.go pip_materialized.go`
- [x] A9: `git mv shard_manager.go pip_shard_manager.go`
- [x] A10: `git mv shard_manager_test.go pip_shard_manager_test.go`
- [x] A11: `git mv prohibition.go pap_prohibition.go`
- [x] A12: `git mv operations.go pap_operations.go`
- [x] A13: `git mv cache_invalidator.go epp_cache_invalidator.go`
- [x] A14: `git mv invalidation.go epp_invalidation.go`
- [x] A15: `git mv version.go epp_version.go`
- [x] A16: `git mv vnpay_graph_test.go pdp_vnpay_graph_test.go`
- [x] A17: `git mv vnpay_scenarios_test.go pdp_vnpay_scenarios_test.go`

## Phase B: Split graph.go → pip_graph.go + pap_graph.go
- [x] B1: Create `pip_graph.go` — Graph struct + NewGraph + all read methods + BFS helpers
- [x] B2: Create `pap_graph.go` — all mutation methods + internal helpers (cycle detection, index cleanup)
- [x] B3: Delete original `graph.go`
- [x] B4: Verify: `go build ./...`

## Phase C: Split store.go → pip_store.go + pap_store.go
- [x] C1: Create `pip_store.go` — Store struct + NewStore + LoadGraph + all read methods
- [x] C2: Create `pap_store.go` — InitSchema + all write methods
- [x] C3: Delete original `store.go`
- [x] C4: Verify: `go build ./...`

## Phase D: Split store_test.go → pip_store_test.go + pap_store_test.go + pdp_access_test.go
- [x] D1: Read `store_test.go` — categorize each test function as PIP, PAP, or PDP
- [x] D2: Create `pip_store_test.go` — shared helpers + tests for FindNodeByName
- [x] D3: Create `pap_store_test.go` — tests for CreateNode, CreateAssignment, CreateAssociation
- [x] D4: Create `pdp_access_test.go` — tests for CheckAccess (ALLOW/DENY/multi-PC/cross-workspace)
- [x] D5: Delete original `store_test.go`

## Phase E: Verification
- [x] E1: `go build ./...` passes
- [x] E2: `go vet ./...` — no warnings
- [x] E3: Confirm no remaining unprefixed files (except models.go, export_test_helpers.go)
