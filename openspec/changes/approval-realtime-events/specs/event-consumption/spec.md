# Event Consumption — Messaging Service

## User Stories

### S5: Messaging service consumes approval events and creates notifications
As a **workspace user**,
I want to receive notifications when approval requests are created, approved, or rejected,
so that I stay informed without checking the approval page.

**Acceptance Criteria:**
- [ ] Messaging consumer subscribes to `approval.events` Kafka topic
- [ ] On `created` event: notification sent to each `assignee_node_ids` user — "New approval request: {template_name}"
- [ ] On `approved` event: notification sent to `created_by` user — "Your request has been approved: {template_name}"
- [ ] On `rejected` event: notification sent to `created_by` user — "Your request was rejected: {template_name}"
- [ ] Notifications stored in `notifications` table (existing infra)
- [ ] Notifications pushed via WebSocket to online users (existing `SendNotification`)

Proto mapping: existing `NotificationEvent` in `ServerEnvelope` — no proto changes needed for notifications
Backend work: Add `approval.events` topic to messaging consumer + handler

---

### S6: Messaging service broadcasts approval state change via WebSocket
As a **user viewing the approval page**,
I want approval status changes to appear in real-time on my screen,
so that I don't need to manually refresh.

**Acceptance Criteria:**
- [ ] New `ApprovalEvent` proto message added to `ServerEnvelope` (field 16)
- [ ] Hub broadcasts `ApprovalEvent` to relevant users via `sendToUser()`
- [ ] Target users: `created_by` (requester) + all `assignee_node_ids` (approvers)
- [ ] Event contains: `request_id`, `status`, `action`, `actor_node_id`, `template_name`
- [ ] Uses Redis pub/sub for cross-instance delivery (existing pattern)

Proto mapping: NEW `ApprovalEvent` in `ws.proto` — field 16 of `ServerEnvelope`

---

## Proto Change Required

```protobuf
// In ws.proto ServerEnvelope — add field 16:
ApprovalEvent approval_event = 16;

// New message:
message ApprovalEvent {
  string request_id     = 1;
  string status         = 2;  // "pending", "approved", "rejected"
  string action         = 3;  // "created", "approved", "rejected", "step_advanced"
  string actor_node_id  = 4;
  string template_name  = 5;
}
```

## Flow

```
Flow: Event Consumption
1. Kafka delivers record from "approval.events" to messaging consumer
2. Consumer deserializes JSON event
3. Consumer calls notifSv.CreateNotification() for relevant users
4. CreateNotification stores in DB + pushes via hub.SendNotification()
5. Consumer ALSO calls hub.BroadcastApprovalEvent() to all relevant users
6. Hub wraps in ServerEnvelope{approval_event} + sends via sendToUser()
7. Frontend receives binary protobuf via WebSocket
```

## States

- **Normal**: Consumer running, events processed in order
- **Kafka unavailable**: Consumer logs warning, retries with backoff (existing pattern)
- **User offline**: Notification stored in DB, WS delivery skipped (existing behavior)
- **User reconnects**: `resyncAfterReconnect()` invalidates approval queries (needs addition)
