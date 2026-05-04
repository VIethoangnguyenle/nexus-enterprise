## ADDED Requirements

### Requirement: ShardManager loads per-tenant graph on demand
The system SHALL provide a `ShardManager` that loads a tenant's graph from DB only when first requested, rather than loading all tenants at startup.

#### Scenario: First request to a cold workspace
- **WHEN** a CheckAccess request arrives for workspace W1 that has no shard loaded
- **THEN** the system SHALL serve the request via CTE SQL fallback AND trigger an async shard load for W1

#### Scenario: Subsequent request to a hot workspace
- **WHEN** a CheckAccess request arrives for workspace W1 that already has a shard loaded
- **THEN** the system SHALL serve the request via in-memory BFS traversal with latency ≤ 1ms

### Requirement: Shard contains complete tenant graph plus dependencies
The system SHALL load all nodes, assignments, and associations belonging to the tenant's PCs (primary + secondary) and PC_Global into a single self-contained `*Graph` instance.

#### Scenario: Tenant with secondary PCs
- **WHEN** workspace W1 has `PC_W1` (primary) and `PC_W1_Confidential` (secondary, tenant_id=W1)
- **THEN** the shard for W1 SHALL contain nodes and edges from BOTH PCs plus PC_Global

#### Scenario: Tenant with cross-tenant shared resources
- **WHEN** user in W1 has a Share_OA assigned to PC_Global
- **THEN** the shard for W1 SHALL include the Share_OA node and its PC_Global assignment

### Requirement: LRU eviction limits memory usage
The system SHALL maintain at most `max_shards` (configurable, default 1000) loaded shards. When the limit is reached, the least recently used shard SHALL be evicted.

#### Scenario: Cache full and new shard requested
- **WHEN** 1000 shards are loaded and a request arrives for workspace W1001
- **THEN** the least recently used shard SHALL be evicted and W1001's shard SHALL be loaded

#### Scenario: Evicted shard re-requested
- **WHEN** a previously evicted shard is requested again
- **THEN** the system SHALL reload the shard from DB and serve future requests via in-memory BFS

### Requirement: Shard invalidation on graph mutation
The system SHALL invalidate (unload) a shard when the underlying graph data is modified via write operations.

#### Scenario: Node added to a loaded shard's workspace
- **WHEN** a CreateNode or CreateAssignment modifies graph data for workspace W1 while W1's shard is loaded
- **THEN** the system SHALL invalidate W1's shard so the next request triggers a fresh load

### Requirement: LoadShard query traces from tenant PCs downward
The system SHALL use a recursive CTE query to discover all nodes reachable from the tenant's PC nodes (downward traversal via assignments), plus any connected secondary PCs.

#### Scenario: LoadShard for workspace with 1000 users and 50 departments
- **WHEN** LoadShard is called for a workspace with 1000 U nodes, 50 UA/OA nodes
- **THEN** the resulting shard SHALL contain all ~1100 nodes plus PC_Global nodes, loaded in a single DB round-trip
