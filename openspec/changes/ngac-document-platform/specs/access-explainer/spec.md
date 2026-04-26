## ADDED Requirements

### Requirement: Access decision explanation
The system SHALL return a human-readable explanation with every access decision. For ALLOW, the explanation SHALL include the full traversal path from user through UAs, associations, and OAs to the object. For DENY, it SHALL explain why no valid path was found.

#### Scenario: ALLOW explanation
- **WHEN** access is granted for (alice, invoice.pdf, read)
- **THEN** the response includes the path: "alice → Acme_Finance (UA) → association[read] → Acme_Invoices (OA) ← invoice.pdf, policy class: PC_Acme"

#### Scenario: DENY explanation
- **WHEN** access is denied for (charlie, invoice.pdf, read)
- **THEN** the response includes: user attributes found, object attributes found, no matching association, and which policy classes were checked

### Requirement: Constraint explanation
The system SHALL include constraint evaluation results in the access explanation. If a constraint blocked access, the explanation SHALL name the constraint and describe why it triggered.

#### Scenario: Constraint denial explained
- **WHEN** access is denied due to weekend constraint
- **THEN** the explanation includes: "Constraint 'weekday-only-editing' denied operation 'write': Editing is only allowed Monday-Friday"

### Requirement: Dedicated access check endpoint
The system SHALL provide a POST /api/access/check endpoint that accepts user_id, object_id, and operation, returning the decision and full explanation without performing the actual operation.

#### Scenario: Debug access check
- **WHEN** an admin calls POST /api/access/check with {user_id, object_id, operation}
- **THEN** the system returns {decision: "ALLOW"|"DENY", explanation: {...}} without side effects
