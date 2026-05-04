## 1. Foundation — Stores & Hooks

- [x] 1.1 Update `ui.store.ts`: add `listPanelWidth` (default 280), `setListPanelWidth`, `resetListPanelWidth`; remove `sidebarExpanded`, `sidebarCollapsed`, `toggleSidebarExpand`, `setSidebarExpanded`; add Zustand persist middleware for `listPanelWidth`
- [x] 1.2 Create `hooks/useResizable.ts`: custom hook with `mousedown/mousemove/mouseup` tracking, `min/max` constraints (200–480px), `requestAnimationFrame` throttling, double-click reset, returns `{ size, handleProps, resetSize }`
- [x] 1.3 Add CSS tokens to `index.css`: resize handle cursor styles (`col-resize`), pill tab styles, updated message density variables, subtle bubble background color

## 2. LarkRail Component

- [x] 2.1 Create `components/patterns/LarkRail.tsx`: fixed ~90px rail with icon + label nav items (Messenger, Docs, Assets, Settings), avatar at top, search shortcut indicator (Ctrl+K), colored active background per module
- [x] 2.2 Implement "Docs" click behavior: `window.open('/drive', '_blank')` instead of `setActiveModule`
- [x] 2.3 Add notification badge support: red dot or count badge on rail items (driven by WebSocket unread state)
- [x] 2.4 Delete `components/patterns/Sidebar.tsx` and `components/patterns/AppRail.tsx`; update `components/patterns/index.ts` exports

## 3. ChatList Component

- [x] 3.1 Create `components/patterns/ChatListItem.tsx`: dense row (~56px) with circle avatar (36px), channel name (bold, truncated), last message preview (gray, one line), timestamp (right-aligned), unread badge (red circle with count)
- [x] 3.2 Create `components/patterns/ChatList.tsx`: scrollable list of ChatListItem components with search input at top, active item highlight, renders workspace channels
- [x] 3.3 Integrate unread tracking: use WebSocket store to derive unread counts per channel for badge display

## 4. Resizable ListPanel

- [x] 4.1 Refactor `components/patterns/ListPanel.tsx`: accept `width` prop from parent, remove fixed `w-[280px]`, make it fill the provided width. Keep internal content rendering (ChatList for messaging, DriveNav, AssetNav, SettingsNav)
- [x] 4.2 Replace ChatList content in ListPanel: when `activeModule === 'messaging'`, render the new `ChatList` component instead of the old `ChannelList`

## 5. Workspace Layout Rewire

- [x] 5.1 Refactor `routes/_workspace.tsx`: replace `<Sidebar>` with `<LarkRail>` + resizable `<ListPanel>` + `<ContentPanel>` 3-column layout. Wire `useResizable` hook for ListPanel resize. Remove topbar (workspace name bar) — LarkRail handles workspace identity
- [x] 5.2 Add resize handle element between ListPanel and ContentPanel: invisible 4px hit area, `col-resize` cursor on hover, subtle accent line during drag
- [x] 5.3 Verify mobile responsive: hide resize handle on ≤768px, ListPanel becomes full-screen overlay, LarkRail becomes bottom nav bar (existing CSS patterns)

## 6. Chat View Restyle

- [x] 6.1 Restyle chat header in `channels.$channelId.tsx`: replace icon buttons with pill-style tabs (Chat active/filled, Pinned outline, + button). Move Search/Members/Settings to right-aligned icons
- [x] 6.2 Update `MessageRow`: add subtle flat bubble background (dark navy/tinted), tighten vertical spacing, ensure consecutive messages from same sender don't repeat avatar
- [x] 6.3 Restyle `ChatEditor`: minimal container, right-aligned tool icons (Aa, emoji, attach, send), placeholder "Message [channel name]", remove heavy borders
- [x] 6.4 Add timestamp dividers: centered date/time dividers between messages with >30 minute gaps

## 7. Standalone Drive Page

- [x] 7.1 Create `routes/_drive.tsx` layout route: standalone 2-column layout (DriveSidebar + DriveContent), no LarkRail, own auth guard, page title "Docs"
- [x] 7.2 Create `components/drive/DriveSidebar.tsx`: ~180px sidebar with Search, Home nav, Drive nav (expandable folder tree: My Folders / Shared Folders), Wiki placeholder
- [x] 7.3 Create `routes/_drive/home.tsx`: Home view with action bar (New, Upload, Templates), tab filters (Recent, Owned by Me, Shared With Me, Favorites), file table
- [x] 7.4 Create `routes/_drive/drive.tsx`: Drive view with folder contents, My Folders / Shared Folders tabs, file table with Name/Modified/Created columns, sort and display controls (list/grid toggle)
- [x] 7.5 Migrate file table rendering: reuse/adapt existing `DriveFileList` and `DriveFileRow` components for the new standalone Drive layout, add colored file-type icons, 36–40px row height, hover "..." menu

## 8. Cleanup & Verification

- [x] 8.1 Remove old Drive route from workspace: delete or redirect `routes/_workspace/drive.tsx` (Drive is now at `/_drive`)
- [x] 8.2 Update route tree: run TanStack Router code generation to pick up new `/_drive` routes
- [x] 8.3 Verify build: `npm run build` passes with no errors
- [x] 8.4 Visual verification: open the app, confirm LarkRail + ChatList + ChatPanel 3-column layout renders correctly; confirm Docs click opens new tab with Drive page; confirm ListPanel drag-to-resize works
