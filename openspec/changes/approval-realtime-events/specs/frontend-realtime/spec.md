# Frontend Real-Time — WebSocket → Query Cache

## User Stories

### S7: Frontend handles approval WebSocket events
As a **user viewing the approval page**,
I want approval list data to update automatically when any user creates/approves/rejects a request,
so that I always see the current state without refreshing.

**Acceptance Criteria:**
- [ ] `websocket.store.ts` handles `approvalEvent` payload type in `handleServerMessage`
- [ ] On any approval event: `invalidateQueries({ queryKey: ['approval'] })` — refreshes all approval tabs
- [ ] Pending tab count updates within 2 seconds of another user's action
- [ ] History tab shows newly approved/rejected items without manual tab switch
- [ ] My Requests tab reflects status changes made by approvers
- [ ] No duplicate refetches (debounced invalidation or single broad invalidation)

Proto mapping: frontend generated types from `ws.proto` changes (S6)

---

### S8: Frontend re-syncs approval data on WebSocket reconnect
As a **user who temporarily lost connection**,
I want approval data to refresh when my WebSocket reconnects,
so that I don't see stale data after a network interruption.

**Acceptance Criteria:**
- [ ] `resyncAfterReconnect()` includes `queryClient.invalidateQueries({ queryKey: ['approval'] })`
- [ ] After reconnect, all visible approval tabs show fresh data
- [ ] No loading flicker if data hasn't changed (TanStack Query structural sharing)

Proto mapping: no proto changes — frontend-only

---

## Flow

```
Flow: Frontend Real-Time Update
1. WebSocket receives binary ServerEnvelope
2. ServerEnvelope.fromBinary() decodes to { payload: { oneofKind: 'approvalEvent', approvalEvent: {...} } }
3. handleServerMessage switch case 'approvalEvent'
4. queryClient.invalidateQueries({ queryKey: ['approval'] })
5. Active approval tab auto-refetches
6. User sees updated data
```

## States

- **Connected**: Events processed in real-time, queries invalidated immediately
- **Disconnected**: Mutations still work (optimistic updates), WS events missed
- **Reconnected**: `resyncAfterReconnect()` catches up all missed events via full query invalidation
- **No approval tab open**: Events received but invalidation is no-op (no active query observers)
