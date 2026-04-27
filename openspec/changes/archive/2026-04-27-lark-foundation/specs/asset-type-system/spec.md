## ADDED Requirements

### Requirement: Define asset type
An admin with `manage` permission on the workspace Assets OA SHALL be able to create a new asset type with a name, description, category, custom fields schema (JSON Schema), and lifecycle state machine definition.

#### Scenario: Create asset type with custom fields
- **WHEN** an admin calls `POST /api/workspaces/{id}/asset-types` with name "Laptop", category "IT Equipment", and fields schema `{"properties": {"serial_number": {"type": "string"}, "ram_gb": {"type": "integer"}, "storage_gb": {"type": "integer"}}}`
- **THEN** the system SHALL create the asset type, validate the JSON Schema, create corresponding NGAC OA nodes (`{workspace}_Laptops` under `{workspace}_IT_Equipment`), and return the type definition

#### Scenario: Invalid JSON Schema rejected
- **WHEN** an admin submits an asset type with malformed JSON Schema
- **THEN** the system SHALL reject with 400 Bad Request and a descriptive validation error

#### Scenario: Duplicate type name rejected
- **WHEN** an admin creates a type with a name that already exists in the workspace
- **THEN** the system SHALL reject with 409 Conflict

### Requirement: Configure lifecycle states
Each asset type SHALL have a configurable lifecycle state machine defining valid states and transitions. Each transition SHALL map to an NGAC permission.

#### Scenario: Define custom lifecycle
- **WHEN** an admin creates asset type "Software License" with states `[active, suspended, expired, revoked]` and transitions `[{active→suspended, permission: manage}, {active→expired, permission: manage}, {suspended→active, permission: manage}, {*→revoked, permission: admin}]`
- **THEN** the system SHALL validate the state machine (all states reachable, no orphans) and store it with the type

#### Scenario: Default lifecycle when none specified
- **WHEN** an admin creates an asset type without specifying a lifecycle
- **THEN** the system SHALL apply the default lifecycle: `requested → approved → assigned → in_use → returned → disposed`

#### Scenario: Invalid state machine rejected
- **WHEN** a lifecycle definition contains unreachable states or missing initial state
- **THEN** the system SHALL reject with 400 and describe which states are unreachable

### Requirement: Update asset type schema
An admin SHALL be able to add new fields to an existing asset type. Removing required fields SHALL NOT be allowed if assets of that type already exist.

#### Scenario: Add optional field to existing type
- **WHEN** an admin adds field `color: {type: "string"}` to the Laptop type that has 10 existing assets
- **THEN** the system SHALL update the schema and existing assets remain valid (new field is optional)

#### Scenario: Remove required field blocked
- **WHEN** an admin attempts to remove a required field from a type that has existing assets
- **THEN** the system SHALL reject with 409 Conflict explaining that existing assets depend on this field

### Requirement: List and retrieve asset types
Any workspace member SHALL be able to list and view asset type definitions within their workspace.

#### Scenario: List asset types in workspace
- **WHEN** a workspace member calls `GET /api/workspaces/{id}/asset-types`
- **THEN** the system SHALL return all asset types in the workspace with name, category, field count, and asset count

#### Scenario: Get asset type detail
- **WHEN** a user calls `GET /api/workspaces/{id}/asset-types/{typeId}`
- **THEN** the system SHALL return the full type definition including fields schema, lifecycle states, and transition rules
