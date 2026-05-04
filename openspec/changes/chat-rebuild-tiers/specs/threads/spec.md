# Threads — Spec

## User Stories

### S1: Open Thread from Message
As a workspace member viewing a message with replies,
I want to click "Reply" or reply count badge to open the thread panel,
so that I can read and participate in threaded conversations.

Acceptance Criteria:
- [ ] Messages with `reply_count > 0` show "N replies" badge below content
- [ ] Clicking "N replies" badge opens ThreadPanel
- [ ] Clicking "Reply" in HoverActionBar opens ThreadPanel
- [ ] ThreadPanel shows all replies in chronological order
- [ ] Each reply shows avatar, sender name, timestamp, and content
- [ ] ThreadPanel opens in a right side panel (PeekPanel, 340px)

Proto mapping: `GET /messages/:id/thread` (REST — EXISTS)
WS event: `threadReply` (EXISTS in ws.proto)
Frontend-only: YES — ThreadPanel + useThread hook exist

### S2: Send Reply in Thread
As a workspace member viewing a thread,
I want to type and send a reply within the thread panel,
so that I can participate in the conversation.

Acceptance Criteria:
- [ ] ThreadPanel has a ChatEditor/input at the bottom
- [ ] Typing a reply and pressing Enter sends it
- [ ] New reply appears immediately in thread (optimistic)
- [ ] Reply count on parent message increments
- [ ] Other users see the reply via WebSocket `threadReply` event

Proto mapping: `POST /channels/:id/messages` with `parent_message_id` (REST — EXISTS)
Frontend-only: YES — sendMessage API supports parent_message_id

## Flow

### Open Thread
1. User sees message with "3 replies" badge
2. User clicks badge → ThreadPanel opens
3. System calls `GET /messages/:id/thread`
4. ThreadPanel renders replies

### Send Reply
1. User types in ThreadPanel editor
2. User presses Enter
3. System calls `POST /channels/:id/messages` with `{ parent_message_id: ":parentId", content: "..." }`
4. Optimistic: reply appears in thread
5. WS `threadReply` broadcasts to others

## States
- **Empty**: Thread with no replies → "No replies yet" message
- **Loading**: Spinner while fetching thread
- **Loaded**: Replies rendered chronologically
- **Error**: ErrorState with retry button
