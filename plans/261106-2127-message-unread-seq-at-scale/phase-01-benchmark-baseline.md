---
phase: 1
title: "Benchmark harness + số liệu nền"
status: pending
priority: P2
effort: "6h"
dependencies: []
---
# Phase 1: Benchmark harness + số liệu nền

## Mục tiêu
Có bộ dữ liệu quy mô Mid chạy lại được và số liệu "trước", để mọi phase sau so sánh. Đồng thời xác nhận hoặc bác các
suy luận về planner trong brainstorm (partial index không dùng được, nhánh `followed` quét xuyên tenant).

## Files
- Tạo `scripts/bench/messages/seed.sql`: sinh dữ liệu bằng `generate_series`, tham số `:tenants`, `:messages`
  (mặc định Mid: 2.000 tenant, 50 triệu tin; phân phối lệch: 1 tenant 5.000 người, phần lớn tenant 20–200 người;
  1 channel "toàn công ty" 5.000 thành viên với 1 triệu tin; 5% tin là reply; 1% tin bị xoá; 2% tin có mention).
  Chỉ ghi các bảng cần cho truy vấn tin nhắn: `users`, `channels`, `channel_members`, `messages`, `read_receipts`,
  `thread_follows`, `channel_notification_levels`, `ngac_nodes` tối thiểu cho FK.
- Tạo `scripts/bench/messages/queries/*.sql`: bản sao tham số hoá của `ListMessages`, `GetUnreadCounts`,
  `LatestActivity`, `SearchAllMessages` (dạng pgbench, biến `:user_id`, `:channel_ids`).
- Tạo `scripts/bench/messages/run.sh`: tạo database `ngac_bench` trong container postgres dev (không phải `ngac_ci`,
  không phải `ngac`), áp `data/init.sql` + `data/migrations/*`, chạy seed, `ANALYZE`, rồi chạy `EXPLAIN (ANALYZE,
  BUFFERS)` và `pgbench -c <viewers> -T 60` cho từng truy vấn; ghi kết quả vào `plans/261106-2127-message-unread-seq-at-scale/reports/baseline.md`.
- Tạo `backend/services/messaging/internal/grpc/hub_bench_test.go`: `BenchmarkBroadcastToChannel` với 1.000 client cục
  bộ, đo thời gian và cấp phát mỗi tin.
- Thêm target `make bench-messages` gọi `run.sh` (không nằm trong `make test`).

## Các bước
1. Kiểm dung lượng đĩa (cần ~60 GB; máy hiện còn 114 GB) và ghi vào report.
2. Viết seed, chạy ở `:messages=5000000` trước để thử, rồi 50.000.000.
3. Chạy `EXPLAIN` từng truy vấn; ghi plan, thời gian, buffers.
4. pgbench mô phỏng bão: `GetUnreadCounts` với `-c 1000` (1.000 người mở cùng channel), mỗi người 200 channel.
5. Chạy hub benchmark: `cd backend/services/messaging && go test -run '^$' -bench BroadcastToChannel ./internal/grpc/`.
6. Ghi `reports/baseline.md`: bảng số liệu + kết luận cho từng giả thuyết trong brainstorm (đúng/sai, kèm plan).

## Validation
- `bash scripts/bench/messages/run.sh --messages 5000000` chạy hết không lỗi, tạo `reports/baseline.md`.
- `make check-layering` và `make build-check` vẫn qua (chỉ thêm file test và script).

## Rủi ro / rollback
Chỉ thêm script và database riêng. Rollback: `DROP DATABASE ngac_bench`. Không đụng `ngac_ci` hay dữ liệu dev.
