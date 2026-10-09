# Phase 03b report: prohibitions in the in-memory graph (2026-10-10)

## Result
Prohibitions load with the graph, swap atomically on ReloadGraph, and are evaluated from memory in
the single and batch paths. The ALLOW path no longer queries the database. Nothing committed or staged.

## Design
- `Graph` holds `prohibitions` (by name) and a `prohibitionsBySubject` index (pip_graph.go, pap_graph.go).
  `AddProhibition` (validates, clones, replace-by-name), `RemoveProhibition`, `ProhibitionsForSubjects`
  (sorted by name so the reported denial is deterministic), `ProhibitionCount`. `replaceWith` swaps them with the graph.
- `GraphReader` gained `ProhibitionsForSubjects`. The engine reads prohibitions from its GLOBAL graph while
  ancestors come from the resolved (shard) graph. Shard graphs carry none; coverage equals the old DB query.
- `loadGraphInto` loads `ngac_prohibitions` (rows.Err checked). An invalid row fails the load (startup exits,
  or reload keeps the previous graph) instead of silently dropping a deny rule.
- `NewDecisionEngine(graph, cte)`: third arg and `ProhibitionFinder` removed; `FindForSubjects` deleted.
- `NewProhibitionStore(pool, graph)`: Create = DB insert, then `graph.AddProhibition`; Remove = DB delete, then
  `graph.RemoveProhibition`. write_server already invalidated caches and published `create_prohibition` /
  `remove_prohibition` afterwards, so order is DB -> memory -> invalidate -> EPP event. No new event kind needed:
  the replica refresher reloads the whole graph on any event, which now includes prohibitions.
- cmd/main.go and cmd/policy-read/main.go: wiring only.
- Removed dead `DenyReasonProhibitionCheckFailed` (no store-error path remains).

## Fail-closed tests changed (justification)
The prohibition-store-error tests (single, batch, evaluator, cancellation) covered a failure mode that no longer
exists on the decision path. Replaced, not weakened:
- store error -> failed CTE fallback via `flakyCTE` (error-derived DENY not cached, recovers afterwards);
- the singleflight cancellation test now blocks on the CTE query (same property: the shared computation is
  detached from the first caller's cancellation);
- new: failed ReloadGraph keeps previous prohibitions and still denies; Create/Remove DB failure leaves memory
  unchanged; a load failure never yields a partial prohibition set.

## Tests
New file `backend/services/policy/internal/ngac/pdp_prohibitions_memory_test.go`:
1. match -> DENY despite association (single + batch);
2. non-matching (subject / operation / target / partial intersection) -> ALLOW stands;
3. ALLOW path: `pool.Stat().AcquireCount()` unchanged across 20 Decide + DecideBatch on a DB-backed graph;
4. create on writer -> writer DENY at once, replica stale until `ReplicaGraphRefresher.Apply`, then DENY, caches
   invalidated for the affected nodes;
5. remove -> ALLOW returns on writer immediately and on replica after the EPP event;
plus: prohibition never touches an existing DENY, validation / replace-by-name / copy semantics, duplicate name
leaves first intact, reload swap and deletion, LoadGraph loads them.
Existing tests (failclosed, vnpay PH-01, replica) converted to graph-held prohibitions.
Red first: the new test file failed to compile (`AddProhibition` undefined) before implementation.

## Verification
- `make test s=policy` (strict, zero skips): pass. `go test ./... -race` in policy: 178 passed.
- `make build-check`: pass. `make test` (all services): pass, exit 0.

## Memory impact
Dev DB `ngac_prohibitions` count: 0. Estimated cost per prohibition about 200-400 B (struct, op/target slices,
two map entries) per process (writer and each policy-read). 100k prohibitions would be tens of MB; the old design
paid a DB round-trip on every ALLOW.

## Files
backend/services/policy/internal/ngac: pip_graph.go, pap_graph.go, pip_graph_reader.go, pip_store.go,
pap_prohibition.go, pdp_decision_engine.go, pdp_decision_batch.go, pdp_evaluator.go (comment), models.go.
Tests: pdp_prohibitions_memory_test.go (new), pdp_fakes_test.go, pdp_decision_engine_failclosed_test.go,
pdp_evaluator_failclosed_test.go, pdp_decision_engine_test.go, pdp_vnpay_scenarios_test.go, epp_replica_refresh_test.go.
backend/services/policy/cmd/main.go, cmd/policy-read/main.go. docs/specs/policy-decision-freshness/spec.md.

## Concerns
- `docs/specs/batch-access-check/spec.md` lines 43-44 still describe "Prohibition store unavailable during a
  batch" (now obsolete). Outside my allowed files; needs a small edit by whoever owns it.
- Between a DB write and a replica applying the EPP event, the replica serves its old prohibition set (same
  eventual-consistency window as associations; unchanged guarantee).
- `LoadGraph` now requires `ngac_prohibitions` to exist (created by init.sql / InitSchema).
- Pre-existing gofmt drift in pdp_evaluator_singleflight_test.go left alone.
