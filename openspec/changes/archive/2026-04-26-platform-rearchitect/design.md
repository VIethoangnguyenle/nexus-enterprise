# Platform Rearchitect — Design

## Part 1: Backend Restructure

### Current State (vấn đề)

```
ngac/                           ← Go code rải rác 3+ nơi
├── go.mod                      ← root module "ngac-platform" (proto deps only)
├── go.sum
├── Makefile                    ← proto gen + build targets
├── bin/                        ← compiled binaries
├── proto/                      ← protobuf definitions + generated Go
├── services/                   ← 7 microservices (ACTIVE)
│   └── {svc}/go.mod               replace ngac-platform => ../..
├── backend/                    ← monolith cũ (DEAD CODE)
│   ├── go.mod                     module "ngac-document-platform" 
│   ├── cmd/server/main.go         Chi router HTTP monolith
│   ├── internal/api/              REST handlers (thay bởi services/gateway)
│   ├── internal/ngac/             NGAC engine (thay bởi services/policy)
│   ├── internal/auth/             JWT (thay bởi services/auth)
│   ├── internal/models/           DB models (thay bởi per-service stores)
│   ├── internal/seed/             Seed data
│   └── Dockerfile
├── frontend/
├── data/
└── docker-compose.yml          ← references services/* only
```

### Target State

```
ngac/
├── backend/                    ← ALL Go code lives here
│   ├── go.mod                     module "ngac-platform"
│   ├── go.sum
│   ├── Makefile                   proto gen + build targets
│   ├── bin/                       compiled binaries
│   ├── proto/                     protobuf contracts
│   │   ├── policy/
│   │   ├── auth/
│   │   ├── workspace/
│   │   ├── document/
│   │   ├── messaging/
│   │   └── asset/
│   └── services/                  microservices
│       ├── policy/
│       │   ├── go.mod                replace ngac-platform => ../..
│       │   ├── cmd/main.go
│       │   ├── internal/
│       │   │   ├── grpc/server.go
│       │   │   ├── ngac/             graph engine, access control
│       │   │   └── events/           Kafka producer
│       │   └── Dockerfile
│       ├── auth/
│       ├── workspace/
│       ├── document/
│       ├── messaging/
│       ├── asset/
│       └── gateway/
├── frontend/                   ← Rebuilt with TanStack
├── data/
├── docker-compose.yml          ← updated build paths
└── Makefile                    ← top-level: delegates to backend/Makefile
```

### Migration Steps (Backend)

1. **Xóa dead code trong `backend/`**: rm -rf backend/ contents
2. **Move files**:
   - `services/` → `backend/services/`
   - `proto/` → `backend/proto/`
   - root `go.mod`, `go.sum` → `backend/go.mod`, `backend/go.sum`
   - `Makefile` → `backend/Makefile`
   - `bin/` → `backend/bin/`
3. **Update go.mod replace directives**: vẫn `replace ngac-platform => ../..` (relative path giữ nguyên vì services vẫn 2 level dưới backend/)
4. **Update docker-compose.yml**: `services/X/Dockerfile` → `backend/services/X/Dockerfile`
5. **Update Dockerfiles**: COPY paths adjust for new build context
6. **Update Makefile**: proto paths relative to `backend/`
7. **Create top-level Makefile**: delegates `make build`, `make proto` to `backend/`
8. **Verify**: build all services, docker-compose build

### Decision: go.mod replace directives

Hiện tại mỗi service có:
```
module ngac-platform/services/auth
replace ngac-platform => ../..
```

Sau khi move, `../..` vẫn trỏ tới `backend/` (nơi có `go.mod` root) → **KHÔNG cần thay đổi go.mod**.

---

## Part 2: Frontend Rebuild

### Current Stack vs New Stack

| Layer | Current | New |
|-------|---------|-----|
| Build | Vite | Vite (giữ nguyên) |
| Routing | react-router-dom v7 | **TanStack Router** (type-safe, file-based) |
| Server State | zustand + axios (manual) | **TanStack Query** (cache, refetch, optimistic) |
| Client State | zustand (mixed) | zustand (UI-only: WebSocket, modals, sidebar) |
| HTTP | axios | **fetch** native (TanStack Query prefer) |
| CSS | Vanilla CSS | Vanilla CSS (giữ design system) |

