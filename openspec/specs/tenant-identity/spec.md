## ADDED Requirements

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
