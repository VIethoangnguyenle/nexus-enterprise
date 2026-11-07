---
phase: 2
title: "Schema seq + backfill (migration 072)"
status: pending
priority: P2
effort: "5h"
dependencies: [1]
---
# Phase 2: Schema seq + backfill

## Mục tiêu
Thêm số thứ tự cho tin chính và reply (cấp bằng trigger `BEFORE INSERT`), vị trí đọc theo seq, và các index cần cho
phase 3. Backfill toàn bộ dữ liệu cũ. `messages.workspace_id` đã bỏ khỏi phạm vi (xem `plan.md`, phần Thiết kế).

## Files
- Tạo `data/migrations/072_message_sequences.sql` (cột, trigger, backfill, trong một transaction) và
  `data/migrations/073_message_sequence_indexes.sql` (index `CONCURRENTLY`, ngoài transaction). **Không sửa** `data/init.sql` hay migration đã có (prod kiểm checksum).
- Sửa `scripts/check-migrations*` chỉ khi script kiểm số thứ tự migration đòi hỏi (kiểm trước).

## Nội dung migration
- `channels.last_seq BIGINT NOT NULL DEFAULT 0`.
- `messages.seq BIGINT` (tin chính), `messages.reply_seq BIGINT` (reply), `messages.last_reply_seq BIGINT NOT NULL DEFAULT 0` (trên gốc).
- Trigger `messages_number` (`BEFORE INSERT`) cấp `seq`/`reply_seq`. `InsertMessageOnce` bỏ `ON CONFLICT DO NOTHING`
  (trigger đã lấy số trước khi thấy xung đột, để lại lỗ); thay bằng giữ dòng channel `FOR UPDATE` rồi kiểm tồn tại.
- `read_receipts.last_read_seq BIGINT NOT NULL DEFAULT 0`, `thread_follows.last_read_reply_seq BIGINT NOT NULL DEFAULT 0`.
- Backfill, idempotent (chỉ chạy trên dòng còn `seq IS NULL`):
  - `seq` = `row_number() OVER (PARTITION BY channel_id ORDER BY created_at, id)` cho tin chính;
  - `reply_seq` tương tự theo `parent_message_id`; `last_reply_seq` = max của gốc; `channels.last_seq` = max seq;
  - `last_read_seq` = max `seq` có `created_at <= last_read_at`; `last_read_reply_seq` tương tự theo `thread_follows.last_read_at`.
- Ràng buộc sau backfill: `CHECK ((parent_message_id IS NULL) = (seq IS NOT NULL))`, `UNIQUE (channel_id, seq)`,
  `UNIQUE (parent_message_id, reply_seq)`.
- Index:
  - `(channel_id, seq DESC) WHERE parent_message_id IS NULL` — trang lịch sử, mọi trạng thái xoá;
  - `(channel_id, seq) WHERE parent_message_id IS NULL AND deleted_at IS NOT NULL` — trừ tin đã xoá khi đếm;
  - `(parent_message_id, reply_seq)` — thread và đếm reply chưa đọc;
  - `(channel_id, created_at DESC) WHERE parent_message_id IS NOT NULL` giữ nguyên; xem xét bỏ `idx_messages_channel`
    (trùng tiền tố với `idx_messages_channel_time`) chỉ khi phase 1 cho thấy không được dùng.
- Index tạo bằng `CREATE INDEX CONCURRENTLY` theo cách migration 050 làm (kiểm `deploy/migrate.sh` có chạy được câu
  CONCURRENTLY ngoài transaction không, như 050).

## Các bước (TDD)
1. Viết test store trước: `store/sequences_test.go` — sau backfill, seq liên tục 1..n theo channel; reply_seq liên tục
   theo gốc; `last_read_seq` khớp `last_read_at`. Chạy, thấy đỏ.
2. Viết migration, `make db-migrate` (dev) và áp vào DB test (`TEST_DATABASE_URL`, ngac_ci trên 5433).
3. Chạy lại migration lần hai: không lỗi, không đổi dữ liệu.
4. Áp vào `ngac_bench`, ghi thời gian backfill 50 triệu dòng vào `reports/baseline.md`.

## Validation
- `make db-migrate` thành công hai lần liên tiếp; `psql -c '\d messages'` có cột và index mới.
- `make test s=messaging` xanh.

## Rủi ro / rollback
Backfill khoá bảng khi chạy `UPDATE` lớn: chạy theo lô 50.000 dòng trong `DO` block. Rollback: các cột mới đều
nullable/có default, code cũ không đọc chúng; bỏ cột bằng một migration mới nếu cần.
