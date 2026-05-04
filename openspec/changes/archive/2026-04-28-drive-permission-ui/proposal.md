## Why

The Drive UI currently renders all files and folders with all actions (download, trash, share) visible to every user regardless of NGAC permissions. There is no frontend permission awareness — actions that the backend will reject are shown, and objects the user cannot read are displayed. This violates the core principle that the UI is a live projection of the NGAC permission graph. Additionally, the Drive lacks a folder tree sidebar, a context/detail panel, and real-time synchronization — all essential for a Lark-grade enterprise file management experience.

## What Changes

- **Add `BatchCheckAccess` RPC** to the Policy Read Service proto, enabling batch permission resolution for lists of objects
- **Add `POST /api/drive/batch-access` REST endpoint** on the Drive service, proxying batch permission checks with tenant scoping
- **Add drive events to WebSocket proto** (`DriveObjectEvent`, `DrivePermEvent`) on the existing messaging WebSocket connection
- **Build frontend permission engine** — `usePermissions` hook with TTL-based in-memory cache, batch fetching, and WebSocket-driven invalidation
- **Refactor Drive UI to 3-panel layout** — folder tree sidebar, virtualized main list, slide-in context panel
- **Permission-aware rendering** — hide inaccessible objects, conditionally show actions based on resolved permissions
- **Share dialog with NGAC sync** — share UI triggers backend NGAC graph mutation, emits `permission_changed` event, invalidates cache, re-renders

## Capabilities

### New Capabilities

- `batch-access-check`: Backend RPC and REST endpoint for batch permission resolution across multiple objects and operations in a single call
- `drive-permission-engine`: Frontend permission cache, batch hook, and WebSocket invalidation layer that drives all UI rendering decisions
- `drive-tree-navigation`: Lazy-loaded folder tree sidebar with expand/collapse persistence and permission-aware visibility
- `drive-context-panel`: Right-side slide-in panel with preview, metadata, permissions, and activity tabs
- `drive-realtime-sync`: WebSocket events for drive object mutations and permission changes, with targeted cache invalidation

### Modified Capabilities

_(none — existing specs are auth-domain, this change is drive-domain)_

## Impact

**Backend (proto + services):**
- `proto/policy/policy_read.proto` — new `BatchCheckAccess` RPC + messages
- `services/policy/` — implement batch evaluator (loop over CTE or optimized batch query)
- `services/drive/internal/rest/` — new batch-access endpoint
- `proto/messaging/ws.proto` — new `DriveObjectEvent` and `DrivePermEvent` envelope types
- `services/messaging/` — emit drive events on WebSocket

**Frontend:**
- `hooks/usePermissions.ts` — new permission cache + batch hook
- `stores/drive.store.ts` — new Zustand store for tree/selection/context panel state
- `routes/_workspace/drive.tsx` — full rewrite to 3-panel layout
- `stores/websocket.store.ts` — handle drive events
- `components/drive/` — new component directory (TreePanel, FileList, ContextPanel, ShareDialog)

**Dependencies:**
- `@tanstack/react-virtual` — virtualized file list rendering
