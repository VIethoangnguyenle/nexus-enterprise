## ADDED Requirements

### Requirement: Create asset
A user with `write` permission on the asset type's OA SHALL be able to create a new asset instance with required custom fields. The asset SHALL start in the initial lifecycle state.

#### Scenario: Create asset with valid fields
- **WHEN** a user calls `POST /api/workspaces/{id}/assets` with type_id, name "MacBook Pro #042", and custom fields `{"serial_number": "C02X...", "ram_gb": 16, "storage_gb": 512}`
- **THEN** the system SHALL validate fields against the type's JSON Schema, create the asset in initial state, create an NGAC Object node assigned to the type's OA, and emit `asset.lifecycle` Kafka event

#### Scenario: Invalid custom fields rejected
- **WHEN** a user submits an asset with fields that don't match the type's JSON Schema (e.g., missing required field, wrong type)
- **THEN** the system SHALL reject with 400 and list specific validation errors

#### Scenario: No write permission denied
- **WHEN** a user without `write` permission on the type's OA attempts to create an asset
- **THEN** the system SHALL return 403 Forbidden with NGAC access explanation

### Requirement: Update asset
A user with `write` permission on the specific asset's OA SHALL be able to update mutable fields. Asset state SHALL NOT be changed via update — use lifecycle transitions.

#### Scenario: Update asset custom fields
- **WHEN** an authorized user calls `PATCH /api/assets/{id}` with updated fields
- **THEN** the system SHALL validate against JSON Schema, update the asset, and return the updated record

#### Scenario: Cannot update state via PATCH
- **WHEN** a user includes `state` in the PATCH body
- **THEN** the system SHALL ignore the state field (state changes only via lifecycle transition API)

### Requirement: List assets with filtering
A user SHALL be able to list assets they have read access to, filtered by type, state, category, or assigned user.

#### Scenario: List all accessible assets
- **WHEN** a user calls `GET /api/workspaces/{id}/assets`
- **THEN** the system SHALL return only assets where `CheckAccess(user, asset_oa, "read")` returns ALLOW, paginated with limit/offset

#### Scenario: Filter by type and state
- **WHEN** a user calls `GET /api/workspaces/{id}/assets?type=laptop&state=in_use`
- **THEN** the system SHALL return only Laptop assets currently in "in_use" state that the user can read

#### Scenario: Filter by assigned user
- **WHEN** a user calls `GET /api/workspaces/{id}/assets?assigned_to={userId}`
- **THEN** the system SHALL return assets currently assigned to the specified user

### Requirement: Get asset detail
A user with `read` permission SHALL be able to view full asset details including custom fields, current state, assignment history, and lifecycle timeline.

#### Scenario: View asset detail
- **WHEN** an authorized user calls `GET /api/assets/{id}`
- **THEN** the system SHALL return the asset with all custom fields, current state, assigned user (if any), created/updated timestamps, and lifecycle history

#### Scenario: No read permission denied
- **WHEN** a user without `read` permission attempts to view an asset
- **THEN** the system SHALL return 403 Forbidden

### Requirement: Delete asset
A user with `dispose` permission SHALL be able to soft-delete an asset. Deleted assets SHALL be excluded from listings but retained for audit.

#### Scenario: Soft-delete asset
- **WHEN** an authorized user calls `DELETE /api/assets/{id}`
- **THEN** the system SHALL mark the asset as deleted, remove its NGAC Object node assignments, and emit `asset.lifecycle` event with state "deleted"

#### Scenario: Deleted assets excluded from listings
- **WHEN** a user lists assets after one has been deleted
- **THEN** the deleted asset SHALL NOT appear in the listing results
