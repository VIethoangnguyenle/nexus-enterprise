---
phase: 4
title: "Đổ chuông, sự kiện realtime, thông báo đẩy"
status: pending
priority: P1
effort: "1d"
dependencies: [3]
---

# Phase 04: Đổ chuông và đẩy

## Requirements
- **WS:** `CallEvent` trong `backend/proto/messaging/ws.proto` (`ServerEnvelope` oneof `call_event = 23`): kiểu
  `ring | cancel | answered | declined | missed | state` + call (id nội bộ, kênh, kind, người gọi tên/avatar, số người,
  started_at). `ring/cancel/answered/declined/missed` gửi tới **mọi thiết bị** của người được gọi (`Hub.sendToUser`)
  và người gọi; `state` gửi theo kênh (`publishToChannel`) để nút "Tham gia"/thẻ cập nhật. `answered` khi người được
  gọi vào phòng (webhook) → thiết bị khác ngừng chuông. Proto hai phía.
- **Đẩy:** `PushIncomingCall` (kind mới) qua `push.Sender` hiện có: tiêu đề tên người gọi, thân "đang gọi video/thoại
  cho bạn", dữ liệu mở `/calls/<id>?answer=video|audio`; hết giờ → đẩy "Cuộc gọi nhỡ" (cùng collapse id thay thế). Tôn
  trọng giờ yên lặng; không đẩy khi thiết bị đang mở app (viewing checker hiện có) — app tự đổ chuông qua WS.
- **Tuỳ chọn:** migration `053_push_incoming_call.sql` thêm cột `incoming_call BOOLEAN NOT NULL DEFAULT TRUE` vào `push_preferences` (backup,
  `make db-migrate`); `GET/PUT /api/me/push-preferences` trả/nhận trường mới.
- Nhóm: không đổ chuông; người được @nhắc trong 2 phút đầu nhận thông báo "X mời bạn vào cuộc gọi" (đường thông báo
  nhắc tên hiện có, kind `mention`).

## Related Code Files
- Modify: `backend/proto/messaging/ws.proto` (+ generated), `internal/grpc/hub_broadcast.go` (`BroadcastCallEvent`),
  `domain/calls.go` (phát sự kiện), `domain/push.go` (kind mới), `store/` push preferences, migration mới
- Modify spec `web-push`, `realtime-event-delivery`, `chat-calls`

## Success Criteria
- [ ] Test: ring tới mọi thiết bị người được gọi; answered làm thiết bị khác nhận `answered`; người ngoài kênh không nhận
  `state` (deny); đẩy tắt khi preference tắt hoặc giờ yên lặng.
- [ ] Proto sinh hai phía; gate xanh.
