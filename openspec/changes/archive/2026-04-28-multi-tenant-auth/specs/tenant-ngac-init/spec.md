## ADDED Requirements

### Requirement: NGAC Policy Class per tenant
The system SHALL create one NGAC Policy Class per tenant (workspace). This is the root of the tenant's access control graph.

#### Scenario: New tenant created
- **WHEN** a new tenant is created during signup
- **THEN** the system creates NGAC nodes: `PC_{tenant_id}`, `TenantOwner_{tenant_id}` (UA), `TenantMember_{tenant_id}` (UA), and base OA nodes for channels, documents, and management

### Requirement: User assignment on tenant join
The system SHALL assign the user's NGAC node to the appropriate tenant UA when they join a tenant.

#### Scenario: Owner joins tenant
- **WHEN** a user creates a new tenant (is the owner)
- **THEN** the user's NGAC node is assigned to `TenantOwner_{tenant_id}` UA
- **AND** the user's NGAC node is assigned to `TenantMember_{tenant_id}` UA

#### Scenario: Member joins tenant
- **WHEN** a user joins an existing tenant via domain auto-join
- **THEN** the user's NGAC node is assigned to `TenantMember_{tenant_id}` UA only

### Requirement: NGAC associations grant tenant-scoped access
The system SHALL create associations between tenant UAs and OAs so that members and owners receive appropriate operations.

#### Scenario: TenantOwner association
- **WHEN** a tenant is initialized
- **THEN** `TenantOwner_{tenant_id}` has association to all tenant OAs with full operations (read, write, manage, invite, create_channel, approve, upload, share)

#### Scenario: TenantMember association
- **WHEN** a tenant is initialized
- **THEN** `TenantMember_{tenant_id}` has association to tenant OAs with member operations (read, write, create_channel)

### Requirement: No authorization outside NGAC
The system SHALL NOT perform any authorization check outside the NGAC graph. Role checks (`if user.role == "admin"`) and membership checks (`if user in members`) are strictly forbidden.

#### Scenario: Access decision
- **WHEN** any service needs to check if a user can perform an operation
- **THEN** it calls `checkAccess(user_ngac_node_id, object_ngac_node_id, operation)` via the Policy Service
- **AND** never inspects `tenant_users.role` for authorization decisions
