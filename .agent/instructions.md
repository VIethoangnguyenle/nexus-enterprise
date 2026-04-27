# Agent Instructions — NGAC Platform

## Skill Routing Rules

### Always-Read Skills (TRƯỚC MỌI task Go code)

Trước khi viết hoặc sửa bất kỳ dòng Go code nào, ĐỌC các skills này:

1. **golang-code-style** — Formatting, conventions, early return
2. **golang-error-handling** — Wrapping, single handling rule, slog
3. **golang-naming** — Packages, functions, errors, enums

### Always-Read Skills (TRƯỚC MỌI task Frontend code)

Trước khi viết hoặc sửa bất kỳ file `.tsx`, `.ts` nào trong `frontend/`, ĐỌC skill này:

1. **frontend-best-practices** — File conventions, TanStack Router/Query patterns, state management, component design, error handling, performance, styling, security


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
- **Error handling**: Domain sentinel errors in `domain/errors.go`, mapped at handler boundary
- **REST framework**: Echo v4 (`github.com/labstack/echo/v4`) — mỗi service tự expose REST
- **gRPC**: `google.golang.org/grpc` with interceptors (service-to-service only)
- **Proto management**: `protoc` with `cd backend && make proto`
- **Infra**: Redis (Pub/Sub, cache, JWT blacklist), Redpanda/Kafka (events)
- **Shared packages**: `backend/pkg/httputil/` (JWT middleware, error mapping), `backend/ngac/` (constants)

### Tech Stack — Frontend
- **Build**: Vite
- **Routing**: TanStack Router (file-based, type-safe)
- **Server state**: TanStack Query (queries, mutations, cache invalidation)
- **Client state**: Zustand (UI-only: WebSocket, sidebar, modals)
- **HTTP**: Native `fetch` (no axios)
- **CSS**: Vanilla CSS (custom design system)

### Architecture Rules (from AGENTS.md)
- All Go code lives under `backend/` — proto, services, pkg, go.mod
- Policy Service is the ONLY PDP — no service decides access on its own
- Services communicate via gRPC — no internal HTTP
- **No Gateway monolith** — mỗi service tự serve REST (Echo) + gRPC
- Traefik routes path-based trực tiếp tới từng service
- Proto is the contract — change `.proto` → `cd backend && make proto` → update consumers
- JWT contains `ngac_node_id` — downstream services use this, never query user table
- Each service has its own Go module with `replace ngac-platform => ../..`
- Frontend uses TanStack Query for ALL server state — Zustand ONLY for UI state
- WebSocket events bridge to TanStack Query via `queryClient.invalidateQueries()`

### Backend Service Structure
```
backend/services/{name}/
├── cmd/main.go           # Wiring only — start REST + gRPC
├── internal/
│   ├── rest/handler.go   # Echo REST handler — client-facing
│   ├── grpc/server.go    # gRPC handler — service-to-service
│   ├── domain/           # Business logic, no REST/gRPC/DB deps
│   │   ├── service.go
│   │   └── errors.go
│   └── store/            # Database layer, no domain deps
│       ├── store.go
│       └── models.go
├── go.mod                   replace ngac-platform => ../...
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

---

## Clean Architecture Rules (12 Rules)

Inspired by [go-clean-arch v4](https://github.com/bxcodec/go-clean-arch) adapted for NGAC microservices.

### Rule 1: Layered Architecture — 3 Layers, No Shortcuts

```
Handler (grpc/)  →  Domain (domain/)  →  Store (store/)
    ↑ thin              ↑ orchestration       ↑ SQL only
    parse request       business logic        no logic
    delegate            policy calls          no proto
    format response     error decisions
```

**Dependency direction: Handler → Domain → Store. Never reverse.**

- Handler KHÔNG import `store/` trực tiếp
- Domain KHÔNG import `grpc/` hoặc proto request/response types
- Store KHÔNG import `domain/` hoặc `grpc/`

Exception: Domain CÓ THỂ import proto types khi cần gọi external gRPC services (policy, auth).

### Rule 2: Line Limits — Strict

| Layer | Max lines/method | Max lines/file | Rationale |
|-------|-----------------|----------------|-----------|
| Handler (`grpc/`) | **20 lines** | 300 | Parse, validate, delegate, respond — xong |
| Domain (`domain/`) | **50 lines** | 500 | Nếu dài hơn → tách private helper |
| Store (`store/`) | **30 lines** | 400 | Một method = một query + scan |

Nếu vượt → **PHẢI tách thành private method** trước khi commit.

### Rule 3: Early Return — Max Nesting Depth = 3

```go
// ❌ FORBIDDEN: nesting depth > 3
func handle(ctx context.Context, req Request) error {
    if req.ID != "" {
        item, err := store.Get(ctx, req.ID)
        if err == nil {
            if item.Status == "active" {
                if hasPermission(ctx, item) {  // depth 4 — VIOLATION
                }
            }
        }
    }
}

