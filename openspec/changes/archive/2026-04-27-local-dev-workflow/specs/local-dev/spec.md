## ADDED Requirements

### Requirement: Start infrastructure only
The system SHALL provide a `make dev-infra` command that starts only Docker infrastructure services (PostgreSQL, Redis, Redpanda, MinIO) without starting any application services.

#### Scenario: Start infra services
- **WHEN** developer runs `make dev-infra`
- **THEN** Docker starts postgres, redis, redpanda, and minio containers
- **THEN** application services (policy, auth, workspace, document, messaging, asset, drive, frontend) SHALL NOT be started

#### Scenario: Infra health check
- **WHEN** infra services are started
- **THEN** the command waits until all infra services pass health checks before returning

### Requirement: Start full dev environment
The system SHALL provide a `make dev` command that starts infrastructure via Docker, runs all Go services natively, and starts the frontend dev server.

#### Scenario: Full dev startup
- **WHEN** developer runs `make dev`
- **THEN** Docker infrastructure services are started and verified healthy
- **THEN** database schema is applied
- **THEN** all Go services are started as background processes with correct port assignments
- **THEN** frontend Vite dev server is started with proxy routing to native services

#### Scenario: Unique REST ports
- **WHEN** Go services run natively on localhost
- **THEN** each service MUST use a unique REST port (auth:8180, workspace:8181, document:8182, messaging:8183, asset:8184, drive:8185)
- **THEN** gRPC ports remain at their default unique values (50051-50057)

### Requirement: Stop dev environment
The system SHALL provide a `make dev-stop` command that stops all native Go services and frontend dev server.

#### Scenario: Clean stop
- **WHEN** developer runs `make dev-stop`
- **THEN** all background Go processes are terminated
- **THEN** PID tracking file is cleaned up
- **THEN** Docker infrastructure services continue running (not stopped)

#### Scenario: Stale PID handling
- **WHEN** a PID file exists but some processes have already exited
- **THEN** the command MUST handle stale PIDs gracefully without errors

### Requirement: Service log visibility
The system SHALL write each service's stdout/stderr to individual log files under `.dev-logs/` directory, enabling agents and developers to read logs via standard file tools.

#### Scenario: Log file per service
- **WHEN** `make dev` starts services
- **THEN** each service's output MUST be written to `.dev-logs/<service>.log`
- **THEN** log files MUST be truncated on each `make dev` start to prevent unbounded growth

#### Scenario: Tail all logs
- **WHEN** developer runs `make dev-logs`
- **THEN** the command tails all service log files simultaneously

#### Scenario: Tail single service log
- **WHEN** developer runs `make dev-logs s=auth`
- **THEN** the command tails only `.dev-logs/auth.log`

#### Scenario: Agent reads logs
- **WHEN** an AI agent needs to debug a service error
- **THEN** the agent SHALL be able to read `.dev-logs/<service>.log` using standard file read tools (cat, grep, tail)

### Requirement: Frontend proxy routing
The Vite dev server SHALL proxy API requests directly to native Go services based on path prefix, bypassing Traefik.

#### Scenario: Path-based routing
- **WHEN** frontend makes request to `/api/auth/*`
- **THEN** Vite proxies to `localhost:8180`
- **WHEN** frontend makes request to `/api/workspaces/*`
- **THEN** Vite proxies to `localhost:8181`
- **WHEN** frontend makes request to `/api/ws`
- **THEN** Vite proxies WebSocket to `ws://localhost:8081`

### Requirement: Backward compatibility
The existing `make deploy` command SHALL continue to work for full Docker deployment including all application services.

#### Scenario: Deploy unchanged
- **WHEN** developer runs `make deploy`
- **THEN** all services (infra + application) are built and started in Docker
- **THEN** behavior is identical to before this change
