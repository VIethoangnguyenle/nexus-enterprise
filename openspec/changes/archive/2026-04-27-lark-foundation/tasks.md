## 1. Proto & Infrastructure

- [x] 1.1 Create `proto/asset/asset.proto` — define AssetTypeService (CreateType, UpdateType, GetType, ListTypes) and AssetService (CreateAsset, UpdateAsset, GetAsset, ListAssets, DeleteAsset, TransitionAsset, GetTransitions, GetHistory) gRPC services
- [x] 1.2 Extend `proto/messaging/messaging.proto` — add thread fields (parent_message_id, linked_entity_type, linked_entity_id, reply_count) to Message, add NotificationService (ListNotifications, MarkRead, MarkAllRead, GetUnreadCount), add notification event types
- [x] 1.3 Create `proto/asset/asset_request.proto` — define AssetRequestService (CreateRequest, ApproveRequest, RejectRequest, AssignAsset, ReturnAsset, ListRequests)
- [x] 1.4 Run `make proto` to generate Go code for all updated protos
- [x] 1.5 Write SQL migration: create `asset_types`, `asset_type_fields`, `assets`, `asset_transitions`, `asset_requests` tables
- [x] 1.6 Write SQL migration: add `parent_message_id`, `linked_entity_type`, `linked_entity_id`, `reply_count` columns to `messages` table
- [x] 1.7 Write SQL migration: create `notifications` table (id, user_id, type, title, body, entity_type, entity_id, read, created_at)
- [x] 1.8 Add Asset Service to `docker-compose.yml` on port 50056 with postgres, redis, redpanda dependencies
- [x] 1.9 Create Kafka topics: `asset.lifecycle`, `asset.request`, `asset.assignment`

## 2. Asset Service — Scaffolding

- [x] 2.1 Initialize `services/asset/` with Go module, cmd/main.go entrypoint, and Dockerfile
- [x] 2.2 Create `internal/store/` — database connection, migration runner
- [x] 2.3 Create `internal/grpc/server.go` — register AssetTypeService and AssetService gRPC handlers
- [x] 2.4 Wire main.go: database, gRPC server, Policy Service client, Kafka producer

## 3. Asset Type System (domain + store + grpc)

- [x] 3.1 Implement `internal/domain/asset_type.go` — DefineType with JSON Schema validation, category assignment, default lifecycle
- [x] 3.2 Implement `internal/domain/lifecycle.go` — state machine definition, validation (reachability check, no orphan states), transition lookup
- [x] 3.3 Implement `internal/store/asset_type.go` — CreateType, GetType, ListTypes, UpdateTypeSchema PostgreSQL queries
- [x] 3.4 Implement `internal/grpc/asset_type_handler.go` — thin gRPC handlers delegating to domain
- [x] 3.5 NGAC integration: on type creation, call Policy Service to create category OA (if new) and type OA under workspace Assets OA, create PC_AssetManagement if first type

## 4. Asset CRUD (domain + store + grpc)

- [x] 4.1 Implement `internal/domain/asset.go` — Create (validate custom fields via JSON Schema, initial state), Update (validate fields, no state change), Delete (soft delete)
- [x] 4.2 Implement `internal/store/asset.go` — CRUD queries with JSONB custom_fields, filtering by type/state/assigned_to, pagination
- [x] 4.3 Implement `internal/grpc/asset_handler.go` — gRPC handlers with NGAC permission checks via Policy Service
- [x] 4.4 On asset creation: create NGAC Object node assigned to type OA
- [x] 4.5 On asset deletion: remove NGAC Object node assignments

## 5. Asset Lifecycle Engine

- [x] 5.1 Implement `internal/domain/transition.go` — ValidateTransition (check state machine), ExecuteTransition (update state, record history), GetAvailableTransitions (filter by NGAC permissions)
- [x] 5.2 Implement `internal/store/transition.go` — InsertTransition, GetTransitionHistory queries
- [x] 5.3 Implement `internal/grpc/lifecycle_handler.go` — TransitionAsset, GetTransitions, GetHistory handlers with NGAC permission check per transition
- [x] 5.4 Kafka producer: emit `asset.lifecycle` event on every state transition

## 6. Asset Request Flow

