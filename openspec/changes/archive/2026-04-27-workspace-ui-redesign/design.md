## Context

The NGAC frontend is a Vite + TanStack Router + TanStack Query SPA with Zustand for UI state. The workspace layout (`_workspace.tsx`) renders a shared sidebar and topbar, with child routes for documents, assets, channels, and settings. All asset pages are currently flat children of `_workspace/`, displayed as individual sidebar items alongside documents and channels.

Current routing structure:
```
routes/
  __root.tsx
  _auth.tsx          → AuthLayout (login/register)
  _workspace.tsx     → WorkspaceLayout (sidebar + topbar + <Outlet/>)
  _workspace/
    documents.tsx
    asset-dashboard.tsx
    assets.tsx
    assets_.$assetId.tsx
    asset-requests.tsx
    asset-types.tsx
    asset-request/new.tsx
    channels.$channelId.tsx
    settings.tsx
```

## Goals / Non-Goals

**Goals:**
- Fix sidebar collapsed CSS so it actually works (narrow mode with icons only)
- Add channel creation flow accessible from the sidebar
- Complete asset type creation UI so the full asset workflow is usable
- Extract asset management into a separate layout with its own navigation
- Zero backend changes — all needed APIs already exist

**Non-Goals:**
- Workspace CRUD (create/switch/delete workspace) from the UI — future change
- Dark/light theme toggle
- Responsive mobile layout
- Channel deletion or channel settings
- Asset type editing/deletion (create-only for this change)

## Decisions

### D1: Asset management as a TanStack Router layout route

**Decision**: Create `_assets.tsx` as a new layout route at the same level as `_workspace.tsx`. Both require authentication. Asset pages become children of `/_assets/`.

**Routing after change:**
```
routes/
  __root.tsx
  _auth.tsx          → AuthLayout
  _workspace.tsx     → WorkspaceLayout
  _workspace/
    documents.tsx
    channels.$channelId.tsx
    settings.tsx
  _assets.tsx        → AssetLayout (NEW — own sidebar)
  _assets/
    dashboard.tsx
    list.tsx
    requests.tsx
    types.tsx
    $assetId.tsx
    request/new.tsx
```

**Rationale**: TanStack Router's file-based layout routes make this natural. `_assets.tsx` renders its own sidebar + topbar + `<Outlet/>`, completely independent of the workspace layout. The user navigates between workspace and assets as top-level contexts.

**Alternatives considered:**
- *Nested route inside `_workspace/`*: Would still share the workspace sidebar, defeating the purpose. Assets need their own navigation context.
- *Tab bar instead of sidebar*: Less room for future growth (asset history, reports, settings). Sidebar is consistent with the workspace pattern.

### D2: Asset layout sidebar design

**Decision**: The asset layout sidebar has:
- Header with "Asset Management" title + back button (← Workspace)
- Nav items: Dashboard, All Assets, Requests, Type Config
- Footer with user info + logout (same as workspace)

```
┌──────────────────────────────────────────────┐
│ ← Workspace   │  Asset Dashboard        🔔  │
├────────────────┼─────────────────────────────│
│                │                             │
│ MANAGEMENT     │  Stats cards...             │
│  📊 Dashboard  │                             │
│  📦 All Assets │  Recent activity...         │
│  📋 Requests   │                             │
│  🏷️ Types      │                             │
│                │                             │
│ ────────────── │                             │
│ ⚙️ Settings    │                             │
│ 🚪 Logout      │                             │
└────────────────┴─────────────────────────────┘
```

### D3: Sidebar collapsed mode CSS

**Decision**: Add proper `.sidebar.collapsed` CSS:
- Width shrinks from 260px to 64px (icon-only)
- Section titles hidden
- Item text hidden (already done in JSX)
- Logo reduces to single letter
- Footer shows only icons
- Smooth transition with `transition: width 0.2s`

### D4: Channel creation modal

**Decision**: Simple modal triggered by "+" button next to "Channels" section header in sidebar. Fields: channel name only. Uses existing `useCreateChannel` hook. After creation, navigates to `/channels/$channelId`.

### D5: Asset type creation

**Decision**: Add inline form or modal on the asset-types page. Fields: name (required), category (required, select from: hardware, software, license, furniture, other). Uses existing `useCreateAssetType` hook. After creation, type appears in list and becomes selectable in the request form.

## Component Architecture

```
WorkspaceLayout (_workspace.tsx)
├── Sidebar (simplified)
│   ├── DOCUMENTS section
│   │   └── All Documents → /documents
│   ├── CHANNELS section
│   │   ├── Channel list (dynamic)
│   │   └── [+] Create Channel → modal
│   └── NAVIGATION
│       └── 📦 Asset Management → /assets/dashboard (full page nav)
│
├── Topbar (workspace name + notifications)
└── <Outlet/> → documents.tsx | channels.$channelId.tsx

AssetLayout (_assets.tsx)
├── AssetSidebar
│   ├── Header (← Workspace + title)
│   ├── Dashboard / All Assets / Requests / Types
│   └── Footer (user + logout)
│
├── Topbar (Asset Management + notifications)
└── <Outlet/> → dashboard.tsx | list.tsx | requests.tsx | types.tsx | $assetId.tsx

CreateChannelModal
├── Input: channel name
├── Uses useCreateChannel mutation
└── onSuccess → navigate to /channels/$channelId
```

## Data Flow

No new APIs needed. Existing hooks used:
- `useCreateChannel(wsId)` — already exists in `useMessaging.ts`
- `useCreateAssetType(wsId)` — already exists in `useAssets.ts` (imported but unused)
- `useChannels(wsId)` — already used for sidebar channel list
- `useAssetTypes(wsId)` — already used in request form

The asset layout needs workspace context (wsId) for API calls. Since asset pages are no longer children of `_workspace.tsx`, the active workspace ID will come from a dedicated hook (`useActiveWorkspace`) that reads from Zustand or TanStack Query cache.
