# Remove Gateway — REST per Service with Clean Architecture

## Problem

Gateway service hiện tại là một **monolithic REST proxy** (1,244 lines, 74 handler methods) ôm toàn bộ REST API của platform. Mỗi khi thêm 1 endpoint ở bất kỳ service nào, Gateway phải sửa theo. Đây là bottleneck phát triển và single point of failure.

```
Hiện tại (SAI):
  Frontend ─REST─▶ Gateway (1 monolith) ─gRPC─▶ Services
                   74 handlers             7 services
                   JWT decode
                   JSON↔proto
```

Ngoài ra, hầu hết services đang vi phạm Clean Architecture:
- 5/7 services **không có domain layer** (logic nằm trực tiếp trong gRPC handler)
- 3/7 services **không có store layer** (SQL nằm trong handler)
- Drive handler: 619 lines, nesting depth 6
- Workspace handler: 385 lines, DB queries trực tiếp

## Solution

Xóa Gateway, mỗi service tự expose REST cho client. Đồng thời enforce Clean Architecture cho toàn bộ services.

```
Mới (ĐÚNG):
  Frontend ─REST─▶ Traefik ─route by path─▶ Service REST endpoint
                     │                         ├── rest/    (client-facing)
                     │                         ├── grpc/    (service-to-service)
                     │                         ├── domain/  (business logic)
                     │                         └── store/   (database)
                     │
                     ├── /api/auth/*       → Auth     :8080
                     ├── /api/workspaces/* → Workspace :8080
                     ├── /api/channels/*   → Messaging :8080
                     ├── /api/drive/*      → Drive     :8080
                     └── /api/assets/*     → Asset     :8080
```

## Goals

1. **Xóa Gateway service** — mỗi service tự serve REST + gRPC
2. **Enforce Clean Architecture** — mọi service phải có 4 layers: rest/, grpc/, domain/, store/
3. **JWT middleware shared** — extract thành package dùng chung, không duplicate
4. **Domain-Driven** — mỗi service là 1 bounded context, tự chủ hoàn toàn
5. **Traefik routes** — path-based routing trực tiếp tới từng service

## Non-Goals

- Không thay đổi proto contracts giữa services
- Không thay đổi frontend API calls (giữ nguyên URL paths)
- Không refactor Policy service (đã ổn)
- Không thêm features mới

## Impact

- **Services affected**: auth, workspace, document, messaging, drive, asset, gateway (delete)
- **Frontend**: Không thay đổi — URL paths giữ nguyên
- **Infrastructure**: docker-compose.yml + Traefik labels update
- **Shared code**: Thêm `backend/pkg/httputil/` cho JWT middleware + response helpers
