## Context

The NGAC document platform is currently a monolithic Go backend serving auth, document management, and NGAC policy evaluation from a single process. The NGAC graph is loaded in-memory from PostgreSQL on startup, and all access checks are local function calls.

The system uses pre-seeded companies (Policy Classes) and departments (User Attributes) — users must register into an existing structure. There is no workspace creation, no messaging, and no dynamic organizational management.

We are extending this into a microservices-based collaboration platform where:
- Users register independently, then create/join workspaces
- Each workspace is a self-governing NGAC policy domain with dynamic roles and permissions
- Messaging (channels, DMs) is integrated as a first-class workspace feature alongside documents
- All services communicate via gRPC, with the NGAC engine centralized as a Policy Service

### Current Architecture
- Single Go binary (Chi router) on :8080
- PostgreSQL 16 (ngac_nodes, ngac_assignments, ngac_associations, users, documents)
- React SPA (Vite + Zustand) on :3000
- Docker Compose: 3 containers (postgres, backend, frontend)

### Target Architecture
- 6 Go microservices communicating via gRPC
- NATS for async event bus
- PostgreSQL (single instance, shared)
- React SPA with WebSocket client
- Docker Compose: 9 containers

## Goals / Non-Goals

**Goals:**
- Split monolith into independently deployable services with clear boundaries
- Centralize NGAC policy evaluation in a dedicated gRPC service
- Enable user-driven workspace creation with dynamic organizational structures
- Add real-time messaging integrated with NGAC access control
- Maintain all existing NGAC guarantees (graph-based access, constraints, explainability)
- Single `docker-compose up` deployment

**Non-Goals:**
- Database-per-service split (single PostgreSQL for now, can split later)
- Service mesh / Kubernetes deployment (Docker Compose only)
- End-to-end encryption for messages
- File attachments in messages
- Video/voice calling
- Mobile-responsive UI (desktop-first)
- Horizontal scaling of Policy Service (single instance, can scale later)
- Message search / full-text indexing

## Decisions

### D1: NGAC Engine as Centralized gRPC Policy Service

**Choice: Dedicated Policy Service that all other services call for access checks and graph mutations**

The NGAC engine (in-memory graph + PostgreSQL persistence) becomes a standalone gRPC service. Other services call `CheckAccess()`, `CreateNode()`, `CreateAssignment()`, etc. via gRPC.

**Why:**
- Single source of truth for the policy graph — no sync issues
- Clean microservice boundary — policy is a cross-cutting concern
- Same pattern as Google Zanzibar, Open Policy Agent, AWS IAM
- gRPC on Docker network is sub-millisecond — negligible overhead
- The existing graph.go, access.go, constraints.go code moves directly with minimal changes

**Alternatives considered:**
- Shared library (each service embeds NGAC engine): Leads to stale graph copies across services, complex invalidation
- Sidecar pattern: Operational overhead of managing sidecar lifecycle per service
- REST between services: Slower than gRPC, no type safety from protobufs

### D2: Single Shared PostgreSQL Database

**Choice: One PostgreSQL instance with all tables in the default schema**

All services connect to the same PostgreSQL database. Tables are logically owned by services but physically co-located.

**Why:**
- Simplest operational model for a reference implementation
- Allows joins across service boundaries during development/debugging
- Can migrate to database-per-service later by extracting tables
- NGAC graph tables are naturally shared (every service reads through Policy Service)

**Trade-off:** Services could accidentally access each other's tables directly. Mitigated by disciplined code — services only touch their own tables and call others via gRPC.

### D3: gRPC for All Inter-Service Communication

**Choice: Protocol Buffers + gRPC for synchronous service-to-service calls**

**Why:**
- Type-safe contracts via .proto files — compile-time verification
- Excellent Go support (grpc-go is first-class)
- Binary protocol — faster than JSON/REST
- Streaming support — useful for future features
- Service discovery is simple in Docker Compose (DNS-based)

**Alternatives considered:**
- REST/JSON: Simpler tooling but no contract enforcement, slower serialization
- GraphQL: Overkill for service-to-service, better suited for client-facing APIs

### D4: NATS for Async Event Bus

**Choice: NATS as lightweight event bus for cross-service notifications**

Used for fire-and-forget events: message.sent, member.invited, document.approved. Not for request-response.

**Why:**
- Single binary, ~10MB — minimal Docker footprint
- Go-native (written in Go, excellent Go client)
- At-most-once delivery is sufficient for notifications
- Simple pub/sub model — no complex consumer groups needed
- Can upgrade to JetStream for persistence later if needed

