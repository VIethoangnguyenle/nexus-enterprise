# Chat Rebuild MVP — Architecture

## Architecture Decision: Enhance, Don't Rewrite

The existing architecture is clean and well-separated:
```
Frontend (React/TanStack) → REST API (Echo) → Domain Layer → Store (pgx) → PostgreSQL
                          ↕ WebSocket (protobuf binary) → Hub (Redis pub/sub) → Clients
```

**Decision**: Enhance existing layers. No new services, no schema changes beyond optional additions.

## Backend Changes (2 items)

### 1. Rename Channel (`PATCH /channels/:chId`)
- **Store**: Add `UpdateChannelName(ctx, id, name string) error` to store.go
- **Domain**: Add `UpdateChannel(ctx, channelID, userNodeID, name string)` to service.go
- **REST**: Add `UpdateChannel` handler + route `api.PATCH("/channels/:chId", h.UpdateChannel)`
- **WS**: No WS event needed for MVP (UI updates locally)

### 2. Presence Events
- **Hub**: On client connect/disconnect, broadcast `PresenceEvent{userId, status}` to all users
- **Proto**: Add `PresenceEvent` to `ws.proto` ServerEnvelope
- **Frontend store**: Handle `PresenceEvent` → update online users map

## Frontend Changes

### Approach: Refactor Existing Components

No new component files. Polish and fix what exists:

1. **ChatList.tsx** (patterns) → Add empty states, improve sorting
2. **ChatListItem.tsx** (patterns) → Add online dot, improve preview
3. **MessageList.tsx** (chat) → Fix auto-scroll, add empty state
4. **ChatEditor.tsx** (chat) → Already good, minor polish
5. **ChannelInfoPanel.tsx** (chat) → Add online status to members
6. **channels.$channelId.tsx** (route) → Integrate presence, improve typing indicator
7. **websocket.store.ts** → Add presence tracking
8. **useMessaging.ts** → Add rename channel hook

### Data Flow (unchanged)
```
Send Message:
  User → ChatEditor → useSendMessage (optimistic) → REST API → Domain → Store
  Domain → Hub.BroadcastToChannel → Redis → All instances → WS → Client cache inject

Receive Message:
  WS binary → websocket.store.ts → protobuf decode → TanStack cache inject → React re-render
```

### Cache Strategy (unchanged)
- Messages: TanStack Query `['messages', channelId]` + WS cache injection
- Channels: TanStack Query `['channels', wsId]` + invalidation on create
- Unread: TanStack Query `['channels', 'unread']` + WS event injection
- Presence: Zustand store (websocket.store.ts) — no caching needed

## Risks & Mitigations
| Risk | Impact | Mitigation |
|------|--------|-----------|
| Presence floods WS | High traffic on large workspaces | Batch presence updates, throttle broadcasts |
| Auto-scroll conflicts | UX jank when scrolling up | Only auto-scroll when at bottom (already implemented) |
| Optimistic dedup | Double messages | Dedup by message ID in WS cache inject (already implemented) |
