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
| Create role, create folder | `manage` |
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

### Requirement: There is no free-form permission grant
The workspace service SHALL NOT offer a route or RPC that associates an arbitrary UA with an arbitrary OA (`POST /api/workspaces/:id/permissions` and the gRPC `CreatePermission` are removed). It checked `manage` on the route's workspace but not that the UA and OA belonged to it, so, with re-grants replacing operations, an owner of one workspace could rewrite another's Members or Owners grants. What a role holds is changed only through the per-area route below, which resolves the OA from the area inside the route's workspace and takes only a role of that workspace.

#### Scenario: The old route
- **WHEN** a client posts `{ua_id, oa_id, operations}` to `/api/workspaces/:id/permissions`
- **THEN** there is no such route (404/405) and nothing is written

#### Scenario: Another workspace's attributes
- **WHEN** an owner of workspace B sets permissions through B's route for a role, Members UA or Owners UA of workspace A
- **THEN** the answer is 404 and A is unchanged; through A's route the same owner is refused with 403

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

### Requirement: Permission areas and their operations come from the server
The workspace service SHALL answer, for a workspace, which resource areas a role can be granted on (management, documents, channels, assets) and, for each, the operations that apply there (`GET /api/workspaces/:id/permission-areas`). The answer SHALL be derived from the constants in `backend/ngac` (`ngac.Areas`, `ngac.AreaOps`), listing an operation on an area only where a service checks it on that kind of OA (a test fails if an area offers `upload`), and SHALL omit an area whose OA the workspace does not have. Clients SHALL NOT keep their own table of which operation fits which area. The caller must belong to the workspace.

#### Scenario: Documents
- **WHEN** a member asks for the areas
- **THEN** documents lists `read`, `write` and `share`, and does not list `upload` (the drive gates an upload on `write`, so no service checks `upload`), `approve` or `manage`

#### Scenario: A workspace with no assets yet
- **WHEN** the workspace has no Assets OA
- **THEN** the assets area is not listed

#### Scenario: Non-member
- **WHEN** a user outside the workspace asks
- **THEN** the answer is 403

### Requirement: A role's permissions are set per area, checked and delegated
`PUT /api/workspaces/:id/roles/:roleId/permissions/:area` SHALL make the posted `operations` exactly what the role holds on that area (an empty list removes the grant). It SHALL require `manage` on the Mgmt OA; the role SHALL be a role an administrator created in this workspace (never Owners or Members); the area SHALL be a known area the workspace has, resolved to its OA inside this workspace and never taken from the body; every operation SHALL be one the area offers. The caller SHALL hold every operation they add, on the area's OA; keeping or removing what the role already has needs nothing more. What the role already has is read from the policy writer, not from a read replica that may be behind. Reading a role's grants and members (`GET .../roles/:roleId`) SHALL require `manage`.

#### Scenario: Operation the area does not offer
- **WHEN** a caller grants `approve` on documents, or any operation on an unknown area
- **THEN** the request is rejected with 400 and nothing is written

#### Scenario: Escalation by editing a role
- **WHEN** a caller with `manage` on Mgmt but only `read` on Documents adds `write` on Documents to a role
- **THEN** the request is rejected with 403 and nothing is written

#### Scenario: Narrowing
- **WHEN** a role holds `read`, `write` on Documents and the caller sets `read`
- **THEN** the role no longer holds `write`, in the database and in every policy graph

#### Scenario: Built-in roles
- **WHEN** the Owners or Members UA, or a UA of another workspace, is edited through this route
- **THEN** the answer is 404 and nothing is written

### Requirement: Assigning a role or a department is a delegation
`PUT` and `DELETE /api/workspaces/:id/members/:nodeId/roles/:roleId` SHALL require `manage` on the Mgmt OA, a role created by an administrator in this workspace, and a person who already belongs to the workspace (a user node under its PC): holding `manage` SHALL NOT bring a stranger in, which is what `invite` is for. Assigning SHALL add one assignment and detach nothing else. Assigning a role, or a person's department, SHALL also require the caller to hold every operation the role (or the department's UA) confers, **including what it inherits from the UAs above it** (a person under a sub-department holds every grant of the departments above it); a grant or an ancestry that cannot be read is treated as not held. Moving a department under a parent is the same delegation, checked against the new parent's whole chain; moving to the root grants nothing. Removing a role needs only `manage`. A person is in at most one department: choosing one detaches the previous department and, if that fails, takes the new assignment back.

