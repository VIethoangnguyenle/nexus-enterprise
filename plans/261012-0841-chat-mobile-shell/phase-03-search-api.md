---
phase: 3
title: "API tìm tin trên mọi cuộc trò chuyện"
status: pending
priority: P1
effort: "1-1.5d"
dependencies: [1]
---

# Phase 03: API tìm tin toàn cục

## Overview
Màn Tìm trong chat (mockup §6) tìm tin trên mọi cuộc trò chuyện. Hiện chỉ có `GET /api/channels/:chId/search`, dùng
`search_vector = to_tsvector('english', content)` (migration 004) — sai cho tiếng Việt và lẫn thẻ HTML của tin định dạng.

## Requirements
- `GET /api/messages/search?q=&limit=&before=&before_id=&has_file=` — trang 25, mới nhất trước (kiểu `messageFeed` của
  Mentions/Starred), cả tin chính và trả lời thread (trả `parent_message_id` để mở đúng thread). Chỉ kênh trong
  `readableChannelIDs`; `q` rỗng hoặc < 2 ký tự → danh sách rỗng; `has_file=true` chỉ tin có đính kèm.
- Đổi cột tìm: migration mới `0NN_message_search_simple.sql` — `search_vector` thành
  `to_tsvector('simple', regexp_replace(content, '<[^>]+>', ' ', 'g'))` (drop + add generated column, tạo lại GIN).
  Truy vấn `plainto_tsquery('simple', q)`, từ cuối thêm tiền tố (`:*`) để gõ dở vẫn ra. **Sửa luôn**
  `Store.SearchMessages` (tìm trong một nhóm) sang cùng cấu hình qua một helper chung — nếu không nó hỏng khi cột đổi.
- Tìm không dấu ("doi soat" ra "đối soát"): **ngoài phạm vi** trừ khi `unaccent` có sẵn và bọc được IMMUTABLE; ghi
  vào spec như giới hạn đã biết.
- Test deny: tin trong nhóm người gọi không còn quyền đọc không bao giờ xuất hiện.

## Related Code Files
- Create: `data/migrations/0NN_message_search_simple.sql` (số kế tiếp sau `049_edit_mention_times.sql`)
- Modify: `backend/services/messaging/internal/store/reactions_pins_receipts.go` (`SearchMessages`), create
  `store/search.go` (+ DB test)
- Modify/Create: `domain/search.go` (+ test), `rest/stars.go` hoặc `rest/search.go` (+ test), `rest/handler.go` route
- Modify: `frontend/src/api/messaging.ts` (`searchAll`), keys
- Verify: `frontend/vite.config.js` — `/api/messages` đã trỏ 8183 và đứng trước `/api/me`
- Create: `docs/specs/chat-search/spec.md`; README index

## Implementation Steps
1. **Backup DB** (`pg_dump`), viết migration, `make db-migrate`, kiểm `\d messages` và `EXPLAIN` dùng GIN.
2. Test store: tiếng Việt có dấu, tin HTML (không khớp tên thẻ), tiền tố, has_file, phân trang; deny ở domain.
3. Store helper chung, endpoint mới, sửa tìm trong nhóm; chạy lại test cũ của tìm trong nhóm.
4. Client API. Spec `chat-search`.

## Success Criteria
- [ ] Migration áp dụng trên DB thật, kiểm bằng truy vấn mẫu tiếng Việt; backup lưu đường dẫn trong report.
- [ ] Test deny xanh; test tìm trong nhóm cũ vẫn xanh.
- [ ] `make test s=messaging`, `check-layering`, `check-ngac`, `build-check` xanh.

## Risk Assessment
- Đổi generated column ghi lại cả bảng `messages` — chạy ở dev nhanh; với prod ghi vào `docs/deployment.md` là migration
  ghi lại bảng (khoá ngắn). Rollback: migration ngược về cấu hình `english` (giữ trong report).
- Nếu người dùng quyết định để tìm toàn cục sau: bỏ phase này và 06, Chat pill rơi về tìm người/nhóm phía client.
