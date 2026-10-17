---
phase: 5
title: "Sự kiện + lặp lại RRULE"
status: pending
priority: P1
effort: "2d"
dependencies: [4]
---

# Phase 05: Sự kiện + lặp lại

## Overview
CRUD sự kiện, mở rộng lần lặp trong khoảng xem, ngoại lệ, sửa/xoá theo phạm vi, dời bằng kéo thả, sự kiện riêng tư.

## Requirements
- `GET /events?from&to&calendars=…`: các lần (occurrence) trong khoảng của lịch được chọn, mở rộng RRULE trong `tzid` gốc
  (DST đúng), gộp override (`recurring_event_id` + `original_start`), bỏ `cancelled`. Mỗi lần: id sự kiện,
  `occurrenceStart`, giờ, all-day, lịch, tiêu đề, địa điểm, `myResponse`, `hasCall`, `isRecurring`, `recurrenceText`,
  `canEdit`. Lịch chỉ freebusy, hoặc sự kiện `private` mà caller không phải chủ/khách → chỉ `{start, end, busy: true}`.
- `POST /events`, `GET /events/:id`, `PATCH /events/:id?scope=this|following|all&occurrence=…` (kèm `version`; xung đột →
  409), `DELETE /events/:id?scope…`. `this` = tạo/sửa override (xoá = override `cancelled`); `following` = UNTIL rule cũ =
  occurrence − 1s + sự kiện mới từ occurrence (chép khách); `all` = sửa gốc, giữ override đã sửa riêng (trả số giữ nguyên).
- Quyền: tạo/sửa/xoá cần `write` trên OA lịch; đọc cần `read` trên OA lịch hoặc `CalEv` (khách). Validation: end > start;
  rrule parse bằng rrule-go, chỉ FREQ DAILY/WEEKLY/MONTHLY/YEARLY, COUNT ≤ 730, không UNTIL thì cắt mở rộng ở 5 năm; title
  1–200.
- `recurrence_text` Go thuần: "Hằng ngày", "Hằng tuần vào thứ Tư", "Hằng tháng vào thứ Tư thứ hai", "Hằng tháng vào ngày
  14", "Hằng năm vào ngày 14 tháng 10", "Mọi ngày trong tuần (thứ Hai đến thứ Sáu)", INTERVAL ("Mỗi 2 tuần…"), COUNT/UNTIL.
- Xoá sự kiện gốc: xoá OA/UA sự kiện qua policy write (nếu có); phát `event.cancelled` (phase 07).

## Related Code Files
- Create: `backend/services/calendar/internal/domain/{events.go,recurrence.go,recurrence_text.go,occurrences.go}`,
  `store/events.go`, `rest/events.go` + tests

## Implementation Steps
1. TDD `recurrence.go`: hằng tuần qua DST (America/New_York 2026-11-01), hằng tháng thứ Tư thứ hai, override, cancelled,
   cắt `following`, COUNT.
2. Domain events + authz (deny: share view sửa; người lạ đọc chi tiết; private ẩn với share view).
3. Store + REST + flow test 3 phạm vi.

## Success Criteria
- [ ] Unit test recurrence ≥ 10 case, recurrence_text ≥ 8 case xanh; flow test 3 phạm vi sửa và xoá.
- [ ] Caller chỉ freebusy không thấy tiêu đề (test).
