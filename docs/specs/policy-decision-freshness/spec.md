# policy-decision-freshness

## Purpose
Keep every policy decision correct after the graph changes and safe when evaluation fails: no
replica serves a decision from a graph it has not refreshed, and no failure is ever turned into,
or remembered as, an answer.

## Requirements

### Requirement: Evaluation failures deny and are never cached
When any part of a decision cannot be evaluated (the CTE fallback for an OA not in memory, or the
shared computation itself), the PDP SHALL return DENY. Such a decision
SHALL be marked error-derived and SHALL NOT be written to the Redis decision cache (the only decision
cache: a runtime check never reads the database). The single and batch paths SHALL agree.

#### Scenario: CTE fallback errors on an otherwise allowed request
- **WHEN** the object is not in the in-memory graph and the CTE query fails
- **THEN** the decision is DENY
- **AND** the next request after the database recovers is evaluated afresh, not served from cache

### Requirement: Prohibitions are evaluated from memory
Prohibitions SHALL be loaded into the in-memory graph with it (`LoadGraph`, `ReloadGraph`) and
swapped in atomically together with it. The PDP SHALL evaluate them from the global graph and
SHALL NOT query the database for them. Decision order is unchanged: prohibitions are deny-overrides
applied only to an ALLOW, and the default is DENY. A reload that fails, including one that cannot
read prohibitions, SHALL leave the previous graph and prohibitions in place.

#### Scenario: Prohibition matches despite an association
- **WHEN** a prohibition on the caller (or one of its user attributes) covers the target and operation
- **THEN** the decision is DENY on both the single and the batch path

#### Scenario: Prohibition does not match
- **WHEN** a prohibition names another subject, another operation, or targets not among the object's containers
- **THEN** the ALLOW from the association stands

#### Scenario: Allow path does not touch the database
- **WHEN** an ALLOW is decided for an object in the in-memory graph
- **THEN** no database connection is used

#### Scenario: Prohibition on a DENY
- **WHEN** the graph already denies the request
- **THEN** the decision is the graph's DENY and carries no prohibition denial

### Requirement: The SQL fallback never overrides a prohibition
The SQL fallback (for an object or a user that is not in the in-memory graph) cannot evaluate prohibitions, and a node missing from memory has no ancestors there to match a prohibition against. Before the PDP promotes a DENY to ALLOW through the fallback, on the single and the batch path, it SHALL evaluate the prohibitions with the database's ancestors: the subjects are the user and their user attributes (from memory when the user is in the graph, from the database when not), the targets are the object and its object attributes (from memory when the object is in the graph, from the database when not), and the matching is the in-memory matching, intersection included. It SHALL deny only on a real match. Nothing is looked up when the graph holds no prohibition, and the object is looked up only when a prohibition covers the user for the requested operation, so a user, an operation or an object no prohibition reaches keeps the fallback's ALLOW exactly as before. A lookup that fails SHALL close the decision to an error-derived DENY that is not cached. A DENY from a matching prohibition is a policy answer: it is cached like any other DENY (30 s) and dropped by the invalidation that follows the graph change, so it does not outlive the replica catching up. The batch path computes this lazily, only when a node it needs is missing from memory, once per batch, and falls back for every object when the user is the missing node, as the single check does, so the two agree.

#### Scenario: Covered user, directory object missing from memory
- **WHEN** a prohibition covers the user's `read` of a subtree and the SQL fallback would answer ALLOW for an OA in that subtree that is not in memory
- **THEN** the decision is DENY on the single and the batch path

#### Scenario: Covered user, other object missing from memory
- **WHEN** the same user reads an OA missing from memory that the prohibition does not target (a drive OA)
- **THEN** the fallback's ALLOW stands

#### Scenario: User missing from memory
- **WHEN** a user absent from the in-memory graph is, in the database, under the prohibition's subject
- **THEN** the decision is DENY on both paths

#### Scenario: Uncovered operation or no prohibition
- **WHEN** no prohibition covers the user for the requested operation, or the graph holds none
- **THEN** the fallback's ALLOW stands and the database is not asked for ancestors

#### Scenario: Ancestor lookup fails
- **WHEN** the database cannot give the ancestors needed to match a covering prohibition
- **THEN** the decision is DENY, error-derived and not cached

### Requirement: Prohibition mutations write the database before memory and reach every replica
Creating or removing a prohibition SHALL write the database first, update the writer's in-memory
graph only after that write succeeds, then invalidate caches and publish `ngac.graph.mutated`
(`create_prohibition` / `remove_prohibition`). Each replica SHALL reload its graph, prohibitions
included, on that event.

#### Scenario: Prohibition created on the writer
- **WHEN** a prohibition is created through the writer
- **THEN** the writer denies at once
- **AND** `policy-read` denies after applying the event, without a restart

#### Scenario: Prohibition removed on the writer
- **WHEN** that prohibition is removed through the writer
- **THEN** the ALLOW returns on the writer at once and on `policy-read` after the event

#### Scenario: Database write fails
- **WHEN** creating or removing a prohibition fails in the database
- **THEN** the in-memory prohibitions are unchanged

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