// ✅ REQUIRED: early return, flat logic
func handle(ctx context.Context, req Request) error {
    if req.ID == "" {
        return domain.ErrInvalidInput
    }
    item, err := store.Get(ctx, req.ID)
    if err != nil {
        return fmt.Errorf("get item: %w", err)
    }
    if item.Status != "active" {
        return domain.ErrInactive
    }
    if !hasPermission(ctx, item) {
        return domain.ErrAccessDenied
    }
    // main logic at depth 1
}
```

**Nếu cần depth > 3 → tách thành function riêng.**

### Rule 4: Domain Errors — Sentinel + Typed

Mỗi service PHẢI có `domain/errors.go`:

```go
package domain

import "errors"

var (
    ErrNotFound     = errors.New("not found")
    ErrAccessDenied = errors.New("access denied")
    ErrAlreadyExists = errors.New("already exists")
    ErrInvalidInput = errors.New("invalid input")
)
```

Handler map domain errors → gRPC codes:

```go
func mapError(err error) error {
    switch {
    case errors.Is(err, domain.ErrNotFound):
        return status.Error(codes.NotFound, err.Error())
    case errors.Is(err, domain.ErrAccessDenied):
        return status.Error(codes.PermissionDenied, err.Error())
    case errors.Is(err, domain.ErrAlreadyExists):
        return status.Error(codes.AlreadyExists, err.Error())
    case errors.Is(err, domain.ErrInvalidInput):
        return status.Error(codes.InvalidArgument, err.Error())
    default:
        return status.Errorf(codes.Internal, "internal: %v", err)
    }
}
```

**Domain KHÔNG import `status` hoặc `codes`. Domain trả domain errors, handler dịch.**

### Rule 5: Handler Pattern — Parse → Validate → Delegate → Respond

Mọi gRPC handler method PHẢI theo template:

```go
func (s *Server) CreateChannel(ctx context.Context, req *pb.CreateChannelRequest) (*pb.Channel, error) {
    // 1. VALIDATE — guard clauses, early return
    if req.Name == "" {
        return nil, status.Error(codes.InvalidArgument, "name required")
    }

    // 2. DELEGATE — single call to domain service
    ch, err := s.svc.CreateChannel(ctx, domain.CreateChannelInput{
        Name:        req.Name,
        WorkspaceID: req.WorkspaceId,
    })

    // 3. MAP ERROR — domain error → gRPC status
    if err != nil {
        return nil, mapError(err)
    }

    // 4. RESPOND
    return ch, nil
}
```

**Handler KHÔNG chứa: SQL, policy calls, for loops, business decisions.**

### Rule 6: Domain Pattern — Orchestrate, Don't Execute

```go
func (s *Service) CreateChannel(ctx context.Context, in CreateChannelInput) (*pb.Channel, error) {
    // 1. Business validation
    if err := s.validateName(in.Name); err != nil {
        return nil, err
    }
    // 2. Policy check (delegate to external service)
    if err := s.checkAccess(ctx, in.UserNodeID, parentOA, ngac.OpWrite); err != nil {
        return nil, err
    }
    // 3. Create NGAC graph nodes
    nodes, err := s.createNGACNodes(ctx, in.Name)
    if err != nil {
        return nil, fmt.Errorf("create NGAC nodes: %w", err)
    }
    // 4. Persist (delegate to store)
    if err := s.store.InsertChannel(ctx, ch); err != nil {
        return nil, fmt.Errorf("insert channel: %w", err)
    }
    return toProto(ch), nil
}
```

**Domain CÓ: business logic, validation, policy calls, error wrapping, orchestration.**
**Domain KHÔNG CÓ: SQL, HTTP parsing, proto request handling.**

### Rule 7: Store Pattern — One Method = One Query

```go
// ✅ ĐÚNG: một method = một SQL operation
func (s *Store) InsertChannel(ctx context.Context, ch *Channel) error {
    _, err := s.db.Exec(ctx,
        `INSERT INTO channels (id, name, type) VALUES ($1, $2, $3)`,
        ch.ID, ch.Name, ch.Type)
    if err != nil {
        return fmt.Errorf("insert channel: %w", err)
    }
    return nil
}

