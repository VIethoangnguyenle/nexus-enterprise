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
- **THEN** `TenantMember_{tenant_id}` has association to tenant OAs with member operations: `read, write, upload, share` on Documents and `read, write, create_channel` on Channels

Members hold `write` on Documents, not only `read`, because the drive gates file creation on it — an upload is CreateFile followed by ConfirmFile and both check `write` on the destination folder. A read-only member sees the Upload button and can never complete an upload. This matches what a member already holds on any channel drive they belong to.

Permissions attach to the folder attribute rather than to individual files, so a member with `write` may also rename, move and delete files in Documents that they did not create. Narrowing that requires per-item attributes, not a smaller grant.

### Requirement: Privilege inheritance runs owner-to-member only
The system SHALL assign the Owners UA under the Members UA, never the reverse. NGAC derives privilege by walking child → parent, so the direction of this single assignment decides who inherits whom. Assigning Members under Owners gives every member the full owner association set and silently voids the member-scoped associations.

#### Scenario: Owner inherits member grants
- **WHEN** a user is assigned to `{workspace_id}_Owners`
- **THEN** access checks resolve both the owner operations and the member operations for that user

#### Scenario: Member does not inherit owner grants
- **WHEN** a user is assigned to `{workspace_id}_Members` and to no other UA
- **THEN** `manage`, `invite` and `approve` on the workspace's Mgmt, Documents and Channels OAs all resolve to DENY, and so does `share` on Mgmt and Channels
- **AND** only the member operations (read, write, upload and share on Documents, and the channel operations) resolve to ALLOW

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

### Requirement: Associations are validated before they are written
The policy service SHALL check that an association starts from an existing UA and ends at an existing OA before it writes the association row, and SHALL answer InvalidArgument when it does not. A refused association SHALL leave no row in `ngac_associations` and no edge in the graph.

#### Scenario: Association from a user node
- **WHEN** `CreateAssociation` names a user (U) node as the source
- **THEN** it fails with InvalidArgument, no `ngac_associations` row exists for it, and the graph is unchanged

#### Scenario: Association to a non-OA or unknown node
- **WHEN** the target is a UA, or either node does not exist
- **THEN** it fails with InvalidArgument and nothing is written

#### Scenario: Valid association
- **WHEN** a UA and an OA are associated
- **THEN** the row is written, the edge is in the graph, and caches are invalidated through the EPP path

### Requirement: A person can be granted access through a personal user attribute
Because only a UA can be the source of an association, granting access to one person SHALL go through a personal UA that contains exactly that user. It is created the first time a grant needs it with the properties `type = personal_ua` and `user_node_id = <the user's node id>` (`ngac.PersonalUAProperties`), is named `ngac.PersonalUAName(userNodeID)` for readability only, and is reused afterwards. It needs no policy class of its own: the user already reaches the workspace and global policy classes through other attributes, so the intersection rule still decides.

A node is accepted as a person's personal UA only by its properties (`ngac.IsPersonalUAOf`) and by the user being inside it, never by its name: workspace administrators choose role names, and a role named like a personal UA must not receive the person's shares. The database allows at most one personal UA per `user_node_id` (unique index), so a lost creation race is resolved by looking the winner up. A workspace shard SHALL carry the personal UA of every user it loads, and only a UA marked as that very user's, because the descent from the tenant policy class never reaches it.

#### Scenario: Grant to a person
- **WHEN** an item is shared with a user who has no personal UA yet
- **THEN** the UA is created, the user is assigned to it, and the association starts from the UA

#### Scenario: A same-named node is never reused
- **WHEN** a role (or any UA) has the name of a person's personal UA but not the properties marking it as theirs, or is marked as another user's
- **THEN** it is ignored: a genuine personal UA is created for the person and the share's association starts from that one

#### Scenario: Second personal UA for one user
- **WHEN** a second UA with `type = personal_ua` and the same `user_node_id` is created
- **THEN** the database refuses it

#### Scenario: Personal UA on a workspace shard
- **WHEN** a workspace shard is loaded for a user who has a personal UA
- **THEN** the shard contains that UA and its associations, so a share to the person grants on the shard exactly what it grants on the global graph; another user's personal UA is not loaded through them

#### Scenario: Grant to a person outside the workspace
- **WHEN** the person does not reach the workspace's policy class
- **THEN** the share grants them nothing, because the object's policy classes are not all reached by the user

### Requirement: Default grants decide who may share
Workspace Owners hold `share` (with every other owner operation) on the Documents OA; Members hold `read, write, upload, share` there (`ngac.MemberDocumentOps`), and members of a channel hold `read, write, upload, share` on its drive (`ngac.ChannelDriveOps`). Members share what they work on. A share grants its grantee `read` or `read, write` only (`ngac.ShareOps`), never `share`, so the right does not spread to the people a file is shared with. Existing associations are backfilled by migration `019_member_share_op.sql`; like the other direct association writes it bypasses EPP invalidation, so the services must be restarted after it is applied.

#### Scenario: Member shares
- **WHEN** a workspace member (or a channel member, on the channel's drive) shares an item
- **THEN** the `share` check on the item resolves to ALLOW

#### Scenario: Grantee of a write share
- **WHEN** a person has been shared an item with "write"
- **THEN** they have `read` and `write` on it and `share`, `manage`, `approve` resolve to DENY

### Requirement: Role names stay out of the platform's namespaces
A role (a UA created by a workspace administrator) SHALL be refused when its name is empty or starts or ends like a name the platform builds itself (`ngac.ValidateRoleName`): the prefixes `User_`, `PC_`, `TenantMember_`, `TenantOwner_`, `Dept_`, `Ch_`, `DriveRoot_`, `Folder_`, `Share_`, `Asset_`, the suffixes `_Owners`, `_Members`, `_Mgmt`, `_Documents`, `_DraftDocs`, `_ApprovedDocs`, `_Channels`, `_Assets`, `_Content`, `_Drive`, and the names `PC_Global` and `PublicUsers`, compared case-insensitively. Node names are matched exactly and the graph's name index keeps the last node written, so a role inside one of these namespaces could be found where the platform expects its own node.

#### Scenario: Reserved role name
- **WHEN** an administrator creates a role named like a personal UA, a workspace's Owners UA or a policy class
- **THEN** the request fails with InvalidArgument and no node is written
