## 1. Proto & Infrastructure

- [x] 1.1 Create `proto/drive/drive.proto` with `DriveService` definition: CreateFolder, ListFolder, GetItem, MoveItem, CopyItem, RenameItem, TrashItem, RestoreItem, DeleteItem, CreateShare, RevokeShare, ListShares, GetSharedWithMe, CreateDriveForChannel, GetQuota, UpdateQuota, CreateFile (initiate upload), ConfirmFile (finalize upload), GetDownloadURL
- [x] 1.2 Refactor `proto/document/document.proto` — rename service to `DocumentStorageService`, keep only: GetUploadURL, ConfirmUpload, GetDownloadURL, DeleteObject, CopyObject, GetObjectInfo. Remove Share, Approve, Publish, List, CheckAccess RPCs
- [x] 1.3 Create `services/drive/` directory structure: `cmd/main.go`, `internal/grpc/server.go`, `internal/store/store.go`, `go.mod`, `Dockerfile`
- [x] 1.4 Add SQL migration: `drive_items`, `drive_shares`, `drive_quotas` tables to `data/init.sql`
- [x] 1.5 Run `make proto` to generate Go code for new/updated protos
- [x] 1.6 Update `backend/Makefile` to include `proto/drive/drive.proto` generation
- [x] 1.7 Add Drive Service to `docker-compose.yml` on port `:50057` with health check, depends_on policy + postgres
- [x] 1.8 Update Gateway service environment to include `DRIVE_SERVICE_ADDR: drive:50057`

## 2. Document Storage Service Refactor

- [x] 2.1 Update `services/document/internal/grpc/server.go` — remove `Share`, `RevokeShare`, `ListShares`, `Approve`, `Publish`, `Unpublish`, `CheckAccess`, and NGAC-filtered `List` methods
- [x] 2.2 Remove Policy Service client dependency from Document Service (`policyRead`, `policyWrite` fields)
- [x] 2.3 Add `CopyObject` RPC implementation — calls MinIO `CopyObject` (server-side copy)
- [x] 2.4 Add `DeleteObject` RPC implementation — calls MinIO `RemoveObject`
- [x] 2.5 Add `GetObjectInfo` RPC implementation — calls MinIO `StatObject`, returns size, mime_type, last_modified
- [x] 2.6 Update `services/document/cmd/main.go` — remove Policy Service gRPC connection
- [x] 2.7 Update `services/document/Dockerfile` if dependencies changed
- [x] 2.8 Update `services/document/go.mod` — remove policy proto dependency
- [x] 2.9 Verify Document Storage Service builds and starts independently

## 3. Drive Service — Core (Folders & Files)

- [x] 3.1 Implement `store.go` — database access layer for `drive_items`, `drive_shares`, `drive_quotas` tables
- [x] 3.2 Implement `CreateFolder` — insert drive_item (type=folder), create NGAC OA node, assign OA under parent OA
- [x] 3.3 Implement `ListFolder` — query drive_items by parent_id (NULL for root), NGAC CheckAccess filter on each item, return items user can see
- [x] 3.4 Implement `GetItem` — get single drive_item by ID, NGAC read check
- [x] 3.5 Implement `CreateFile` — NGAC check on parent folder, quota check, create NGAC O node, assign to parent OA, insert drive_item (status=pending), call Document Storage GetUploadURL, return presigned URL + file_id
- [x] 3.6 Implement `ConfirmFile` — call Document Storage ConfirmUpload, update drive_item status to active, update quota (used_bytes, used_files)
- [x] 3.7 Implement `GetDownloadURL` — NGAC read check, call Document Storage GetDownloadURL, return presigned URL
- [x] 3.8 Implement `MoveItem` — validate same drive context, NGAC write check on source and destination, update parent_id, NGAC RemoveAssignment + CreateAssignment
- [x] 3.9 Implement `CopyItem` — create new drive_item + new NGAC node, call Document Storage CopyObject for files, assign to destination folder OA
- [x] 3.10 Implement `RenameItem` — NGAC write check, update drive_items.name, update NGAC node name
- [x] 3.11 Implement `TrashItem` — NGAC write check, set status=trashed + trashed_at, recursive for folders (trash all children)
- [x] 3.12 Implement `RestoreItem` — restore from trash, re-validate parent exists
- [x] 3.13 Implement `DeleteItem` — permanent delete: remove NGAC node, call Document Storage DeleteObject, delete drive_item row, update quota
- [x] 3.14 Implement `cmd/main.go` — connect to PostgreSQL, Policy Read/Write, Document Storage Service, start gRPC on :50057