// ❌ SAI: store method chứa logic
func (s *Store) GetActiveItems(ctx context.Context, wsID string) ([]*Item, error) {
    items, _ := s.getAll(ctx, wsID)
    var active []*Item
    for _, item := range items {
        if item.Status == "active" {  // ← LOGIC THUỘC VỀ DOMAIN
            active = append(active, item)
        }
    }
    return active, nil
}
```

**Store CÓ: SQL queries, row scanning, `fmt.Errorf` wrapping.**
**Store KHÔNG CÓ: business logic, if/else decisions, policy calls.**

### Rule 8: Store Models — Separate from Proto

```go
// ✅ store/models.go — internal DB representation
type Channel struct {
    ID          string
    Name        string
    ChannelType string
    WorkspaceID string
    CreatedAt   time.Time
}

// ✅ domain/service.go — conversion helpers
func toProto(ch *store.Channel) *pb.Channel { ... }
func scanChannel(row pgx.Row) (*store.Channel, error) { ... }

// ❌ SAI: dùng proto struct trong store
func (s *Store) Insert(ctx context.Context, ch *pb.Channel) error { ... }
```

### Rule 9: Query Optimization — No N+1, No SELECT *

```go
// ❌ FORBIDDEN: N+1 query pattern
for _, channel := range channels {
    members, _ := store.GetMembers(ctx, channel.ID)  // N queries!
}

// ✅ REQUIRED: batch query
membersByChannel, _ := store.GetMembersByChannelIDs(ctx, channelIDs)  // 1 query

// ❌ FORBIDDEN: SELECT * hoặc fetch-all-then-filter-in-Go
items, _ := store.GetAll(ctx, wsID)
for _, item := range items {
    if item.Status == "active" { ... }  // filtering in Go
}

// ✅ REQUIRED: filter in SQL
items, _ := store.ListByStatus(ctx, wsID, "active")  // WHERE status = $2
```

**Query rules:**
- KHÔNG `SELECT *` — luôn list columns rõ ràng
- KHÔNG query trong loop — dùng `WHERE id = ANY($1)` hoặc JOIN
- Luôn có `LIMIT` cho list queries (default 100)
- Index mọi foreign key và column trong WHERE/ORDER BY
- Cursor-based pagination, KHÔNG offset

### Rule 10: Constants — No Hardcoded Strings

```go
// ❌ FORBIDDEN: hardcoded NGAC strings
s.policyWrite.CreateNode(ctx, &policypb.CreateNodeRequest{
    Name: fmt.Sprintf("%s_Owners", name), NodeType: "UA",
})
s.policyRead.CheckAccess(ctx, &policypb.CheckAccessRequest{
    Operation: "read",
})

// ✅ REQUIRED: use ngac package constants
s.policyWrite.CreateNode(ctx, &policypb.CreateNodeRequest{
    Name: ngac.OwnersUAName(name), NodeType: ngac.TypeUA,
})
s.policyRead.CheckAccess(ctx, &policypb.CheckAccessRequest{
    Operation: ngac.OpRead,
})
```

Shared constants live in `backend/ngac/` package. Include:
- Node types: `TypeUA`, `TypeOA`, `TypePC`, `TypeO`, `TypeU`
- Operations: `OpRead`, `OpWrite`, `OpApprove`, `OpUpload`, `OpShare`, `OpManage`
- Name builders: `PCName()`, `OwnersUAName()`, `MembersUAName()`, etc.

### Rule 11: Constructor Injection — No Global State

```go
// ❌ FORBIDDEN: global variables
var db *pgxpool.Pool

// ❌ FORBIDDEN: init() for setup
func init() {
    db = connectDB()
}

