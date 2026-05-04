## Context

The NGAC platform's Drive module currently renders a flat file table without permission awareness. All actions (download, trash, share) are visible to every authenticated user. Backend authorization exists (`CheckAccess` gRPC on PolicyReadService), but the frontend never queries it — creating a disconnect between what users see and what they can actually do.

The workspace layout already implements a 3-column pattern (Rail + ListPanel + Content) used by Messaging. Drive currently bypasses ListPanel entirely, rendering a single-pane table inside the Content area. The WebSocket infrastructure is mature (protobuf-binary, reconnect, per-channel subscriptions) but only handles messaging events.

Key constraints:
- NGAC permission graph is dynamic — permissions change via share/revoke at any time
- Workspace membership implies read access to workspace-owned items (no per-item read check needed)
- PolicyReadService currently supports only single `CheckAccess(user, object, operation)` — no batch variant
- Frontend stack: Vite + TanStack Router + TanStack Query + Zustand

## Goals / Non-Goals

**Goals:**
- Frontend renders actions strictly based on resolved NGAC permissions (no `if (role === 'admin')`)
- Batch permission resolution in a single network call for visible items
- 3-panel Drive layout matching the Messaging module's Lark-like density
- Real-time permission and object change propagation via existing WebSocket
- Permission cache with TTL + event-driven invalidation
- Phase 1 deliverable: working 3-panel with batch permissions and read-only context panel

**Non-Goals:**
- Offline support or service worker caching
- File versioning or conflict resolution
- Cross-tenant file sharing
- Full-text file content search
- Mobile-specific layout optimization

## Decisions

### D1: Hybrid Permission Resolution (Option C)

**Decision:** Server filters out unreadable items. Client batch-checks write/delete/share for visible items.

**Alternatives considered:**
- **(A) Server-enriched** — embed permissions in list response. Rejected: payload bloat (100 items × 4 ops), no real-time reactivity without refetch.
- **(B) Client-only batch** — client fetches all items then batch-checks. Rejected: metadata leak (client sees item IDs it can't read), double round-trip on initial load.

**Rationale:** Hybrid gives us security (server never sends unreadable items) + reactivity (client cache invalidates on WebSocket events without refetching the item list).

### D2: Drive Service Owns Batch Access Endpoint

**Decision:** `POST /api/drive/batch-access` on Drive REST, which internally calls `PolicyRead.BatchCheckAccess` gRPC.

**Alternatives considered:**
- Auth service proxy — auth doesn't know about drive objects, can't validate tenant scoping.
- Policy service REST — Policy has no REST API, adding one breaks the architecture rule (services expose their own REST).

**Rationale:** Drive already has JWT middleware + tenant context. Drive validates that requested object IDs belong to the current workspace before forwarding to Policy.

### D3: Reuse Messaging WebSocket

**Decision:** Extend the existing `ServerEnvelope` protobuf oneof with drive event types. Single WebSocket connection serves both messaging and drive.

**Alternatives considered:**
- Separate SSE/EventSource for drive — doubles connection overhead, splits sync logic.
- Separate WebSocket — same overhead issues.

**Rationale:** One connection = one auth handshake, one reconnect handler, unified event dispatch.

### D4: @tanstack/react-virtual for List Virtualization

**Decision:** Use `@tanstack/react-virtual` (TanStack ecosystem alignment).

**Alternatives considered:**
- `react-window` — works, but different API style. No ecosystem synergy.
- Native CSS `content-visibility` — not enough control for dynamic row heights.

### D5: Permission Cache Architecture

**Decision:** In-memory Map in a Zustand store, keyed by `${tenantId}:${objectId}`, with 60s TTL. Invalidated by WebSocket `DrivePermEvent` or tenant switch.

```
permissionCache: Map<string, {
  perms: { read: boolean, write: boolean, delete: boolean, share: boolean },
  expiresAt: number
}>
```

**Why not TanStack Query for permissions?** Permissions are cross-cutting (used by tree, list, context panel simultaneously). A dedicated cache avoids query key explosion and gives us synchronous reads after first batch.

## Risks / Trade-offs

**[Risk] BatchCheckAccess performance at scale** — 100 items × 4 operations = 400 graph traversals.
→ Mitigation: Policy service can optimize batch internally (shared ancestor caching, parallel CTE). Phase 1 can loop single CheckAccess; optimize later.

**[Risk] Permission cache staleness** — 60s TTL means up to 60s of stale UI.
→ Mitigation: WebSocket events provide near-instant invalidation for active changes. TTL is fallback for missed events.

**[Risk] WebSocket reconnect during permission change** — events missed during disconnect.
→ Mitigation: `resyncAfterReconnect()` already invalidates queries. Extend to invalidate permission cache + refetch visible folder.

**[Trade-off] No server-side read filtering in Phase 1** — workspace members see all items (read implied by membership).
→ Acceptable: True per-item read restrictions only matter for shared-with-me and cross-workspace scenarios (Phase 2+).
