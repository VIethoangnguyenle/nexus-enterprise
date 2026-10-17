---
phase: 2
title: "Spec + mô hình NGAC + test vector"
status: completed
priority: P1
effort: "1d"
dependencies: [1]
---

# Phase 02: Spec + mô hình NGAC

## Overview
Định nghĩa capability `calendar` và mô hình quyền; thêm helper tên/ID vào `backend/ngac`; test vector PDP (allow + deny).

## Requirements
- `docs/specs/calendar/spec.md` (format README: Purpose, `### Requirement: … SHALL`, `#### Scenario` WHEN/THEN): lịch
  (3 loại, tạo lười), chia sẻ 3 mức, sự kiện + lặp + phạm vi sửa, khách + RSVP + mời thêm, bận/rảnh, nhắc giờ, realtime,
  cuộc gọi sự kiện, xoá. Thêm dòng vào `docs/specs/README.md`. Cập nhật `docs/specs/chat-calls/spec.md` (cuộc gọi của sự
  kiện), `docs/specs/realtime-event-delivery/spec.md` (domain `calendar`), `docs/specs/notifications/spec.md` (loại mới).
- `backend/ngac/ids.go`: `CalendarID`, `CalendarEventID`.
- `backend/ngac/ngac_ops.go`: `CalendarOAName(CalendarID)` = `Cal_<id>`, `CalendarEventOAName(CalendarEventID)` =
  `CalEv_<id>`, `CalendarEventGuestsUAName(CalendarEventID)` = `CalEvGuests_<id>`; ops: `CalendarOwnerOps()`
  `[read write share manage]`, `CalendarShareOps(level)` (`view`→`[read]`, `edit`→`[read write]`, `manage`→owner ops),
  `CalendarGuestOps(canInvite bool)` `[read]` (+`invite`). Không op mới. `make check-ngac` xanh.
- Test vector `backend/services/policy/internal/ngac/pdp_calendar_scenarios_test.go` (kiểu `pdp_vnpay_scenarios_test.go`):
  graph PC ws → OA lịch cá nhân / workspace / nhóm, CalEv, CalEvGuests.
  - ALLOW: chủ read/write/share/manage lịch cá nhân; share view read; share edit write; member read lịch workspace; owner
    write lịch workspace; thành viên nhóm read/write lịch nhóm; khách read CalEv; khách có cờ invite → invite.
  - DENY: member khác không được chia sẻ read lịch cá nhân; share view write; member write lịch workspace; người ngoài nhóm
    read lịch nhóm; khách write CalEv; khách không cờ invite → invite; người ở PC khác dù có association (giao PC); khách
    sau khi bị gỡ assignment.

## Related Code Files
- Create: `docs/specs/calendar/spec.md`, `backend/services/policy/internal/ngac/pdp_calendar_scenarios_test.go`
- Modify: `backend/ngac/ids.go`, `backend/ngac/ngac_ops.go` (+ test), `docs/specs/README.md`, `docs/specs/chat-calls/spec.md`,
  `docs/specs/realtime-event-delivery/spec.md`, `docs/specs/notifications/spec.md`

## Implementation Steps
1. TDD: test vector trước (dùng helper mới → compile fail), thêm helper, `make test s=policy`.
2. Viết spec.

## Success Criteria
- [x] `make test s=policy` xanh với ≥ 8 deny case; `make check-ngac` xanh.

## Risk Assessment
- PersonalUA (`User_<node>`, tạo lười như `drive/internal/domain/sharing.go:180-200`) có thể không nằm dưới PC workspace →
  giao PC DENY sai cho chia sẻ. Tín hiệu: test ALLOW "share view read" fail. Phản ứng định sẵn: dùng UA chia sẻ theo lịch
  `CalShare_<calId>_<level>` nằm dưới PC (gán người được chia sẻ vào UA đó), cập nhật spec + helper.
