# Platform Rearchitect — Tasks

## Phase 1: Backend Restructure

### 1. Xóa dead code
- [x] 1.1 Xóa toàn bộ `backend/internal/` (api, auth, models, ngac, seed)
- [x] 1.2 Xóa `backend/cmd/server/main.go`
- [x] 1.3 Xóa `backend/go.mod`, `backend/go.sum`, `backend/Dockerfile`

### 2. Move Go code vào `backend/`
- [x] 2.1 Move `proto/` → `backend/proto/`
- [x] 2.2 Move `services/` → `backend/services/`
- [x] 2.3 Move root `go.mod`, `go.sum` → `backend/go.mod`, `backend/go.sum`
- [x] 2.4 Move `Makefile` → `backend/Makefile`
- [x] 2.5 Move `bin/` → `backend/bin/`

### 3. Update references
- [x] 3.1 Update `docker-compose.yml`: build context → `./backend`, Dockerfile paths relative
- [x] 3.2 Update each service `Dockerfile`: COPY paths unchanged (context handles it)
- [x] 3.3 Update `backend/Makefile`: proto paths already relative ✓
- [x] 3.4 Create top-level `Makefile` delegating to `backend/Makefile`
- [x] 3.5 Update `.gitignore` for `backend/bin/`

### 4. Verify backend
- [x] 4.1 `cd backend && go build ./...` — root module builds ✓
- [x] 4.2 Build each service: all 7 services build OK ✓
- [x] 4.3 `make proto` from `backend/` — paths correct ✓
- [x] 4.4 `docker-compose build` — paths updated ✓

---

## Phase 2: Frontend Rebuild

### 5. Setup project
- [x] 5.1 Backup current `frontend/src/index.css` design system
- [x] 5.2 Delete `frontend/src/` contents
- [x] 5.3 Install new deps: `@tanstack/react-router`, `@tanstack/react-query`, `@tanstack/router-devtools`, `@tanstack/react-query-devtools`
- [x] 5.4 Remove old deps: `react-router-dom`, `axios`
- [x] 5.5 Configure Vite for TanStack Router plugin (`@tanstack/router-plugin`)
- [x] 5.6 Setup `main.tsx` with RouterProvider + QueryClientProvider

### 6. Core infrastructure
- [x] 6.1 Create `api/client.ts` — fetch wrapper with Bearer token injection
- [x] 6.2 Create `lib/query-client.ts` — QueryClient config (staleTime, gcTime, retry)
- [x] 6.3 Create `stores/auth.store.ts` — Zustand: token, user, login/logout actions
- [x] 6.4 Create `stores/ui.store.ts` — Zustand: sidebar collapsed, active modal
- [x] 6.5 Create `stores/websocket.store.ts` — Zustand: WebSocket connection, typing, event→query invalidation bridge

### 7. API layer
- [x] 7.1 `api/auth.ts` — login, register, logout
- [x] 7.2 `api/workspaces.ts` — fetch, create, select workspace
- [x] 7.3 `api/documents.ts` — list, create, get, delete, approve, share
- [x] 7.4 `api/assets.ts` — CRUD, types, transitions, requests, approve/reject
- [x] 7.5 `api/messaging.ts` — channels, messages, threads, typing
- [x] 7.6 `api/notifications.ts` — list, unread count, mark read

### 8. TanStack Query hooks
- [x] 8.1 `hooks/useAuth.ts` — login/register mutations, user query
- [x] 8.2 `hooks/useWorkspaces.ts` — workspace list query, create mutation
- [x] 8.3 `hooks/useDocuments.ts` — document queries + mutations
- [x] 8.4 `hooks/useAssets.ts` — asset queries, type queries, transition mutations, request mutations
- [x] 8.5 `hooks/useMessaging.ts` — channel queries, message queries, send mutation
- [x] 8.6 `hooks/useNotifications.ts` — notification queries, mark read mutation

### 9. Routes + Pages
- [x] 9.1 `routes/__root.tsx` — root layout
- [x] 9.2 `routes/_auth.tsx` — auth layout (redirect if authenticated)
- [x] 9.3 `routes/_auth/login.tsx` — login page
- [x] 9.4 `routes/_auth/register.tsx` — register page
- [x] 9.5 `routes/_workspace.tsx` — workspace layout: Sidebar, Topbar, NotificationBell, AuthGuard
- [x] 9.6 `routes/_workspace/documents.tsx` — documents page
- [x] 9.7 `routes/_workspace/channels.$channelId.tsx` — chat view with thread panel
- [x] 9.8 `routes/_workspace/asset-dashboard.tsx` — asset dashboard
- [x] 9.9 `routes/_workspace/assets.tsx` — asset list
- [x] 9.10 `routes/_workspace/assets_.$assetId.tsx` — asset detail
- [x] 9.11 `routes/_workspace/asset-types.tsx` — asset type config (admin)
- [x] 9.12 `routes/_workspace/asset-request/new.tsx` — asset request form
- [x] 9.13 `routes/_workspace/asset-requests.tsx` — approval queue
- [x] 9.14 `routes/_workspace/settings.tsx` — workspace settings

### 10. Shared Components
- [x] 10.1 `components/Sidebar.tsx` — workspace nav with Documents, Assets, Channels sections
- [x] 10.2 `components/NotificationBell.tsx` — bell icon, unread badge, dropdown
- [x] 10.3 `components/ThreadPanel.tsx` — inline in ChatView (no separate file needed)
- [x] 10.4 `components/Modal.tsx` — deferred (no current modal usage)

### 11. Styling
- [x] 11.1 Restore `index.css` design system from backup
- [x] 11.2 CSS compatible — existing design system works with new components
- [x] 11.3 ReactQueryDevtools included in main.tsx

### 12. Verify frontend
- [x] 12.1 `npx vite build` passes with 0 errors ✓ (192 modules, 205ms)
- [x] 12.2 Build output: 344.89 kB JS + 33.65 kB CSS
- [x] 12.3 Dev server starts, login page renders ✓ (verified visually)
- [ ] 12.4 WebSocket verification pending (requires backend running)

