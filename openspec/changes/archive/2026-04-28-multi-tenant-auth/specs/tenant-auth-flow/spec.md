## ADDED Requirements

### Requirement: Signup with domain auto-join
The system SHALL extract the email domain on signup. If a tenant exists with a matching `domain`, the user is automatically joined to that tenant. Otherwise, a new tenant is created.

#### Scenario: Email domain matches existing tenant
- **WHEN** user signs up with email `alice@acme.com` and a tenant has `domain = 'acme.com'`
- **THEN** the user is created and automatically added to the matching tenant as a `member`
- **AND** no new tenant is created

#### Scenario: No matching domain
- **WHEN** user signs up with email `bob@newcorp.com` and no tenant has `domain = 'newcorp.com'`
- **THEN** a new tenant is created with the user as `owner`
- **AND** a #general channel is auto-provisioned

#### Scenario: Signup with explicit tenant name
- **WHEN** user provides a `tenant_name` in the signup request
- **THEN** the system creates a new tenant with that name regardless of domain matching

### Requirement: Signin returns tenant list
The system SHALL authenticate the user and return a list of all tenants the user belongs to, plus a JWT scoped to the default tenant.

#### Scenario: User belongs to one tenant
- **WHEN** user signs in with valid credentials and belongs to one tenant
- **THEN** the response includes a JWT with `tenant_id` set to that tenant and a `tenants` list of length 1

#### Scenario: User belongs to multiple tenants
- **WHEN** user signs in and belongs to 3 tenants
- **THEN** the response includes a JWT scoped to the default tenant (owner tenant or most recent) and a `tenants` list of length 3

#### Scenario: Invalid credentials
- **WHEN** user provides wrong email or password
- **THEN** the system returns "invalid credentials" error

### Requirement: Switch tenant re-issues JWT
The system SHALL provide a switch-tenant endpoint that verifies membership and issues a new JWT scoped to the target tenant.

#### Scenario: Valid switch
- **WHEN** authenticated user requests switch to tenant they belong to
- **THEN** the system returns a new JWT with the target `tenant_id`

#### Scenario: User not member of target tenant
- **WHEN** user requests switch to tenant they do NOT belong to
- **THEN** the system returns "access denied" error

### Requirement: JWT carries tenant context
The system SHALL include `tenant_id` and `session_id` in all JWTs. All downstream services receive tenant context automatically.

#### Scenario: JWT structure
- **WHEN** a JWT is issued (signup, signin, switch-tenant)
- **THEN** the token payload contains `user_id`, `username`, `ngac_node_id`, `tenant_id`, and `session_id`

#### Scenario: Backward compatibility
- **WHEN** a service receives a JWT without `tenant_id` (legacy token)
- **THEN** the service SHALL accept it and treat `tenant_id` as empty string

### Requirement: GET /me returns current user with tenant context
The system SHALL provide a `/me` endpoint that returns the authenticated user's info and their current tenant membership.

#### Scenario: Authenticated user
- **WHEN** user calls GET /api/me with valid JWT
- **THEN** the response includes user info (id, email, display_name) and current tenant info (id, name, role, open_id)
