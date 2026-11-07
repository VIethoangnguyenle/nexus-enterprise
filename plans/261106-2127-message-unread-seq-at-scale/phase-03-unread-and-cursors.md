---
phase: 3
title: "Đếm chưa đọc O(1), vị trí đọc theo seq, phân trang theo seq"
status: pending
priority: P2
effort: "10h"
dependencies: [2]
---
# Phase 3: Đếm chưa đọc O(1), vị trí đọc theo seq, phân trang theo seq

## Mục tiêu
Mọi đường ghi cấp seq; mọi vị trí đọc lưu seq; `GetUnreadCounts` không còn `COUNT(*)` trên tin chính và không còn
quét xuyên tenant; lịch sử và thread phân trang theo seq.

## Files (backend/services/messaging)
- `internal/store/store.go`: `insertMessageRow` nhận seq; `InsertMessage` và `InsertMessageOnce` cấp seq trong
  transaction (`UPDATE channels SET last_seq = last_seq + 1 WHERE id = $1 RETURNING last_seq`; reply:
  `UPDATE messages SET last_reply_seq = last_reply_seq + 1 … RETURNING`, gộp vào `recordReply`). `InsertMessageOnce`
  hiện không chạy trong transaction — chuyển sang `pgx.BeginFunc`, và chỉ tăng `last_seq` khi dòng thật sự được ghi.
  Ghi `workspace_id`.
- `internal/store/store.go`: `ListMessages(ctx, channelID, beforeSeq int64, limit)` — `WHERE channel_id=$1 AND
  parent_message_id IS NULL AND seq < $2 … ORDER BY seq DESC`, lọc `(deleted_at IS NULL OR reply_count > 0)` sau index.
  `GetThread(ctx, rootID, afterReplySeq, limit)` trả gốc + trang reply + `has_more`.
- `internal/store/reactions_pins_receipts.go`: `UpsertReadReceipt` ghi `last_read_seq` (= seq của `lastMessageID`,
  hoặc `channels.last_seq` khi trống) và không bao giờ lùi (`GREATEST`), trừ khi "đánh dấu chưa đọc".
  `GetUnreadCounts` viết lại:
  - tin chính: `c.last_seq − rr.last_read_seq − (SELECT count(*) FROM messages d WHERE d.channel_id = c.id AND
    d.parent_message_id IS NULL AND d.deleted_at IS NOT NULL AND d.seq > rr.last_read_seq)`;
  - CTE `followed`: nhánh ngầm định giới hạn `channel_id = ANY($2)`;
  - reply chưa đọc: `r.reply_seq > f.last_read_reply_seq` qua index `(parent_message_id, reply_seq)`;
  - mức `all`: giữ đếm reply nhưng theo seq và index mới;
  - mention: giữ GIN `mentions` + `channel_id = ANY($2)`, so `seq > last_read_seq` thay cho thời gian.
  `PinnedMessageIDsIn` → `PinnedAmong(ctx, msgIDs)` chỉ hỏi các tin của trang.
- `internal/store/lifecycle.go`: `SetChannelReadBefore`/`SetThreadReadBefore` → theo seq (`seq − 1`).
- `internal/store/threads.go`: `MarkThreadRead`, `ThreadStates`, `UnreadThreads`, `FollowedThreads` đọc
  `last_read_reply_seq`.
- `internal/domain/messages.go`, `lifecycle.go`, `reactions_pins_receipts.go`, `threads.go`: nhận/trả cursor mới;
  khi gửi tin, vị trí đọc của người gửi tiến tới seq mới (kiểm hành vi hiện tại trước: nếu hiện nay tin mình gửi
  không tính chưa đọc nhờ client gọi mark-read, giữ nguyên cơ chế đó và ghi lại trong test).
- `backend/proto/messaging/messaging.proto`: `GetMessagesRequest.before_seq`, `GetThreadRequest.after_reply_seq` +
  `limit`, `Message.seq`, `Message.reply_seq`, `MessageList.has_more` cho thread. Giữ `before` (thời gian) một bản phát
  hành, server ưu tiên `before_seq`.
- `internal/rest/*.go` và `frontend/vite.config.js`: chỉ thêm query param, không thêm route (kiểm lại; nếu có route mới
  thì thêm vào proxy và `deploy/nginx/default.conf`).

## Test (viết trước, đỏ rồi xanh)
- `store/unread_seq_test.go`: ma trận đếm — tin mới; tin của chính mình; tin xoá trước khi đọc / sau khi đọc / tin
  chưa đọc bị xoá; "đánh dấu chưa đọc" giữa channel; reply trong thread đang theo dõi / không theo dõi / đã bỏ theo dõi;
  mức `off`/`main`/`all`; mention mới và mention thêm bằng sửa tin; channel chưa từng đọc.
- **Deny**: channel không nằm trong tập đọc được không xuất hiện; tin của tenant khác cùng `sender_id` không làm thay
  đổi số đếm (nhánh `followed` không còn xuyên tenant).
- `store/pagination_seq_test.go`: 3 tin cùng `created_at` ở ranh giới trang không mất/trùng; tin đã xoá có reply vẫn
  hiện; thread 250 reply phân 3 trang.
- `store/sequences_concurrency_test.go`: 50 goroutine gửi vào một channel → seq 1..50 không trùng, không lỗ.
- Test domain hiện có (`edit_mention_unread_test.go`, `threads_test.go`, `activity_test.go`) phải xanh mà không sửa kỳ vọng
  hành vi (chỉ sửa chữ ký hàm).

## Các bước
1. Test → đỏ. 2. Đường ghi seq. 3. Vị trí đọc. 4. `GetUnreadCounts`. 5. Phân trang. 6. Proto: `make proto` **và**
   `cd frontend && npm run proto:gen`. 7. Frontend chỉ đổi kiểu cho tới phase 6 (vẫn gửi `before`).
8. Chạy lại `run.sh` trên `ngac_bench`, ghi số liệu "sau" cho `GetUnreadCounts` và `ListMessages`.

## Validation
- `make test s=messaging`, `make build-check`, `make check-layering`, `make check-ngac` xanh.
- `ngac_bench`: `GetUnreadCounts` < 20 ms p95 (200 channel), chênh < 20% giữa 5 triệu và 50 triệu tin.

## Rủi ro / rollback
Khoá dòng `channels` khi cấp seq: đo bằng test concurrency và pgbench ghi 200 tin/giây vào một channel. Rollback: revert
code; cột seq ở lại vô hại.
