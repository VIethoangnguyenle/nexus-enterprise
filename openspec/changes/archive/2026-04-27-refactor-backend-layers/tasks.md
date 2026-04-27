## 1. NGAC Constants Package

- [x] 1.1 Create `backend/ngac/ngac_ops.go` with operation constants (`OpRead`, `OpWrite`, `OpUpload`, `OpApprove`, `OpShare`, `OpManage`, `OpInvite`, `OpCreateChannel`)
- [x] 1.2 Add well-known node name constants (`NodePCGlobal`, `NodePublicUsers`)
- [x] 1.3 Add naming convention functions (`ChannelsOAName`, `MembersUAName`, `OwnersUAName`, `ChannelContentOAName`, etc.)
- [x] 1.4 Add node type constants (`TypePC`, `TypeUA`, `TypeOA`, `TypeU`, `TypeO`)
- [x] 1.5 Add access decision constants (`DecisionAllow`, `DecisionDeny`)
- [x] 1.6 Verify build: `go build ./ngac/`

## 2. Database Migration

- [x] 2.1 Add `channel_members` table to `data/init.sql` (`channel_id`, `ngac_node_id`, unique constraint)

## 3. Messaging Store Layer

- [x] 3.1 Create `messaging/internal/store/models.go` with `Channel` and `Message` DB structs
- [x] 3.2 Create `messaging/internal/store/store.go` with channel operations (`InsertChannel`, `GetChannel`, `ListChannelsByWorkspace`, `ListAllDMs`)
- [x] 3.3 Add `FindDMByMembers` using `INTERSECT` query (replace N+1 gRPC calls)
- [x] 3.4 Add `InsertChannelMember` for DM lookup optimization
- [x] 3.5 Add `GetWorkspaceName` helper (workspace name + PC node ID)
- [x] 3.6 Add message operations (`InsertMessage`, `ListMessages`, `GetThread`, `FindByEntity`)
- [x] 3.7 Add thread operations (`IncrementReplyCount`, `TrackThreadParticipant`)

## 4. Messaging Domain Layer

- [x] 4.1 Create `messaging/internal/domain/service.go` with `Service` struct and constructor
- [x] 4.2 Implement `CreateChannel` — orchestrate NGAC node creation, workspace assignment, permissions, DB insert, drive creation
- [x] 4.3 Implement `ListChannels` and `ListDMs` — query + NGAC access filtering
- [x] 4.4 Implement `FindOrCreateDM` — optimized DB lookup, fallback to create
- [x] 4.5 Implement `SendMessage` — access check, insert, thread tracking, username lookup
- [x] 4.6 Implement `GetMessages` — access check, pagination, delegation to store
- [x] 4.7 Implement `GetThread` and `FindThreadsByEntity` — delegation to store
- [x] 4.8 Implement `AddMember`, `RemoveMember`, `ListMembers` — NGAC + store operations
- [x] 4.9 Add proto conversion functions (`channelToProto`, `messageToProto`)

## 5. Messaging Handler Refactor

- [x] 5.1 Refactor `messaging/internal/grpc/server.go` — inject `domain.Service` instead of raw dependencies
- [x] 5.2 Slim each handler to parse → delegate → return pattern (max 20 lines each)
- [x] 5.3 Remove all `s.db` references from handler (SQL lives only in store)
- [x] 5.4 Update `messaging/cmd/main.go` — wire store → domain → handler
- [x] 5.5 Replace all hardcoded strings with `ngac` package constants
- [x] 5.6 Verify build: `go build ./services/messaging/cmd/`
- [x] 5.7 Run tests: `go test ./services/messaging/...`

## 6. Drive Layer Completion

- [x] 6.1 Create `drive/internal/domain/service.go` with `DriveService` struct
- [x] 6.2 Move `ensureRoot` logic from handler to domain layer
- [x] 6.3 Move `ConfirmFile` size update from handler `s.db.Exec` to `store.UpdateFileSize`
- [x] 6.4 Add `store.GetWorkspacePCID` to replace direct DB query in handler
- [x] 6.5 Refactor `drive/internal/grpc/server.go` — delegate to domain, remove all `s.db` direct calls
- [x] 6.6 Replace hardcoded strings with `ngac` package constants (`FolderNodeName`, `DriveRootName`, `ShareOAName`, etc.)
- [x] 6.7 Verify build: `go build ./services/drive/cmd/`
- [x] 6.8 Run tests: `go test ./services/drive/...`

## 7. Gateway Cleanup

- [x] 7.1 Remove `cors.Handler` middleware from Gateway (Traefik handles CORS)
- [x] 7.2 Remove `handleWebSocket` method and `websocket` imports
- [x] 7.3 Remove `wsAddr` field from `Gateway` struct and WS route registration
- [x] 7.4 Create `decodeAndValidate[T]` helper that returns 400 on decode errors
- [x] 7.5 Add required field validation to channel/workspace/document creation endpoints
- [x] 7.6 Update docker-compose.yml — remove WS-related env vars from Gateway
- [x] 7.7 Verify build: `go build ./services/gateway/cmd/`
- [x] 7.8 Run tests: `go test ./services/gateway/...`

## 8. Cross-Service Constants Migration

- [x] 8.1 Update `workspace/internal/grpc/server.go` — replace hardcoded operations and naming patterns with `ngac` constants
- [x] 8.2 Update `drive/internal/grpc/sharing.go` — replace `"PublicUsers"` and operations with constants
- [x] 8.3 Grep verify: no raw operation strings remain in services (`grep -rn '"read"' services/ --include="*.go"` should show only test files)

## 9. Integration Verification

- [x] 9.1 `docker compose build` — all services build successfully
- [x] 9.2 `docker compose up` — all services start and pass healthchecks
- [x] 9.3 Smoke test: create workspace → create channel → send message → receive via WebSocket
- [x] 9.4 Smoke test: create folder → upload file → confirm → download
