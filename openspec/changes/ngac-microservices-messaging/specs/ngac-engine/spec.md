## MODIFIED Requirements

### Requirement: NGAC Policy Service
The NGAC engine SHALL operate as a standalone gRPC service (Policy Service). The core graph traversal algorithm, access check logic, and constraint evaluation SHALL remain unchanged. The service SHALL expose all graph operations via gRPC.

#### Scenario: Access check via gRPC
- **WHEN** any service calls `CheckAccess(user_node_id, object_node_id, operation)` via gRPC
- **THEN** the Policy Service SHALL perform the same in-memory graph traversal as the current monolith and return an AccessDecision with explanation

#### Scenario: Graph mutation via gRPC
- **WHEN** a service calls `CreateNode`, `CreateAssignment`, or `CreateAssociation` via gRPC
- **THEN** the Policy Service SHALL validate the operation, persist to PostgreSQL, and update the in-memory graph (write-through)

#### Scenario: Constraint evaluation preserved
- **WHEN** an access check is performed for a write operation on a weekend
- **THEN** the WeekdayOnlyConstraint SHALL still deny access, same as the current system

### Requirement: New operations
The system SHALL support the following additional NGAC operations beyond the existing set (read, write, upload, approve, share):
- `manage` — workspace management operations
- `invite` — invite users to workspace
- `create_channel` — create new channels within workspace

#### Scenario: Manage operation check
- **WHEN** `CheckAccess(user, ws_mgmt_oa, "manage")` is called
- **THEN** the Policy Service SHALL evaluate it using the same graph traversal algorithm as any other operation

### Requirement: Graph query operations
The Policy Service SHALL expose query operations for traversing the graph.

#### Scenario: Find user's workspaces
- **WHEN** a service calls `GetAncestors` for a user node and filters by PC type
- **THEN** the Policy Service SHALL return all Policy Classes reachable from the user (i.e., all workspaces they belong to)

#### Scenario: Find workspace members
- **WHEN** a service calls `GetDescendants` for a workspace PC and filters by User type
- **THEN** the Policy Service SHALL return all User nodes under that PC's UA hierarchy