#### Scenario: Role that confers more than the caller holds
- **WHEN** a manager who cannot `write` Documents assigns a role that grants `write` on Documents
- **THEN** the request is rejected with 403 and no assignment is written

#### Scenario: Assigning to a non-member
- **WHEN** a manager assigns a role, or a department, to a user who is not in the workspace
- **THEN** the answer is 404 and nothing is written

#### Scenario: Moving department
- **WHEN** a member of department A is put in department B
- **THEN** they hold B only, and the Owners or role assignments they have are untouched

### Requirement: Deleting a department keeps its people and sub-departments reachable
`DELETE /api/workspaces/:id/departments/:deptId` SHALL, before the department's node is deleted, assign each sub-department's UA to the deleted department's parent (the workspace PC for a root), and assign the people directly under it to that parent. If any of that cannot be written, nothing is deleted. The people the department conferred rights on keep exactly what they could already reach through it.

#### Scenario: Deleting a root department that has a child
- **WHEN** a root department with a sub-department is deleted
- **THEN** the sub-department's UA is assigned under the workspace PC first, so its people still reach the workspace

#### Scenario: Graph write fails
- **WHEN** moving a sub-department up fails
- **THEN** the department's node and row are untouched and the call fails

### Requirement: The people table
`GET /api/workspaces/:id/admin/members` SHALL require `manage` and return each person of the workspace with a display name, email, picture, title, standing, whether they are an Owner, their department and their administrator-made roles, each by name; a person with no profile is named from their graph node or "Thành viên", never by an identifier. A department's head count (`GET /departments`) SHALL be counted from the same graph membership the table lists, not from a cached column.

### Requirement: Inviting records an offer; the invitee accepts it
`POST /api/workspaces/:id/members` with `email` (and optionally `role_id`, `department_id`) SHALL require `invite`, validate only that the address is well formed (400 otherwise), and record or refresh one pending invitation for that address in that workspace (migration 029, `workspace_invitations`: normalised email, optional role and department, the inviter, status, `created_at`, `expires_at` 7 days on). It SHALL answer `202 {"status":"invited"}` for every well-formed address and SHALL NOT consult accounts or memberships, assign anyone, or list anyone: the answer and the stored effect are the same whether or not an account exists or the person already belongs. A role or department attached SHALL belong to the workspace, and attaching one takes `manage` and holding everything it confers (with inheritance). Inviting is limited per caller (40 per hour per process; over it, 429; a caller who may not invite spends none). `GET /api/workspaces/:id/invitations` (needs `invite`) lists the open offers by address and names; `DELETE /api/workspaces/:id/invitations/:invitationId` (needs `invite`) withdraws one of this workspace.

The invitee acts through `GET /api/invitations`, `POST /api/invitations/:id/accept` and `POST /api/invitations/:id/decline`. The person is the verified token's user; an invitation is matched to the address on that user's account record (`users.email`), never to anything in the request, and only when that address is **verified** (`users.email_verified_at` is set). An account whose address is unverified, such as one made by password signup with someone else's address, sees no invitations (the list is empty) and gets 404 on accept and decline, exactly as for an unknown invitation. The match is case-insensitive. An invitation that is another address's, or unknown, is 404; one already accepted, declined or revoked is 409; an expired one is 400. Accepting SHALL be refused (403, and the offer stays open) if the inviter no longer holds `invite` on the Mgmt OA; otherwise it claims the invitation in one conditional update (two accepts at once add the person once), adds the person to Members the way a direct invite always did and lists them in `tenant_users`, and gives the attached role and department **only if the inviter, now, still holds `manage` and everything the role or department confers**; otherwise the person joins without it (`role_applied` / `department_applied` false). If adding fails, the invitation is reopened.

