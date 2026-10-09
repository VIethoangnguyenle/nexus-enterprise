---
phase: 8
title: "Backend shared packages and layering"
status: pending
priority: P2
effort: 3-4d
dependencies: [2]
---

# Phase 08 — Backend shared packages and layering

## Overview
Gỡ trùng lặp giữa 8 service, đưa mọi service về cùng một layering (transport → domain → store),
xử lý lỗi và transaction đúng. Không đổi hành vi ra ngoài.

## Key insights
- Boilerplate `cmd/main.go`: `envOr` ×9, `connectDB` ×7, `recoveryInterceptor` ×8,
  `loggingInterceptor` ×7, `gracefulShutdown` ×4, `connectRedis` ×4, `grpc.NewClient` ×12.
  approval gRPC không có interceptor (`approval/cmd/main.go:70`); drive chỉ có recovery.
  approval đọc `POLICY_ADDR`, các service khác `POLICY_SERVICE_ADDR`.
- Policy-check wrapper copy 6 lần (drive grpc :47, drive domain :40, `asset_server.go:376`,
  `request_server.go:311`, `messaging/internal/domain/service.go:568`, `approval/cmd/main.go:161`).
- `mapGRPCError` local ×4 (drive/asset/document/workspace REST), mỗi bản thiếu code mà
  `httputil.MapGRPCError` xử lý. Domain→gRPC mapper ×4. JWT `Claims` khai báo 2 lần.
- `MapDomainError` và mapper local trả `err.Error()` cho 500 → lộ SQL/chi tiết nội bộ.
- Layering lệch: drive (logic trong `internal/grpc/server.go`, `internal/domain/` là code chết),
  asset (không có domain), workspace (REST gọi gRPC in-process), document (không domain/store),
  messaging (`notification_server.go` chạy SQL ở transport).
- Chỉ có **1** transaction trong toàn codebase (`approval/internal/store/store.go:57`). Lỗi store
  bị bỏ qua: drive `grpc/server.go:360-600` (11 chỗ, `:465` UpdateNGACNodeID sau move), messaging
  `service.go:97,329,392-393,517`, asset `InsertTransition`, approval `execution.go:426` (audit).

## Requirements
- `backend/pkg/policyclient`: Check/BatchCheck, fail-closed, dùng `ngac.Op*`.
- `backend/pkg/grpcutil`: interceptors (recovery+logging cho mọi server), dial, domain→status.
- `backend/pkg/bootstrap`: env, DB, Redis, graceful shutdown. Một tên env cho policy addr
  (giữ alias cũ một release, log deprecation).
- 500 trả message chung + request ID; chi tiết chỉ vào log.
- drive/asset/document có domain layer; REST không gọi gRPC server in-process.
- Mutation nhiều câu lệnh chạy trong transaction; không bỏ qua lỗi store.

## Related files
- create `backend/pkg/{policyclient,grpcutil,bootstrap}/`
- modify `backend/pkg/httputil/{errors,grpcerrors,claims}.go`
- modify `backend/services/*/cmd/main.go`, `backend/services/*/internal/rest/handler.go`
- modify/create `backend/services/{drive,asset,document}/internal/domain/`
- delete `backend/services/drive/internal/domain/service.go` (code chết) sau khi domain mới thay thế
- modify `docker-compose.yml`, `Procfile.dev`, `.env.example` (env name)

## Implementation steps
1. Tạo pkg mới kèm test; chuyển từng service một, mỗi service một commit, `make test s=<svc>` xanh.
2. Test cho 500 sanitize (không chứa chuỗi lỗi gốc).
3. drive → asset → document: tách domain, test domain trước khi di chuyển logic.
4. Transaction + error handling, test lỗi giữa chừng không để lại state nửa vời.
5. Quyết định shard manager (Unresolved Q4).

## Success criteria
- [ ] Không còn `mapGRPCError`/`envOr`/`connectDB` local.
- [ ] Mọi gRPC server có recovery + logging.
- [ ] Không còn `_ =` nuốt lỗi store ở đường ghi.

## Spec
Hành vi không đổi ngoài thông điệp lỗi 500 → ghi vào spec chung nếu có; còn lại không cần spec.
