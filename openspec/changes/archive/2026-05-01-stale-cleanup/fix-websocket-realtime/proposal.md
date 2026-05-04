## Why

WebSocket real-time messaging is completely broken — three root causes found:

1. **WS proxy path mismatch** ✅ FIXED — Go handler was on `/ws`, Vite proxied `/api/ws` → 404.

2. **REST handler missing Hub broadcast** ✅ FIXED — REST `SendMessage` had no `*Hub`, never called `BroadcastToChannel()`.

3. **Frontend never subscribes to channel** 🔴 NEW — `channels.$channelId.tsx` uses `typingUsers` and `sendTyping` from the WebSocket store, but **never calls `sendSubscribe(channelId)`**. The Hub's `h.channels[channelID]` map is always empty, so `broadcastLocal()` delivers to zero clients.

## What Changes

### Phase 1 (Done)
- ✅ `cmd/main.go` — WS path `/ws` → `/api/ws`
- ✅ `rest/handler.go` — `Broadcaster` interface, Hub injection, broadcast in `SendMessage`

### Phase 2 (New)
- **MODIFY**: `routes/_workspace/channels.$channelId.tsx` — add `useEffect` that calls `sendSubscribe(channelId)` on mount and `sendUnsubscribe(channelId)` on unmount/channel change

## Impact

- **Frontend only** — 1 file, ~6 lines added
- `frontend/src/routes/_workspace/channels.$channelId.tsx` [MODIFY]
