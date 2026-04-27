## ADDED Requirements

### Requirement: Lifecycle state transition
A user with the required NGAC permission for a transition SHALL be able to move an asset from one state to another according to the type's state machine definition.

#### Scenario: Valid transition with permission
- **WHEN** an IT admin calls `POST /api/assets/{id}/transition` with action "approve" on an asset in "requested" state
- **THEN** the system SHALL verify `CheckAccess(user, asset_oa, "approve")`, validate the transition is allowed by the state machine, update the state, record the transition in history, and emit `asset.lifecycle` Kafka event

#### Scenario: Invalid transition rejected
- **WHEN** a user attempts transition "dispose" on an asset in "requested" state (no such transition defined)
- **THEN** the system SHALL reject with 400 Bad Request listing valid transitions from current state

#### Scenario: No permission for transition
- **WHEN** a user without "approve" permission attempts to approve an asset
- **THEN** the system SHALL return 403 Forbidden with NGAC access explanation

### Requirement: Lifecycle history tracking
Every state transition SHALL be recorded with actor, timestamp, from/to states, and optional comment. The full history SHALL be queryable.

#### Scenario: View lifecycle history
- **WHEN** a user calls `GET /api/assets/{id}/history`
- **THEN** the system SHALL return chronological list of all state transitions with actor name, from_state, to_state, timestamp, and comment

#### Scenario: Transition with comment
- **WHEN** a user transitions an asset and includes comment "Approved for Engineering team"
- **THEN** the comment SHALL be stored with the transition record and visible in history

### Requirement: Available transitions query
A user SHALL be able to query which transitions are available for an asset based on its current state AND the user's NGAC permissions.

#### Scenario: Query available transitions
- **WHEN** an engineer calls `GET /api/assets/{id}/transitions`
- **THEN** the system SHALL return only transitions that are (1) valid from current state AND (2) the user has the required NGAC permission for
- **EXAMPLE** asset in "requested" state, user has "approve" but not "assign" → returns `[{action: "approve", to_state: "approved"}]`

#### Scenario: No available transitions
- **WHEN** a user with only "read" permission queries transitions on an asset in "in_use" state
- **THEN** the system SHALL return an empty list
