# workspace-admin-authorization

## Purpose
Make every change to a workspace's membership, roles, folders, departments and permissions a
decision of the policy service, so that holding a session in a workspace is never enough on its
own to administer it or to grant oneself access.

## Requirements

### Requirement: Admin writes are checked on the workspace management OA
The workspace service SHALL ask the PDP, before any write, whether the caller holds the required
operation on the workspace's management OA (`ngac.MgmtOAName(wsID)`), resolved among the direct
children of that workspace's own PC. The caller SHALL be taken from verified JWT claims, never
from the request body.

| Operation | Required op |
|---|---|
| Invite member, remove member | `invite` |
| Create permission, create role, create folder | `manage` |
| Department create / update / delete / move / member assignment | `manage` |
| Update member roles, transfer or add owner, remove owner, delete role, delete folder, delete permission | `manage` |

#### Scenario: Member without manage creates a role
- **WHEN** a workspace member whose roles carry no `manage` association on the Mgmt OA calls `POST /api/workspaces/:id/roles`
- **THEN** the request is rejected with 403
- **AND** no node, assignment or association is written

#### Scenario: Owner creates a role
- **WHEN** a workspace owner (Owners UA holds `manage` on the Mgmt OA) calls `POST /api/workspaces/:id/roles`
- **THEN** the role is created

#### Scenario: Policy service unavailable
- **WHEN** the PDP call errors or returns anything other than ALLOW
- **THEN** the write is rejected as a denial

#### Scenario: Body carries a forged requester
- **WHEN** the request body names another user as the requester
- **THEN** the service ignores it and authorizes the caller from the JWT

### Requirement: Permission grants cannot exceed the granter's own rights
`CreatePermission` SHALL require `manage` on the Mgmt OA **and** that the caller holds every
requested operation on the target OA. Any denial rejects the whole request. Only the eight
operations defined in `backend/ngac` are accepted.

#### Scenario: Escalation attempt
- **WHEN** a caller with `manage` on the Mgmt OA but only `read` on the Documents OA grants `manage` on Documents
- **THEN** the request is rejected with 403

#### Scenario: Unknown operation
- **WHEN** the request lists an operation that is not one of the eight NGAC operations, or lists none
- **THEN** the request is rejected with 400

### Requirement: Node IDs must belong to the workspace in the route
Role, folder, parent and department IDs supplied by the client SHALL be nodes under the route's
workspace PC; otherwise the request is rejected as not found. Holding `manage` on one workspace
SHALL confer nothing on another.

#### Scenario: Another workspace's role ID
- **WHEN** an owner of workspace A deletes a role, passing the ID of a role in workspace B through A's route
- **THEN** the request is rejected with 404 and B is unchanged

### Requirement: Workspace reads require membership
Reading a workspace, its members, roles, departments and folders SHALL require the workspace's
PC to be among the caller's ancestors (the same test the workspace list uses).

#### Scenario: Non-member reads members
- **WHEN** a user who does not belong to the workspace calls `GET /api/workspaces/:id/members`
- **THEN** the request is rejected with 403
