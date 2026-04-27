# Design — Remove Gateway, REST per Service

## Architecture Overview

```
                     ┌────────────┐
                     │  Frontend  │ Vite + TanStack
                     │    :80     │
                     └─────┬──────┘
                           │ HTTP/JSON + WebSocket
                     ┌─────▼──────┐
                     │  Traefik   │ CORS, TLS, path routing
                     │  :80/:8082 │
                     └─────┬──────┘
                           │ route by PathPrefix
        ┌──────────────────┼──────────────────────┐
        │                  │                      │
   ┌────▼────┐        ┌────▼────┐            ┌────▼────┐
   │  Auth   │        │  Msg    │            │  Drive  │
   │REST:8080│        │REST:8080│            │REST:8080│
   │gRPC:50052        │gRPC:50055            │gRPC:50057
   └─────────┘        └─────────┘            └─────────┘
        Each service: rest/ + grpc/ + domain/ + store/
```

## Service Directory Structure (Chuẩn)

```
backend/services/{name}/
├── cmd/
│   └── main.go                 # Wiring: start REST + gRPC servers
├── internal/
│   ├── rest/                   # REST layer — client-facing HTTP/JSON
│   │   ├── handler.go          # Echo router + handlers
│   │   └── middleware.go       # Service-specific middleware (if any)
│   ├── grpc/                   # gRPC layer — service-to-service
│   │   └── server.go           # gRPC handlers
│   ├── domain/                 # Domain layer — pure business logic
│   │   ├── service.go          # Orchestration: store + policy + external
│   │   ├── errors.go           # Sentinel errors
│   │   └── models.go           # Domain input/output structs
│   └── store/                  # Store layer — database access
│       ├── store.go            # SQL queries (one method = one query)
│       └── models.go           # DB row models
├── go.mod
└── Dockerfile
```

## Dependency Flow

```
                  cmd/main.go
                      │ creates & wires
         ┌────────────┼────────────┐
         ▼            ▼            ▼
     rest/handler  grpc/server  domain/service
         │            │            │
         └────────────┘            │
              both call            │
            domain/service         │
                                   ▼
                              store/store

Import rules:
  rest/   → domain/  ✅
  grpc/   → domain/  ✅
  domain/ → store/   ✅
  rest/   → store/   ❌ (never bypass domain)
  grpc/   → store/   ❌
  store/  → domain/  ❌ (never reverse)
```

## Shared Packages

### `backend/pkg/httputil/` — JWT + Echo helpers

```go
package httputil

import "github.com/labstack/echo/v4"

// Claims holds JWT payload extracted from token.
type Claims struct {
    UserID     string
    Username   string
    NGACNodeID string
}

// JWTMiddleware returns Echo middleware that validates Bearer token
// and stores Claims in echo.Context.
func JWTMiddleware(secret string) echo.MiddlewareFunc

// GetClaims retrieves Claims from echo.Context (set by JWTMiddleware).
func GetClaims(c echo.Context) *Claims

// ErrorResponse is the standard error JSON body.
type ErrorResponse struct {
    Message string `json:"message"`
}

// MapDomainError maps a domain sentinel error to an Echo HTTP error.
func MapDomainError(err error) *echo.HTTPError
```

Đây là code hiện tại trong Gateway, tách ra thành shared package.

### `backend/ngac/` — NGAC constants (đã có)

Giữ nguyên, mọi service import từ đây.

## REST Handler Pattern

Mỗi REST handler method PHẢI theo template:

```go
func (h *Handler) CreateChannel(c echo.Context) error {
    // 1. EXTRACT claims from JWT context
    claims := httputil.GetClaims(c)

    // 2. BIND + VALIDATE body
    var body CreateChannelRequest
    if err := c.Bind(&body); err != nil {
        return echo.NewHTTPError(http.StatusBadRequest, err.Error())
    }

    // 3. DELEGATE to domain
    ch, err := h.svc.CreateChannel(c.Request().Context(), domain.CreateChannelInput{
        Name:       body.Name,
        UserID:     claims.UserID,
        UserNodeID: claims.NGACNodeID,
    })

    // 4. MAP ERROR → HTTP status
    if err != nil {
        return httputil.MapDomainError(err)
    }

    // 5. RESPOND
    return c.JSON(http.StatusCreated, ch)
}
```

**Max 20 lines.** Handler KHÔNG chứa SQL, policy calls, hoặc business logic.

