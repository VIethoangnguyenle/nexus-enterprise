# Presence — TIER 1 Spec

## Proto Mapping

| Feature | RPC/Event | Status |
|---------|-----------|--------|
| Typing indicator | WS `TypingEvent` + `TypingRequest` | ✅ EXISTS |
| Online/offline | — | ❌ MISSING |

## Backend Gaps

### Online/Offline Status
- Hub already tracks `users map[string]map[*Client]bool` (hub.go line 29)
- Can derive online status from: user has ≥1 active WS client
- Need: WS event `PresenceEvent` for online/offline broadcasts
- Need: REST endpoint `GET /users/presence` or embed in member list response
- Redis: can use Redis SET for cross-instance presence tracking

> **BA Decision**: Simplify online/offline to WS-connection-based. User is "online" if they have an active WS connection. No last-seen timestamp for MVP. This leverages existing Hub.users map.

---

## User Stories

### US-9: Typing Indicator

As a **workspace member**,
I want to **see when someone is typing in the current channel**,
so that **I know a response is coming**.

Acceptance Criteria:
- [ ] When another user types, indicator appears below messages: "{name} is typing..."
- [ ] Indicator disappears after 3 seconds of no typing
- [ ] Multiple typers: "{name1} and {name2} are typing..."
- [ ] Own typing does NOT show to self
- [ ] Typing events sent at most once per 2 seconds (throttled)

Proto: WS `TypingRequest` (client) + `TypingEvent` (server) — existing
Type: Frontend polish (logic exists, improve UX)

### US-10: Online/Offline Indicator

As a **workspace member**,
I want to **see who is online in a channel**,
so that **I know who might respond quickly**.

Acceptance Criteria:
- [ ] Green dot on user avatar = online
- [ ] Gray dot or no dot = offline
- [ ] Status updates within 5 seconds of connect/disconnect
- [ ] In member list: online users appear first
- [ ] In DM chat header: show online/offline status of the other user
- [ ] No "last seen" for MVP — just online/offline binary

Proto: NEW — needs `PresenceEvent` in ws.proto
Type: Backend (WS event on connect/disconnect) + Frontend (display)

---

## Flows

### Flow: Typing Indicator
1. User starts typing in ChatEditor
2. System: throttled (2s) → sends WS `TypingRequest{channelId}`
3. Server: broadcasts `TypingEvent{channelId, username}` to channel (excluding sender)
4. Recipients: show "{username} is typing..." below messages
5. After 3s no new typing event → indicator fades out

### Flow: Online/Offline
1. User opens app → WS connects → auth success
2. Server: adds user to Hub.users → broadcasts `PresenceEvent{userId, status: "online"}`
3. Other users: update presence dot on avatars
4. User closes app → WS disconnects
5. Server: removes from Hub.users → broadcasts `PresenceEvent{userId, status: "offline"}`

---

## States

| Component | State | Display |
|-----------|-------|---------|
| Typing indicator | No one typing | Hidden |
| Typing indicator | 1 user typing | "{name} is typing..." with dots animation |
| Typing indicator | 2+ users typing | "{name1} and {name2} are typing..." |
| Presence dot | Online | Green dot (8px) |
| Presence dot | Offline | Gray dot or none |
