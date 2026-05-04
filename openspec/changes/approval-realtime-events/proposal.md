# Approval Real-Time Events

## Evidence Summary
- Backend approval service: **EXISTS** — `backend/services/approval/` (REST + gRPC + domain + store + events consumer)
- Kafka infrastructure: **EXISTS** — approval already has `ReconciliationConsumer` on `ngac.graph.mutated` topic, but NO producer
- Messaging service WebSocket: **EXISTS** — `Hub` with `SendNotification()`, `sendToUser()`, Redis pub/sub cross-instance
- Messaging event consumer: **EXISTS** — consumes `asset.lifecycle`, `asset.request`, `asset.assignment` topics → notifications → WS
- Proto `ServerEnvelope`: **EXISTS** — 14 payload types (no approval type yet)
- Frontend `websocket.store.ts`: **EXISTS** — handles all `ServerEnvelope` payload types via `handleServerMessage` switch
- Frontend approval hooks: **EXISTS** — uses `invalidateQueries` on mutation success only (no WS subscription)

## Product Assessment
- Size: **M** — Backend has Kafka client, proto+WS patterns are well-established, need to add producer + topic + proto type + consumer handler + frontend handler
- Risk: **Low** — exact pattern exists for `asset.lifecycle` → notifications → WebSocket. Copy and adapt.
- Target user: **Approvers and requesters** — multi-user workflow participants
- Core action: **See approval status changes instantly** without manual refresh

## Scope
### In scope
1. **Approval event producer** — approval service publishes to `approval.events` topic on create/approve/reject
2. **Proto message** — add `ApprovalEvent` message to `ws.proto` + `ServerEnvelope` field 16
3. **Messaging consumer** — subscribe `approval.events`, forward to WS via `sendToUser()` for relevant users
4. **Frontend handler** — handle `approvalEvent` in `websocket.store.ts`, invalidate approval query caches
5. **Notification creation** — create notifications for approval actions (reuse existing `CreateNotification`)

### Out of scope
- Granular cache injection (direct TanStack Query data mutation) — invalidation is sufficient for MVP
- Approval-specific WS subscription/unsubscription — all approval users receive events via user-level WS delivery
- Custom toast/popup UI for approval events — existing notification system handles this

### Deferred
- Fine-grained event filtering (only send events to stakeholders of a specific request)
- Approval event history/audit via event store

## Success Criteria
- 2 browsers open on `/approval` → user A approves → user B sees status change within 2 seconds without manual refresh
- New approval request created → pending count updates for all relevant approvers
- Reject action → requester sees rejection status without reload
