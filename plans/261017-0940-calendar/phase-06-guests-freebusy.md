---
phase: 6
title: "Khách, RSVP, bận/rảnh, tìm giờ, tìm kiếm"
status: pending
priority: P1
effort: "1.5d"
dependencies: [5]
---

# Phase 06: Khách, RSVP, bận/rảnh

## Requirements
- Khách trong `POST/PATCH /events` (`attendees: [userNodeId]`, chỉ người cùng workspace; `inviteChannelId` = mời cả nhóm,
  mở rộng thành viên lúc mời, cần `read` trên Content OA nhóm). Có khách hoặc `hasCall` → `EnsureNode` CalEv (assign dưới
  OA lịch) + CalEvGuests (dưới PC) + association `CalendarGuestOps(guests_can_invite)`; thêm/gỡ khách = assign/deassign;
  đổi cờ invite = đổi association. Người tổ chức là attendee `is_organizer`, `accepted`.
- `POST /events/:id/rsvp` {response, comment, scope this|all, occurrence}: chỉ attendee, `read` trên CalEv.
- `POST /events/:id/guests` (khách có `invite`).
- `ooo`: được mời vào khoảng có sự kiện `ooo` của mình → tự `declined`, comment "Đang vắng mặt".
- `GET /freebusy?users=…&from&to`: mỗi người: khoảng bận (busy, không declined, gồm lần lặp) + giờ làm việc; caller và
  người đó cùng workspace; nếu caller có `read` lịch thì kèm tiêu đề.
- `GET /freebusy/suggest?users&durationMin&day`: ≤ 3 khoảng sớm nhất trong giờ làm việc mọi người rảnh, bước 30 phút.
- `GET /events/search?q`: tiêu đề, địa điểm, mô tả, tên khách trên lịch caller `read` được; ILIKE, ≤ 50.

## Related Code Files
- Create: `domain/{attendees.go,freebusy.go,search.go}`, `store/{attendees.go,freebusy.go}`,
  `rest/{attendees.go,freebusy.go}` + tests

## Success Criteria
- [ ] Deny: không phải khách RSVP → 403; khách không invite thêm khách → 403; freebusy người khác workspace → 403; khách bị
  gỡ đọc sự kiện → 403.
- [ ] Gợi ý giờ đúng với dữ liệu cố định.
