# Agent Instructions — NGAC Platform

## Skill Routing Rules

### Always-Read Skills (TRƯỚC MỌI task Go code)

Trước khi viết hoặc sửa bất kỳ dòng Go code nào, ĐỌC các skills này:

1. **golang-code-style** — Formatting, conventions, early return
2. **golang-error-handling** — Wrapping, single handling rule, slog
3. **golang-naming** — Packages, functions, errors, enums

### Context-Triggered Skills

Đọc thêm skill tương ứng khi task liên quan:

| Context | Skills cần đọc |
|---------|----------------|
| gRPC server/client | `golang-grpc` |
| Database queries | `golang-database` |
| Concurrency (goroutines, channels) | `golang-concurrency` |
| Context handling | `golang-context` |
| Struct/Interface design | `golang-structs-interfaces` |
| Design decisions, architecture | `golang-design-patterns` |
| Security review | `golang-security` |
| Dependency injection | `golang-dependency-injection` |
| Testing | `golang-testing`, `golang-stretchr-testify` |
| Performance | `golang-performance`, `golang-benchmark` |
| New dependency | `golang-popular-libraries` |
| Logging/Metrics | `golang-observability` |
| Nil safety, panic prevention | `golang-safety` |
| Linting | `golang-lint` |
| Project layout | `golang-project-layout` |
| Modernize old patterns | `golang-modernize` |

### samber/* Library Skills

Khi code sử dụng thư viện samber, đọc skill tương ứng:

| Import | Skill |
|--------|-------|
| `samber/lo` | `golang-samber-lo` |
| `samber/do` | `golang-samber-do` |
| `samber/oops` | `golang-samber-oops` |
| `samber/mo` | `golang-samber-mo` |
| `samber/hot` | `golang-samber-hot` |
| `samber/slog-*` | `golang-samber-slog` |

---

## Project Conventions

### Tech Stack — Backend
- **Go version**: 1.26+
- **Database driver**: `pgx/v5` (NOT sqlx, NOT GORM)
- **Logging**: `log/slog` (migrate from `log.Printf`)
- **Error handling**: stdlib `fmt.Errorf` with `%w`, `status.Errorf` at gRPC boundaries
- **gRPC**: `google.golang.org/grpc` with interceptors
- **Proto management**: `protoc` with `cd backend && make proto`
- **Infra**: Redis (Pub/Sub, cache, JWT blacklist), Redpanda/Kafka (events)

### Tech Stack — Frontend
- **Build**: Vite
- **Routing**: TanStack Router (file-based, type-safe)
- **Server state**: TanStack Query (queries, mutations, cache invalidation)
- **Client state**: Zustand (UI-only: WebSocket, sidebar, modals)
- **HTTP**: Native `fetch` (no axios)
- **CSS**: Vanilla CSS (custom design system)

### Architecture Rules (from AGENTS.md)
- All Go code lives under `backend/` — proto, services, go.mod
- Policy Service is the ONLY PDP — no service decides access on its own
- Services communicate via gRPC — no internal HTTP
- Gateway is the ONLY external entry point
- Proto is the contract — change `.proto` → `cd backend && make proto` → update consumers
- JWT contains `ngac_node_id` — downstream services use this, never query user table
- Each service has its own Go module with `replace ngac-platform => ../..`
- Frontend uses TanStack Query for ALL server state — Zustand ONLY for UI state
- WebSocket events bridge to TanStack Query via `queryClient.invalidateQueries()`

### Backend Service Structure
```
backend/services/{name}/
├── cmd/main.go           # Wiring only
├── internal/
│   ├── grpc/server.go    # Thin handler, delegates to domain
│   ├── domain/           # Business logic, no gRPC/DB deps
│   └── store/            # Database layer, no domain deps
├── go.mod                   replace ngac-platform => ../..
└── Dockerfile
```

### Frontend Structure
```
frontend/src/
├── routes/               # TanStack Router file-based routes
│   ├── __root.tsx           root layout
│   ├── _auth.tsx            auth layout (redirect if logged in)
│   ├── _auth/login.tsx      login page
│   ├── _workspace.tsx       workspace layout (Sidebar, Topbar, AuthGuard)
│   └── _workspace/*.tsx     workspace pages
├── hooks/                # TanStack Query hooks (useAssets, useDocuments, etc.)
├── api/                  # fetch wrappers (client.ts, auth.ts, assets.ts, etc.)
├── stores/               # Zustand stores (auth, ui, websocket)
├── components/           # Shared components (Sidebar, NotificationBell)
└── lib/query-client.ts   # QueryClient configuration
```

### Forbidden Patterns
- ❌ `init()` functions — use explicit constructors
- ❌ `log.Fatal` in library code — only in `main()`
- ❌ `_ = someFunc()` — always handle errors
- ❌ Raw `error` from gRPC handlers — use `status.Errorf` with proper codes
- ❌ `fmt.Sprintf` with user input in SQL — always parameterized
- ❌ `TODO` comments — fix now or create OpenSpec task
- ❌ Functions > 50 lines — split into smaller functions
- ❌ ORM (GORM, ent) — use raw SQL with pgx

### Required Patterns
- ✅ Graceful shutdown with signal handling in every service
- ✅ gRPC interceptors for logging and recovery
- ✅ Health check service registration (`grpc_health_v1`)
- ✅ Connection pool config for database
- ✅ Context propagation to all DB operations
- ✅ `defer rows.Close()` immediately after Query
- ✅ Error wrapping: `fmt.Errorf("operation_name: %w", err)`
- ✅ Conventional Commits for git messages

### gRPC Error Code Mapping
| Situation | Code |
|-----------|------|
| Invalid request input | `codes.InvalidArgument` |
| Entity not found | `codes.NotFound` |
| Entity already exists | `codes.AlreadyExists` |
| No permission (NGAC deny) | `codes.PermissionDenied` |
| Missing/invalid JWT | `codes.Unauthenticated` |
| Unexpected internal error | `codes.Internal` |
| Upstream timeout | `codes.DeadlineExceeded` |

---

## Git Workflow

### Branch Strategy
```
main (production-ready)
  ├── feat/{feature-name}
  ├── fix/{bug-name}
  ├── refactor/{scope}
  └── chore/{task}
```

### Commit Convention (Conventional Commits)
```
<type>: <description>

[optional body]
```

Types: `feat`, `fix`, `refactor`, `chore`, `docs`, `ci`, `test`, `perf`

### Rules
- Every commit must build (`cd backend/services/X && go build ./cmd/` passes)
- Frontend must build (`cd frontend && npx vite build` passes)
- One logical change per commit
- Commit message in English, lowercase
- Push after completing each logical unit of work
