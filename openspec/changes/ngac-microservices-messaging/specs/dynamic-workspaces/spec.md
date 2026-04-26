## ADDED Requirements

### Requirement: Workspace creation
A registered user SHALL be able to create a new workspace. Creating a workspace SHALL generate the following NGAC graph nodes and associations:
- Policy Class node (the workspace)
- `{ws}_Owners` UA assigned to the PC
- `{ws}_Members` UA assigned to the PC
- `{ws}_Workspace_Mgmt` OA assigned to the PC
- `{ws}_Documents` OA assigned to the PC
- `{ws}_DraftDocs` OA assigned to the PC
- `{ws}_ApprovedDocs` OA assigned to the PC
- `{ws}_Channels` OA assigned to the PC
- Default associations granting Owners full permissions on all OAs
- Creator's user node assigned to `{ws}_Owners`

#### Scenario: User creates workspace
- **WHEN** a registered user calls `POST /api/workspaces` with name "Acme Corp"
- **THEN** a workspace SHALL be created with PC node "PC_Acme_Corp" and the creator SHALL be assigned as owner

#### Scenario: Workspace scaffolding
- **WHEN** a workspace is created
- **THEN** the NGAC graph SHALL contain all required UA and OA nodes with correct assignments to the workspace PC

### Requirement: Dynamic role creation
A user with `manage` permission on the workspace management OA SHALL be able to create custom User Attributes (roles, departments, teams) within the workspace.

#### Scenario: Owner creates a department
- **WHEN** the workspace owner calls `POST /api/workspaces/{id}/roles` with name "Engineering"
- **THEN** a UA node "Engineering" SHALL be created and assigned to the workspace's Policy Class

#### Scenario: Non-manager cannot create roles
- **WHEN** a user without `manage` permission attempts to create a role
- **THEN** the request SHALL be denied with 403 Forbidden

### Requirement: Dynamic permission configuration
A user with `manage` permission SHALL be able to create Associations (permission grants) between UAs and OAs within the workspace.

#### Scenario: Owner grants engineering read access to docs
- **WHEN** the owner creates an association: Engineering UA → Documents OA with operations [read, write]
- **THEN** all users assigned to the Engineering UA SHALL be able to read and write documents in that OA

#### Scenario: Cross-workspace association prevention
- **WHEN** a user attempts to create an association referencing a UA or OA from a different workspace
- **THEN** the request SHALL be denied — associations MUST be scoped to the workspace's PC

### Requirement: Dynamic folder creation
A user with `manage` permission SHALL be able to create custom Object Attributes (document folders, channel groups) within the workspace.

#### Scenario: Create sub-folder
- **WHEN** the owner creates an OA "Engineering_Docs" assigned to the workspace Documents OA
- **THEN** documents assigned to "Engineering_Docs" SHALL inherit workspace-level permissions and also be targetable by specific associations

### Requirement: Member invitation
A user with `invite` permission on the workspace management OA SHALL be able to invite other registered users to the workspace by assigning them to specific UAs.

#### Scenario: Owner invites user with roles
- **WHEN** the owner calls `POST /api/workspaces/{id}/invite` with user_id and attribute_ids ["Engineering", "Viewer"]
- **THEN** the target user's NGAC node SHALL be assigned to the specified UAs within the workspace

#### Scenario: Invited user has no auto-access
- **WHEN** a user is invited to a workspace
- **THEN** they SHALL only have access to resources granted by their assigned UAs — no channels or documents are auto-accessible

#### Scenario: Non-inviter cannot invite
- **WHEN** a user without `invite` permission attempts to invite another user
- **THEN** the request SHALL be denied with 403 Forbidden

### Requirement: Member removal
A user with `manage` permission SHALL be able to remove a member from the workspace by removing all their UA assignments within that workspace.

#### Scenario: Remove member
- **WHEN** an admin removes a member from the workspace
- **THEN** all of the member's assignments to UAs under that workspace's PC SHALL be removed, and the member SHALL lose all access to workspace resources

### Requirement: Workspace listing
A user SHALL be able to list all workspaces they are a member of.

#### Scenario: List my workspaces
- **WHEN** a user calls `GET /api/workspaces`
- **THEN** the response SHALL contain all workspaces where the user's NGAC node is assigned to any UA under that workspace's PC

### Requirement: Workspace member listing
A user with workspace access SHALL be able to list all members and their role assignments.

#### Scenario: List members with roles
- **WHEN** a user calls `GET /api/workspaces/{id}/members`
- **THEN** the response SHALL contain all users assigned to any UA under that workspace's PC, with their UA assignments listed
