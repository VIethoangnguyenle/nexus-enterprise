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

- the workspace sign-in makes for a person who has none (OTP or Google, no company domain) is `personal`;
- a workspace created on purpose ("Tạo workspace", `POST /api/me/workspaces`) is `organization`;
- the workspace made for a verified Google Workspace domain (`hd`) is `organization`.

The workspace service's `CreateWorkspace` SHALL refuse a missing or unknown type (`InvalidArgument`)
before writing anything to the graph or the database. The type is returned with the workspace on
`GetWorkspace`, `ListWorkspaces` (`GET /api/workspaces`), `GET /api/me/workspaces` and
`GET /api/me` (`current_tenant.type`); the frontend reads it through `useWorkspaceType()`, which
reports nothing (not `personal`) while the type is unknown.

Workspaces that existed before the type was set at creation were backfilled once by migration 044:
a workspace named "<someone>'s Workspace" with exactly one active member and no claimed domain is
`personal`; every other one is `organization`.

#### Scenario: First sign-in makes a personal workspace
- **WHEN** a person with no workspace signs in with a one-time code or a consumer Google account
- **THEN** the workspace made for them has type `personal`

#### Scenario: A workspace created on purpose is an organization
- **WHEN** a signed-in person creates a workspace
- **THEN** it has type `organization`, and so does the one created for a new company domain

#### Scenario: A type nobody stated is refused
- **WHEN** `CreateWorkspace` is called with no type, or with any value other than the two
- **THEN** it fails with `InvalidArgument` and nothing is created
