## Why

The current NGAC document platform is a monolithic Go application with pre-seeded company/department structures. This limits its utility as a real-world reference implementation:

1. **No self-service**: Users can't create their own workspaces — companies and departments must be pre-seeded.
2. **No messaging**: The platform only handles documents. Real collaboration requires integrated messaging (channels, DMs).
3. **Rigid organization**: The company→department hierarchy is fixed. Real organizations need dynamic roles, teams, and permission structures.
4. **Monolithic backend**: A single Go binary handles auth, documents, NGAC engine, and admin — limiting independent scalability and deployment.

This change evolves the platform into a microservices-based collaboration system where users create workspaces, define their own organizational structure (roles, departments, permissions), manage documents, and communicate via channels — all governed by NGAC.

## What Changes

- **BREAKING**: Registration simplified — no longer requires company/department at signup
- **BREAKING**: Remove pre-seeded company/department model — workspaces are user-created
- **BREAKING**: Backend split from single monolith into 6 microservices communicating via gRPC
- Add workspace creation with dynamic organizational structure (custom roles, departments, permissions)
- Add workspace ownership and ownership transfer with NGAC-enforced constraints
- Add messaging system: workspace channels, private channels, direct messages
- Add real-time communication via WebSocket
- Add NATS event bus for async inter-service communication
- Add API Gateway for routing, JWT validation, and WebSocket proxying
- Existing NGAC engine (graph traversal, access checks, constraints) moves to dedicated Policy Service — core algorithm unchanged
- Existing document features (upload, share, approve, publish) move to dedicated Document Service
- Frontend extended with workspace selector, channel/chat UI, and workspace admin panel

## Capabilities

### New Capabilities
- `microservice-architecture`: Split monolith into 6 services (Gateway, Auth, Policy, Workspace, Document, Messaging) communicating via gRPC, with NATS event bus and shared PostgreSQL
- `dynamic-workspaces`: User-created workspaces with dynamic organizational structure — custom roles (UAs), document folders (OAs), permission associations — all as NGAC graph mutations
- `workspace-ownership`: Ownership semantics via NGAC User Attributes with transfer support and last-owner protection
- `messaging-system`: Channel-based messaging integrated with NGAC — workspace channels, private channels, DMs — with channel membership as UA assignments and access enforced via standard CheckAccess
- `realtime-websocket`: WebSocket-based real-time message delivery, typing indicators, and presence — proxied through API Gateway

### Modified Capabilities
- `user-auth`: Registration no longer requires company/department; users register with username+password only, then create/join workspaces separately
- `ngac-engine`: Core algorithm unchanged, but extracted into standalone gRPC Policy Service; store layer updated for new schema (workspaces, channels tables)
- `document-management`: Documents scoped to workspaces instead of pre-seeded companies; document service becomes independent microservice calling Policy Service for access checks
- `frontend-ui`: Major overhaul — workspace selector, sidebar navigation (documents/channels/DMs/settings), chat interface, workspace admin panel for role/permission management

## Impact

**Backend (major restructure)**:
- `backend/` directory replaced by `services/` with 6 independent Go services
- Shared protobuf definitions in `proto/` directory
- Each service has its own Dockerfile, entrypoint, and internal packages
- NGAC engine code (`graph.go`, `access.go`, `constraints.go`) migrates directly to Policy Service

**Database**:
- Single PostgreSQL instance retained (no split for now)
- New tables: `workspaces`, `channels`, `messages`
- Existing tables modified: `users` (remove company/department requirement), `documents` (add workspace reference)
- NGAC tables unchanged: `ngac_nodes`, `ngac_assignments`, `ngac_associations`

**Infrastructure**:
- Docker Compose updated: 6 service containers + PostgreSQL + NATS + frontend
- New dependency: NATS server for event bus
- New dependency: gRPC + Protocol Buffers for inter-service communication

**Frontend**:
- New pages: workspace management, messaging/chat
- New components: workspace selector, channel list, chat window, admin panel
- Zustand stores extended for workspace, channel, and message state
- WebSocket client for real-time messaging

**API Surface**:
- Existing REST endpoints preserved at Gateway level but routed to respective services
- New workspace, channel, and messaging endpoints
- WebSocket endpoint at `/api/ws`
