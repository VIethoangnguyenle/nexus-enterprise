---
phase: 7
title: "Nhắc giờ, thông báo, realtime, cuộc gọi"
status: completed
priority: P1
effort: "1.5d"
dependencies: [6]
---

# Phase 07: Nhắc giờ, thông báo, realtime, cuộc gọi

## Requirements
- **Redpanda `calendar.events`** (producer như `asset/internal/events`): `event.invited`, `event.updated` (giờ/địa điểm),
  `event.cancelled`, `event.rsvp`, `calendar.shared`, `event.reminder`, `event.created` (có `channelId` khi lịch nhóm).
  Payload: id, workspace, actor node, recipients, tiêu đề, giờ (client dựng câu).
- **messaging consumer** (`internal/events/consumer.go`) tiêu thụ `calendar.events` → `CreateNotification` loại
  `calendar.*`; `event.reminder` và `event.invited` → đẩy OneSignal (`pkg/push`), kiểu như cuộc gọi. Tôn trọng mức thông
  báo hiện có.
- **Nhắc giờ:** goroutine calendar mỗi 30s: lần sắp tới trong 24h có reminder, thời điểm = start − minutes; insert
  `calendar_reminder_deliveries` ON CONFLICT DO NOTHING (chống trùng nhiều bản), phát `event.reminder`. Bỏ người declined.
  Dừng sạch khi shutdown.
- **Realtime:** `realtime.Connect("calendar").Emit(Event{Domain: "calendar", Kind: "event.changed" | "calendar.changed",
  …, UserNodeIDs})` tới người đăng ký lịch + khách; frontend invalidate (phase 08).
- **Cuộc gọi sự kiện (messaging):** `POST /api/calendar-events/:eventId/call` (bắt đầu/tham gia, trả như StartCall); cấp vé
  cho call có `calendar_event_id` kiểm `read` trên `ngac.CalendarEventOAName`. Một cuộc gọi mở mỗi sự kiện. Proxy vite +
  nginx cho `/api/calendar-events` → messaging.

## Related Code Files
- Create: `calendar/internal/events/producer.go`, `calendar/internal/domain/reminders.go` + test
- Modify: `messaging/internal/events/consumer.go`, `messaging/internal/domain/{notifications.go,calls.go}`,
  `messaging/internal/rest/calls.go`, `messaging/internal/store/calls.go`, `frontend/vite.config.js`,
  `deploy/nginx/default.conf`

## Success Criteria
- [x] Reminder không gửi trùng khi chạy 2 lần; declined không nhận.
- [x] Vé cuộc gọi sự kiện: deny khi không phải khách, allow khi là khách.
- [x] Consumer: `event.invited` tạo notification cho từng khách.
