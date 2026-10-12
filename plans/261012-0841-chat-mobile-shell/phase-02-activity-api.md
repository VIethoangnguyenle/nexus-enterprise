---
phase: 2
title: "API tóm tắt hoạt động theo cuộc trò chuyện"
status: pending
priority: P0
effort: "1-1.5d"
dependencies: [1]
---

# Phase 02: API tóm tắt hoạt động

## Overview
Trang chủ cần xem trước ngay khi mở app. Hiện xem trước chỉ đến từ WebSocket trong phiên
(`frontend/src/stores/websocket.store.ts` `lastMessages`/`lastReplies`). Thêm một endpoint trả, cho **mỗi cuộc trò
chuyện người gọi còn quyền đọc**, tin mới nhất ở luồng chính và trả lời thread mới nhất.

## Requirements
- `GET /api/channels/activity` (tenant từ claims, như `GET /api/channels/unread`). Trả `{"channels": [...]}`, mỗi phần tử:
  `channel_id`, `latest` (tin luồng chính mới nhất), `latest_reply` (trả lời thread mới nhất, **chỉ khi mới hơn
  `latest`**). Mỗi tin: `id`, `sender_id`, `sender_name`, `preview` (văn bản thuần qua `plainText`, cắt ≤ 140 ký tự
  theo rune, không cắt giữa từ nếu được), `kind` (`text` | `attachment` | `poll` | `task` | `card` — lấy từ dữ liệu
  đã có: tệp đính kèm / linked entity; tên tệp đầu tiên khi `attachment`), `mentions` (user id được nhắc), `created_at`;
  `latest_reply` thêm `parent_message_id`.
- Bỏ tin đã xoá (`deleted_at`); tin đã sửa dùng nội dung hiện tại. Kênh chưa có tin: không có phần tử.
- `GET /api/threads/followed?limit=&before=&before_id=` — thread người gọi đang theo dõi trên **mọi** cuộc trò chuyện đọc
  được, hoạt động mới nhất trước (bản xuyên kênh của `ListThreads` filter theo dõi, cùng hình với
  `GET /api/threads/unread`). Dùng cho nút **Thread** ở Trang chủ (chip "Có tin mới" = `/threads/unread`, "Đang theo
  dõi" = endpoint này) và "Thread đang theo dõi" ở tab Thêm. `/api/threads` đã trong proxy dev.
- Quyền: chỉ kênh trong `readableChannelIDs` (cùng đường với unread/mentions/unread-threads). Không thêm nhánh PDP mới,
  nhưng **test deny bắt buộc**: người không là thành viên / bị gỡ khỏi nhóm / DM của người khác → kênh không có trong
  kết quả.
- Một truy vấn cho tất cả kênh (không N+1): `DISTINCT ON (channel_id)` trên `messages` với
  `channel_id = ANY($1)`, `parent_message_id IS NULL`, `deleted_at IS NULL`, `ORDER BY channel_id, created_at DESC`;
  truy vấn thứ hai cho trả lời (`parent_message_id IS NOT NULL`). Kiểm `EXPLAIN` trên DB thật; chỉ khi không dùng
  index (`idx_messages_channel_time`) mới thêm migration index một phần — khi đó backup, `make db-migrate`, kiểm.

## Architecture
transport (`rest/`) → domain `Service.ConversationActivity(ctx, userID, userNodeID, workspaceID)` →
`readableChannelIDs` → store `LatestActivity(ctx, channelIDs)`. Kiểu trả: message proto mới trong
`backend/proto/messaging/messaging.proto` (như `ChannelUnread`), chạy **cả** `make proto` và
`cd frontend && npm run proto:gen`. Không có RPC gRPC mới trừ khi service khác cần.

## Related Code Files
- Modify: `backend/proto/messaging/messaging.proto` (+ generated Go và `frontend/src/generated/`)
- Create: `backend/services/messaging/internal/domain/activity.go` (+ `_test.go`)
- Create: `backend/services/messaging/internal/store/activity.go` (+ `_test.go`, DB test như `threads_test.go`)
- Modify: `backend/services/messaging/internal/rest/handler.go` (route), create `rest/activity.go` (+ test)
- Modify: `frontend/src/api/messaging.ts` (`conversationActivity()`), `frontend/src/hooks/keys/` (key mới)
- Verify: `frontend/vite.config.js` — `/api/channels` đã trỏ 8183; thêm comment nếu cần, không cần entry mới
- Modify: `store/threads.go`, `domain/threads.go`, `rest/threads.go` (+ test) cho `threads/followed`; spec `chat-threads` cập nhật
- Create: `docs/specs/chat-activity-feed/spec.md`; cập nhật `docs/specs/README.md` index

## Implementation Steps
1. Test store trước (DB thật): kênh có tin chính + trả lời mới hơn / cũ hơn; tin xoá; kênh rỗng; tệp đính kèm.
2. Test domain: chỉ kênh đọc được; deny (không thành viên, bị gỡ, DM người khác); preview HTML → văn bản thuần, cắt 140.
3. Proto + generate hai phía. Store, domain, REST. Test REST (401 không claims, 200 shape).
4. Client API + query key (chưa dùng ở UI — phase 05).
5. Spec `chat-activity-feed` (Requirements + Scenarios, có scenario deny).

## Success Criteria
- [ ] `make test s=messaging` xanh, có test deny; `make check-layering`, `make check-ngac`, `make build-check` xanh.
- [ ] Trên stack thật: gọi endpoint với 2 người, gỡ một người khỏi nhóm → nhóm biến mất khỏi kết quả của người đó.
- [ ] `EXPLAIN ANALYZE` ghi vào report, dùng index.
- [ ] Proto sinh cả Go và TS.

## Risk Assessment
- Nhiều kênh (hàng trăm) → `DISTINCT ON` vẫn một lần quét theo index; nếu > 50ms ở dữ liệu thật thì thêm index một phần
  `(channel_id, created_at DESC) WHERE parent_message_id IS NULL AND deleted_at IS NULL` (migration, có backup).
- Lộ nội dung qua xem trước: chặn bằng `readableChannelIDs`; test deny là gate.
