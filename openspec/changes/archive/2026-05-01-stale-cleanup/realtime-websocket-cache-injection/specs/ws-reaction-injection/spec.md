# WS Reaction Cache Injection

## Proto Mapping
- WS Event: `ReactionEvent` — exists in ws.proto (field 8)
- Fields: `message_id`, `channel_id`, `user_id`, `username`, `emoji`, `action` ("add"|"remove")
- Frontend type: `ReactionGroup { emoji, count, user_ids[] }` in messages cache

## User Stories

### Story 1: Real-time reaction updates from other users
As a channel member viewing chat, I want to see reactions update in real-time when other users react, so that I see live engagement without page refresh.

**Acceptance Criteria:**
- [ ] When another user adds a reaction, the emoji count increments in the message bubble within 200ms
- [ ] When another user removes a reaction, the emoji count decrements or the reaction group disappears
- [ ] No HTTP GET `/messages/{id}/reactions` is triggered by the WS event
- [ ] Network tab shows zero refetch requests when reactions arrive via WS

### Story 2: Optimistic reaction toggle
As a user, I want my reaction to appear instantly when I click an emoji, so that the interaction feels immediate.

**Acceptance Criteria:**
- [ ] Clicking a reaction emoji immediately updates the count and adds my user_id to the group
- [ ] If the API call fails, the reaction is rolled back
- [ ] The WS event from my own action is deduplicated (no double-count)

**States:**
- Loading: Reaction appears immediately (optimistic)
- Error: Reaction rolls back, toast shows error
- Normal: Reaction persisted, confirmed by WS event dedup

## Flows

### Flow: Another user adds reaction
1. WS receives `reactionEvent` with `action: "add"`
2. Handler finds `['messages', channelId]` cache
3. Finds message by `message_id`
4. Adds/increments reaction group for `emoji`
5. UI updates — no HTTP call

### Flow: Sender toggles reaction (optimistic)
1. User clicks emoji on message
2. `onMutate`: update `reactions[]` in message cache (add user_id, increment count)
3. POST `/messages/{id}/reactions`
4. `onSuccess`: confirm (no action needed — WS event dedup handles it)
5. `onError`: rollback reactions to previous snapshot
