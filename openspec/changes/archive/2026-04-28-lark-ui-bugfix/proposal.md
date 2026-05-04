## Why

The Lark UI redesign (change: `lark-ui-redesign`) is architecturally complete — 3-column layout, LarkRail, resizable ListPanel, standalone Drive page all render and build. However, a visual smoke test reveals **7 bugs** spanning CSS density, CORS infrastructure, NGAC policy gaps, and incomplete data wiring that prevent the UI from being usable in practice.

These bugs block the platform from being demo-ready. Each one creates friction or outright failure in core user workflows: reading messages, uploading files, downloading attachments, and navigating the workspace.

## What Changes

### Bug 1 — Chat timestamp misaligned
Timestamps in message headers drift to the far right of the content area instead of sitting tight next to the sender name. Fix: constrain the name+timestamp flex container.

### Bug 2 — File download "access denied" + ugly `alert()`
Clicking a file attachment in chat triggers a native browser `alert()` with "Download failed: access denied." Two issues: (a) NGAC policy may deny read on the file's O-node if the user→UA→OA assignment chain is broken after upload; (b) the frontend uses `alert()` instead of a proper toast notification.

### Bug 3 — Message bubble nearly invisible
`.msg-bubble { background: rgba(255,255,255,0.03) }` is 3% opacity — functionally transparent on dark backgrounds. Needs ~8% to provide subtle visual grouping without being heavy.

### Bug 4 — Drive empty state missing table structure
When Drive folder is empty, the page shows a centered empty-state icon but no table column headers. Users lose spatial context — they don't know what columns exist until files appear.

### Bug 5 — LarkRail missing workspace name
The rail shows avatar + module icons but no workspace name/org identity. Lark always shows the org/workspace name at the top of the rail.

### Bug 6 — ChatList missing message preview & timestamp
Channel items in the list panel only show the channel name. Missing: last message preview text, timestamp, and unread count badge — the three key signals that make a chat list scannable.

### Bug 7 — File upload fails (CORS + MinIO)
Upload flow: browser → Drive API (create file) → presigned PUT URL → browser PUTs directly to MinIO:9000. The presigned URL points to `localhost:9000`, which is cross-origin from `localhost:5173`. MinIO container had no CORS config, so the browser blocks the preflight OPTIONS request. **Partially fixed**: `MINIO_API_CORS_ALLOW_ORIGIN` added to docker-compose.yml. Remaining: error handling UX in ChatEditor when upload fails.

## Capabilities

### Modified Capabilities
- `lark-messaging-layout`: Fix timestamp alignment, bubble opacity, upload error handling in chat view
- `lark-rail-navigation`: Add workspace name display to LarkRail
- `chat-list-panel`: Wire last message preview, timestamp, and unread badge into ChatListItem
- `drive-standalone-page`: Show table headers in empty state; improve download error UX

## Impact

- **Frontend components**: `channels.$channelId.tsx` (MessageRow timestamp fix), `ChatEditor.tsx` (upload error toast), `LarkRail.tsx` (workspace name), `ChatList.tsx` / `ChatListItem.tsx` (preview/timestamp wiring), Drive page (empty state table headers), `FilePreviewCard.tsx` (download error toast)
- **CSS**: `index.css` (msg-bubble opacity bump)
- **Infrastructure**: `docker-compose.yml` (MinIO CORS — already applied)
- **No backend Go changes** — all bugs are frontend/infra layer
