## ADDED Requirements

### Requirement: Service decomposition
The system SHALL be composed of 6 independent Go microservices: API Gateway, Auth Service, Policy Service, Workspace Service, Document Service, and Messaging Service.

#### Scenario: All services start successfully
- **WHEN** `docker-compose up` is executed
- **THEN** all 6 services, PostgreSQL, NATS, and the frontend container SHALL start and report healthy status

#### Scenario: Service isolation
- **WHEN** the Document Service is restarted
- **THEN** the Auth Service, Policy Service, Workspace Service, and Messaging Service SHALL continue operating without interruption

### Requirement: gRPC inter-service communication
All synchronous service-to-service communication SHALL use gRPC with Protocol Buffer definitions shared in the `proto/` directory.

#### Scenario: Service calls Policy Service
- **WHEN** any service needs to check access or mutate the NGAC graph
- **THEN** it SHALL call the Policy Service via gRPC using the defined `PolicyService` proto

#### Scenario: Proto contract enforcement
- **WHEN** a developer changes a `.proto` file and runs `make proto`
- **THEN** Go code SHALL be regenerated for all affected services with compile-time type checking

### Requirement: API Gateway routing
The API Gateway SHALL be the single public entry point, routing HTTP requests to backend services via gRPC and proxying WebSocket connections to the Messaging Service.

#### Scenario: HTTP request routing
- **WHEN** a client sends `POST /api/auth/login`
- **THEN** the Gateway SHALL forward the request to the Auth Service via gRPC and return the response as JSON

#### Scenario: JWT validation at gateway
- **WHEN** a client sends a request to a protected endpoint with a valid JWT
- **THEN** the Gateway SHALL validate the token and inject user identity into the gRPC metadata

#### Scenario: Invalid JWT rejected
- **WHEN** a client sends a request with an expired or invalid JWT
- **THEN** the Gateway SHALL return 401 Unauthorized without forwarding to any service

### Requirement: NATS event bus
The system SHALL use NATS for asynchronous event notifications between services. Events are fire-and-forget (at-most-once delivery).

#### Scenario: Message sent event
- **WHEN** a new message is created in the Messaging Service
- **THEN** a `message.sent` event SHALL be published to NATS with channel_id, sender_id, and message preview

#### Scenario: Member invited event
- **WHEN** a user is invited to a workspace
- **THEN** a `member.invited` event SHALL be published to NATS with workspace_id and user_id

### Requirement: Shared PostgreSQL database
All services SHALL connect to a single PostgreSQL instance. Each service SHALL only access tables it owns, using gRPC to access data owned by other services.

#### Scenario: Cross-service data access
- **WHEN** the Document Service needs user information
- **THEN** it SHALL call the Auth Service via gRPC, NOT query the users table directly

### Requirement: Docker Compose orchestration
The entire system SHALL be deployable via a single `docker-compose.yml` with proper dependency ordering and health checks.

#### Scenario: Clean startup
- **WHEN** `docker-compose up --build` is run from the project root
- **THEN** services SHALL start in dependency order: PostgreSQL → NATS → Policy → Auth → Workspace/Document/Messaging → Gateway → Frontend

#### Scenario: Service restart recovery
- **WHEN** a service crashes and Docker restarts it
- **THEN** the service SHALL reconnect to PostgreSQL and gRPC dependencies and resume operation
