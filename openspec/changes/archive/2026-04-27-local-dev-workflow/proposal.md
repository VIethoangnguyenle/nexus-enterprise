## Why

Hiện tại mỗi lần debug backend phải `make deploy` hoặc `make redeploy s=<service>` — build Docker image, restart container, chờ health check. Chu kỳ này mất 30-60s mỗi lần thay đổi code. Cần một workflow `make dev` chạy tất cả Go services native trên máy (hỗ trợ Delve debugger, instant restart, không build image) trong khi Docker chỉ chạy infrastructure (postgres, redis, redpanda, minio).

**Quan trọng: Agent debugging.** Khi AI coding agents (Gemini, Claude, etc.) cần test và debug code, chúng cần chạy services trên local để xem log stdout/stderr realtime, trace errors, và verify behavior. Docker container logs bị buffer, khó grep, và agent không thể attach debugger. `make dev` cho phép agent chạy `go run`, đọc log trực tiếp, và iterate nhanh.

## What Changes

- **Docker Compose profiles**: Gán `profiles: [app]` cho tất cả application services (policy, auth, workspace, document, messaging, asset, drive, frontend). `make deploy` dùng `--profile app`, `make dev-infra` chỉ start infra.
- **Makefile `dev` targets**: Thêm `make dev` (start infra + tất cả Go services + frontend), `make dev-infra` (chỉ infra), `make dev-stop` (kill background processes).
- **REST port separation**: Mỗi service nhận REST port riêng khi chạy native (auth:8180, workspace:8181, document:8182, messaging:8183, asset:8184, drive:8185) thông qua env override.
- **Vite proxy routing**: Cập nhật `vite.config.js` để route `/api/*` trực tiếp tới từng native service thay vì qua Traefik.
- **`.env.dev` file**: Centralized env config cho dev mode (DATABASE_URL trỏ localhost:5432, service addrs localhost:port).
- **Expose Redpanda external port**: Đảm bảo `ports: ["19092:19092"]` trong docker-compose cho Kafka client kết nối từ host.
- **Service log files**: Mỗi service ghi stdout/stderr ra file log riêng (`logs/<service>.log`). `make dev-logs` tail tất cả, `make dev-logs s=auth` tail một service. Agent có thể đọc file log trực tiếp để debug.

## Capabilities

### New Capabilities
- `local-dev`: Quy trình phát triển local — chạy Go services native, Docker chỉ chạ infrastructure, Vite proxy routing, hot debug workflow.

### Modified Capabilities
_(Không có — thay đổi này chỉ ảnh hưởng tooling/DX, không thay đổi behavior của service nào)_

## Impact

- **docker-compose.yml**: Thêm `profiles: [app]` cho application services, expose Redpanda `19092`, expose PostgreSQL `5432`
- **Makefile**: Thêm targets `dev`, `dev-infra`, `dev-stop`, `dev-logs`, cập nhật `deploy` thêm `--profile app`
- **frontend/vite.config.js**: Thêm per-service proxy routes cho dev mode
- **Tạo mới `.env.dev`**: Environment variables cho local dev
- **Tạo mới `.logs/`**: Directory chứa log files cho mỗi service khi chạy native
- **Không breaking change**: `make deploy` vẫn hoạt động như cũ (full Docker stack)
