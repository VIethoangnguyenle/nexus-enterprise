## Context

The Lark UI redesign (`lark-ui-redesign` change) established the 3-column workspace architecture (LarkRail → ListPanel → ContentPanel) and standalone Drive page. All 28 implementation tasks completed and `npm run build` passes. However, visual smoke testing exposed 7 bugs across CSS density, data wiring, error handling, and infrastructure configuration.

Current state of affected files:
- `index.css` (L445): `.msg-bubble` at 3% opacity
- `channels.$channelId.tsx` (L298): timestamp in unconstrained flex row
- `ChatEditor.tsx` (L85): `alert()` for upload errors
- `FilePreviewCard.tsx` (L41): `alert()` for download errors
- `LarkRail.tsx`: no workspace name rendered
- `ChatList.tsx` / `ChatListItem.tsx`: no preview/timestamp/unread props wired
- `drive.tsx` (Drive page L237): empty state shows icon only, no table structure
- `docker-compose.yml` (L86): MinIO missing CORS — **already fixed**

## Goals / Non-Goals

**Goals:**
- Fix all 7 identified visual/functional bugs to make the UI demo-ready
- Maintain the architectural decisions from `lark-ui-redesign` — no structural changes
- Keep fixes minimal and surgical — one bug = one focused change

**Non-Goals:**
- No new features or layout changes
- No backend Go service changes (all bugs are frontend/infra)
- No mobile-specific fixes (desktop first)
- No toast notification system overhaul — use a minimal inline approach for now

## Decisions

### Decision 1: Timestamp alignment — use `width: fit-content` on parent

**Choice**: Add `width: fit-content` (or `display: inline-flex`) to the name+timestamp row so it doesn't stretch to fill the flex-1 content column.

**Alternatives considered:**
- Move timestamp inside sender name span: Breaks semantic structure
- Use `flex-grow: 0` on the row: Still stretches due to parent flex-1

**Rationale**: The parent `.flex-1.min-w-0` div correctly fills the remaining space for message content, but the name+timestamp row inside it doesn't need to stretch. `width: fit-content` makes it hug its content.

### Decision 2: Replace `alert()` with inline error feedback

**Choice**: Replace native `alert()` with a brief `console.error` + set an error state that displays as a small red text beneath the editor (ChatEditor) or card (FilePreviewCard).

**Alternatives considered:**
- Build a full toast notification system: Overengineered for this bugfix scope
- Use `window.alert()` (keep as-is): Ugly, blocks the UI thread, unprofessional

**Rationale**: Inline error text is the lowest-effort improvement that feels professional. A proper toast system can come later.

### Decision 3: Message bubble opacity — bump to 6%

**Choice**: `rgba(255, 255, 255, 0.06)` — visible enough to provide grouping, subtle enough to not feel heavy.

**Alternatives considered:**
- 10%: Too prominent for dark mode, creates visual noise
- 3% (current): Invisible
- Use a gray token (`var(--color-gray-4)`): Would work but changes the tinting approach

**Rationale**: 6% provides the "barely there" grouping that Lark uses — messages feel connected without explicit borders.

### Decision 4: Drive empty state — always show table headers

**Choice**: Render `DriveFileList` column headers even when `items.length === 0`, with the empty state illustration below the header row.

**Alternatives considered:**
- Show full table with empty rows: Misleading, looks like a loading bug
- Show only the empty state: Current behavior — loses spatial context

**Rationale**: Google Drive, Notion, and Lark all show column headers even in empty folders. It preserves spatial orientation.

### Decision 5: ChatList preview data — derive from WebSocket cache

**Choice**: Use `useWebSocketStore` last-message cache per channel. If no cached message, show "No messages yet" in gray italic.

**Alternatives considered:**
- Add backend API field `last_message_preview` to channel list: Requires Go changes (out of scope)
- Fetch last message per channel: N+1 query, not scalable

**Rationale**: The WebSocket store already tracks incoming messages. We can maintain a `lastMessages` map keyed by channelId that updates on each `new_message` event. Zero backend changes needed.

### Decision 6: LarkRail workspace name — show at top above avatar

**Choice**: Add a small workspace name text (truncated, max 54px width) at the very top of the rail, above the avatar.

**Alternatives considered:**
- Show in a tooltip on avatar hover: Not discoverable enough
- Show in the ListPanel header instead: Rail needs to establish workspace identity

**Rationale**: Lark shows the org/team name at the top of the rail. A truncated text with tooltip on hover matches.

## Risks / Trade-offs

- **[WebSocket cache for preview]** → If user opens the app fresh, there's no cached last message. The preview will show "No messages yet" until the first WebSocket event arrives. Acceptable for now — backend API enhancement can fill this gap later.

- **[MinIO CORS already applied]** → The docker-compose.yml change was already made and MinIO restarted. If the user runs `docker-compose down && docker-compose up`, the compose v1 `ContainerConfig` bug may resurface. The manual `docker run` workaround is documented.

- **[No toast system]** → Inline errors are less visible than toasts. This is an intentional trade-off: we fix the "native alert" UX regression now, and build a proper toast system in a future change.
