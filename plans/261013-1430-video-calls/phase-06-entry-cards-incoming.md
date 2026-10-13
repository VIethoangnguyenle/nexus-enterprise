---
phase: 6
title: "Frontend điểm bắt đầu, thẻ, cuộc gọi đến, cài đặt"
status: completed
priority: P1
effort: "1.5d"
dependencies: [4, 5]
---

# Phase 06: Điểm bắt đầu, thẻ, cuộc gọi đến

## Requirements
- **Thanh đầu cuộc trò chuyện** (máy tính + điện thoại): DM — Gọi thoại, Gọi video; nhóm — "Bắt đầu cuộc gọi" / "Tham
  gia" + pill "Đang gọi · mm:ss" (từ `call_event state` + `GET /api/calls/:id`). Ẩn khi không có quyền gửi tin.
- **Thẻ cuộc gọi** trong `MessageBlock` (đường thẻ chat-cards, `linked_entity_type = call`): đang diễn ra / kết thúc /
  nhỡ / "Mở lại cuộc gọi" khi đang ở trong; xem trước Trang chủ "📹 …".
- **Cuộc gọi đến:** `websocket.store.ts` xử lý `call_event`; máy tính — thẻ nổi `IncomingCallToast` ở layout (mọi màn),
  chuông bằng WebAudio (không file âm thanh), Esc tắt chuông; điện thoại — `IncomingCallScreen` toàn màn; `answered`
  ở thiết bị khác → đóng + "Đã trả lời trên thiết bị khác"; đang trong cuộc gọi khác → "Kết thúc và trả lời".
- **Đẩy:** service worker xử lý bấm thông báo cuộc gọi → mở/focus `/calls/<id>?answer=...` (đường bấm thông báo hiện có).
- **Cài đặt › Thông báo:** Switch "Cuộc gọi đến" (trường `incoming_call` phase 04).
- Test cho từng phần; ảnh Playwright các trạng thái mockup §1, §3.

## Related Code Files
- Modify: thanh đầu cuộc trò chuyện (`components/spaces/SpaceView.tsx` và header DM/phone tương ứng),
  `components/spaces/MessageBlock.tsx` (+ test), `lib/conversations.ts` (xem trước call), `stores/websocket.store.ts`,
  service worker push handler, `components/settings/` thông báo
- Create: `components/calls/IncomingCallToast.tsx`, `IncomingCallScreen.tsx`, `CallCard.tsx`, `lib/ringtone.ts` (+ test)

## Success Criteria
- [ ] Gọi DM: chuông ở mọi thiết bị người kia, trả lời một nơi thì nơi khác ngừng; nhỡ → thẻ + đẩy "nhỡ".
- [ ] Nhóm: thẻ + nút Tham gia cập nhật thời gian thực cho mọi thành viên.
- [ ] Gate frontend xanh.
