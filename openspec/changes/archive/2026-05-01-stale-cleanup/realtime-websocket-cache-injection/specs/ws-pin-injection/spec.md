# WS Pin Cache Injection

## Proto Mapping
- WS Event: `PinEvent` — exists in ws.proto (field 9)
- Fields: `channel_id`, `message_id`, `user_id`, `action` ("pin"|"unpin")

## User Stories

### Story 1: Real-time pin updates
As a channel member, I want to see pins update in real-time, so pinned messages appear/disappear without refresh.

**Acceptance Criteria:**
- [ ] When another user pins a message, `is_pinned` flag updates in messages cache
- [ ] Pins list cache (`['pins', channelId]`) is updated (add/remove entry)
- [ ] No HTTP GET refetch triggered by WS pin event

### Story 2: Optimistic pin toggle
As a user, I want pin/unpin to reflect immediately.

**Acceptance Criteria:**
- [ ] Toggling pin updates `is_pinned` on the message instantly
- [ ] If API fails, `is_pinned` rolls back
- [ ] WS event is deduplicated

**States:**
- Normal: Pin icon visible on message
- Error: Rollback pin state, show error toast

## Flows

### Flow: WS pin event
1. Receive `pinEvent` with `action: "pin"`
2. Update `['messages', channelId]` → set `is_pinned = true` on target message
3. Add entry to `['pins', channelId]` cache (if cache exists)
4. No HTTP call
