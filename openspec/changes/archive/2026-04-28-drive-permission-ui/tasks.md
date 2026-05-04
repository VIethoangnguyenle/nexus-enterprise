## 1. Proto & Backend — BatchCheckAccess

- [x] 1.1 Add `BatchCheckAccess` RPC, `BatchCheckAccessRequest`, `BatchAccessResult`, `ObjectPerms` to `proto/policy/policy_read.proto`
- [x] 1.2 Run `make proto` to regenerate Go stubs
- [x] 1.3 Implement `BatchCheckAccess` in policy-read service (loop single CheckAccess or optimized batch CTE)
- [x] 1.4 Build and verify policy-read service compiles

## 2. Drive REST — Batch Access Endpoint

- [x] 2.1 Add `POST /api/drive/batch-access` handler in `services/drive/internal/rest/handler.go`
- [x] 2.2 Validate request: extract tenant_id from JWT, verify object_ids belong to workspace
- [x] 2.3 Call PolicyRead.BatchCheckAccess gRPC and map response to JSON
- [x] 2.4 Build and verify drive service compiles

## 3. WebSocket Proto — Drive Events

- [x] 3.1 Add `DriveObjectEvent` and `DrivePermEvent` message types to `proto/messaging/ws.proto`
- [x] 3.2 Add `drive_object` and `drive_perm` cases to `ServerEnvelope` oneof
- [x] 3.3 Regenerate TypeScript protobuf stubs for frontend
- [x] 3.4 Build and verify messaging service compiles

## 4. Frontend — Permission Engine

- [x] 4.1 Create `stores/permission.store.ts` — Zustand store with cache Map, TTL, invalidate, clear
- [x] 4.2 Create `api/access.ts` — `batchCheckAccess(objectIds, operations)` API call
- [x] 4.3 Create `hooks/usePermissions.ts` — batch hook that reads cache, queues misses, coalesces fetches
- [x] 4.4 Add tenant switch handler — clear permission cache on tenant change
- [x] 4.5 Verify: permission hook returns correct data for mocked API response

## 5. Frontend — Drive Zustand Store

- [x] 5.1 Create `stores/drive.store.ts` — selectedItemId, viewMode (list/grid), contextPanelOpen, contextPanelTab
- [x] 5.2 Add tree state: expandedFolders Set, activePath array
- [x] 5.3 Add selection state: multi-select support for future bulk actions

## 6. Frontend — Folder Tree Panel

- [x] 6.1 Create `components/drive/DriveTreePanel.tsx` — recursive tree with lazy-load children
- [x] 6.2 Add expand/collapse toggle with arrow icon rotation animation
- [x] 6.3 Add active path highlighting (selected folder + ancestors)
- [x] 6.4 Integrate with ListPanel slot in workspace layout for Drive route
- [x] 6.5 Verify: tree loads root folders, expands on click, persists state across route transitions

## 7. Frontend — Main File List (Virtualized)

- [x] 7.1 Install `@tanstack/react-virtual` dependency
- [x] 7.2 Create `components/drive/DriveFileList.tsx` — virtualized rows using useVirtualizer
- [x] 7.3 Create `components/drive/DriveFileRow.tsx` — memoized row with permission-aware actions
- [x] 7.4 Integrate `usePermissions` — hide actions when permission is false, show skeleton while loading
- [x] 7.5 Add hover-driven action buttons (download, rename, share, trash) with fade-in animation
- [x] 7.6 Add right-click context menu
- [x] 7.7 Verify: list renders 500+ items smoothly, actions match permissions

## 8. Frontend — 3-Panel Drive Layout

- [x] 8.1 Refactor `routes/_workspace/drive.tsx` — split into DriveTreePanel | DriveMainPanel | DriveContextPanel
- [x] 8.2 Add breadcrumb navigation bar in DriveMainPanel header
- [x] 8.3 Wire folder tree selection → main list folder change
- [x] 8.4 Wire file selection → context panel open
- [x] 8.5 Verify: 3-panel layout renders correctly, panels resize properly

## 9. Frontend — Context Panel

- [x] 9.1 Create `components/drive/DriveContextPanel.tsx` — slide-in panel with tab bar
- [x] 9.2 Implement PreviewTab — file icon + basic metadata display
- [x] 9.3 Implement MetadataTab — full metadata (name, size, type, owner, dates)
- [x] 9.4 Implement PermissionsTab — list shares, conditionally visible when share=true
- [x] 9.5 Implement ShareDialog — add user, assign permissions, calls createShare API
- [x] 9.6 Add slide-in/slide-out animation (transform + opacity transition)
- [x] 9.7 Verify: panel opens on selection, tabs switch, share dialog creates/revokes shares

## 10. Frontend — WebSocket Drive Events

- [x] 10.1 Add `driveObject` and `drivePerm` cases to `handleServerMessage` in `websocket.store.ts`
- [x] 10.2 On `driveObject` — invalidate drive folder queries for affected workspace
- [x] 10.3 On `drivePerm` — invalidate permission cache entry for affected item_id
- [x] 10.4 Extend `resyncAfterReconnect` — add drive queries and permission cache clear
- [x] 10.5 Verify: file created by another user appears in real-time, permission change updates action buttons

## 11. Integration & Polish

- [x] 11.1 End-to-end test: signup → create workspace → upload file → verify permissions → share → verify recipient sees file
- [x] 11.2 Verify tenant switch clears all drive state and permission cache
- [x] 11.3 Verify permission revocation while viewing — context panel closes, actions update
- [x] 11.4 Add loading/empty/error states for tree, list, and context panel
- [x] 11.5 Performance check: render 500+ items with virtualization, measure FPS
