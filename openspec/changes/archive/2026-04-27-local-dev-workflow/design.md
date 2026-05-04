## Context

NGAC platform gồm 8 microservices (policy, policy-read, auth, workspace, document, messaging, asset, drive) + frontend. Hiện tại tất cả chạy trong Docker với Traefik làm reverse proxy. Mỗi lần sửa code backend → rebuild Docker image → restart → chờ health check (~30-60s).

**Hiện trạng cấu hình services:**
- Mỗi service dùng `envOr(key, fallback)` pattern để đọc config
- gRPC ports đã unique (50051-50057, 50061)
- REST ports **tất cả default 8080** → conflict khi chạy native
- Messaging service có thêm WS port 8081
- Thirdparty: PostgreSQL, Redis, Redpanda (Kafka), MinIO

## Goals / Non-Goals

**Goals:**
- `make dev` khởi động toàn bộ platform native trong <10 giây
- Docker chỉ chạy infrastructure (postgres, redis, redpanda, minio)
- Go services chạy trực tiếp trên máy — hỗ trợ Delve debugger, instant restart
- Frontend Vite proxy route trực tiếp tới services (không cần Traefik)
- `make deploy` vẫn hoạt động y như cũ (backward compatible)
- `make dev-stop` dọn dẹp tất cả background processes

**Non-Goals:**
- Hot reload tự động (air/watchexec) — để iteration sau
- Chạy song song Docker full-stack và native dev (sẽ conflict ports)
- Thay đổi production deployment workflow

## Decisions

### 1. Docker Compose profiles (thay vì file riêng)

**Chọn:** Gán `profiles: [app]` cho application services trong `docker-compose.yml`

**Alternatives:**
- `docker-compose.override.yml` — fragile, dễ bị apply ngoài ý muốn
- `docker-compose.dev.yml` riêng — duplicate config, out-of-sync risk

**Rationale:** Docker profiles là built-in, 1 file duy nhất, `docker compose up` (không profile) chỉ start infra, `docker compose --profile app up` start full stack.

### 2. REST port separation qua env override

**Chọn:** `.env.dev` file set `REST_PORT` riêng cho mỗi service

```
# .env.dev — sourced by make dev
AUTH_REST_PORT=8180
WORKSPACE_REST_PORT=8181
DOCUMENT_REST_PORT=8182
MESSAGING_REST_PORT=8183
ASSET_REST_PORT=8184
DRIVE_REST_PORT=8185
```

**Rationale:** Không cần sửa code Go. `envOr` đã hỗ trợ override qua env vars. Makefile set `REST_PORT=818x` khi `go run`.

### 3. Vite proxy routing (thay vì Traefik)

**Chọn:** Cập nhật `vite.config.js` với per-path proxy rules khi dev

```
/api/auth, /api/users     → localhost:8180
/api/workspaces            → localhost:8181
/api/documents             → localhost:8182
/api/channels, /api/messages, /api/dms, /api/threads, /api/notifications
                           → localhost:8183
/api/assets, /api/asset-types, /api/asset-requests
                           → localhost:8184
/api/drive                 → localhost:8185
/api/ws                    → ws://localhost:8081
```

**Rationale:** Vite dev server đã có proxy capability. Không cần chạy Traefik container chỉ để route requests trong dev mode.

### 4. policy-read không cần instance riêng khi dev

**Chọn:** `POLICY_READ_SERVICE_ADDR=localhost:50051` (trỏ về policy chính)

**Rationale:** Policy service đã register cả `PolicyReadService` + `PolicyWriteService`. Chạy thêm instance read riêng khi dev là overhead không cần thiết.

### 5. Background process management bằng PID file

**Chọn:** `make dev` spawn background processes, ghi PIDs vào `.dev-pids`. `make dev-stop` đọc file, kill all.

**Alternatives:**
- `tmux`/`screen` — yêu cầu cài thêm tool
- `foreman`/`overmind` — dependency ngoài
- `Procfile` — cần thêm tool

**Rationale:** Zero dependency, chỉ dùng bash. PID file pattern đơn giản, reliable.

### 6. Expose thêm ports cho infra containers

```yaml
postgres:
  ports: ["5432:5432"]    # hiện tại chưa expose

redpanda:
  ports: ["19092:19092"]  # external Kafka listener, đã cấu hình advertise
```

**Rationale:** Native services cần kết nối postgres/redpanda qua localhost. MinIO (9000, 9001) và Redis (6379) cần expose tương tự nếu chưa.

### 7. Service log files cho Agent debugging

**Chọn:** Mỗi `go run` redirect stdout/stderr ra `.dev-logs/<service>.log`. Agent đọc file trực tiếp, không cần `docker logs`.

```bash
# Trong Makefile make dev
mkdir -p .dev-logs
DATABASE_URL=... REST_PORT=8180 go run ./backend/services/auth/cmd/ \
  > .dev-logs/auth.log 2>&1 &
```

**Targets:**
- `make dev-logs` — `tail -f .dev-logs/*.log` (tất cả services)
- `make dev-logs s=auth` — `tail -f .dev-logs/auth.log` (1 service)
- Agent có thể `cat .dev-logs/auth.log | tail -50` để xem log gần nhất

**Rationale:** AI agents không thể tương tác với `docker logs -f` (interactive stream). File-based logs cho phép agent dùng `cat`, `grep`, `tail` — các tool có sẵn trong toolbox. `.dev-logs/` thêm vào `.gitignore`.

## Risks / Trade-offs

| Risk | Mitigation |
|------|------------|
| Port conflict nếu chạy Docker full-stack + native cùng lúc | Document rõ: `make dev` và `make deploy` **không dùng đồng thời**. `make dev-stop` + `make down` trước khi switch |
| PID file bị stale (crash không cleanup) | `make dev-stop` dùng `kill -0` check trước khi kill. `make dev` check PID file tồn tại → cảnh báo |
| Vite proxy config phải sync với Traefik routes | Document mapping table. Routes ít thay đổi (path-based, stable) |
| Database default port mismatch (5433 vs 5432) | `.env.dev` override `DATABASE_URL` cho tất cả services → trỏ `localhost:5432` |
| Log files grow indefinitely | `make dev` truncate log files khi start. `.dev-logs/` trong `.gitignore` |
