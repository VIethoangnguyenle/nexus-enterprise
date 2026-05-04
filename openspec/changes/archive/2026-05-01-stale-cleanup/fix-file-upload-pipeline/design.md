## Architecture

No new services. Fixes span infrastructure (docker-compose), backend (messaging + drive REST), and frontend (upload flow + API).

## Root Cause Analysis

```
Upload Flow (currently):
┌──────────┐   POST /channels/:chId/drive     ┌─────────────┐
│ Frontend │ ─────────────────────────────────▶│ Messaging   │
│          │   expects { items: DriveItem[] }  │ REST        │
│          │   ◀──── gets pb.Channel (WRONG!)  │ handler     │
│          │                                   └─────────────┘
│          │   POST /workspaces/:id/drive/files
│          │ ─────────────────────────────────▶┌─────────────┐
│          │   parentId=undefined (no root)    │ Drive       │
│          │   ◀──── { file_id, upload_url }   │ REST        │
│          │                                   └─────────────┘
│          │   PUT upload_url → MinIO ✅
│          │   POST /drive/files/:id/confirm ✅
│          │   POST /channels/:chId/messages
│          │   { linked_entity: drive_file }
│          │   ◀──── message saved ✅
└──────────┘

Problem: file created at orphan root (no channel context),
channelDrive returns wrong type, message IS sent with linked_entity.
```

```
After fix:
┌──────────┐   GET /workspaces/:id/drive         ┌─────────────┐
│ Frontend │   ?drive_context=channel             │ Drive       │
│          │   &drive_context_id=:chId            │ REST        │
│          │   ◀──── { items: DriveItem[] } ✅    │ (existing!) │
│          │                                      └─────────────┘
│          │   POST /workspaces/:id/drive/files
│          │   { parent_id: channelRootFolderId }
│          │   ◀──── { file_id, upload_url } ✅
└──────────┘
```

## Detailed Changes

### 1. Redpanda OOM Fix
- `docker-compose.yml`: `--memory 128M` → `--memory 256M`
- Clear stale volume: `docker volume rm ngac_redpandadata` after stopping

### 2. Frontend — Fix `channelDrive` API call
- `api/drive.ts`: Change `channelDrive()` to call Drive service's `ListRoot` with query params:
  ```
  GET /api/workspaces/:wsId/drive?drive_context=channel&drive_context_id=:channelId
  ```
  This requires passing `wsId` as parameter.

### 3. Frontend — Fix `handleFileUpload` in chat
- Get channel root folder from `channelDrive(wsId, channelId)` 
- If no root folder exists, create one via `driveApi.createFolder(wsId, channelName, parentId, 'channel', channelId)`
- Upload file into that folder as `parentId`

### 4. Backend — Remove misplaced handler
- `messaging/internal/rest/handler.go`: Remove `GetChannelDrive` handler and route
- The drive service's existing `ListRoot` endpoint already supports `drive_context` filter

## Decisions

| Decision | Choice | Rationale |
|----------|--------|-----------|
| Channel drive endpoint | Reuse Drive ListRoot with query params | Already implemented, no new backend code needed |
| Redpanda memory | 256M | Minimum for stable dev operation |
| Remove messaging channelDrive | Yes | Returns wrong type, confusing |
