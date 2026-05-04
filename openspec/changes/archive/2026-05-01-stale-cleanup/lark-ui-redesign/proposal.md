## Why

The current UI uses a Slack/Discord mental model — a single expandable sidebar with channel navigation, full-width chat, and embedded Drive. The real Lark interface uses a fundamentally different paradigm: **Global Rail → Contextual List → Content Panel**. This redesign replaces the entire navigation and layout architecture to match Lark's workspace model, where Chat is a real-time communication space and Drive is a separate structured knowledge space opened in a new browser tab.

## What Changes

- **BREAKING**: Remove the current `Sidebar.tsx` component (240px/64px expandable sidebar with inline channel list, module nav, and user section)
- **BREAKING**: Remove `AppRail.tsx` (48px icon-only rail — too narrow, no labels)
- **NEW**: `LarkRail` component (~90px fixed, icon + short label, search shortcut, colored active state, notification badges)
- **NEW**: `ChatList` component — dense conversation list with avatar, name, last message preview, timestamp, unread badge (replaces bare `#channel-name` links)
- **NEW**: Resizable `ListPanel` — drag the right border to resize between 200–480px, persisted to localStorage
- **NEW**: `/_drive` route group — standalone Drive/Docs page that opens in a **new browser tab** when clicking "Docs" in the LarkRail
- **NEW**: Drive page with its own 2-column layout: folder tree sidebar + file table with action bar (New, Upload, Templates), tab filters, sort/display controls
- Restyle chat header with pill-style tabs (Chat, Pinned, +) instead of icon buttons
- Restyle message rows with subtle flat bubble backgrounds and tighter density
- Restyle input bar to match Lark's minimal style (right-aligned tools: Aa, emoji, attach, send)

## Capabilities

### New Capabilities
- `lark-rail-navigation`: Fixed-width (~90px) global navigation rail with icon + label, search shortcut (Ctrl+K), module switching, notification badges, and avatar. Clicking "Docs" opens Drive in a new browser tab.
- `chat-list-panel`: Dense conversation list (ChatList) rendered inside a resizable ListPanel (200–480px, drag-to-resize, localStorage persistence). Each item shows avatar, name, last message preview, timestamp, and unread badge.
- `drive-standalone-page`: Standalone Drive/Docs module at `/_drive` route group, opened in new browser tab. Own layout with folder tree sidebar (My Folders, Shared Folders) and file table (Name, Modified, Created columns), action bar (New, Upload, Templates), tab filters, sort/display controls.

### Modified Capabilities
- `lark-sidebar-layout`: **REPLACED** — the hierarchical 240px sidebar is removed entirely in favor of the LarkRail + ListPanel architecture.
- `lark-messaging-layout`: Chat header redesigned with pill tabs; message row density tightened further; input bar restyled.
- `lark-data-table-layout`: Drive table now lives in its own standalone page, not embedded within workspace layout.

## Impact

- **Routes**: New `/_drive` layout route group; `/_workspace` layout restructured to use LarkRail + ListPanel + ContentPanel
- **Components**: `Sidebar.tsx` and `AppRail.tsx` deleted; new `LarkRail.tsx`, `ChatList.tsx`, `ChatListItem.tsx`, `ResizablePanel.tsx` created; `ListPanel.tsx` refactored for resize support
- **Stores**: `ui.store.ts` updated — add `listPanelWidth`, remove `sidebarExpanded`/`sidebarCollapsed`
- **CSS**: New resize handle styles, pill tab styles, updated message density tokens
- **No backend changes** — this is a pure frontend layout/UX refactor
