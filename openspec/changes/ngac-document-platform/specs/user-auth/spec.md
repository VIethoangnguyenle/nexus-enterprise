## ADDED Requirements

### Requirement: User registration
The system SHALL allow new users to register with username, password, company, and department. Registration SHALL create a User node in the NGAC graph and assign it to the corresponding department UA and to PublicUsers UA.

#### Scenario: Successful registration
- **WHEN** a user registers with username "alice", password "pass123", company "Acme", department "Finance"
- **THEN** a users table record is created, an NGAC User node is created, and the user is assigned to UA "Acme_Finance" and UA "PublicUsers"

#### Scenario: Duplicate username rejected
- **WHEN** a user attempts to register with an existing username
- **THEN** the system SHALL return a 409 Conflict error

### Requirement: User login with JWT
The system SHALL authenticate users via username and password, returning a JWT token containing the user ID and username. The token SHALL expire after 24 hours.

#### Scenario: Successful login
- **WHEN** a user provides valid credentials
- **THEN** the system SHALL return a JWT token

#### Scenario: Invalid credentials
- **WHEN** a user provides incorrect password
- **THEN** the system SHALL return a 401 Unauthorized error

### Requirement: JWT middleware
The system SHALL protect all API endpoints except /auth/login and /auth/register with JWT authentication middleware. The middleware SHALL extract the user identity from the token and attach it to the request context.

#### Scenario: Valid token accepted
- **WHEN** a request includes a valid JWT in the Authorization header
- **THEN** the request proceeds to the handler with user context attached

#### Scenario: Missing or invalid token rejected
- **WHEN** a request lacks a JWT or provides an expired/invalid token
- **THEN** the system SHALL return 401 Unauthorized
