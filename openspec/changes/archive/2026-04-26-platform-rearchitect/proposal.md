## Why

Project NGAC hiện có cấu trúc thư mục rải rác: Go backend code nằm ở 3 nơi khác nhau (`backend/`, `services/`, `proto/`, root `go.mod`). Frontend dùng stack cơ bản (react-router-dom + axios + zustand monolith) không scale tốt cho ứng dụng phức tạp. Cần rearchitect cả hai để đạt clean architecture, developer experience tốt, và sẵn sàng cho growth.

## What Changes

### Backend Restructure
- **Xóa toàn bộ dead code** trong `backend/` — monolith cũ đã được thay thế hoàn toàn bởi microservices trong `services/`
- **Consolidate tất cả Go code** vào `backend/` — move `services/`, `proto/`, root `go.mod`, `go.sum`, `Makefile`, `bin/` vào `backend/`
- **Update tất cả references** — docker-compose build paths, go.mod replace directives, Makefile paths, Dockerfiles

### Frontend Rebuild
- **Greenfield rebuild** frontend với Vite + TanStack Router + TanStack Query
- **Zustand** chỉ giữ cho client-only state (WebSocket connections, UI state)
- **Vanilla CSS** giữ nguyên design system hiện tại
- **Xóa toàn bộ frontend cũ** và rebuild từ đầu với kiến trúc mới

## Capabilities

### Modified Capabilities
- `backend-structure`: Tổ chức lại toàn bộ Go code vào `backend/` với clean directory layout
- `frontend-ui`: Rebuild với TanStack Router (type-safe file-based routing), TanStack Query (server state management), Zustand (UI-only state)
- `realtime-websocket`: Tích hợp WebSocket vào TanStack Query invalidation + Zustand store

## Impact

- **Deleted**: `backend/internal/*`, `backend/cmd/*`, `backend/go.mod`, `backend/go.sum`, `backend/Dockerfile` (dead monolith code)
- **Moved**: `services/` → `backend/services/`, `proto/` → `backend/proto/`, root `go.mod` → `backend/go.mod`
- **Deleted then rebuilt**: Toàn bộ `frontend/src/`
- **Modified**: `docker-compose.yml`, `Makefile`, tất cả service `go.mod` files, tất cả `Dockerfile` files
- **New dependencies**: `@tanstack/react-router`, `@tanstack/react-query`, `@tanstack/router-devtools`
- **Removed dependencies**: `react-router-dom`, `axios`
