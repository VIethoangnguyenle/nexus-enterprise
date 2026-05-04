## Why

File upload to MinIO works (confirmed by user — files exist in bucket), but the UI doesn't display uploaded files. Two root causes:

1. **Redpanda OOM crash** — allocated only 128M, crashes under load with `Failed to allocate 81920 bytes`, causing Kafka consumer failures across services and preventing WebSocket real-time delivery.
2. **`channelDrive` API mismatch** — frontend calls `GET /api/channels/:chId/drive` expecting `{ items: DriveItem[] }`, but the messaging service handler returns a `pb.Channel` object (channel info, not drive items). This means `rootId` for file creation is always `undefined`, and there's no way to list channel files.
3. **FilePreviewCard download fails silently** — even when linked_entity is saved, the download URL endpoint on drive service may fail because file was created without proper channel drive context.

## What Changes

### Infrastructure
- **FIX**: Increase Redpanda memory from 128M → 256M in docker-compose.yml
- **FIX**: Clear stale Redpanda data volume to recover from corruption

### Backend — Drive Service
- **FIX**: Add `GET /api/channels/:chId/drive` endpoint on Drive service REST (not messaging) that lists drive items with `drive_context='channel'` and `drive_context_id=channelId`

### Backend — Messaging Service
- **REMOVE**: Remove `GetChannelDrive` handler from messaging REST — it returns wrong data type
- Route `/api/channels/:chId/drive` should be handled by Drive service

### Frontend
- **FIX**: Simplify `handleFileUpload` — use `driveApi.upload()` orchestrator with channel context
- **FIX**: Ensure `FilePreviewCard` download path works end-to-end

## Impact

- **docker-compose.yml**: Redpanda memory config
- **Drive REST handler**: New channel drive endpoint
- **Messaging REST handler**: Remove misplaced channelDrive handler
- **Frontend upload flow**: Simplified file upload in chat
- **No proto changes, no store schema changes**
