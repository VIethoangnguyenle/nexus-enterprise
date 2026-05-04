## Tasks

### Infrastructure
- [x] Increase Redpanda memory from 128M → 256M in docker-compose.yml
- [ ] Restart Redpanda with clean volume to recover from OOM corruption

### Backend — Messaging Service
- [x] Remove `GetChannelDrive` handler and its route from messaging REST handler

### Frontend — Drive API
- [x] Update `driveApi.channelDrive()` to call `GET /api/workspaces/:wsId/drive?drive_context=channel&drive_context_id=:channelId` (requires wsId param)
- [x] Update `useDrive.ts` hook `channelDriveQueryOptions` to pass wsId
- [x] Update `ChannelDrivePanel.tsx` to accept and pass wsId prop

### Frontend — Chat Upload Flow
- [x] Fix `handleFileUpload` in `channels.$channelId.tsx` to use correct channelDrive call with wsId
- [x] Find channel root folder (by item_type='folder') instead of blindly using first item's parent_id
- [x] Ensure `send.mutate` includes correct `linkedEntity` after upload (already correct)

### Verification
- [x] `go build` pass for messaging service
- [x] `npm run build` pass for frontend
- [ ] Manual test: upload file in chat → file card appears in message
- [ ] Manual test: download file from FilePreviewCard works
