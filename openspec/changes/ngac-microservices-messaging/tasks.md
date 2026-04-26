## 1. Project Structure & Infrastructure

- [ ] 1.1 Create new project directory structure: `proto/`, `services/{gateway,auth,policy,workspace,document,messaging}/`
- [ ] 1.2 Define Protocol Buffer files: `proto/policy/policy.proto`, `proto/auth/auth.proto`, `proto/workspace/workspace.proto`, `proto/document/document.proto`, `proto/messaging/messaging.proto`
- [ ] 1.3 Create Makefile with `proto` target to generate Go code from .proto files
- [ ] 1.4 Initialize `go.mod` for each service with shared proto dependency
- [ ] 1.5 Update `docker-compose.yml` with all 9 containers (6 services + postgres + nats + frontend), health checks, and dependency ordering
- [ ] 1.6 Create Dockerfile for each service (multi-stage Go build)
- [ ] 1.7 Add PostgreSQL init script to create schema for all tables (users, workspaces, documents, channels, messages + existing ngac_* tables)

## 2. Policy Service (NGAC Engine)

- [ ] 2.1 Migrate existing `ngac/graph.go`, `ngac/access.go`, `ngac/constraints.go` into `services/policy/internal/ngac/`
- [ ] 2.2 Migrate existing `ngac/store.go` and `ngac/helpers.go` — update InitSchema for new tables (workspaces, channels, messages)
- [ ] 2.3 Add new operations to models: `manage`, `invite`, `create_channel`
- [ ] 2.4 Implement gRPC server for PolicyService proto: CheckAccess, CreateNode, DeleteNode, FindNodeByName, GetNodesByType
- [ ] 2.5 Implement gRPC server for graph operations: CreateAssignment, RemoveAssignment, CreateAssociation, RemoveAssociation
- [ ] 2.6 Implement gRPC server for graph queries: GetAncestors, GetDescendants, GetChildren, GetParents
- [ ] 2.7 Create `services/policy/cmd/main.go` — load graph on startup, register constraint engine, start gRPC server on :50051
- [ ] 2.8 Verify: Policy Service starts, loads graph from DB, and responds to gRPC calls

## 3. Auth Service

- [ ] 3.1 Migrate existing `auth/` package (JWT, password hashing) into `services/auth/internal/auth/`
- [ ] 3.2 Implement simplified registration — username + password only, no company/department
- [ ] 3.3 On registration: call Policy Service gRPC to create User node and assign to PublicUsers UA
- [ ] 3.4 Implement login — validate credentials, return JWT with user_id, username, ngac_node_id claims
- [ ] 3.5 Implement gRPC server for AuthService proto: Register, Login, GetUserByID, ValidateToken
- [ ] 3.6 Create `services/auth/cmd/main.go` — connect to PostgreSQL and Policy Service, start gRPC on :50052
- [ ] 3.7 Verify: Register user, login, verify JWT contains correct claims

## 4. Workspace Service

- [ ] 4.1 Implement workspace creation: create PC node, Owners/Members UAs, Mgmt/Documents/DraftDocs/ApprovedDocs/Channels OAs, default associations — all via Policy Service gRPC calls
- [ ] 4.2 Implement workspace listing — find user's workspaces by traversing NGAC graph (user → UAs → PCs)
- [ ] 4.3 Implement member invitation — assign user's NGAC node to specified workspace UAs, with `invite` permission check
- [ ] 4.4 Implement member removal — remove all user's assignments to UAs under the workspace PC
- [ ] 4.5 Implement member listing — find all User nodes under workspace's PC UA hierarchy with their role assignments
- [ ] 4.6 Implement dynamic role (UA) creation within workspace — with `manage` permission check and PC scoping validation
- [ ] 4.7 Implement dynamic folder (OA) creation within workspace — with `manage` permission check and PC scoping validation
- [ ] 4.8 Implement permission (association) management — create/delete associations between workspace UAs and OAs, with scoping validation
- [ ] 4.9 Implement ownership transfer — assign new owner to Owners UA, remove old owner, with last-owner protection
- [ ] 4.10 Implement add/remove co-owner with last-owner constraint
- [ ] 4.11 Implement role assignment changes for existing members
- [ ] 4.12 Create `services/workspace/cmd/main.go` — connect to PostgreSQL, Policy Service, and NATS, start gRPC on :50053
- [ ] 4.13 Verify: Create workspace, invite member, create roles, assign permissions, transfer ownership

## 5. Document Service

- [ ] 5.1 Migrate existing document upload/download logic into `services/document/internal/`
- [ ] 5.2 Refactor document operations to call Policy Service for access checks (replacing local graph calls)
- [ ] 5.3 Scope documents to workspaces — upload creates NGAC Object node under workspace's Documents OA
- [ ] 5.4 Implement document listing per workspace with NGAC filtering (user can only see docs they have read access to)
- [ ] 5.5 Preserve approve workflow (draft→approved) via Policy Service graph mutations
- [ ] 5.6 Preserve share workflow (scoped Share OAs under SharedDocs) via Policy Service
- [ ] 5.7 Preserve publish/unpublish (PublicDocs assignment) via Policy Service
- [ ] 5.8 Implement gRPC server for DocumentService proto: Upload, Download, List, Delete, Approve, Share, Publish
- [ ] 5.9 Create `services/document/cmd/main.go` — connect to PostgreSQL, Policy Service, and NATS, start gRPC on :50054
- [ ] 5.10 Verify: Upload document to workspace, share cross-workspace, approve, publish — all with NGAC enforcement

## 6. Messaging Service

