---
phase: 2
title: "Mô hình dữ liệu và API thread"
status: pending
priority: P1
effort: "2d"
dependencies: [1]
---

# Phase 02: Mô hình dữ liệu và API thread

## Overview
Backend cho tóm tắt thread, theo dõi thread và chưa đọc theo thread. Hiện `messages` chỉ có `reply_count`;
`read_receipts` theo kênh; `GetUnreadCounts` lọc `parent_message_id IS NULL` nên tin trả lời không bao giờ chưa đọc.

## Requirements
- Functional:
  - `messages.last_reply_at` (backfill từ reply hiện có); cập nhật cùng transaction với `reply_count` khi trả lời.
  - Bảng `thread_follows(user_id, root_message_id, following BOOL, last_read_at, updated_at)`, PK `(user_id, root_message_id)`.
  - Tự theo dõi khi: tạo tin gốc (người gửi), trả lời (người trả lời), được nhắc trong tin gốc hoặc trả lời.
    Bỏ theo dõi thủ công phải "dính" (`following=false` không bị tự bật lại khi chính người đó trả lời? → Google
    Chat: trả lời thì theo dõi lại; giữ đúng hành vi này).
  - Tóm tắt thread trong `Message` (cho mỗi tin gốc, theo người gọi): `last_reply_at`, `reply_participant_ids`
    (≤ 3 người trả lời gần nhất, khác nhau), `following`, `thread_unread_count`.
  - API:
    - `POST /api/messages/:msgId/follow`, `DELETE /api/messages/:msgId/follow`
    - `POST /api/messages/:msgId/thread/read` (đặt `last_read_at` của thread = now)
    - `GET /api/channels/:chId/threads?filter=all|following|unread&before=` — thread của kênh, sắp theo `last_reply_at`.
    - `GET /api/channels/unread` thêm `thread_unread` (tổng tin chưa đọc trong thread đang theo dõi) bên cạnh số hiện có.
  - Quyền: mọi endpoint cần `read` trên OA kênh của tin (op từ `backend/ngac`); người không có quyền → 403.
- Non-functional: truy vấn tóm tắt cho một trang tin là **một** câu SQL (không N+1); index
  `messages(parent_message_id, created_at)`, `thread_follows(user_id, following)`.

## Scout (2026-10-10, trước khi code)
- Đã có `thread_participants(message_id, user_id, joined_at)` (`data/init.sql`) và `InsertMessage` ghi vào đó trong
  cùng transaction với `reply_count` (`internal/store/store.go`) → `thread_follows` có thể backfill từ bảng này.
- `GetMessages`/`GetThread` của domain chỉ nhận `userNodeID`; tóm tắt theo người gọi cần thêm `users.id`
  (`claims.UserID`) — đổi chữ ký domain + REST + gRPC (caller từ `grpcauth.CallerFrom`).
- `SendMessage` **không kiểm** `parent_message_id` có cùng kênh với tin gửi → thêm kiểm tra (test deny: trả lời
  vào tin của kênh khác → 400) trong phase này.
- `mentions` là mảng `users.id` (comment proto) → auto-follow khi được nhắc dùng thẳng mảng này.
- `read_receipts` khoá `(user_id, channel_id)`; `GetUnreadCounts` lọc `parent_message_id IS NULL`.
- Mẫu test domain: `internal/domain/authz_test.go` (`scriptedPolicyRead` cấp đúng một bộ (user, OA, op)) + DB thật
  qua `testutil` (`TEST_DATABASE_URL`).
- Công cụ: `protoc` 29.3 + `protoc-gen-go` v1.36.11 + `protoc-gen-go-grpc` v1.6.1 khớp header file sinh hiện có
  (container này đã cài vào `~/.local/bin`, `~/go/bin`; máy khác: `make -C backend proto-install` + protoc 29.3).

## Architecture
Store (`internal/store`) → domain (`internal/domain`, kiểm quyền qua helper kênh có sẵn) → gRPC + REST.
Proto `messaging.proto`: thêm field vào `Message` + RPC follow/unfollow/read/list threads. `make proto`;
`npm run proto:gen` nếu `ws.proto` đổi (phase 03).

## Related Code Files
- Create: `data/migrations/037_thread_follows.sql`
- Modify: `backend/proto/messaging/messaging.proto`
- Modify: `backend/services/messaging/internal/store/store.go` (`messageCols`, `ListMessages`, reply insert),
  `reactions_pins_receipts.go` (`GetUnreadCounts`), new `threads.go`
- Modify: `backend/services/messaging/internal/domain/*` (follow rules, auth), `internal/grpc/*`, `internal/rest/*`
- Modify: `frontend/vite.config.js` (xác nhận `/api/messages`, `/api/channels` đã proxy tới 8183; thêm nếu thiếu)
- Create: `docs/specs/chat-threads/spec.md` + index `docs/specs/README.md`

## Implementation Steps
1. Backup DB; viết migration (cột, bảng, index, backfill `last_reply_at`, backfill follow cho người gửi tin gốc và người đã trả lời); `make db-migrate`; kiểm chứng `\d`.
2. Test đỏ (store, DB thật): tóm tắt thread đúng; đếm chưa đọc thread chỉ cho thread đang theo dõi; auto-follow 3 trường hợp; unfollow rồi trả lời → theo dõi lại.
3. Test đỏ (domain): follow/read/list cho người không có `read` → 403 (deny), có quyền → OK (allow).
4. Hiện thực store → domain → gRPC/REST; proto + `make proto`.
5. Spec `chat-threads` (Requirement + Scenario) cùng commit.

## Success Criteria
- [ ] Migration áp trên DB chạy thật, có backup.
- [ ] `make test s=messaging` xanh, 0 skip; có test deny cho mọi endpoint mới.
- [ ] Một trang 50 tin = 1 truy vấn tóm tắt (kiểm bằng test đếm query hoặc EXPLAIN).
- [ ] Spec `chat-threads` + index cập nhật; drift check xanh.

## Risk Assessment
- Backfill chậm trên bảng lớn → chạy theo lô; tín hiệu: migration > 30s trên DB test → tách backfill thành script.
- `GetUnreadCounts` chậm hơn → đo trước/sau; nếu > 2x, thêm cột đếm cache trong `thread_follows`.