## 4. Drive Service — Sharing

- [x] 4.1 Implement `CreateShare` — create Share OA, assign file/folder O/OA under Share OA, CreateAssociation(target_UA → Share OA, operations), insert drive_shares row. Handle 4 share types: user, role, workspace, public
- [x] 4.2 Implement `RevokeShare` — remove NGAC association, delete NGAC Share OA, delete drive_shares row
- [x] 4.3 Implement `ListShares` — query drive_shares by drive_item_id, return share details with target labels
- [x] 4.4 Implement `GetSharedWithMe` — query drive_shares where target matches user's NGAC node or any UA the user belongs to, return aggregated list of shared items
- [x] 4.5 Ensure folder sharing inheritance works — sharing a folder OA means all child O/OA nodes are accessible via NGAC graph traversal (verify with test)

## 5. Drive Service — Channel/DM Drives

- [x] 5.1 Implement `CreateDriveForChannel` — create channel drive OA (Ch_{name}_Drive), assign under channel's Content OA, create root drive_item (type=folder) for channel, association: channel Members UA → drive OA [read, write, upload]
- [x] 5.2 Implement `GetChannelDrive` — look up drive_items root folder by drive_context='channel' + drive_context_id=channel_id
- [x] 5.3 Update Messaging Service `CreateChannel` — after creating channel, call Drive Service `CreateDriveForChannel`
- [x] 5.4 Update Messaging Service `CreateDM` — after creating DM channel, call Drive Service `CreateDriveForChannel` with DM channel ID
- [x] 5.5 Update Messaging Service `go.mod` — add drive proto dependency
- [x] 5.6 Update Messaging Service `cmd/main.go` — add Drive Service gRPC client connection

## 6. Drive Service — Quotas

- [x] 6.1 Implement `GetQuota` — return workspace quota (create default unlimited row if not exists)
- [x] 6.2 Implement `UpdateQuota` — admin-only: update max_bytes, max_files for workspace
- [x] 6.3 Add quota check in `CreateFile` — reject with RESOURCE_EXHAUSTED if quota exceeded
- [x] 6.4 Add quota update in `ConfirmFile` — atomically increment used_bytes and used_files
- [x] 6.5 Add quota update in `DeleteItem` — atomically decrement used_bytes and used_files

## 7. Workspace Integration

- [x] 7.1 Update Workspace Service `CreateWorkspace` — after creating workspace, call Drive Service to create root drive folder (type=folder, parent=NULL, drive_context=workspace)
- [x] 7.2 Create Drive root OA in workspace NGAC structure: `{WorkspaceName}_Drive` OA under Documents OA, with associations for Owners (all ops) and Members (read)
- [x] 7.3 Update Workspace Service `go.mod` — add drive proto dependency
- [x] 7.4 Update Workspace Service `cmd/main.go` — add Drive Service gRPC client connection

## 8. Gateway — Drive Routes

- [x] 8.1 Add Drive Service gRPC client connection in Gateway
- [x] 8.2 Implement Drive REST routes:
  - `POST /api/workspaces/{id}/drive/folders` → CreateFolder
  - `GET /api/workspaces/{id}/drive` → ListFolder (root)
  - `GET /api/drive/folders/{folderId}` → ListFolder (subfolder)
  - `GET /api/drive/items/{itemId}` → GetItem
  - `POST /api/drive/items/{itemId}/move` → MoveItem
  - `POST /api/drive/items/{itemId}/copy` → CopyItem
  - `PUT /api/drive/items/{itemId}/rename` → RenameItem
  - `DELETE /api/drive/items/{itemId}` → TrashItem
  - `POST /api/drive/items/{itemId}/restore` → RestoreItem
  - `DELETE /api/drive/items/{itemId}/permanent` → DeleteItem
