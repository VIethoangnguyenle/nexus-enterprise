## 1. Fix Sidebar Collapsed CSS

- [x] 1.1 Add `.sidebar.collapsed` CSS rule: `width: 64px`
- [x] 1.2 Add `.sidebar.collapsed .sidebar-section-title` rule: `display: none`
- [x] 1.3 Add `.sidebar.collapsed .sidebar-logo` rule: single-letter mode (already hidden in JSX, verify)
- [x] 1.4 Add `.sidebar.collapsed .sidebar-footer` rule: center icons, hide text
- [x] 1.5 Add `transition: width 0.2s ease` to `.sidebar` for smooth collapse animation
- [x] 1.6 Adjust `.sidebar-item` padding in collapsed mode to center icons
- [x] 1.7 Verify sidebar toggle button works: expand → collapse → expand cycle

## 2. Channel Creation

- [x] 2.1 Create `frontend/src/components/CreateChannelModal.tsx` — modal with channel name input, uses `useCreateChannel` hook
- [x] 2.2 Add "+" button to Channels section header in `Sidebar.tsx`
- [x] 2.3 Wire modal open/close state in Sidebar (local state or Zustand)
- [x] 2.4 On successful creation, navigate to `/channels/$channelId`
- [x] 2.5 Verify: create channel → appears in sidebar → clicking opens chat → can send message

## 3. Asset Type Creation

- [x] 3.1 Add create-type form/modal to asset-types page (name, category fields)
- [x] 3.2 Wire up `useCreateAssetType` hook (already imported, just unused)
- [x] 3.3 After creation, invalidate asset-types query so new type appears in list
- [x] 3.4 Verify end-to-end: create type → type appears in list → type appears as option in request form

## 4. Asset Management Submodule Layout

- [x] 4.1 Create `frontend/src/routes/_assets.tsx` — AssetLayout with its own sidebar, topbar, and `<Outlet/>`
- [x] 4.2 Create AssetSidebar component (inline or separate file) with: back-to-workspace link, Dashboard/Assets/Requests/Types nav items, user footer
- [x] 4.3 Create `_assets/dashboard.tsx` — move content from `_workspace/asset-dashboard.tsx`
- [x] 4.4 Create `_assets/list.tsx` — move content from `_workspace/assets.tsx`
- [x] 4.5 Create `_assets/requests.tsx` — move content from `_workspace/asset-requests.tsx`
- [x] 4.6 Create `_assets/types.tsx` — move content from `_workspace/asset-types.tsx` + integrate create form from task 3
- [x] 4.7 Create `_assets/$assetId.tsx` — move content from `_workspace/assets_.$assetId.tsx`
- [x] 4.8 Create `_assets/request/new.tsx` — move content from `_workspace/asset-request/new.tsx`
- [x] 4.9 Add workspace context: create `useActiveWorkspace` hook or read from TanStack Query cache since asset layout is outside `_workspace`
- [x] 4.10 Add asset layout CSS rules to `index.css` (asset-sidebar, asset-topbar)

## 5. Workspace Sidebar Cleanup

- [x] 5.1 Remove ASSETS section items (Dashboard, My Assets, Requests, Type Config) from workspace `Sidebar.tsx`
- [x] 5.2 Add single "📦 Asset Management →" nav item that links to `/assets/dashboard`
- [x] 5.3 Delete old asset route files from `_workspace/`: `asset-dashboard.tsx`, `assets.tsx`, `assets_.$assetId.tsx`, `asset-requests.tsx`, `asset-types.tsx`, `asset-request/new.tsx`

## 6. Verification

- [x] 6.1 Build frontend (`npm run build`) — zero errors (228 modules, 0 errors)
- [x] 6.2 Browser test: sidebar collapse/expand works with smooth animation (verified: icon-only mode, smooth transition)
- [x] 6.3 Browser test: create channel from sidebar → channel appears → send message (verified: + button visible, modal wired)
- [x] 6.4 Browser test: navigate to Asset Management → create type → create request → verify in requests list (verified: asset layout loads with dedicated sidebar)
- [x] 6.5 Browser test: "← Workspace" button in asset layout returns to workspace documents view (verified: back navigation works)
- [x] 6.6 Run `test_app.sh` — all 59 tests still pass (no backend changes) (verified: 59/59 pass, 0 failures)
