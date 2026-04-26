## ADDED Requirements

### Requirement: NGAC graph node types
The system SHALL support five node types: User (U), User Attribute (UA), Object (O), Object Attribute (OA), and Policy Class (PC). Each node SHALL have a unique ID, name, type, and optional JSONB properties.

#### Scenario: Create each node type
- **WHEN** a node is created with type U, UA, O, OA, or PC
- **THEN** the node is persisted in the database and loaded into the in-memory graph

#### Scenario: Reject invalid node type
- **WHEN** a node creation is attempted with an invalid type
- **THEN** the system SHALL return an error

### Requirement: NGAC assignments
The system SHALL support assignment edges representing containment: U→UA, UA→UA, O→OA, OA→OA, UA→PC, OA→PC. Assignments form a directed acyclic graph.

#### Scenario: Create valid assignment
- **WHEN** an assignment is created from a child node to a valid parent node
- **THEN** the edge is persisted and the in-memory graph is updated

#### Scenario: Prevent invalid assignment types
- **WHEN** an assignment is attempted between incompatible node types (e.g., U→O)
- **THEN** the system SHALL reject it with an error

#### Scenario: Prevent circular assignments
- **WHEN** an assignment would create a cycle in the graph
- **THEN** the system SHALL reject it with an error

### Requirement: NGAC associations
The system SHALL support association edges linking UA to OA with a set of operations (read, write, approve, upload, share). Associations represent permissions.

#### Scenario: Create association
- **WHEN** an association is created from UA to OA with operations [read, write]
- **THEN** the association is persisted and queryable during access decisions

#### Scenario: Update association operations
- **WHEN** an existing association's operations are updated
- **THEN** subsequent access checks reflect the new operations

### Requirement: Access decision via graph traversal
The system SHALL determine access by traversing the graph: find all UAs reachable from user (upward through assignments), find all OAs reachable from object (upward through assignments), check if any UA→OA association exists with the requested operation, and verify both paths reach a common Policy Class.

#### Scenario: Allow when path exists
- **WHEN** user "alice" belongs to UA "Acme_Finance" which has association [read] to OA "Acme_Invoices" which contains object "invoice.pdf", and both are under PC "PC_Acme"
- **THEN** the access check for (alice, invoice.pdf, read) SHALL return ALLOW

#### Scenario: Deny when no path exists
- **WHEN** user "charlie" belongs to UA "Beta_Engineering" and object "invoice.pdf" is under OA "Acme_Invoices" with no association from Beta_Engineering to Acme_Invoices
- **THEN** the access check SHALL return DENY

### Requirement: In-memory graph with persistent backing
The system SHALL load the full NGAC graph from PostgreSQL into memory on startup. All graph mutations SHALL write-through to both the in-memory graph and PostgreSQL.

#### Scenario: Graph available after restart
- **WHEN** the backend service restarts
- **THEN** the in-memory graph SHALL be fully rebuilt from PostgreSQL data

#### Scenario: Write-through on mutation
- **WHEN** a new node or edge is added to the graph
- **THEN** both the in-memory graph and PostgreSQL SHALL be updated atomically
