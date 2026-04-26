## MODIFIED Requirements

### Requirement: User registration
The system SHALL allow users to register with only a username and password. Company and department SHALL NOT be required at registration. The user SHALL be created as an NGAC User node assigned only to the `PublicUsers` UA under `PC_Global`.

#### Scenario: Simplified registration
- **WHEN** a user calls `POST /api/auth/register` with username and password
- **THEN** a user record and NGAC User node SHALL be created, assigned to `PublicUsers` UA

#### Scenario: No company/department required
- **WHEN** a user registers without company or department fields
- **THEN** registration SHALL succeed — these fields are no longer part of the registration flow

#### Scenario: Post-registration workspace creation
- **WHEN** a newly registered user wants to collaborate
- **THEN** they SHALL create a workspace via `POST /api/workspaces` or be invited to an existing one

### Requirement: User login
The system SHALL authenticate users and return a JWT token with user_id, username, and ngac_node_id claims.

#### Scenario: Successful login
- **WHEN** a user provides valid credentials to `POST /api/auth/login`
- **THEN** the system SHALL return a JWT token and user info (without company/department, as those are now workspace-scoped)

### Requirement: Auth as gRPC service
The Auth Service SHALL expose user operations via gRPC for other services to call.

#### Scenario: Get user by ID
- **WHEN** another service calls `GetUserByID` via gRPC
- **THEN** the Auth Service SHALL return user info including NGAC node ID

#### Scenario: Validate token
- **WHEN** the Gateway calls `ValidateToken` via gRPC
- **THEN** the Auth Service SHALL validate the JWT and return the user identity
