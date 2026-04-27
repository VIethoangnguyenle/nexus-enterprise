## ADDED Requirements

### Requirement: Ownership representation in NGAC
Workspace ownership SHALL be represented as assignment to a `{ws}_Owners` User Attribute within the workspace's Policy Class. The Owners UA SHALL be assigned to the Members UA (UA→UA), so owners automatically inherit all member permissions.

#### Scenario: Owner inherits member permissions
- **WHEN** a user is assigned to `{ws}_Owners`
- **THEN** they SHALL also have all permissions granted to `{ws}_Members` via the UA→UA assignment chain

### Requirement: Ownership transfer
A workspace owner SHALL be able to transfer ownership to another workspace member.

#### Scenario: Transfer ownership
- **WHEN** an owner calls `POST /api/workspaces/{id}/transfer-ownership` with new_owner_user_id
- **THEN** the new owner's NGAC node SHALL be assigned to `{ws}_Owners` AND the requesting owner's node SHALL be removed from `{ws}_Owners`

#### Scenario: Non-owner cannot transfer
- **WHEN** a non-owner attempts to transfer ownership
- **THEN** the request SHALL be denied with 403 Forbidden

#### Scenario: Target must be workspace member
- **WHEN** an owner attempts to transfer ownership to a user who is not a member of the workspace
- **THEN** the request SHALL be denied with 400 Bad Request

### Requirement: Last owner protection
The system SHALL prevent removing the last owner from a workspace.

#### Scenario: Cannot remove last owner
- **WHEN** there is only one user assigned to `{ws}_Owners` and a transfer or removal is attempted that would leave zero owners
- **THEN** the operation SHALL be denied with an error message explaining that at least one owner must exist

#### Scenario: Add co-owner then remove
- **WHEN** a workspace has 2 owners and one transfers ownership
- **THEN** the transfer SHALL succeed because one owner remains

### Requirement: Add co-owner
A workspace owner SHALL be able to add additional owners without removing themselves.

#### Scenario: Add co-owner
- **WHEN** an owner calls `POST /api/workspaces/{id}/owners` with user_id
- **THEN** the target user SHALL be assigned to `{ws}_Owners` in addition to existing owners

### Requirement: Remove co-owner
A workspace owner SHALL be able to remove another owner (but not themselves if they are the last).

#### Scenario: Remove co-owner
- **WHEN** an owner calls `DELETE /api/workspaces/{id}/owners/{userId}` for another owner
- **THEN** the target user SHALL be removed from `{ws}_Owners` but remain in their other workspace UAs
