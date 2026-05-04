## MODIFIED Requirements

### Requirement: CheckAccessRequest includes optional workspace routing
The `CheckAccessRequest` proto message SHALL include an optional `workspace_id` field to enable shard routing.

#### Scenario: Request with workspace_id (hot path)
- **WHEN** CheckAccess is called with workspace_id=W1 and W1's shard is loaded
- **THEN** the system SHALL route directly to W1's shard for in-memory BFS evaluation

#### Scenario: Request with workspace_id (cold path)
- **WHEN** CheckAccess is called with workspace_id=W1 and W1's shard is NOT loaded
- **THEN** the system SHALL evaluate via CTE SQL fallback and trigger async shard load

#### Scenario: Request without workspace_id (backward compatible)
- **WHEN** CheckAccess is called WITHOUT workspace_id
- **THEN** the system SHALL evaluate via CTE SQL fallback (no shard routing possible)
