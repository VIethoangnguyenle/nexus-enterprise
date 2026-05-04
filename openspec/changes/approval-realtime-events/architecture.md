# Architecture: Approval Real-Time Events

## Service Boundaries

```
┌──────────────────────┐     Kafka: "approval.events"      ┌──────────────────────┐
│   Approval Service   │ ─────────────────────────────────► │  Messaging Service   │
│  (REST + gRPC)       │     fire-and-forget JSON           │  (WS Hub + Consumer) │
│                      │                                    │                      │
│  cmd/main.go         │                                    │  cmd/main.go         │
│  internal/           │                                    │  internal/           │
│    rest/handler.go   │                                    │    events/consumer.go│
│    events/producer.go│◄─── NEW                            │    grpc/hub.go       │
│    domain/           │                                    │                      │
│    store/            │                                    │                      │
└──────────────────────┘                                    └──────────┬───────────┘
                                                                       │
                                                            WebSocket  │ sendToUser()
                                                            (protobuf) │
                                                                       ▼
                                                            ┌──────────────────────┐
                                                            │     Frontend         │
                                                            │  websocket.store.ts  │
                                                            │  → invalidateQueries │
                                                            │    ['approval', *]   │
                                                            └──────────────────────┘
```

## Data Flow

```
1. User Action (approve/reject/create)
   │
   ▼
2. REST Handler → Domain Service → Store (DB commit)
   │
   ▼
3. REST Handler calls producer.PublishApprovalEvent()
   │                                                    ← AFTER DB commit
   ▼
4. Kafka Producer → "approval.events" topic (async, fire-and-forget)
   │
   ▼
5. Messaging Consumer receives record
   │
   ├─► notifSv.CreateNotification()  → DB insert + hub.SendNotification()
   │                                    └─► ServerEnvelope{notification} → WS → frontend
   │
   └─► hub.BroadcastApprovalEvent()  → ServerEnvelope{approval_event} → WS → frontend
                                        └─► invalidateQueries(['approval'])
```

## Integration Points

### Approval Service → Kafka (NEW: producer.go)
- **Library**: `github.com/twmb/franz-go/pkg/kgo` (already in go.mod for consumer)
- **Topic**: `approval.events`
- **Pattern**: Same as messaging service's `events/producer.go` — async `Produce()` with error logging
- **Injection**: Producer passed to REST handler via constructor (dependency injection)

### Messaging Service → Kafka (MODIFY: consumer.go)
- **Addition**: Subscribe to `approval.events` topic (add to `ConsumeTopics()`)
- **Handler**: New `handleApprovalEvent()` function
- **Pattern**: Same as existing `handleRequestEvent()` — deserialize JSON, create notification, broadcast WS

### Messaging Service → WebSocket (MODIFY: hub.go)
- **Addition**: New `BroadcastApprovalEvent()` method
- **Pattern**: Same as `BroadcastAssetUpdated()` — wraps in `ServerEnvelope`, sends to target users via `sendToUser()`
- **Difference**: `BroadcastAssetUpdated` broadcasts to ALL users; `BroadcastApprovalEvent` targets specific users (requester + assignees)

### Proto (MODIFY: ws.proto)
- **Addition**: `ApprovalEvent` message + field 16 in `ServerEnvelope`
- **Proto regen**: `make proto` to regenerate Go + TS types

### Frontend (MODIFY: websocket.store.ts)
- **Addition**: `case 'approvalEvent'` handler
- **Action**: `queryClient.invalidateQueries({ queryKey: ['approval'] })`
- **Reconnect**: Add `['approval']` to `resyncAfterReconnect()`

## NGAC Permission Model

No NGAC changes. Event publishing is a side-effect of already-authorized actions:
- `Approve` RPC already validates user's assignment before acting
- `Reject` RPC already validates user's assignment before acting
- `CreateRequest` already validates user identity
- Events carry `user_node_id` for audit, but no additional permission checks needed

## Files Changed

| Service | File | Change Type |
|---------|------|-------------|
| approval | `internal/events/producer.go` | NEW |
| approval | `cmd/main.go` | MODIFY — init producer, pass to handler |
| approval | `internal/rest/handler.go` | MODIFY — call producer after mutations |
| messaging | `internal/events/consumer.go` | MODIFY — add topic + handler |
| messaging | `internal/grpc/hub.go` | MODIFY — add `BroadcastApprovalEvent()` |
| messaging | `cmd/main.go` | MODIFY — pass hub to consumer (if not already) |
| proto | `proto/messaging/ws.proto` | MODIFY — add ApprovalEvent |
| frontend | `stores/websocket.store.ts` | MODIFY — add event handler |
| frontend | `generated/proto/messaging/ws.ts` | AUTO — regenerated |

## Edge Cases

1. **Kafka unavailable**: Producer logs warning, approval operations succeed. No WS event sent. Users rely on manual refresh (existing behavior).
2. **User offline when event fires**: Notification stored in DB. When user opens approval page, TanStack Query fetches fresh data.
3. **Duplicate events**: `invalidateQueries` is idempotent — multiple events just trigger one refetch.
4. **Consumer crash/restart**: Kafka consumer group offset tracking ensures events are reprocessed on restart.
5. **Race: user approves while viewing stale pending list**: WS event triggers refetch, which may return 404 for already-approved items. Optimistic removal in `useApprove` handles this gracefully.