// ✅ REQUIRED: explicit constructor injection
func NewService(store *store.Store, policy PolicyClient) *Service {
    return &Service{store: store, policy: policy}
}
```

All wiring happens in `cmd/main.go`. Domain and Store receive dependencies through constructors.

### Rule 12: Interface at Consumer Side

```go
// ✅ ĐÚNG: Handler khai báo interface nó cần (consumer side)
// internal/grpc/server.go
type ChannelService interface {
    CreateChannel(ctx context.Context, in CreateChannelInput) (*pb.Channel, error)
    ListChannels(ctx context.Context, wsID string) ([]*pb.Channel, error)
}

type MessagingServer struct {
    svc ChannelService  // interface, not concrete
}

// ✅ ĐÚNG: Domain khai báo interface cho store nó cần
// internal/domain/service.go
type ChannelStore interface {
    InsertChannel(ctx context.Context, ch *Channel) error
    GetChannel(ctx context.Context, id string) (*Channel, error)
}

type Service struct {
    store ChannelStore  // interface, not concrete
}
```

**Benefits: testable with mocks, loose coupling, dependency inversion.**

### Rule 13: Function Naming Convention

| Layer | Naming pattern | Examples |
|-------|---------------|----------|
| Store | CRUD verbs | `InsertChannel`, `GetChannel`, `ListByWorkspace`, `UpdateName`, `Delete` |
| Domain | Business verbs | `CreateChannel`, `FindOrCreateDM`, `AddMember`, `SendMessage` |
| Handler | Matches proto RPC | `CreateChannel`, `ListChannels` (mirrors `.proto` service definition) |
| Helpers | Descriptive | `toProto`, `fromProto`, `buildChannel`, `scanChannel`, `mapError` |

### Rule 14: Package Documentation

Mỗi package PHẢI có doc comment ở file đầu tiên:

```go
// Package store handles all database operations for the messaging service.
// It owns the SQL queries and row scanning; no business logic lives here.
package store

// Package domain contains the business logic for the messaging service.
// It orchestrates between the store (database), NGAC policy (access control),
// and external services. No SQL or protobuf parsing lives here.
package domain

// Package grpc provides thin gRPC handlers for the messaging service.
// Each handler parses the request, delegates to the domain layer, and returns the response.
// No SQL, no business logic, no direct policy calls.
package grpc
```

---

## Forbidden Patterns

- ❌ `init()` functions — use explicit constructors
- ❌ `log.Fatal` in library code — only in `main()`
- ❌ `_ = someFunc()` — always handle errors
- ❌ `fmt.Sprintf` with user input in SQL — always parameterized
- ❌ `TODO` comments — fix now or create OpenSpec task
- ❌ ORM (GORM, ent) — use raw SQL with pgx
- ❌ `SELECT *` — always list columns explicitly
- ❌ Query inside a loop — use batch/IN/JOIN
- ❌ Nesting depth > 3 — extract to a function
- ❌ Handler method > 20 lines — you're doing too much
- ❌ Domain importing `status`/`codes` — domain returns domain errors
- ❌ Store with business logic — filter in SQL, not Go
- ❌ `s.db.Query` in handler — all DB access through store layer
- ❌ Hardcoded NGAC strings — use `ngac` package constants
- ❌ Proto types in store layer — use internal models + conversion

## Required Patterns

- ✅ Graceful shutdown with signal handling in every service
- ✅ gRPC interceptors for logging and recovery
- ✅ Health check service registration (`grpc_health_v1`)
- ✅ Connection pool config for database
- ✅ Context propagation to all DB operations
- ✅ `defer rows.Close()` immediately after Query
- ✅ Error wrapping: `fmt.Errorf("operation_name: %w", err)`
- ✅ Conventional Commits for git messages
- ✅ Domain sentinel errors in `domain/errors.go`
- ✅ `mapError()` helper in handler to translate domain → gRPC errors
- ✅ Early return guard clauses — fail fast, happy path last
- ✅ Constructor injection for all dependencies
- ✅ Interface at consumer side for testability
- ✅ Store models separate from proto types

### gRPC Error Code Mapping

| Domain Error | gRPC Code |
|-------------|-----------|
| `domain.ErrInvalidInput` | `codes.InvalidArgument` |
| `domain.ErrNotFound` | `codes.NotFound` |
| `domain.ErrAlreadyExists` | `codes.AlreadyExists` |
| `domain.ErrAccessDenied` | `codes.PermissionDenied` |
| Missing/invalid JWT | `codes.Unauthenticated` |
| `domain.ErrConflict` | `codes.Aborted` |
| Unknown / unexpected | `codes.Internal` |
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