## Error Mapping (Domain → HTTP)

```go
// In backend/pkg/httputil/errors.go
func MapDomainError(err error) *echo.HTTPError {
    switch {
    case errors.Is(err, domain.ErrNotFound):
        return echo.NewHTTPError(http.StatusNotFound, err.Error())
    case errors.Is(err, domain.ErrAccessDenied):
        return echo.NewHTTPError(http.StatusForbidden, err.Error())
    case errors.Is(err, domain.ErrAlreadyExists):
        return echo.NewHTTPError(http.StatusConflict, err.Error())
    case errors.Is(err, domain.ErrInvalidInput):
        return echo.NewHTTPError(http.StatusBadRequest, err.Error())
    default:
        return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
    }
}
```

## Traefik Routing

```yaml
# docker-compose.yml labels per service
messaging:
  labels:
    - "traefik.enable=true"
    - "traefik.http.routers.messaging.rule=PathPrefix(`/api/channels`) || PathPrefix(`/api/dms`) || PathPrefix(`/api/messages`) || PathPrefix(`/api/threads`) || PathPrefix(`/api/notifications`)"
    - "traefik.http.routers.messaging.middlewares=cors-all@docker"
    - "traefik.http.services.messaging.loadbalancer.server.port=8080"

drive:
  labels:
    - "traefik.enable=true"
    - "traefik.http.routers.drive.rule=PathPrefix(`/api/drive`) || PathPrefix(`/api/workspaces/{id}/drive`)"
    - "traefik.http.routers.drive.middlewares=cors-all@docker"
    - "traefik.http.services.drive.loadbalancer.server.port=8080"
```

## Migration Strategy (Per Service)

Mỗi service migrate theo 5 bước:

1. **Tạo `domain/errors.go`** — sentinel errors
2. **Tạo/refactor `store/`** — tách SQL từ handler nếu chưa có
3. **Tạo/refactor `domain/service.go`** — tách logic từ handler
4. **Tạo `rest/handler.go`** — copy handler logic từ Gateway, delegate tới domain
5. **Update `cmd/main.go`** — start cả REST + gRPC server
6. **Update docker-compose** — thêm Traefik labels, expose port 8080

## Service Migration Status

| Service | Store | Domain | gRPC handler | REST handler | Priority |
|---------|-------|--------|-------------|-------------|----------|
| Auth | ✅ | ❌ Need | ✅ 168 lines | ❌ Need | P1 (public routes) |
| Messaging | ✅ | ✅ | ✅ 166 lines | ❌ Need | P1 |
| Drive | ✅ | ❌ Need | ⚠️ 619 lines | ❌ Need | P2 |
| Workspace | ❌ Need | ❌ Need | ⚠️ 385 lines | ❌ Need | P2 |
| Asset | ✅ | ✅ partial | ⚠️ 3 files | ❌ Need | P3 |
| Document | N/A | N/A | ✅ 149 lines | ❌ Need | P3 (legacy) |
| Gateway | N/A | N/A | N/A | DELETE | Last |

## `cmd/main.go` Pattern

```go
func main() {
    // Config
    cfg := loadConfig()

    // Dependencies
    db := connectDB(cfg.DatabaseURL)
    store := store.NewStore(db)
    svc := domain.NewService(store, policyClient, ...)

    // gRPC server (for service-to-service)
    grpcServer := grpc.NewServer(...)
    pb.RegisterMessagingServiceServer(grpcServer, grpcHandler.NewServer(svc))

    // REST server (for client) — Echo
    e := echo.New()
    e.Use(middleware.Logger())
    e.Use(middleware.Recover())
    rest.RegisterRoutes(e, svc, cfg.JWTSecret)

    // Start both
    go func() { grpcServer.Serve(grpcLis) }()
    go func() { e.Start(":8080") }()

    // Graceful shutdown
    gracefulShutdown(grpcServer, httpServer)
}
```

## Key Design Decisions

1. **REST và gRPC cùng delegate tới domain** — không duplicate logic
2. **JWT middleware ở shared package** — không mỗi service tự implement
3. **Domain errors là contract** — cả REST và gRPC handler đều map từ domain errors
4. **Traefik routes path-based** — frontend URLs giữ nguyên 100%
5. **Gateway xóa cuối cùng** — sau khi mọi service đã có REST, verify, rồi mới xóa