- [x] 8.3 Implement Drive file routes:
  - `POST /api/workspaces/{id}/drive/files` → CreateFile (returns upload URL)
  - `POST /api/drive/files/{fileId}/confirm` → ConfirmFile
  - `GET /api/drive/files/{fileId}/download` → GetDownloadURL
- [x] 8.4 Implement Drive sharing routes:
  - `POST /api/drive/items/{itemId}/share` → CreateShare
  - `DELETE /api/drive/shares/{shareId}` → RevokeShare
  - `GET /api/drive/items/{itemId}/shares` → ListShares
  - `GET /api/drive/shared-with-me` → GetSharedWithMe
- [x] 8.5 Implement channel drive routes:
  - `GET /api/channels/{chId}/drive` → GetChannelDrive + ListFolder
  - `POST /api/channels/{chId}/drive/files` → CreateFile in channel drive
- [x] 8.6 Implement quota routes:
  - `GET /api/workspaces/{id}/drive/quota` → GetQuota
- [x] 8.7 Remove old Document sharing/approval/publish routes from Gateway (or keep as deprecated redirects)

## 9. Data Migration

- [x] 9.1 Write SQL migration to create `drive_items` from existing `documents` — each document becomes a file in workspace root drive folder
- [x] 9.2 Create NGAC OA nodes for workspace root drives (for workspaces that don't have them yet)
- [x] 9.3 Verify migrated documents are accessible through Drive Service
- [x] 9.4 Add backward-compatible Document download route in Gateway (old `/api/documents/{docId}/download-url` maps to Drive's `GetDownloadURL`)

## 10. Frontend — Drive UI

- [x] 10.1 Create Drive page route: `_workspace/drive.tsx` with folder tree sidebar and file grid/list view
- [x] 10.2 Create `DriveItem` component — renders file or folder with icon, name, size, modified date, owner
- [x] 10.3 Create `DriveBreadcrumb` component — clickable path navigation
- [x] 10.4 Create `CreateFolderModal` component — name input for new folder
- [x] 10.5 Create `FileUploadButton` component — file picker + upload progress with presigned URL flow
- [x] 10.6 Create `ShareDialog` component — share with user/role/workspace/public, permission level selector (view/edit)
- [x] 10.7 Create `SharedWithMe` page/view — aggregated list of files/folders shared with current user
- [x] 10.8 Add Drive API hooks: `useDriveFolder`, `useCreateFolder`, `useUploadFile`, `useShareItem`, `useSharedWithMe`, `useDriveQuota`
- [x] 10.9 Add Drive navigation entry in workspace sidebar (between Documents and Channels)
- [x] 10.10 Update chat input — add file upload button that uploads to channel drive, attaches as linked entity
- [x] 10.11 Create `FilePreviewCard` component — renders file attachment in messages (filename, size, download link, thumbnail for images)
- [x] 10.12 Add channel drive panel — accessible from channel header, shows files uploaded in this channel

## 11. Testing & Verification

- [x] 11.1 Write Drive Service unit tests — CreateFolder, ListFolder (NGAC filtering), CreateFile, MoveItem, CopyItem, TrashItem, RestoreItem
- [x] 11.2 Write Drive sharing tests — share with user, share with role (verify inheritance), share publicly, revoke share
- [x] 11.3 Write channel drive tests — create channel with drive, upload file, verify only channel members can access
- [x] 11.4 Write quota tests — upload within quota, reject over quota, delete file reduces quota
- [x] 11.5 Verify Document Storage Service builds without Policy dependency
- [x] 11.6 Verify end-to-end: upload file to drive → share with user → user downloads → revoke share → user denied
- [x] 11.7 Verify chat file upload: upload in chat → message shows file card → channel members can download → non-members denied
- [x] 11.8 Verify folder sharing inheritance: share folder → add file to folder → new file automatically accessible
- [x] 11.9 Verify move vs copy: move within drive preserves explicit shares; copy across drives creates independent file
- [x] 11.10 Full `docker-compose up` — all services healthy including new Drive Service
