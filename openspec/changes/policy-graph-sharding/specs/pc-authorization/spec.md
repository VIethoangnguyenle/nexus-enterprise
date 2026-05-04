## ADDED Requirements

### Requirement: CreateNode rejects PC type from non-system callers
The write server SHALL reject CreateNode requests with `node_type=PC` unless the caller has system-level or tenant-admin privilege.

#### Scenario: Regular service creates UA node
- **WHEN** a gRPC caller sends CreateNode with node_type=UA
- **THEN** the system SHALL allow the creation

#### Scenario: Regular service creates PC node
- **WHEN** a gRPC caller sends CreateNode with node_type=PC without admin scope
- **THEN** the system SHALL return PermissionDenied error

#### Scenario: Workspace service creates tenant PC
- **WHEN** the workspace service (system caller) sends CreateNode with node_type=PC
- **THEN** the system SHALL allow the creation

### Requirement: PC nodes store tenant ownership metadata
PC nodes SHALL store scope and ownership information in the `properties` JSONB field.

#### Scenario: System creates tenant PC
- **WHEN** a workspace is created and the tenant PC is auto-created
- **THEN** the PC node properties SHALL contain `{"scope":"tenant","tenant_id":"<workspace_id>"}`

#### Scenario: Tenant admin creates secondary PC
- **WHEN** a tenant admin creates PC_W1_Confidential for workspace W1
- **THEN** the PC node properties SHALL contain `{"scope":"tenant","tenant_id":"W1"}`

#### Scenario: PC_Global has global scope
- **WHEN** PC_Global is created during bootstrap
- **THEN** the PC node properties SHALL contain `{"scope":"global"}`

### Requirement: Shard loader uses tenant_id to discover secondary PCs
The shard loader SHALL query all PC nodes with matching `tenant_id` to build the complete shard.

#### Scenario: Workspace with secondary PCs
- **WHEN** LoadShard is called for workspace W1 which has PC_W1 and PC_W1_Confidential (both tenant_id=W1)
- **THEN** the loader SHALL discover and include both PCs in the shard
