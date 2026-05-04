## ADDED Requirements

### Requirement: SQL function ngac_check_access exists and evaluates access
The system SHALL provide a PostgreSQL function `ngac_check_access(user_id TEXT, object_id TEXT, operation TEXT)` that evaluates NGAC access decisions using recursive CTE without requiring in-memory graph.

#### Scenario: Cold workspace access check via CTE
- **WHEN** CheckAccess is called for a workspace with no loaded shard
- **THEN** the system SHALL evaluate access using `ngac_check_access` SQL function and return correct ALLOW/DENY within 10ms

#### Scenario: CTE returns same result as BFS
- **WHEN** both BFS (in-memory) and CTE (SQL) evaluate the same access request
- **THEN** both SHALL return the identical ALLOW/DENY decision

### Requirement: CTE evaluates PC intersection correctly
The SQL function SHALL implement ALL-PC intersection: user MUST reach ALL PCs that the object reaches for ALLOW.

#### Scenario: Object belongs to single PC
- **WHEN** object OA reaches only PC_W1 and user reaches PC_W1
- **THEN** CTE SHALL return ALLOW (if matching association exists)

#### Scenario: Object belongs to multiple PCs
- **WHEN** object OA reaches PC_W1 and PC_W1_Confidential, but user only reaches PC_W1
- **THEN** CTE SHALL return DENY

### Requirement: Auto-promote cold to hot after CTE evaluation
The system SHALL trigger async shard loading after a successful CTE evaluation for a cold workspace.

#### Scenario: First request promotes workspace
- **WHEN** CTE serves a request for cold workspace W1
- **THEN** an async LoadShard(W1) SHALL be triggered so subsequent requests use in-memory BFS
