# tenant-identity

## Purpose
Separate who a person is globally from who they are inside a tenant, so one human can hold memberships in many tenants without their identities leaking across the boundary.

## Requirements

### Requirement: Users have global identity
The system SHALL store a global user identity with `email` (unique, primary login), `union_id` (cross-tenant identifier), and `display_name`.

#### Scenario: User registers with email
- **WHEN** a user provides email, password, and display_name
- **THEN** the system creates a user record with a unique `union_id` and stores the email as the primary login credential

#### Scenario: Email uniqueness enforced
- **WHEN** a user attempts to register with an email that already exists
- **THEN** the system rejects the registration with an "already exists" error

### Requirement: Users have tenant-scoped identity
The system SHALL assign each user a unique `open_id` per tenant membership. The `open_id` is generated on join and stored in `tenant_users`.

#### Scenario: User joins a tenant
- **WHEN** a user is added to a tenant (via signup, invite, or domain auto-join)
- **THEN** the system creates a `tenant_users` record with a unique `open_id` for that user-tenant pair

#### Scenario: open_id is unique across all tenants
- **WHEN** the system generates an open_id
- **THEN** the open_id is globally unique (UUID) and can be used to look up the user-tenant pair

### Requirement: Users can belong to multiple tenants
The system SHALL support a user belonging to multiple tenants via the `tenant_users` table with `(tenant_id, user_id)` as the composite primary key.

#### Scenario: User belongs to two tenants
- **WHEN** a user is a member of Tenant A and Tenant B
- **THEN** the system returns both tenants when listing the user's memberships
- **AND** each membership has its own `open_id`, `role`, and `status`

### Requirement: Tenant membership has status tracking
The system SHALL track membership status as one of: `active`, `invited`, `disabled`.

#### Scenario: Invited user before signup
- **WHEN** an admin invites an email that has no registered user
- **THEN** the system creates a `tenant_users` record with status `invited`

#### Scenario: Invited user completes signup
- **WHEN** an invited user registers with the invited email
- **THEN** the system updates the `tenant_users` status from `invited` to `active`

### Requirement: A workspace's type is decided when it is made
Every workspace SHALL have a type, `personal` or `organization` (`workspaces.type`, NOT NULL,
CHECK-constrained, no default — an insert that does not state it fails), set by how it came to be:

- every new account gets a `personal` workspace of its own when it is created (OTP or Google, a
  company account with a Google Workspace domain included); signing in never creates an organization.
  That workspace is named after the person (their display name, nothing added) and follows their name
  when they change it — first of all at the profile step, from the handle sign-up gave them — for as
  long as it still carries their previous name; a workspace someone renamed keeps its name;
- a workspace created on purpose ("Tạo workspace", `POST /api/me/workspaces` with `type`) has the type
  the person chose, `personal` or `organization`; a person may have several of either. A request with
  no `type` (a client older than the choice) creates an `organization`; any other value is refused (400)
  and nothing is created.

The workspace service's `CreateWorkspace` SHALL refuse a missing or unknown type (`InvalidArgument`)
before writing anything to the graph or the database. The type is returned with the workspace on
`GetWorkspace`, `ListWorkspaces` (`GET /api/workspaces`), `GET /api/me/workspaces` and
`GET /api/me` (`current_tenant.type`); the frontend reads it through `useWorkspaceType()`, which
reports nothing (not `personal`) while the type is unknown.

Workspaces that existed before the type was set at creation were backfilled once by migration 044:
a workspace named "<someone>'s Workspace" with exactly one active member and no claimed domain is
`personal`; every other one is `organization`.

#### Scenario: First sign-in makes a personal workspace
- **WHEN** a new account is created by a one-time code or by Google, with or without a company domain
- **THEN** the workspace made for them has type `personal`
- **AND** a company account whose domain an organization already owns also joins that organization

#### Scenario: A workspace created on purpose has the chosen type
- **WHEN** a signed-in person creates a workspace and chooses `personal` or `organization`
- **THEN** it has that type

#### Scenario: A type nobody stated is refused
- **WHEN** `CreateWorkspace` is called with no type, or with any value other than the two
- **THEN** it fails with `InvalidArgument` and nothing is created

