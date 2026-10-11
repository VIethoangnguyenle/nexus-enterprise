---
phase: 3
title: "Sự kiện realtime cho thread"
status: completed
priority: P1
effort: "1d"
dependencies: [2]
---

# Phase 03: Sự kiện realtime cho thread

## Overview
Đưa trạng thái thread qua WebSocket: tóm tắt thread cập nhật cho mọi người đang xem kênh; số "mới" tăng cho
người theo dõi; trạng thái theo dõi/đã đọc đồng bộ giữa các thiết bị của cùng người.

## Requirements
- `ThreadReplyEvent` (đã có) mang thêm tóm tắt mới: `reply_count`, `last_reply_at`, `reply_participant_ids`.
- Sự kiện mới `ThreadStateEvent {root_message_id, channel_id, following, unread_count}` gửi **riêng** cho người
  dùng đó (mọi phiên): khi theo dõi/bỏ theo dõi, đọc thread, hoặc có trả lời mới trong thread họ theo dõi.
- Người không theo dõi nhận tóm tắt cập nhật (qua kênh) nhưng không nhận `ThreadStateEvent` tăng chưa đọc.
- Giữ phạm vi tenant/kênh đã có (`resource-pep-coverage`, `realtime-event-delivery`).

## Related Code Files
- Modify: `backend/proto/messaging/ws.proto`; `make proto` + `cd frontend && npm run proto:gen`
- Modify: `backend/services/messaging/internal/grpc/hub.go` (phát theo user), domain reply path
- Modify: `frontend/src/stores/websocket.store.ts` (reducer → key factory `hooks/keys`)
- Modify: `docs/specs/realtime-event-delivery/spec.md`

## Implementation Steps
1. Test đỏ (hub): người theo dõi nhận `ThreadStateEvent` tăng; người không theo dõi chỉ nhận tóm tắt; tenant khác không nhận gì.
2. Test đỏ (frontend reducer): sự kiện → cập nhật/invalidate đúng query key (tin, thread, threads list, unread).
3. Proto + regenerate cả hai phía; hiện thực; cập nhật spec.

## Success Criteria
- [x] Test hub + reducer xanh; `npm run proto:gen` và `make proto` cùng commit.
- [x] Hai phiên cùng người: đọc thread ở phiên 1 → phiên 2 về 0 (gửi `thread_state` tới mọi phiên của node;
  kiểm trên stack thật ở phase 05).

## Risk Assessment
- Bão sự kiện ở thread đông người → gộp invalidation theo frame (cơ chế sẵn có); tín hiệu: > 20 refetch/s trong test tải.

## Kết quả (2026-10-11)
- `ws.proto`: `ThreadReplyEvent` thêm tóm tắt (field 3–5); `ThreadStateEvent` mới (field 20 của envelope, có `as_of`).
- Tin trả lời: `thread_reply` tới người đăng ký kênh (tóm tắt chung), `thread_state` riêng cho từng người theo dõi qua
  đường user-node có tenant; follow/unfollow/đọc đẩy trạng thái mới tới mọi phiên của chính người đó và trả về JSON.
- Sau review (`plans/reports/code-reviewer-261011-0722-chat-phase-03.md`): kiểm `read` lại cho từng người nhận (người
  được nhắc không phải thành viên / đã bị gỡ không nhận gì; policy lỗi → không gửi); tenant lấy theo workspace của
  kênh (DM: tenant người gửi); fan-out chạy nền với context riêng + timeout, lỗi vẫn phát tin trả lời trần; client bỏ
  frame cũ (`as_of`, số trả lời không lùi); trả lời vẫn cập nhật xem trước/thứ tự cuộc trò chuyện như trước.
- Chưa làm (thấp): gộp invalidation theo frame; `chatMessageOf` trùng `messageToChatMessage` (khác tầng).
