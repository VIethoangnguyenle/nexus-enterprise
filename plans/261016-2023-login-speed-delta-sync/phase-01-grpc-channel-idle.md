---
phase: 1
title: "Kênh gRPC không idle, kết nối sẵn, tự nối lại"
status: completed
priority: P1
effort: "done"
dependencies: []
---

# Phase 01: Kênh gRPC không idle

## Overview
Chẩn đoán (ak:debug 2026-10-16, brainstorm report §Chẩn đoán): lời gọi policy đầu tiên sau > 30 phút idle chờ ~8 s
(13/13), policy trả lời ≤ 10 ms; DNS và conntrack đã loại.

## Đã làm (nhánh `fix/grpc-channel-idle-stall`, worktree `../nexus-enterprise-wt/grpc-idle`)
- `backend/pkg/grpcauth/interceptors.go`: `Dial` thêm `grpc.WithIdleTimeout(0)`, `conn.Connect()` ngay, goroutine
  `watchChannel` nối lại khi kênh về IDLE (pick_first sau khi mất kết nối), WARN khi CONNECTING→READY > 1 s và khi
  TRANSIENT_FAILURE.
- `backend/pkg/grpcauth/dial_test.go` (viết trước, fail trước): không idle, áp idle timeout, kết nối không chờ lời gọi,
  log kết nối chậm, tự nối lại khi server quay về. `go test -race -count=3` xanh.
- Cổng: `make build-check`, `make check-layering`, 202 test shared, 8 service `go test` — chỉ fail
  `TestNotificationServer_ServesTheCallersOwnNotifications`, fail y hệt trên `origin/main` (lệch schema DB dev, có sẵn).

## Còn lại
- Gom vào commit chung (phase 10). Sau deploy: đếm lại 72 h log, idle > 30 phút & chậm phải về 0; xem log có WARN
  "grpc channel slow to become ready" không.

## Deploy
- Lên production 2026-10-16 22:49 (#20, run 38926146050). Chờ đếm lại sau ≥ 1 khoảng idle > 30 phút.
