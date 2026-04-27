## Why

The NGAC frontend workspace has four critical UX issues blocking usability:

1. **Sidebar collapsed layout is broken** — The CSS class `.sidebar.collapsed` is applied in DOM but has no corresponding CSS rules. Section headers (DOCUMENTS, ASSETS, CHANNELS) remain visible when items are hidden, creating visual inconsistency. The sidebar toggle exists but produces a broken half-state.

2. **No way to create or join channels** — The Channels section in the sidebar lists existing channels but has no "Create Channel" button. A fresh workspace has zero channels, leaving the messaging feature completely inaccessible from the UI.

3. **Asset type creation is non-functional** — The `asset-types.tsx` page imports `useCreateAssetType` but never uses it — there's no create form or button. Since the asset request form depends on selecting an asset type (`<select>` from `typesData?.types`), a workspace with zero types has a dead-end flow: can't create types → can't create requests → can't use assets.

4. **Asset management needs its own layout** — All asset pages (Dashboard, My Assets, Requests, Type Config) are flat sidebar items competing for attention alongside Documents and Channels. Asset management is a distinct domain with its own navigation flow and should be a separate submodule with its own layout, making the primary workspace sidebar cleaner.

## What Changes

- **Fix sidebar collapsed mode**: Add `.sidebar.collapsed` CSS rules, hide section titles when collapsed, set proper collapsed width (64px), fix footer layout
- **Add channel creation**: "+" button in sidebar Channels section, create-channel modal with name input, auto-navigate to new channel after creation
- **Complete asset type CRUD**: Add create-type form/modal to `asset-types.tsx` page with name and category fields, wire up the existing `useCreateAssetType` hook
- **Asset management submodule layout**: Extract asset pages into a dedicated `/_assets` layout route with its own sidebar/navigation, add a single "Asset Management →" entry in the workspace sidebar that navigates to `/assets/dashboard`

## Capabilities

### New Capabilities
- `channel-management`: Create/list channels from the workspace sidebar UI
- `asset-submodule`: Standalone asset management layout with dedicated navigation

### Modified Capabilities
- `sidebar-layout`: Fix collapsed mode CSS and section header visibility
- `asset-type-crud`: Complete the create-type form in asset-types page

## Impact

- **`frontend/src/index.css`**: Add `.sidebar.collapsed` rules, asset-submodule layout styles
- **`frontend/src/components/Sidebar.tsx`**: Add "+" button for channel creation, replace flat asset links with single "Asset Management" link, fix collapsed section title visibility
- **`frontend/src/routes/_workspace.tsx`**: No changes (workspace layout stays)
- **`frontend/src/routes/_assets.tsx`** [NEW]: Asset management layout component with its own sidebar
- **`frontend/src/routes/_assets/dashboard.tsx`** [NEW]: Move from `_workspace/asset-dashboard.tsx`
- **`frontend/src/routes/_assets/list.tsx`** [NEW]: Move from `_workspace/assets.tsx`
- **`frontend/src/routes/_assets/requests.tsx`** [NEW]: Move from `_workspace/asset-requests.tsx`
- **`frontend/src/routes/_assets/types.tsx`** [NEW]: Move from `_workspace/asset-types.tsx` + add create form
- **`frontend/src/routes/_assets/$assetId.tsx`** [NEW]: Move from `_workspace/assets_.$assetId.tsx`
- **`frontend/src/routes/_assets/request/new.tsx`** [NEW]: Move from `_workspace/asset-request/new.tsx`
- **`frontend/src/routes/_workspace/asset-*.tsx`** [DELETE]: Old flat asset routes removed
- **`frontend/src/components/CreateChannelModal.tsx`** [NEW]: Modal for creating channels
- No backend changes — all APIs already exist
