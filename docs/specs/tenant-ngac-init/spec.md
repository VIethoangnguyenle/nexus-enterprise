# tenant-ngac-init

## Purpose
Establish the NGAC graph for a new tenant so that every later authorization question has a policy class, user attributes, and object attributes to traverse — and so no service is ever tempted to answer it from a role column.

## Requirements

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
- **THEN** `TenantMember_{tenant_id}` has association to tenant OAs with member operations: `read, write, upload` on Documents and `read, write, create_channel` on Channels

Members hold `write` on Documents, not only `read`, because the drive gates file creation on it — an upload is CreateFile followed by ConfirmFile and both check `write` on the destination folder. A read-only member sees the Upload button and can never complete an upload. This matches what a member already holds on any channel drive they belong to.

Permissions attach to the folder attribute rather than to individual files, so a member with `write` may also rename, move and delete files in Documents that they did not create. Narrowing that requires per-item attributes, not a smaller grant.

### Requirement: Privilege inheritance runs owner-to-member only
The system SHALL assign the Owners UA under the Members UA, never the reverse. NGAC derives privilege by walking child → parent, so the direction of this single assignment decides who inherits whom. Assigning Members under Owners gives every member the full owner association set and silently voids the member-scoped associations.

#### Scenario: Owner inherits member grants
- **WHEN** a user is assigned to `{workspace_id}_Owners`
- **THEN** access checks resolve both the owner operations and the member operations for that user

#### Scenario: Member does not inherit owner grants
- **WHEN** a user is assigned to `{workspace_id}_Members` and to no other UA
- **THEN** `manage`, `invite`, `approve` and `share` on the workspace's Mgmt, Documents and Channels OAs all resolve to DENY
- **AND** only the member operations (read, write and upload on Documents, and the channel operations) resolve to ALLOW

#### Scenario: Member of another workspace
- **WHEN** a user's attributes reach only a different workspace's Policy Class
- **THEN** every operation on this workspace's OAs resolves to DENY, even where an association exists, because the object's PC is not among the user's PCs

### Requirement: Channel membership is authorized per channel
The system SHALL authorize adding and removing channel members with the `invite` operation on that channel's Content OA. The channel's Members UA SHALL hold `invite` on its own Content OA, so that belonging to a channel is what permits bringing someone else into it. The grant SHALL confer nothing on any other channel and nothing at the workspace level.

#### Scenario: Member adds someone to their own channel
- **WHEN** a user assigned to `Ch_{channel_id}_Members` adds another user to that channel
- **THEN** the check for `invite` on `Ch_{channel_id}_Content` resolves to ALLOW

#### Scenario: Member of a different channel
- **WHEN** a user assigned only to `Ch_{other_id}_Members` attempts to add someone to `Ch_{channel_id}`
- **THEN** the check resolves to DENY

#### Scenario: Workspace member who is in no channel
- **WHEN** a user holds only the workspace Members UA and belongs to no channel
- **THEN** `invite` on any channel's Content OA resolves to DENY

#### Scenario: Direct messages cannot gain a third participant
- **WHEN** a participant of a DM attempts to add another user to it
- **THEN** the request is rejected before any graph mutation, because a DM is a fixed two-party conversation
- **AND** this guard lives in the messaging service, not in the graph, since the graph cannot express "exactly two sides"

### Requirement: Policy enforcement points fail closed
Every service that consults the Policy Service SHALL treat anything other than an explicit ALLOW as a denial — including a transport error, an unreachable Policy Service, an empty response, and any unrecognised decision string. Enforcement points SHALL NOT compare against the DENY constant, because that grants access for every value that is neither ALLOW nor DENY.

#### Scenario: Policy Service unreachable
- **WHEN** an access check cannot reach the Policy Service
- **THEN** the calling service denies the request
- **AND** does not fall through to the protected operation

#### Scenario: Single interpretation of a decision
- **WHEN** a service needs to interpret an access decision
- **THEN** it calls `ngac.Allowed(resp.GetDecision(), err)` from `backend/ngac`
- **AND** does not compare the decision string itself

### Requirement: No authorization outside NGAC
The system SHALL NOT perform any authorization check outside the NGAC graph. Role checks (`if user.role == "admin"`) and membership checks (`if user in members`) are strictly forbidden.

#### Scenario: Access decision
- **WHEN** any service needs to check if a user can perform an operation
- **THEN** it calls `checkAccess(user_ngac_node_id, object_ngac_node_id, operation)` via the Policy Service
- **AND** never inspects `tenant_users.role` for authorization decisions
