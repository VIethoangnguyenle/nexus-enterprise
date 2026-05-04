# Chat Rebuild — TIER 1 MVP

## Evidence Summary

- **Backend**: EXISTS — full messaging service at `backend/services/messaging/`
  - Domain: `internal/domain/service.go` (538 lines) — channels, messages, members, DMs, threads
  - Store: `internal/store/store.go` (273 lines) — channels, messages, channel_members
  - gRPC: `internal/grpc/server.go` + hub.go (572 lines — WS hub with Redis pub/sub)
  - REST: `internal/rest/handler.go` (10.9KB) — full HTTP API for channels/messages/members
  - Events: `internal/events/consumer.go` + `producer.go` — Redpanda integration
- **Frontend**: EXISTS but incomplete quality
  - Route: `routes/_workspace/channels.$channelId.tsx` — chat view exists
  - Components: `components/chat/` — 10 files (ChatEditor, MessageList, ChannelInfoPanel, etc.)
  - Patterns: `components/patterns/` — ChatList, ChatListItem, MessageItem, ListPanel
  - Hooks: `hooks/useMessaging.ts` (435 lines) — full CRUD hooks with optimistic UI
  - API: `api/messaging.ts` (162 lines) — full REST client
  - Store: `stores/websocket.store.ts` (464 lines) — protobuf WS with cache injection
- **Proto**: EXISTS — `proto/messaging/messaging.proto` (457 lines) + `ws.proto` (187 lines)
  - MessagingService: 20+ RPCs (channels, messages, members, reactions, pins, search, polls, tasks)
  - WS events: 14 event types (chat, typing, reaction, pin, read receipt, poll, task, drive, approval)
- **DB**: EXISTS — `data/init.sql` lines 89-144
  - channels, messages, thread_participants, channel_members tables
  - Proper indexes on channel_id, created_at, parent_message_id
- **Dependencies**: ALL present
  - Auth service: user lookup via gRPC ✓
  - Policy service: NGAC access control ✓
  - WebSocket hub: Redis pub/sub for horizontal scaling ✓
  - Redpanda: event streaming ✓

## Product Assessment

- **Size**: L (large) — backend complete but frontend needs significant UI overhaul + realtime hardening
- **Risk**: Medium — backend APIs work, but current frontend has quality/consistency gaps that need systematic fix
- **Target user**: All workspace members who collaborate via messaging
- **Core action**: Send/receive realtime messages, manage conversations, view/add members

## Key Finding: Infrastructure is SOLID, UI is the Problem

Backend has full CRUD + realtime via WebSocket + Redis pub/sub. The system works.
The problem is:
1. **UI quality** — inconsistent, not polished, missing empty states
2. **Feature integration** — individual features work but aren't cohesive
3. **Conversation management** — create/rename/list conversations works but UX is weak
4. **Member management** — API works but UI is minimal
5. **Presence** — typing indicator exists, online/offline NOT implemented
6. **Message status** — no delivery/read indicators on individual messages

## Scope

### In scope (TIER 1 MVP)

1. **Messaging (harden existing)**
   - Send/receive message (already works, harden realtime)
   - Edit/delete message (basic — need to add backend + frontend)
   - Message status indicators (sent/delivered visual feedback)

2. **Conversation (improve UX)**
   - Create conversation (1-1 via DM, group via channel) — API exists, improve UI
   - Rename conversation — need to add API endpoint + UI
   - Conversation list with last message preview + unread count — partially works, polish UI

3. **Presence (new)**
   - Online/offline indicator — need backend tracking + WS event + frontend display
   - Typing indicator — already works

4. **Member (improve UX)**
   - View members — works, improve UI
   - Add member (simple flow) — works, improve UX with user picker

5. **UI Rebuild (redesign in Stitch)**
   - Chat list — redesign for Lark-quality
   - Chat window — polish consistency
   - Message item — redesign with proper grouping, hover actions
   - Empty states — design for no-messages, no-channels
   - Mobile responsive — ensure 375px works

### Out of scope

- Emoji reactions (TIER 2)
- @mention (TIER 2)
- Message actions beyond edit/delete (TIER 2)
- Remove member (TIER 3)
- Roles admin/member (TIER 3)
- Group settings avatar (TIER 3)
- Search (TIER 4)
- Thread, attachment, pin, notification control (TIER 5)

### Deferred

- Message edit/delete: if backend complexity is too high, defer to TIER 2
- Online/offline: if realtime tracking needs Redis key expiry infra, simplify to last-seen

## Success Criteria

1. User A sends message → User B sees it within 1 second (no page refresh)
2. Conversation list shows last message + unread count, updates in realtime
3. Create group conversation → members can immediately chat
4. Add member → they can immediately see and send messages
5. Typing indicator shows when other user is typing
6. UI displays display names (NEVER UUIDs)
7. All views work on mobile (375px)
8. Empty states for all zero-data scenarios
