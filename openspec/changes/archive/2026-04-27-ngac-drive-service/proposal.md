## Why

The current Document Service conflates two concerns: file storage (MinIO operations) and file organization/access control (NGAC permissions, sharing, listing). This makes it impossible to build a Google Drive-like experience where:

1. Files live in a navigable folder hierarchy
2. Folders and files can be shared independently with fine-grained NGAC permissions
3. Every chat channel and DM has its own file space (channel drive)
4. Files uploaded in chat are automatically organized and access-controlled
5. Public sharing ("anyone with the link") is a first-class feature

The platform needs a dedicated Drive Service that owns file organization, hierarchy, sharing, and access control — while the Document Service becomes a thin storage layer.

## What Changes

- **NEW**: Drive Service (`:50057`) — folder hierarchy, file registration, NGAC access control, sharing, quotas
- **BREAKING**: Document Service refactored to pure storage API (`DocumentStorageService`) — no more NGAC, no sharing, no listing with permissions
- **MODIFIED**: Messaging Service — channel/DM creation calls Drive to create channel drives; file uploads in chat go through Drive
- **MODIFIED**: Gateway — new Drive routes, updated Document routes, file-in-chat upload flow
- **MODIFIED**: Workspace Service — workspace creation calls Drive to create root drive folder
- **NEW**: Database tables: `drive_items`, `drive_shares`, `drive_quotas`
- **MIGRATION**: Existing `documents` rows become `drive_items` entries in workspace root folders
- **NEW**: Frontend Drive UI — folder tree, file browser, share dialog, "Shared with me" view

## Capabilities

### New Capabilities
- `drive-service`: Dedicated microservice for file organization, hierarchy, and NGAC-based access control
- `folder-hierarchy`: Nested folder structure within workspace and channel drives, with NGAC OA nodes as folders
- `drive-sharing`: Share files or entire folders with users, roles, workspace members, or publicly — all via NGAC associations with inheritance
- `channel-drive`: Every channel and DM automatically gets a drive for file attachments, scoped to channel member permissions
- `public-sharing`: "Anyone with the link" sharing via PublicUsers UA association in NGAC
- `storage-quotas`: Per-workspace storage quota infrastructure (default unlimited)
- `chat-file-upload`: Upload files directly in chat, stored in channel drive, linked to messages

### Modified Capabilities
- `document-management`: Document Service becomes pure storage (presigned URLs, object CRUD). All access control, listing, sharing, approval workflows move to Drive Service
- `messaging-system`: Channel/DM creation triggers channel drive creation. File uploads in chat route through Drive Service
- `dynamic-workspaces`: Workspace creation triggers root drive folder creation with NGAC OA hierarchy

## Impact

**Backend (new service + refactoring)**:
- New `services/drive/` with standard service structure
- New `proto/drive/drive.proto` with DriveService definition
- Refactored `proto/document/document.proto` → minimal `DocumentStorageService`
- Updated `services/document/` — remove NGAC dependency, simplify to storage-only
- Updated `services/messaging/` — integrate with Drive for channel drives
- Updated `services/workspace/` — integrate with Drive for workspace root drive
- Updated `services/gateway/` — new Drive routes, updated flow

**Database**:
- New tables: `drive_items`, `drive_shares`, `drive_quotas`
- Migration: existing `documents` data populates `drive_items`
- Existing `documents` table retained for storage metadata

**Infrastructure**:
- New Docker container for Drive Service
- Drive Service connects to: PostgreSQL, Policy Read/Write, Document Storage Service
- New port `:50057`

**Frontend**:
- New Drive page with folder tree and file browser
- Share dialog with user/role/public sharing options
- "Shared with me" aggregation view
- Chat input file upload integration
- File preview cards in messages
