# Chat Rebuild — Tiers 2–5 Consolidated

## Evidence Summary

### Backend (messaging service)
- **Reactions**: ✅ EXISTS — REST endpoints at `/messages/:id/reactions` (POST/DELETE/GET). WS `reactionEvent` broadcast. DB: `message_reactions` table.
- **Pins**: ✅ EXISTS — REST at `/channels/:id/pins` (POST/DELETE/GET). WS `pinEvent` broadcast.
- **Threads**: ✅ EXISTS — REST at `/messages/:id/thread` (GET). WS `threadReply` event.
- **Search**: ✅ EXISTS — REST at `/channels/:id/search?q=` (GET).
- **Members**: ✅ EXISTS — REST at `/channels/:id/members` (GET/POST/DELETE). Remove member endpoint present.
- **Polls**: ✅ EXISTS — REST at `/channels/:id/polls` (POST), `/polls/:id/vote` (POST/DELETE), `/polls/:id` (GET).
- **Tasks**: ✅ EXISTS — REST at `/channels/:id/tasks` (POST/GET), `/tasks/:id` (PATCH).
- **Read receipts**: ✅ EXISTS — REST at `/channels/:id/read` (POST), `/channels/unread` (GET).
- **Edit/Delete message**: ❌ MISSING — No PUT/DELETE endpoint for messages.
- **Mentions**: PARTIAL — Proto field exists (`repeated string mentions`), ChatEditor extracts @mentions, but no backend @mention notification handler.

### Frontend (hooks / stores / API)
- **API client**: ✅ ALL endpoints wired in `api/messaging.ts` (lines 118–162)
- **Hooks**: ✅ `useReaction`, `useTogglePin`, `useThread` — all with optimistic UI
- **WS Store**: ✅ Handlers for `reactionEvent`, `threadReply`, `pinEvent` — all with cache injection
- **Components**:
  - `HoverActionBar.tsx` — React/Reply/Pin/More buttons ✅
  - `EmojiPicker.tsx` — emoji-mart integrated ✅
  - `ReactionBar.tsx` — reaction display with toggle ✅
  - `ThreadPanel.tsx` — PeekPanel thread view ✅
  - `MessageContent.tsx` — @mention highlighting ✅
- **MISSING UI**:
  - Emoji picker not wired into HoverActionBar flow
  - Reaction toggle not connected in MessageList
  - Thread reply not showing reply count on messages
  - Pin list not rendering in ChannelInfoPanel
  - Search UI not implemented (API exists, no UI)
  - Member remove not wired in ChannelInfoPanel
  - @mention autocomplete popup missing
  - Notification settings UI missing

### Proto
- ✅ `messaging.proto` (457 lines) — 20+ RPCs covering all features
- ✅ `ws.proto` (187 lines) — 14 event types

### DB
- ✅ channels, messages, thread_participants, channel_members, message_reactions tables

## Product Assessment

- **Size**: L (large) — but effort is 80% UI wiring, not new infra
- **Risk**: Low — all backend APIs exist, all hooks exist, just need to wire UI
- **Target user**: All workspace members using chat
- **Core action**: Full-featured messaging with reactions, threads, pins, search, and member management

## Key Finding: Backend + Hooks DONE, UI Wiring MISSING

The infrastructure is complete. Every API endpoint exists. Every hook exists with optimistic UI. Every WebSocket handler exists with cache injection. The ONLY work needed is:
1. Wiring existing components together
2. Adding small missing UI elements (emoji picker popup, search panel, pin list)
3. Connecting existing handlers to existing UI actions

## Scope

### In scope (Tier 2 — Reactions + Message Actions)
1. **Emoji reactions** — Wire EmojiPicker popup from HoverActionBar "React" button → call useReaction toggle
2. **Reaction display** — Ensure ReactionBar renders below messages, toggle on click works
3. **@mention autocomplete** — Show member dropdown when typing @ in ChatEditor

### In scope (Tier 3 — Member Management)
4. **Remove member** — Add remove button in ChannelInfoPanel member row → call removeMember API
5. **Member role display** — Show admin/member role if available in member list

### In scope (Tier 4 — Search)
6. **Search UI** — Search panel in ChannelInfoPanel with text input → call searchMessages → render results with jump-to-message

### In scope (Tier 5 — Thread + Pin + Notification)
7. **Thread reply count** — Show "N replies" badge on messages with reply_count > 0
8. **Thread reply input** — Add ChatEditor inside ThreadPanel for sending replies
9. **Pin list** — Render pinned messages in ChannelInfoPanel Pins tab
10. **Attachment/file preview** — FilePreviewCard + ImagePreviewCard already exist, ensure they render in messages

### Out of scope
- Edit/delete message (backend endpoint missing — requires new Go handler)
- @mention notification backend (requires new consumer/producer)
- Notification preferences/settings UI
- Polls UI (complex — separate change)
- Tasks UI (complex — separate change)
- Channel avatar/settings

### Deferred
- Message edit/delete → after backend endpoint is added
- Polls + Tasks → separate autopilot changes
- Advanced notification control → separate change

## Success Criteria

1. User hovers a message → sees React/Reply/Pin/More action bar
2. User clicks React → emoji picker opens → selects emoji → reaction appears below message
3. User clicks existing reaction → toggles (add/remove) instantly
4. User types @ in editor → member autocomplete dropdown appears
5. User clicks Reply → ThreadPanel opens, shows replies, can send new reply
6. Messages with replies show "N replies" clickable badge
7. User clicks Pin → message marked as pinned, appears in Pins tab
8. User searches in Search tab → results appear with message previews
9. Admin can remove member from channel via member panel
10. All interactions work at 375px (mobile), 768px (tablet), 1280px (desktop)
