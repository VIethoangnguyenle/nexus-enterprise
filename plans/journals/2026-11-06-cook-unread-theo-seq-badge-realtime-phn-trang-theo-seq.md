---
title: "Cook: unread theo seq, badge realtime, phân trang theo seq"
date: 2026-11-06
summary: "Làm xong phase 2-7 (phase 5 backend để lại), review sửa 11 điểm, đo 50M tin: p95 đếm 308→12 ms"
---

# Cook: unread theo seq, badge realtime, phân trang theo seq

## What happened
Làm plan `plans/261106-2127-message-unread-seq-at-scale` trên nhánh `feat/message-unread-seq` (chưa commit).
- 072/073: trigger cấp `seq`/`reply_seq`, `read_receipts_number` giữ số đọc đúng cho code chỉ biết thời gian; lock_timeout; 073 tự kiểm index INVALID.
- Đếm chưa đọc bằng phép trừ; mention đi từ GIN `mentions` (CTE MATERIALIZED) — chỗ này mới là phần lớn mức giảm: planner cũ BitmapAnd với index channel 1,48 triệu dòng.
- `ConversationActivityEvent` cho thành viên, cache member có epoch; client reducer `lib/unread-activity.ts`.
- Phân trang `before_seq` / `after_reply_seq`; reconnect full jitter.

## Số đo (ngac_bench, 50M tin)
GetUnreadCounts 1 lần gọi mức thường p95 308 → 12 ms; 50 client 24 → 440 lượt/giây. 072 trên 50M dòng: 2 giờ 44 phút khoá bảng.

## Bài học
- Seed 50M trong Docker: `/dev/shm` 64 MB làm hỏng CREATE INDEX và VACUUM song song → tắt parallel.
- `head` sau psql với pipefail ⇒ SIGPIPE exit 141.
- Đã sửa file Go bằng script Python hai lần — vi phạm rule repo; từ đó dùng Serena.

## Next steps
E2E hai người cho badge; đo 5M để chứng minh độc lập số tin; commit theo hygiene (không gồm staged deletions của người dùng).

> Historical work record — not durable authority. Prefer docs/specs/ADRs for current decisions.