### Frontend Architecture

```
frontend/
├── src/
│   ├── main.tsx                       ← entry, router + query provider
│   ├── routeTree.gen.ts               ← auto-generated route tree
│   ├── routes/
│   │   ├── __root.tsx                 ← root layout (QueryProvider, AuthGuard)
│   │   ├── _auth.tsx                  ← auth layout (redirect if logged in)
│   │   ├── _auth.login.tsx            ← /login
│   │   ├── _auth.register.tsx         ← /register
│   │   ├── _workspace.tsx             ← workspace layout (Sidebar, Topbar, NotificationBell)
│   │   ├── _workspace.documents.tsx   ← /documents
│   │   ├── _workspace.assets.tsx      ← /assets (list)
│   │   ├── _workspace.assets.$id.tsx  ← /assets/:id (detail)
│   │   ├── _workspace.asset-dashboard.tsx
│   │   ├── _workspace.asset-types.tsx
│   │   ├── _workspace.asset-requests.tsx
│   │   ├── _workspace.asset-request.new.tsx
│   │   ├── _workspace.channels.$id.tsx
│   │   └── _workspace.settings.tsx
│   ├── api/
│   │   ├── client.ts                  ← fetch wrapper with auth token
│   │   ├── auth.ts                    ← login, register, logout
│   │   ├── workspaces.ts              ← workspace CRUD
│   │   ├── documents.ts               ← document CRUD
│   │   ├── assets.ts                  ← asset CRUD, types, requests
│   │   ├── messaging.ts               ← channels, messages, threads
│   │   └── notifications.ts           ← notification CRUD
│   ├── hooks/
│   │   ├── useAuth.ts                 ← TanStack Query + auth state
│   │   ├── useAssets.ts               ← useQuery/useMutation for assets
│   │   ├── useDocuments.ts
│   │   ├── useMessaging.ts
│   │   └── useNotifications.ts
│   ├── stores/
│   │   ├── auth.store.ts              ← token, user (persisted)
│   │   ├── ui.store.ts                ← sidebar state, modals
│   │   └── websocket.store.ts         ← WebSocket connection, typing indicators
│   ├── components/
│   │   ├── Sidebar.tsx
│   │   ├── NotificationBell.tsx
│   │   ├── ThreadPanel.tsx
│   │   └── ...
│   ├── index.css                      ← design system (migrate from current)
│   └── lib/
│       └── query-client.ts            ← QueryClient config
└── package.json
```

### Key Patterns

**TanStack Router — File-based routing:**
```typescript
// routes/_workspace.assets.tsx
export const Route = createFileRoute('/_workspace/assets')({
  loader: () => queryClient.ensureQueryData(assetsQueryOptions()),
  component: AssetList,
})
```

**TanStack Query — Server state:**
```typescript
// hooks/useAssets.ts
export const assetsQueryOptions = (wsId: string) =>
  queryOptions({
    queryKey: ['assets', wsId],
    queryFn: () => assetApi.list(wsId),
  })

export function useAssets(wsId: string) {
  return useQuery(assetsQueryOptions(wsId))
}

export function useCreateAsset(wsId: string) {
  return useMutation({
    mutationFn: (data) => assetApi.create(wsId, data),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: ['assets', wsId] }),
  })
}
```

**Zustand — UI-only state:**
```typescript
// stores/websocket.store.ts
export const useWebSocketStore = create((set) => ({
  connected: false,
  typingUsers: {},
  connect: (token) => { /* WebSocket setup, invalidate queries on events */ },
}))
```

**WebSocket → TanStack Query bridge:**
```typescript
// On WebSocket message:
if (data.type === 'asset_updated') {
  queryClient.invalidateQueries({ queryKey: ['assets'] })
}
if (data.type === 'notification') {
  queryClient.invalidateQueries({ queryKey: ['notifications'] })
}
```

### CSS Strategy

Migrate `index.css` design system as-is. Giữ toàn bộ CSS variables, component styles. Chỉ thay đổi class names nếu cần align với component mới.
