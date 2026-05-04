# Platform-Wide WebSocket Cache Injection — Overview

## Module Summary
Convert all WebSocket event handlers from `invalidateQueries` (full HTTP refetch) to `setQueryData` (direct cache injection). Convert all mutation hooks from refetch-on-success to optimistic update + WS event confirmation.

## Proto Mapping

| WS Event (ws.proto) | Frontend Handler | Status |
|---------------------|-----------------|--------|
| `ChatMessage` | `case 'chatMessage'` | ✅ Done (previous change) |
| `ThreadReplyEvent` | `case 'threadReply'` | ✅ Done (previous change) |
| `ReactionEvent` | `case 'reactionEvent'` (default fallback) | 🔴 Needs handler |
| `PinEvent` | `case 'pinEvent'` (default fallback) | 🔴 Needs handler |
| `PollVoteEvent` | `case 'pollVote'` (default fallback) | 🔴 Needs handler |
| `TaskUpdateEvent` | `case 'taskUpdate'` (default fallback) | 🔴 Needs handler |
| `DriveObjectEvent` | `case 'driveObject'` | 🟡 Has handler but uses invalidation |
| `AssetUpdatedEvent` | `case 'assetUpdated'` | 🟡 Keep invalidation (partial payload) |
| `DrivePermEvent` | `case 'drivePerm'` | ✅ Keep (needs permission re-eval) |
| `UnreadCountEvent` | `case 'unreadCount'` | 🟡 Keep invalidation but remove polling |

## Dependency List
- **No backend changes** — all WS events already broadcast
- **No proto changes** — all payloads already complete
- **Files affected**: `websocket.store.ts`, `useMessaging.ts`, `useDrive.ts` (3 files)

## Architecture: Event Flow

```
Mutation (user action)
  ├── 1. Optimistic update → setQueryData (instant UI)
  ├── 2. HTTP POST/PATCH → server
  │     ├── 3a. onSuccess → replace optimistic data with server response
  │     └── 3b. onError → rollback to previous state
  └── 4. WS event arrives → setQueryData with dedup (skip if already in cache)

WS event (from another user)
  └── setQueryData → inject into cache → UI updates instantly
```