**Alternatives considered:**
- Redis Pub/Sub: Requires Redis deployment, less purpose-built
- RabbitMQ: Heavyweight for this use case
- Kafka: Massively overkill

### D5: Workspace as Self-Governing NGAC Policy Domain

**Choice: Each workspace is a Policy Class with owner-created UAs, OAs, and Associations**

When a user creates a workspace, the system creates:
1. A Policy Class node (the workspace)
2. An Owners UA (creator auto-assigned)
3. A Members UA (empty — explicit invitation only)
4. A Workspace Management OA (target for manage/invite permissions)
5. A Documents OA (container for workspace documents)
6. A Channels OA (container for workspace channels)
7. Default associations granting Owners full permissions

The workspace owner can then:
- Create custom UAs (roles, departments, teams)
- Create custom OAs (document folders, channel groups)
- Create Associations (permission grants between UAs and OAs)
- Assign users to UAs (role/team membership)

This is pure NGAC — no hardcoded roles, no permission tables.

**Why:**
- Maximum flexibility — each workspace can have its own organizational model
- Demonstrates NGAC's killer feature: dynamic policy structure
- "Admin" is just a UA with `manage`+`invite` associations, not a built-in concept
- Permission inheritance via UA→UA assignments (e.g., Owners inherits from Members)

### D6: Messages as Database Rows, Not NGAC Nodes

**Choice: Messages stored only in PostgreSQL messages table. Access controlled at channel level via NGAC.**

Channels are NGAC objects (O node assigned to a Channel Content OA). Messages are plain DB rows with a `channel_id` FK. Before any message read/write, the service calls `CheckAccess(user, channel_oa, "read"/"write")`.

**Why:**
- Avoids graph explosion — a channel with 10K messages would add 10K NGAC nodes
- Individual message-level permissions aren't needed (all-or-nothing channel access)
- Same pattern real chat systems use (Slack doesn't have per-message ACLs)
- Channel-level NGAC check is sufficient and performant

### D7: WebSocket via API Gateway Proxy

**Choice: WebSocket connections terminate at the API Gateway, which proxies to the Messaging Service**

The Gateway authenticates the WebSocket upgrade (JWT), then proxies the connection to the Messaging Service. The Messaging Service maintains a hub of active connections per channel.

**Why:**
- Single entry point for all client connections (HTTP and WS)
- Auth handled consistently at the Gateway level
- Messaging Service doesn't need its own public port
- Can add load balancing later at the Gateway level

### D8: Project Structure

```
ngac/
├── proto/                          ← shared protobuf definitions
│   ├── policy/policy.proto
│   ├── auth/auth.proto
│   ├── workspace/workspace.proto
│   ├── document/document.proto
│   └── messaging/messaging.proto
│
├── services/
│   ├── gateway/                    ← HTTP + WS → gRPC proxy
│   ├── auth/                       ← registration, login, JWT
│   ├── policy/                     ← NGAC engine (graph + access checks)
│   ├── workspace/                  ← workspace CRUD, roles, members
│   ├── document/                   ← document upload, share, workflow
│   └── messaging/                  ← channels, messages, WebSocket hub
│
├── frontend/                       ← React SPA
├── docker-compose.yml
└── Makefile                        ← proto generation, build
```

Each service follows:
```
services/<name>/
├── Dockerfile
├── go.mod
├── cmd/
│   └── main.go
└── internal/
    ├── grpc/          ← gRPC server handlers
    ├── models/        ← service-specific models
    └── store/         ← database access (if applicable)
```

## Risks / Trade-offs

- **[gRPC latency per request]** → Every access check is a network call. Mitigated by Docker network locality (~0.1ms). Can add caching layer to Policy Service later if needed.
- **[Single PostgreSQL contention]** → All services share one DB. Mitigated by low traffic in reference implementation. Can split databases later — table ownership is already clear.
- **[Policy Service as single point of failure]** → If Policy Service goes down, all access checks fail. Mitigated by health checks and restart policy in Docker Compose. Can add replicas later.
- **[Protobuf maintenance overhead]** → Changes require regenerating Go code. Mitigated by Makefile targets and clear proto ownership per service.
- **[WebSocket state management]** → Messaging Service holds WebSocket connections in memory. If it restarts, all connections drop. Clients must implement reconnect logic.
- **[Migration from existing monolith]** → Existing seed data and database schema must be migrated. Mitigated by clean-slate approach for new schema (drop and recreate during development).
- **[NATS message loss]** → At-most-once delivery means events can be lost. Acceptable for notifications — critical operations use synchronous gRPC.
