# Platform-Wide WebSocket Cache Injection

## Evidence Summary

### Backend WebSocket Infrastructure — ✅ COMPLETE
- **Hub**: `grpc/hub.go` — Redis Pub/Sub, binary protobuf, per-channel broadcast
- **Proto**: `ws.proto` — 15 event types covering all domains:
  - `ChatMessage` ✅ (already cache-injected)
  - `ThreadReplyEvent` ✅ (already cache-injected)
  - `ReactionEvent` — has full payload (message_id, channel_id, user_id, emoji, action)
  - `PinEvent` — has full payload (channel_id, message_id, action)
  - `PollVoteEvent` — has payload (poll_id, option_id, vote_count, total_votes)
  - `TaskUpdateEvent` — has payload (task_id, channel_id, status, assignee_id, title)
  - `AssetUpdatedEvent` — has payload (asset_id, new_state)
  - `DriveObjectEvent` — has payload (event_type, item_id, parent_id, workspace_id)
  - `DrivePermEvent` — has payload (item_id, workspace_id)

### Frontend — ❌ PERVASIVE INVALIDATION PATTERN
- **Total `invalidateQueries` calls**: 65+ across all hooks and WebSocket store
- **High-traffic refetches**: reactions, pins, tasks, polls — all trigger full GET after mutation
- **WS events with full data payloads being IGNORED**: reaction, pin, poll_vote, task_update
- **Default handler** (lines 290-304): catches ALL unhandled events and fires 5 broad invalidations

### Modules Affected

| Module | Current Pattern | WS Event Available | Can Inject? |
|--------|----------------|-------------------|-------------|
| Chat messages | ✅ setQueryData | chatMessage | Done |
| Thread replies | ✅ setQueryData | threadReply | Done |
| **Reactions** | ❌ invalidateQueries | reactionEvent ✅ | **YES** |
| **Pins** | ❌ invalidateQueries | pinEvent ✅ | **YES** |
| **Poll votes** | ❌ invalidateQueries | pollVoteEvent ✅ | **YES** |
| **Task updates** | ❌ invalidateQueries | taskUpdateEvent ✅ | **YES** |
| **Assets** | ❌ invalidateQueries | assetUpdated (partial) | **Partial** |
| **Drive folders** | ❌ invalidateQueries | driveObject ✅ | **YES** |
| **Drive perms** | ❌ invalidateQueries | drivePerm ✅ | **Keep invalidation** |
| Unread counts | ✅ invalidateQueries | unreadCount | Keep (aggregation) |
| Notifications | ✅ invalidateQueries | notification | Keep (lightweight) |

## Product Assessment

- **Size**: M (Medium) — Frontend-only, ~6 files, no backend changes, no proto changes
- **Risk**: Medium — Cache consistency patterns need careful dedup/ordering
- **Target user**: ALL users — every user on the platform benefits from reduced latency
- **Core action**: All interactive features update in real-time without redundant HTTP refetches

## Scope

### In scope
1. **Messaging module** — Optimistic mutations for: reactions (toggle), pins (toggle), tasks (create/update), polls (vote)
2. **WebSocket store** — Add dedicated handlers for: `reactionEvent`, `pinEvent`, `pollVoteEvent`, `taskUpdateEvent` with cache injection
3. **Drive module** — `driveObject` handler: inject new/updated items into folder cache instead of broad invalidation
4. **Default handler cleanup** — Remove the broad 5-query invalidation from the `default` case; add specific event type logging
5. **Mutation hooks** — Convert mutation `onSuccess` from `invalidateQueries` → `setQueryData` where WS event provides confirmation
6. **Unread count polling** — Remove `refetchInterval: 30000` polling since WS `unreadCount` event exists

### Out of scope
- Backend changes (all WS events already broadcast correctly)
- Proto changes (all payloads already complete)
- Asset module deep injection (AssetUpdatedEvent only has asset_id + new_state, not full asset object — keep invalidation)
- Drive permission events (need server-side permission re-evaluation — keep invalidation)
- Notification events (lightweight, low-frequency — keep invalidation)

### Deferred
- Backend cursor-based pagination for messages (would further reduce payload size)
- WebSocket heartbeat/ping mechanism for connection health

## Success Criteria
1. **Zero unnecessary GET calls** — Network tab shows no refetch after mutation when WS event arrives
2. **Sub-100ms update** — UI reflects changes within 100ms of WS event receipt
3. **Deduplication works** — Sender sees exactly 1 reaction/pin/vote update, not duplicates
4. **Build passes** — `vite build` clean
5. **Polling eliminated** — `refetchInterval` removed from unread counts