- [ ] 6.1 Implement channel creation: create Channel_Content OA, channel Object, Channel_Members UA, association — all via Policy Service gRPC. Store channel metadata in `channels` table
- [ ] 6.2 Implement channel listing per workspace — filter channels where user is assigned to Members UA
- [ ] 6.3 Implement channel member management — add/remove users from Channel_Members UA via Policy Service
- [ ] 6.4 Implement send message — CheckAccess(user, channel_oa, "write") via Policy Service, then INSERT into `messages` table
- [ ] 6.5 Implement read messages — CheckAccess(user, channel_oa, "read"), then SELECT from `messages` with pagination (cursor-based using timestamp)
- [ ] 6.6 Implement DM creation — create DM channel with Content OA and Members UA under PC_Global, assign both users, deduplicate existing DMs
- [ ] 6.7 Implement DM listing — find all DM channels where user is a member
- [ ] 6.8 Implement WebSocket hub — connection management, channel subscriptions, message broadcast
- [ ] 6.9 Implement WebSocket authentication — validate JWT from query param on upgrade
- [ ] 6.10 Implement typing indicator events via WebSocket
- [ ] 6.11 Implement NATS event publishing for message.sent, channel.created events
- [ ] 6.12 Implement gRPC server for MessagingService proto: CreateChannel, ListChannels, AddMember, SendMessage, GetMessages, CreateDM, ListDMs
- [ ] 6.13 Create `services/messaging/cmd/main.go` — connect to PostgreSQL, Policy Service, NATS, start gRPC on :50055 + WebSocket server
- [ ] 6.14 Verify: Create channel, add members, send messages, receive via WebSocket, DM flow

## 7. API Gateway

- [ ] 7.1 Implement HTTP router (Chi) with CORS, logging, recovery middleware
- [ ] 7.2 Implement JWT validation middleware (stateless — verify signature, extract claims)
- [ ] 7.3 Implement gRPC client connections to all 5 backend services
- [ ] 7.4 Implement REST→gRPC proxy for auth routes: POST /api/auth/register, POST /api/auth/login
- [ ] 7.5 Implement REST→gRPC proxy for workspace routes: CRUD, invite, members, roles, folders, permissions, ownership
- [ ] 7.6 Implement REST→gRPC proxy for document routes: upload, download, list, approve, share, publish
- [ ] 7.7 Implement REST→gRPC proxy for messaging routes: channels, messages, DMs
- [ ] 7.8 Implement WebSocket proxy — authenticate upgrade, proxy connection to Messaging Service
- [ ] 7.9 Create `services/gateway/cmd/main.go` — connect to all services, start HTTP server on :8080
- [ ] 7.10 Verify: All API endpoints accessible through Gateway, JWT enforced on protected routes

## 8. Frontend — Core Layout

- [ ] 8.1 Add workspace selector component (top bar or sidebar header) with workspace list and "Create Workspace" button
- [ ] 8.2 Add workspace creation dialog with name input
- [ ] 8.3 Restructure App.jsx routing: add workspace-scoped routes (`/workspace/{id}/...`)
- [ ] 8.4 Implement sidebar navigation component with sections: Documents, Channels, Direct Messages, Settings
- [ ] 8.5 Add Zustand stores for workspace state (current workspace, workspace list, members)
- [ ] 8.6 Simplify registration page — remove company/department fields
- [ ] 8.7 Update login response handling — no company/department in user info

## 9. Frontend — Messaging UI

- [ ] 9.1 Create channel list component in sidebar — shows channels user is a member of
- [ ] 9.2 Create chat window component — message list with sender, timestamp, content
- [ ] 9.3 Create message input component with send button and Enter key handler
- [ ] 9.4 Create DM list component in sidebar
- [ ] 9.5 Create "New Channel" dialog with name input
- [ ] 9.6 Create "New DM" dialog with user selector
- [ ] 9.7 Add Zustand stores for messaging state (channels, messages, active channel)
- [ ] 9.8 Implement WebSocket client with auto-reconnect (exponential backoff)
- [ ] 9.9 Integrate WebSocket events: real-time message display, typing indicators
- [ ] 9.10 Implement optimistic message sending (show message immediately, confirm on server response)
- [ ] 9.11 Implement message pagination (scroll up to load older messages)

## 10. Frontend — Workspace Admin Panel

- [ ] 10.1 Create admin settings page (visible only to users with `manage` permission)
- [ ] 10.2 Create roles management tab — list, create, delete custom UAs
- [ ] 10.3 Create permissions management tab — list, create, delete associations (UA → OA + operations)
- [ ] 10.4 Create members management tab — list members with role tags, invite, remove, change roles
- [ ] 10.5 Create invite member dialog — user search + role selection
- [ ] 10.6 Create channel members management — add/remove members from channels
- [ ] 10.7 Add Zustand stores for admin state (roles, permissions, members)

## 11. Seed Data & Testing

- [ ] 11.1 Create new seed data: sample workspace with roles (Admin, Engineering, Viewer), channels (#general, #engineering), sample documents, 4 users with different role assignments
- [ ] 11.2 Verify workspace creation + member invitation flow end-to-end
- [ ] 11.3 Verify channel access isolation — non-members cannot read messages
- [ ] 11.4 Verify DM privacy — only 2 participants can access
- [ ] 11.5 Verify workspace boundary — users in Workspace A cannot access Workspace B resources
- [ ] 11.6 Verify ownership transfer with last-owner protection
- [ ] 11.7 Verify dynamic role creation and permission assignment
- [ ] 11.8 Verify existing constraints (weekday-only editing) still work through Policy Service
- [ ] 11.9 Full docker-compose up test — all services healthy, frontend loads, end-to-end flow works