### Requirement: A personal workspace has no assets and no approvals
The workspace type SHALL be enforced by the services that own the features, not only hidden by the screens. In a
`personal` workspace the asset service and the approval service refuse every operation (PermissionDenied, REST 403;
see `asset-authorization` and `approval-screens`), and the messaging service refuses a message that links an
approval request (`linked_entity_type` `approval`; a direct message is judged by the workspace the sender is signed in
to). Each reads `workspaces.type` itself, before it asks the policy, and fails closed: an unknown workspace or a
type that is not `organization` is a refusal, and a failure to read it is an internal error. Nothing is deleted:
what a personal workspace already holds is kept, hidden, and is usable again if it becomes an organization. The type is
not a node of the policy graph and no decision of the policy service depends on it. The document service has no
approval operation of its own (a text's `draft`/`active`/`archived` status is not an approval), so it has nothing to refuse.

#### Scenario: Assets and approvals refuse in a personal workspace
- **WHEN** any asset or approval operation is called in a personal workspace, by anyone, whatever they hold in the graph
- **THEN** it fails with PermissionDenied / 403 and changes nothing

#### Scenario: An approval card in a personal workspace
- **WHEN** a message linking an approval request is sent in a personal workspace
- **THEN** it is refused (403) and not stored; other messages and cards are sent as usual

#### Scenario: Organization unaffected
- **WHEN** the same operations are called in an organization workspace
- **THEN** they are decided by the policy as before

### Requirement: The owner converts a personal workspace to an organization, once
A personal workspace becomes an organization through `POST /api/workspaces/{id}/convert-to-organization` (workspace
service). The caller SHALL hold `manage` on the workspace's management OA **and** be an owner of it (a member of its Owners
group, the same test the owner changes and the last-owner rule use, so an owner added later can convert); either missing is a refusal (403), and a caller who may not convert is refused before anyone
learns the workspace's type. A workspace that is an organization already, whether made one or converted, answers 409
`{"reason":"already_organization"}`; an unknown workspace answers 404. Conversion is one way and cannot be repeated.

It changes only `workspaces.type`, in one conditional statement (`... WHERE id = $1 AND type = 'personal'`), together
with `converted_by` (the user who converted) and `converted_at` (migration 046; both stay empty for a workspace that was
made an organization). The owner check, the type check and the update run under the workspace's owner lock (the one owner changes take), so a conversion cannot land after the caller stopped being an owner. Of two requests at once exactly one converts and the other gets 409. Nothing is written to the
policy graph: an organization needs no node a personal workspace lacks (an asset type's OA is created on first use,
approvals use the tenant schema, which the approval service creates on first use). Nothing is deleted: what the
workspace held while personal is kept and is usable now. The workspace service announces a workspace update in realtime
so every open session refreshes. The workspace service has no administrator audit log; `converted_by` and `converted_at`
are the record of who converted and when, shown by name in the settings screen.

Success answers `200 {"type":"organization","converted_by_name":"<display name>","converted_at":"<RFC 3339>"}`.
`GET /api/workspaces/{id}/details` also returns `type`, `is_owner` (the caller is in the Owners group), `owner_name` (the names of the current owners joined with ", ", never an id),
`owner_names` (the names of the current owners as an array, never an id), `converted_by_name` and `converted_at` (RFC 3339, or empty when never converted).

#### Scenario: The owner converts
- **WHEN** an owner of a personal workspace (the creator or one added later) converts it
- **THEN** its type becomes `organization`, the answer names who converted it and when, and open sessions refresh
- **AND** assets and approvals, refused until now, are decided by the policy as in any organization

#### Scenario: Not the owner
- **WHEN** a member, or someone holding `manage` who is not an owner, or a person from another workspace, converts it
- **THEN** it fails with 403 and the type is unchanged

#### Scenario: Already an organization
- **WHEN** the owner converts a workspace that is an organization, or converts it a second time
- **THEN** it fails with 409 `already_organization` and nothing changes

#### Scenario: Details after conversion
- **WHEN** a member reads the workspace details after it was converted
- **THEN** they see type `organization`, the owner's and the converter's names, and the time of conversion
