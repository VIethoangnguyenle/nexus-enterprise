## Context

The NGAC frontend currently uses a Slack-hybrid layout: a single expandable `Sidebar.tsx` (240px/64px) with inline module navigation and channel lists, rendering content into a full-width `<Outlet>`. An unused `AppRail.tsx` (48px icon-only) and `ListPanel.tsx` (280px context list) were built in a previous migration attempt but never wired into the layout.

After studying Lark's actual UI (dark mode, desktop), the correct architecture is:
- **LarkRail** (~90px, fixed): icon + short label, not 48px icon-only
- **ListPanel** (resizable 200–480px): contextual list per module (ChatList for messaging, folder tree for drive)
- **ContentPanel** (flex-1): active content (chat, file table, etc.)
- **Drive** opens in a **new browser tab** — completely separate route group and layout

Current files affected: `_workspace.tsx`, `Sidebar.tsx`, `AppRail.tsx`, `ListPanel.tsx`, `channels.$channelId.tsx`, `drive.tsx`, `ui.store.ts`.

## Goals / Non-Goals

**Goals:**
- Replace the Sidebar + AppRail with a single LarkRail component matching Lark's ~90px labeled rail
- Build a dense ChatList component with avatar, name, message preview, timestamp, unread badge
- Make ListPanel resizable via drag (200–480px range, persisted to localStorage)
- Separate Drive into its own route group (`/_drive`) that opens in a new browser tab
- Build the Drive page with its own 2-column layout (folder tree sidebar + file table)
- Restyle chat header (pill tabs), message rows (tighter density, subtle bubbles), and input bar (minimal, right-aligned tools)

**Non-Goals:**
- No backend API changes — this is purely frontend layout/UX
- No DM/direct message support — chat list shows channels only (for now)
- No real-time unread count from backend — use local WebSocket events
- No mobile-first redesign — desktop layout first, mobile responsive later
- No Documents module rework — Documents stays as-is (navigable via rail)

## Decisions

### Decision 1: LarkRail width — 90px with icon + label

**Choice**: Fixed 90px rail with icon (20px) + short label text below/beside it.

**Alternatives considered:**
- 48px icon-only (current `AppRail.tsx`): Too narrow, Lark's actual UI shows text labels
- 240px full sidebar (current `Sidebar.tsx`): Too wide, wastes space, Slack pattern
- 60px icon-only with tooltip: Lark screenshot clearly shows labels visible without hover

**Rationale**: The Lark screenshot shows ~90px with labels like "Messenger", "Meetings", "Calendar", "Docs" always visible. This is the correct density.

### Decision 2: "Docs" in rail opens new browser tab

**Choice**: `window.open('/drive', '_blank')` instead of SPA route navigation.

**Alternatives considered:**
- SPA route swap (same tab): Doesn't match Lark behavior; Lark opens Docs at a separate URL
- iframe embed: Fragile, breaks browser navigation

**Rationale**: Lark literally opens a new tab at `larksuite.com/drive/home/`. The Drive page has its own layout (no LarkRail), its own sidebar, its own URL. This separation reinforces the mental model: Chat = communication, Drive = knowledge.

### Decision 3: Resizable ListPanel via custom `useResizable` hook

**Choice**: Pure React `mousedown/mousemove/mouseup` tracking, stored in Zustand + localStorage.

**Alternatives considered:**
- CSS `resize` property: Limited styling control, can't persist or constrain precisely
- Library (react-resizable-panels): Adds dependency; our use case is simple enough for ~40 lines of custom code
- Fixed width: Doesn't match Lark — users can drag to resize the chat list

**Rationale**: A custom hook gives exact control over min/max constraints, cursor styling, and persistence with zero dependencies.

### Decision 4: Drive route group `/_drive` as separate layout

**Choice**: New layout route `/_drive.tsx` with its own sidebar (folder tree) and content area. No LarkRail on this page.

**Alternatives considered:**
- Keep Drive inside `/_workspace` with module switching: Doesn't match Lark's new-tab behavior
- Separate SPA entry point: Over-engineered; TanStack Router supports multiple layout routes natively

**Rationale**: The Lark Drive page (screenshot) shows zero LarkRail — it's a standalone "Lark Docs" page with its own "Home / Drive / Wiki" sidebar. A separate layout route achieves this cleanly.

### Decision 5: Component file structure

**Choice**: New components in `components/patterns/`:
- `LarkRail.tsx` — replaces both Sidebar.tsx and AppRail.tsx
- `ChatList.tsx` — dense conversation list
- `ChatListItem.tsx` — single conversation row
- `ResizablePanel.tsx` — generic resizable wrapper

Drive-specific in `components/drive/`:
- `DriveLayout.tsx` — 2-column layout for Drive page
- `DriveSidebar.tsx` — folder tree sidebar (Home, Drive, Wiki)

**Rationale**: Follows existing project conventions. `patterns/` for app-level layout components, `drive/` for Drive-specific components.

## Risks / Trade-offs

- **[Breaking change]** → Removing Sidebar.tsx breaks the entire current layout. Mitigation: implement LarkRail + ListPanel + ContentPanel as a single atomic commit to `_workspace.tsx`.

- **[Chat list data gap]** → Backend doesn't return "last message preview" or "unread count" per channel in the list API. Mitigation: use WebSocket store for unread tracking; for message preview, add a lightweight field to the channels list API response (or derive from cached messages client-side). Phase 1 can show channel name only if backend isn't ready.

- **[Drive in new tab loses auth context]** → Opening `/drive` in a new tab requires the JWT to be accessible. Mitigation: JWT is already in localStorage (via auth.store.ts with Zustand persist), so new tabs automatically have the token.

- **[Resize performance]** → Dragging the panel border triggers frequent re-renders. Mitigation: use `requestAnimationFrame` throttling in the `useResizable` hook; set `will-change: width` on the panel for GPU acceleration.

- **[Existing specs conflict]** → Three specs exist (`lark-sidebar-layout`, `lark-messaging-layout`, `lark-data-table-layout`) that partially conflict with this redesign. Mitigation: this change modifies those specs with updated requirements; after archiving, the old requirements are replaced.
