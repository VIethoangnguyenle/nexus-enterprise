## 1. Sidebar Component Migration (Phase 2)

- [x] 1.1 Create `Sidebar.tsx` component to replace `AppRail.tsx`
- [x] 1.2 Implement sidebar toggle state in `useUiStore` (expanded 240px, collapsed 64px)
- [x] 1.3 Add global search input to the top of the Sidebar
- [x] 1.4 Implement hierarchical navigation links with icons and labels
- [x] 1.5 Update `_workspace.tsx` layout to use the new Sidebar instead of AppRail

## 2. Common Data Table & Peek Panel (Foundation for Phases 4 & 5)

- [x] 2.1 Create `DataTable.tsx` component with header, rows, and flat hover states
- [x] 2.2 Create `PeekPanel.tsx` component with slide-in animation from the right
- [x] 2.3 Create `Breadcrumbs.tsx` component for hierarchical navigation

## 3. Drive Module Migration (Phase 4)

- [x] 3.1 Update `routes/_workspace/drive.tsx` layout to use `DataTable` instead of grid
- [x] 3.2 Implement Breadcrumbs at the top of the Drive view
- [x] 3.3 Wire file selection to open `PeekPanel` for file details instead of a modal
- [x] 3.4 Ensure no backdrop-blur or box-shadow glow is present in Drive UI

## 4. Assets Module Migration (Phase 5)

- [x] 4.1 Update `routes/_assets.tsx` layout to use `DataTable`
- [x] 4.2 Replace floating filters with a flat filter bar above the table
- [x] 4.3 Wire asset selection to open `PeekPanel` for asset details

## 5. Messaging Module Migration (Phase 3)

- [x] 5.1 Remove message bubble backgrounds/gradients from `MessageItem.tsx`
- [x] 5.2 Implement dense vertical layout for message lists
- [x] 5.3 Implement right-side Thread Peek Panel in `channels.$channelId.tsx`
- [x] 5.4 Connect "Reply" action to open the Thread Peek Panel

## 6. Verification

- [x] 6.1 Run browser tests for all modules (Sidebar, Drive, Assets, Messaging)
- [x] 6.2 Verify sidebar collapse/expand behavior
- [x] 6.3 Verify PeekPanel slide-in animations
- [x] 6.4 Check Vite build for any styling conflicts