- [x] 6.1 Implement `internal/domain/request.go` — CreateRequest, ApproveRequest (cannot approve own), RejectRequest, AssignAsset (check not already assigned), ReturnAsset
- [x] 6.2 Implement `internal/store/request.go` — request CRUD, filter by status/requester/approver
- [x] 6.3 Implement `internal/grpc/request_handler.go` — gRPC handlers with NGAC permission checks (request, approve, assign, manage)
- [x] 6.4 Kafka producer: emit `asset.request` on create/approve/reject, `asset.assignment` on assign/return
- [x] 6.5 On assign: update asset assigned_to, transition asset to "assigned" state, create NGAC assignment linking user to asset

## 7. Messaging — Thread Support

- [x] 7.1 Update message store: support parent_message_id, linked_entity_type/id fields
- [x] 7.2 Update send message handler: handle threaded replies (set parent_message_id, increment parent reply_count)
- [x] 7.3 Add GetThread handler: return parent message + all replies ordered by creation time
- [x] 7.4 Update list messages: exclude threaded replies from main channel feed (WHERE parent_message_id IS NULL)
- [x] 7.5 Thread participant tracking: store participants, send thread_reply notification to participants
- [x] 7.6 WebSocket: broadcast `thread_reply` event to thread participants

## 8. Messaging — Notification System

- [x] 8.1 Create notification store: InsertNotification, ListByUser, MarkRead, MarkAllRead, GetUnreadCount
- [x] 8.2 Implement Kafka consumer in Messaging Service: subscribe to `asset.lifecycle`, `asset.request`, `asset.assignment` topics
- [x] 8.3 Notification routing: on asset event, query Policy Service to determine which users have relevant permissions → create notifications for those users
- [x] 8.4 Add NotificationService gRPC handlers: ListNotifications, MarkRead, MarkAllRead, GetUnreadCount
- [x] 8.5 WebSocket notification push: on notification creation, push to connected user's WebSocket
- [x] 8.6 WebSocket: send `notification_count` event on connection establish
- [x] 8.7 System messages: post formatted system messages to configured notification channels on asset events

## 9. Gateway — Route Extensions

- [x] 9.1 Add asset routes: proxy REST → gRPC for asset types, assets, lifecycle, requests
- [x] 9.2 Add notification routes: proxy REST → gRPC for notifications CRUD
- [x] 9.3 Add thread routes: GET /api/messages/{id}/thread, GET /api/threads (with entity filter)
- [x] 9.4 Update WebSocket proxy: pass notification and asset_updated events

## 10. Frontend — Asset Management UI

- [x] 10.1 Add Zustand store slices: assetTypes, assets, assetRequests, notifications
- [x] 10.2 Add API service functions for all asset and notification endpoints
- [x] 10.3 Create AssetDashboard page: summary cards by type, state distribution, recent activity
- [x] 10.4 Create AssetList page: table/card view with type/state/assigned filters, pagination
- [x] 10.5 Create AssetDetail page: custom fields display, state badge, transition buttons, lifecycle timeline, linked threads
- [x] 10.6 Create AssetTypeConfig page (admin): type creation form with field builder, lifecycle editor
- [x] 10.7 Create AssetRequestForm component: type selector, justification input, submit
- [x] 10.8 Create ApprovalQueue page: pending requests list with approve/reject actions

## 11. Frontend — Messaging Enhancements

- [x] 11.1 Add thread panel component: opens on reply click, shows thread messages, reply input
- [x] 11.2 Add reply count indicator on messages with threads
- [x] 11.3 Add notification bell component in header: unread badge, dropdown with recent notifications
- [x] 11.4 WebSocket handler updates: handle `notification`, `notification_count`, `thread_reply`, `asset_updated` events
- [x] 11.5 Add "Assets" section to sidebar navigation with Dashboard, My Assets, Requests links

## 12. Integration & Verification

- [x] 12.1 Build all services: `go build ./cmd/` for asset, messaging, gateway
- [x] 12.2 Run `docker-compose up` and verify all services healthy (13/13 services healthy)
- [x] 12.3 End-to-end test: create asset type → create asset → request → approve → assign → return → dispose (test suite 59/59 pass)
- [x] 12.4 End-to-end test: asset events → Kafka → notifications appear in messaging (notification list/unread/mark-read verified)
- [x] 12.5 End-to-end test: thread creation, replies, entity linking (thread get verified)
- [x] 12.6 NGAC verification: verify permission inheritance (category → type → asset), prohibitions, cross-workspace isolation (59/59 tests with NGAC checks pass)
