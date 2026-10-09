# policy-decision-freshness

## Purpose
Keep every policy decision correct after the graph changes and safe when evaluation fails: no
replica serves a decision from a graph it has not refreshed, and no failure is ever turned into,
or remembered as, an answer.

## Requirements

### Requirement: Evaluation failures deny and are never cached
When any part of a decision cannot be evaluated (the prohibition lookup, the CTE fallback for an
OA not in memory, or the shared computation itself), the PDP SHALL return DENY. Such a decision
SHALL be marked error-derived and SHALL NOT be written to the Redis decision cache or the
materialized cache. The single and batch paths SHALL agree.

#### Scenario: Prohibition store errors on an otherwise allowed request
- **WHEN** the caller holds a matching association but the prohibition lookup fails
- **THEN** the decision is DENY
- **AND** the next request after the store recovers is evaluated afresh, not served from cache

#### Scenario: Prohibition matches despite an association
- **WHEN** a prohibition on the caller covers the target and operation
- **THEN** the decision is DENY on both the single and the batch path

### Requirement: Shared evaluations do not inherit a caller's cancellation
Concurrent identical requests MAY share one evaluation. That evaluation SHALL run on its own
context with its own timeout, so one caller cancelling cannot fail or poison the result for the
others.

#### Scenario: First caller cancels
- **WHEN** two identical requests share an evaluation and the first caller cancels
- **THEN** the first caller gets an error-derived DENY that is not cached
- **AND** the second caller receives the correct decision

### Requirement: Every PDP replica refreshes on graph mutations
Each replica (the writer and every `policy-read` instance) SHALL consume `ngac.graph.mutated`
independently (no shared consumer group), reload its graph atomically, and apply the same
invalidation the writer applies. A full graph reload on the writer SHALL be propagated as a
`load_graph` event. A reload that fails SHALL leave the previous graph in place.

#### Scenario: Revocation reaches a read replica
- **WHEN** an association is removed through the writer
- **THEN** `policy-read` returns DENY for that user and object without a restart

### Requirement: Cache keys and invalidation patterns share one builder
Decision and scope cache keys SHALL be built by one function set that the invalidator also uses,
with a workspace segment always present (`_global` when the request names none), so that every
invalidation pattern provably matches the keys it is meant to remove.

#### Scenario: User invalidation
- **WHEN** a user's assignments change
- **THEN** that user's cached decisions are removed for every workspace and for `_global`
- **AND** other users' keys survive

### Requirement: Mutations write the database before memory
Every graph mutation SHALL write the database first and update the in-memory graph only after
the write succeeds. Node deletion SHALL resolve the affected workspaces and shards before the
delete and invalidate them after it.

#### Scenario: Database write fails
- **WHEN** removing an association fails in the database
- **THEN** the in-memory graph still contains the association
