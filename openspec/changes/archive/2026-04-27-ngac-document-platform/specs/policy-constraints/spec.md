## ADDED Requirements

### Requirement: Time-based editing constraint
The system SHALL enforce a policy constraint that denies write and upload operations on weekends (Saturday and Sunday). This constraint SHALL be implemented as a separate evaluation layer, NOT hardcoded in business logic.

#### Scenario: Write allowed on weekday
- **WHEN** a user attempts to write a document on a Monday
- **THEN** the constraint layer does NOT block the operation

#### Scenario: Write denied on weekend
- **WHEN** a user attempts to write a document on a Saturday
- **THEN** the constraint layer denies the operation with explanation "Editing is only allowed Monday-Friday"

### Requirement: Constraint evaluation order
The system SHALL evaluate constraints AFTER graph traversal. A request is allowed only if: (1) the NGAC graph traversal finds a valid path, AND (2) no active constraint denies the operation.

#### Scenario: Graph allows but constraint denies
- **WHEN** a user has valid graph access for write but the current time is Saturday
- **THEN** the final decision is DENY with explanation including both the found graph path and the triggered constraint

### Requirement: Constraint extensibility
The system SHALL support registering multiple policy constraints. Each constraint SHALL define a name, target operations, condition function, and effect (deny).

#### Scenario: Multiple constraints evaluated
- **WHEN** an access check is performed
- **THEN** all registered constraints applicable to the requested operation are evaluated
