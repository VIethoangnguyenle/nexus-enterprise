---
phase: 3
title: "Sự kiện realtime cho thread"
status: pending
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
- [ ] Test hub + reducer xanh; `npm run proto:gen` và `make proto` cùng commit.
- [ ] Hai phiên cùng người: đọc thread ở phiên 1 → phiên 2 về 0 trong ≤ 1s.

## Risk Assessment
- Bão sự kiện ở thread đông người → gộp invalidation theo frame (cơ chế sẵn có); tín hiệu: > 20 refetch/s trong test tải.
