# Messaging — TIER 1 Spec

## Proto Mapping

| Feature | RPC | Status |
|---------|-----|--------|
| Send message | `SendMessage` | ✅ EXISTS |
| Get messages | `GetMessages` | ✅ EXISTS (paginated) |
| Edit message | — | ❌ MISSING (needs new RPC + DB column) |
| Delete message | — | ❌ MISSING (needs new RPC + soft delete) |
| Mark read | `MarkChannelRead` | ✅ EXISTS |
| Get unread | `GetUnreadCounts` | ✅ EXISTS |

## Backend Gaps

### Edit Message
- Need: `UpdateMessage` RPC in proto
- Need: `updated_at` column in messages table
- Need: Store `edited` flag for UI display
- REST: `PATCH /channels/{channelId}/messages/{messageId}`

### Delete Message
- Need: `DeleteMessage` RPC in proto
- Soft delete: `deleted_at` column OR `status` column
- REST: `DELETE /channels/{channelId}/messages/{messageId}`
- Realtime: WS event `MessageUpdatedEvent` for edit/delete sync

> **BA Decision**: Defer edit/delete to TIER 2. Reason: requires proto change + migration + WS event. Core messaging (send/receive) already works. Focus TIER 1 on quality of existing send/receive flow.

---

## User Stories

### US-1: Send Message (realtime)

As a **workspace member**,
I want to **send a message in a channel and see it appear immediately**,
so that **I can communicate in realtime with my team**.

Acceptance Criteria:
- [ ] User types message in editor and presses Enter/Send
- [ ] Message appears in chat window within 500ms (optimistic UI)
- [ ] Other channel members see the message within 1 second (WS broadcast)
- [ ] Message displays sender name (NOT UUID), timestamp
- [ ] Empty message cannot be sent (button disabled / Enter ignored)
- [ ] Long messages render correctly (no overflow/truncation)
- [ ] Send button shows loading state during mutation
- [ ] Error: if send fails, message shows error state with retry option

Proto: `SendMessage` (existing)
Type: Frontend-only (backend already works)

### US-2: Receive Messages (realtime, no polling)

As a **workspace member**,
I want to **see new messages appear automatically without refreshing**,
so that **conversations feel live and immediate**.

Acceptance Criteria:
- [ ] When another user sends a message, it appears within 1s
- [ ] NO invalidateQueries for message delivery — cache injection via WS
- [ ] Messages maintain correct chronological order
- [ ] New messages trigger scroll-to-bottom (if user is at bottom)
- [ ] If user has scrolled up, show "New messages" indicator instead of auto-scroll
- [ ] After reconnect (WS drop), messages sync automatically

Proto: WS `ChatMessage` event (existing)
Type: Frontend-only (WS already broadcasts)

### US-3: Message History (pagination)

As a **workspace member**,
I want to **scroll up to see older messages**,
so that **I can read conversation history**.

Acceptance Criteria:
- [ ] Initial load shows latest 50 messages
- [ ] Scroll to top triggers load-more (infinite scroll)
- [ ] Loading indicator shows while fetching older messages
- [ ] "No more messages" state when history ends
- [ ] Messages grouped by date separator (Today, Yesterday, date)

Proto: `GetMessages` with cursor pagination (existing)
Type: Frontend-only

### US-4: Conversation Unread Count

As a **workspace member**,
I want to **see unread message counts on each conversation**,
so that **I know where new activity is**.

Acceptance Criteria:
- [ ] Unread count badge appears on conversation list items
- [ ] Count updates in realtime when new messages arrive (WS)
- [ ] Count resets when user opens the conversation
- [ ] Mark-as-read fires when user views messages for >1 second
- [ ] Unread indicator style: badge with number (1-99), "99+" for more

Proto: `GetUnreadCounts`, `MarkChannelRead` (existing)
Type: Frontend polish

---

## Flows

### Flow: Send Message
1. User is in a channel view (`/channels/$channelId`)
2. User types in ChatEditor at bottom of screen
3. User presses Enter or clicks Send button
4. System: optimistic insert (temp message with sender info)
5. System: calls `POST /channels/{channelId}/messages`
6. System: WS broadcasts `ChatMessage` to channel subscribers
7. System: replaces temp message with server response (dedup by ID)
8. Other users: WS delivers message, cache injection adds to list

Error flow:
- If API call fails → roll back optimistic message, show error toast
- If WS disconnects → reconnect with backoff, resync on auth

### Flow: View Unread
1. User opens app → WS connects → auth
2. System: fetches unread counts via `GET /channels/unread`
3. Chat list renders with unread badges
4. User clicks a channel → navigates to channel view
5. After 1s visible → system calls `POST /channels/{channelId}/read`
6. Unread count resets to 0

---

## States

| Screen | Empty | Loading | Loaded | Error |
|--------|-------|---------|--------|-------|
| Message list | "No messages yet. Start the conversation!" | Skeleton/spinner | Messages rendered | "Failed to load messages" + retry |
| Chat editor | Placeholder "Type a message..." | Send button spinner | Normal input | Error toast below editor |
