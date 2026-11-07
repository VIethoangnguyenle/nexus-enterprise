---
phase: 6
title: "Frontend: cursor seq, giới hạn cache lịch sử, phân trang thread"
status: pending
priority: P2
effort: "4h"
dependencies: [3]
---
# Phase 6: Frontend lịch sử và thread

## Mục tiêu
Client phân trang bằng `before_seq`, thread tải theo trang, và cache lịch sử một channel không phình vô hạn.

## Files
- `frontend/src/api/messaging.ts`: `listMessages(channelId, beforeSeq?)`, `getThread(messageId, afterReplySeq?, limit?)`.
- `frontend/src/hooks/useThreads.ts`: `useLoadOlderMessages` dùng seq của tin cũ nhất đã lưu (bỏ `timestampToRfc3339`);
  hook tải thêm reply cho thread.
- Cắt cache: khi rời channel (unmount `SpaceView`) chỉ giữ 2 trang mới nhất (100 tin) trong `keys.messaging.messages(id)`,
  đặt `has_more = true`. Kiểm `lib/query-persist.ts` không persist danh sách đã cắt sai.
- Component thread (tìm qua `keys.messaging.thread(`): nút/scroll "tải reply cũ hơn".

## Test (viết trước)
- Vitest: `useLoadOlderMessages` gửi `before_seq` của tin cũ nhất không phải tin tạm; không nhân đôi tin; cắt cache giữ
  đúng 100 tin mới nhất và `has_more`.
- Vitest: thread 250 reply tải 3 trang đúng thứ tự.

## Validation
`cd frontend && npm test && npm run lint && npm run typecheck:diff && npm run build`.

## Rủi ro / rollback
Đổi hành vi cuộn: không đổi UI (theo `DESIGN.md`, chỉ đổi nguồn dữ liệu). Rollback: revert; server vẫn nhận `before`
thời gian trong một bản phát hành.