#### Scenario: Caller without invite
- **WHEN** a caller who may not invite posts an email
- **THEN** the answer is 403, no invitation is stored and no account is consulted

#### Scenario: Same answer for everyone
- **WHEN** an inviter posts an address that has an account, one that has none, and one belonging to a current member
- **THEN** all three are `202 {"status":"invited"}`, and nobody is added

#### Scenario: Someone else's invitation
- **WHEN** a user accepts or declines an invitation addressed to another address
- **THEN** the answer is 404 and nothing is written

#### Scenario: Expired, declined or revoked
- **WHEN** a user accepts an invitation that is expired, declined or revoked
- **THEN** it is refused and nothing is written

#### Scenario: Inviter lost the right
- **WHEN** the inviter no longer holds `invite` when the invitee accepts
- **THEN** the answer is 403, nobody is added, and the invitation is still pending

#### Scenario: Inviter lost rights over the role
- **WHEN** the inviter no longer holds an operation the attached role confers
- **THEN** the person joins as a plain member and `role_applied` is false

### Requirement: Removing a member protects owners
`DELETE /api/workspaces/:id/members/:nodeId` SHALL require `invite`; removing someone in the Owners UA SHALL additionally require `manage` **and that the caller is an Owner** (a role carrying `manage` is not enough), and the last owner SHALL NOT be removed. Adding an owner (`TransferOwnership`) and removing one (`RemoveOwner`) likewise require the caller to be an Owner, and a new owner must already belong to the workspace. The Owners UA is read, checked and changed under a per-workspace advisory lock (`pg_advisory_xact_lock`), so two owner changes at once run one after the other and the last-owner rule is re-checked under the lock. A removed person's `tenant_users` listing for this workspace is dropped, and every pending invitation to the removed person's verified address in this workspace is revoked, so removal is not undone by a stale offer. Nothing adds a person by node ID: `POST /workspaces/:id/invite` and the gRPC `InviteMember` do not exist (404/405 and Unimplemented); people join only through an invitation they accept.

#### Scenario: Inviter removes an owner
- **WHEN** a caller holding only `invite`, or `manage` through a role without being an Owner, removes an owner
- **THEN** the answer is 403 and the owner stays

#### Scenario: Removed person's open offer
- **WHEN** a member whose verified address has a pending invitation to this workspace is removed
- **THEN** that invitation is revoked and cannot be accepted

#### Scenario: Two owners remove each other at once
- **WHEN** two owners each remove the other at the same moment
- **THEN** exactly one removal happens and one owner remains

### Requirement: Authorization reads come from the writer
Every graph read that feeds an authorization or write decision in the workspace service (owner lists, ancestry and descendants for membership and scope, the associations and ancestry behind a delegation check) SHALL go to the policy writer, not to a read replica that may lag. Only `CheckAccess` uses the read service.

#### Scenario: Replica still shows a removed owner
- **WHEN** the read side lists two owners but the writer lists one
- **THEN** removing the last owner is refused and nothing changes

#### Scenario: Replica has not seen a grant
- **WHEN** the read side shows a role granting nothing and the writer shows it granting write
- **THEN** a caller who lacks write cannot hand the role out

## Status
Known gaps, recorded rather than resolved:
- Nothing sends an email: an invitation is found by the invitee on their next sign-in (the screen that lists and answers invitations on workspace selection belongs to the sign-in group). An address with no account can be invited, and the offer waits for it.
- An invitation is matched to `users.email` only when `users.email_verified_at` is set. That is set by Google sign-in (`email_verified` true) and by a one-time code that a real sender delivered to the address. Fixed-code test mode and the log sender prove nothing, so accounts made through them never see invitations until the address is proved by one of the real routes. Profile edits do not change the address.
- The invite limit is counted in the workspace service's memory (a fixed window per caller), so with several replicas the budget is per replica.
- Nothing records who changed a role, a permission, a member or a department, so there is no administrators' activity feed.
- A role cannot be renamed: the policy service has no node update.
- `UpdateMemberRoles` and `CreatePermission` no longer exist on the workspace gRPC service.
