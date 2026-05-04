## 1. CSS & Visual Fixes

- [x] 1.1 Fix message bubble opacity: update `.msg-bubble` in `index.css` from `rgba(255,255,255,0.03)` to `rgba(255,255,255,0.06)`
- [x] 1.2 Fix timestamp alignment in `channels.$channelId.tsx` `MessageRow`: add `w-fit` (or `width: fit-content`) to the name+timestamp flex row so it hugs content instead of stretching to full width

## 2. Error Handling UX

- [x] 2.1 Replace `alert()` in `ChatEditor.tsx` `handleFileSelect`: set a temporary error state string, render it as red text below the editor, auto-clear after 5 seconds
- [x] 2.2 Replace `alert()` in `FilePreviewCard.tsx` `handleDownload`: show inline error text on the card (e.g., "Download failed" in red below filename), auto-clear after 5 seconds

## 3. LarkRail — Workspace Identity

- [x] 3.1 Add workspace name to `LarkRail.tsx`: accept `workspaceName` prop, render truncated text (max 54px, `text-overflow: ellipsis`) at the top of the rail above the avatar. Show full name in `title` tooltip
- [x] 3.2 Update `_workspace.tsx`: pass `workspaceName` from the resolved workspace data to `<LarkRail>`

## 4. ChatList — Message Preview & Badges

- [x] 4.1 Extend `ChatListItem.tsx` props: add `preview?: string`, `timestamp?: string`, `unreadCount?: number`. Render preview (gray, single line, truncated) below channel name, timestamp (right-aligned, top row), unread badge (red circle with count, right side)
- [x] 4.2 Update `ChatList.tsx`: derive last-message preview and timestamp from WebSocket store or channel data. Pass `preview`, `timestamp`, `unreadCount` to each `ChatListItem`
- [x] 4.3 Update WebSocket store (`websocket.store.ts`): add `lastMessages: Record<string, { content: string, timestamp: string }>` map, update it on each `new_message` event

## 5. Drive Empty State

- [x] 5.1 Update Drive page empty state in `_drive/drive.tsx`: when `items.length === 0`, render the `DriveFileList` table header row (Name, Modified, Created columns) above the empty state illustration, so users see the table structure even in empty folders
- [x] 5.2 ~~Update `DriveFileList.tsx`~~ — SKIPPED: header inlined directly in drive.tsx empty state

## 6. Infrastructure (Already Applied)

- [x] 6.1 Add `MINIO_API_CORS_ALLOW_ORIGIN` env var to MinIO service in `docker-compose.yml` — allows `http://localhost:5173`, `http://localhost`, `http://zump-biz.vn`
- [x] 6.2 Restart MinIO container with CORS config applied — verified with `curl -X OPTIONS` preflight test

## 7. Verification

- [x] 7.1 Build check: `npm run build` passes with no errors (2286 modules, 774ms)
- [ ] 7.2 Visual smoke test: verify all 7 fixes render correctly in browser
