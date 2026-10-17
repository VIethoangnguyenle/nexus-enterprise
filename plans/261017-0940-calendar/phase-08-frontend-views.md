---
phase: 8
title: "Frontend: lưới, popover, tạo nhanh, kéo thả"
status: pending
priority: P1
effort: "2.5d"
dependencies: [5]
---

# Phase 08: Frontend lõi

## Requirements (render đúng DESIGN.md §6 Lịch + mockup §1, 2, 6, 7)
- `lib/nav-destinations.ts`: module `calendar` "Lịch" sau chat (có cả ở workspace cá nhân); icon lucide `Calendar`; cập nhật
  `nav-destinations.test.ts`, `AppSidebar.test.tsx`, `MainMenuDrawer.test.tsx`.
- Route `routes/_workspace/calendar.tsx`, `validateSearch` {view: day|week|month|agenda|3day, date: yyyy-mm-dd}.
- `api/calendar.ts` (apiFetch), `hooks/useCalendar.ts` + `hooks/keys/calendar.ts`.
- `components/calendar/`: `CalendarScreen` (thanh trên kiểu Google + cột lịch + vùng xem), `MiniMonth`, `CalendarList`
  (Lịch của tôi / Lịch khác, checkbox màu, ⋯ màu/ẩn/chia sẻ), `TimeGrid` (ngày/3 ngày/tuần, hàng cả ngày, vạch giờ hiện tại,
  bố trí trùng giờ), `EventBlock` (đặc / đã qua / chưa trả lời / từ chối / bận), `MonthGrid` (+ "+N nữa"), `AgendaList`,
  `EventPopover` (Popover API, Esc trả focus, RSVP pill), `QuickCreate` (kéo chọn → khối nháp → popover), kéo dời/đổi độ
  dài (pointer events, bước 15 phút, Alt 5 phút, lạc quan + rollback + toast Hoàn tác; sự kiện lặp mở hộp phạm vi), phím
  tắt D W M A X T J K C E Delete Esc.
- `lib/calendar-layout.ts` thuần (cột trùng giờ, vị trí, tuần từ Thứ Hai) + `lib/calendar-format.ts` (date-fns locale vi,
  24h) + unit test.
- Màu lịch: `--color-person-N` / `-wash` / accent; tuân eslint (không hex, không giá trị tuỳ ý).
- Realtime: `lib/realtime.ts` `planFor` domain `calendar` invalidate keys; người khác dời → lớp phủ màu người + nhãn.
- Trạng thái: Skeleton lưới, Empty, Error + Thử lại.

## Related Code Files
- Create: `frontend/src/routes/_workspace/calendar.tsx`, `frontend/src/api/calendar.ts`, `frontend/src/hooks/useCalendar.ts`,
  `frontend/src/hooks/keys/calendar.ts`, `frontend/src/components/calendar/*`, `frontend/src/lib/calendar-*.ts` + tests
- Modify: `frontend/src/lib/nav-destinations.ts`, `frontend/src/hooks/keys/index.ts`, `frontend/src/lib/realtime.ts`,
  `frontend/package.json`

## Success Criteria
- [ ] Vitest: layout trùng giờ, tuần Thứ Hai, popover RSVP gọi API, kéo dời gọi PATCH đúng scope, nav có Lịch.
- [ ] Lint + production bundle xanh; `routeTree.gen.ts` do generator sinh.
