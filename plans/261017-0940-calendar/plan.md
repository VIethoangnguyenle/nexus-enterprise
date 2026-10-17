---
title: "Lịch kiểu Google Calendar"
description: "Service calendar thứ 9 + module Lịch: ngày/tuần/tháng/lịch biểu, kéo thả, khách + RSVP, lặp RRULE, nhắc giờ, bận/rảnh + tìm giờ, chia sẻ lịch, lịch workspace và lịch nhóm chat, cuộc gọi video của sự kiện. Phân quyền hoàn toàn NGAC."
status: in-progress
priority: P1
effort: 12-14d
issue:
branch: feat/calendar
tags: [feature, frontend, backend, ngac, realtime]
blockedBy: []
blocks: []
created: 2026-10-17
---

# Lịch kiểu Google Calendar

Nguồn: [brainstorm](../reports/brainstorm-261017-0909-calendar.md) (hướng A, người dùng chọn: lõi đầy đủ, không đồng bộ
Google, ba loại lịch cá nhân / workspace / nhóm chat). Mockup: `design/mockups/calendar.html` (người dùng yêu cầu "giống
Google Calendar" 2026-10-17; bản kiểu Google). Người dùng cho chạy plan rồi cook không hỏi lại. Nền: `main` @ 0bf4455.

## Mục tiêu (contract)
- **Outcome:** mục **Lịch** trong menu chính. Xem Ngày / Tuần / Tháng / Lịch biểu / 3 ngày; kéo để tạo, kéo để dời/đổi độ dài;
  popover chi tiết; trình soạn đầy đủ (giờ, múi giờ, lặp lại, cuộc gọi video, địa điểm, nhắc, lịch, bận/rảnh, riêng tư, mô
  tả, khách); tìm giờ theo cột khách; lặp RRULE + sửa/xoá "chỉ lần này / lần này và sau / tất cả"; khách RSVP Có/Không/Có
  thể + lời nhắn; nhắc giờ qua thông báo + đẩy; cập nhật realtime; chia sẻ lịch cá nhân (3 mức); xem lịch người khác; lịch
  workspace; lịch của nhóm chat (tab Lịch + thẻ sự kiện trong luồng); điện thoại và máy tính bảng.
- **Constraints:** NGAC là lớp quyền duy nhất; không op mới (dùng `read/write/share/manage/invite`); event không phải node,
  kiểm trên OA của lịch hoặc OA sự kiện; mọi tên node/op từ `backend/ngac` (helper nhận ID type); ghi graph chỉ qua policy
  write gRPC (EPP tự invalidation); route mới vào `vite.config.js` (khối regex) + nginx prod; migration có backup +
  `make db-migrate`; test nhánh deny cho mọi đường quyết định mới; UI không lộ ID, chọn người bằng Picker; transport →
  domain → store, không SQL ở transport; `make check-layering`, `make check-ngac` xanh.
- **Non-goals:** đồng bộ / import / export Google hay ICS; khách ngoài workspace bằng email; đặt phòng/tài nguyên; Tasks;
  lịch công khai; khách sửa sự kiện; nối Vắng mặt với đề nghị nghỉ phép.
- **Acceptance:** xem mục "Tiêu chí hoàn thành" cuối file; mỗi phase có Success Criteria riêng.

## Quyết định đã chốt (2026-10-17)
1. **Bận/rảnh:** người cùng workspace (PC của workspace nằm trong tổ tiên của cả hai, như `workspace.authorizeMember`)
   thấy bận/rảnh của lịch cá nhân; chỉ khoảng giờ, không tiêu đề. Không có mức chia sẻ "bận/rảnh".
2. **Chia sẻ lịch cá nhân:** 3 mức = association từ `User_<node>` (PersonalUA) tới OA lịch: Xem mọi chi tiết `[read]`,
   Sửa sự kiện `[read write]`, Sửa và quản lý `[read write share manage]`.
3. **Khách:** OA sự kiện `CalEv_<id>` nằm dưới OA lịch; UA khách `CalEvGuests_<id>` dưới PC workspace; association
   `[read]` (+`invite` khi "Khách có thể mời người khác"). OA/UA sự kiện chỉ tạo khi sự kiện có khách hoặc cuộc gọi.
   Khách không sửa được sự kiện ở đợt này.
4. **Lịch workspace:** OA `Cal_<id>` dưới PC; `{ws}_Members → [read]`, `{ws}_Owners → [read write share manage]`.
   **Lịch nhóm:** OA `Cal_<id>` dưới `Ch_<id>_Content` → thành viên nhóm thừa hưởng `read write invite`. Chỉ nhóm
   (không DM). **Lịch cá nhân:** OA dưới PC, owner PersonalUA `[read write share manage]`. Tạo lười khi lần đầu cần.
5. **Cuộc gọi của sự kiện:** `calls` thêm `calendar_event_id` (channel_id thành nullable, CHECK đúng một); messaging cấp vé
   sau khi kiểm `read` trên `CalEv_<id>`. Sự kiện trong lịch nhóm dùng luôn cuộc gọi của nhóm.
6. **Thông báo:** calendar phát Redpanda `calendar.events`; messaging consumer tạo notification (+ đẩy cho nhắc giờ và lời
   mời). Realtime: domain `calendar` thêm vào `realtime.Topics()`.
7. **Bảng:** schema `public` có `workspace_id` (như `calls`), không per-tenant schema.
8. **Cổng:** REST 8187, gRPC 50059 (chỉ health). **Thư viện:** Go `github.com/teambition/rrule-go`; frontend `date-fns`
   (chưa có thư viện ngày nào). Câu mô tả quy tắc lặp bằng tiếng Việt do server trả (`recurrence_text`), frontend không
   tự dịch RRULE.

## Phases
| # | Phase | Status | Effort | Depends |
|---|---|---|---|---|
| 01 | [Thiết kế: DESIGN.md + nav](phase-01-design.md) | pending | 0.5d | — |
| 02 | [Spec + mô hình NGAC + test vector](phase-02-spec-ngac.md) | pending | 1d | 01 |
| 03 | [Schema + khung service + đăng ký hạ tầng](phase-03-schema-scaffold.md) | pending | 1d | 02 |
| 04 | [Lịch, đăng ký xem, chia sẻ](phase-04-calendars-sharing.md) | pending | 1.5d | 03 |
| 05 | [Sự kiện + lặp lại RRULE](phase-05-events-recurrence.md) | pending | 2d | 04 |
| 06 | [Khách, RSVP, bận/rảnh, tìm giờ, tìm kiếm](phase-06-guests-freebusy.md) | pending | 1.5d | 05 |
| 07 | [Nhắc giờ, thông báo, realtime, cuộc gọi](phase-07-reminders-notify-calls.md) | pending | 1.5d | 06 |
| 08 | [Frontend: lưới, popover, tạo nhanh, kéo thả](phase-08-frontend-views.md) | pending | 2.5d | 05 |
| 09 | [Frontend: trình soạn, tìm giờ, lặp, chia sẻ, điện thoại](phase-09-frontend-editor-mobile.md) | pending | 2d | 06, 08 |
| 10 | [Lịch nhóm chat + thẻ sự kiện](phase-10-space-calendar.md) | pending | 1d | 07, 09 |
| 11 | [Kiểm chứng + tài liệu](phase-11-verification.md) | pending | 0.5d | all |

## Tiêu chí hoàn thành
- `make build-check`, `make check-layering`, `make check-ngac`, `make test s=calendar`, `make test s=policy`,
  `make test s=messaging`, `cd frontend && npm test && npm run lint && npm run build` xanh (lỗi drift cũ đã biết của
  notification test được ghi rõ, không che).
- Migration áp bằng `make db-migrate` trên DB dev (đã backup) và kiểm bảng.
- Matrix PDP có test deny: người lạ cùng workspace (không được chia sẻ) đọc lịch cá nhân → DENY; người khác workspace →
  DENY; khách bị gỡ → DENY ngay; không phải thành viên nhóm → DENY lịch nhóm; share `read` mà sửa → DENY.
- Smoke thật (dev stack): tạo sự kiện lặp, sửa "lần này và sau", mời khách, khách RSVP, nhắc giờ ra notification, kéo dời
  giờ thấy realtime ở tab thứ hai.
- `docs/specs/calendar/spec.md` + README index; DESIGN.md có mục Lịch.

## Rủi ro chính
- Phạm vi lớn → làm theo phase, mỗi phase commit riêng, build/test xanh trước khi qua phase sau.
- RRULE + múi giờ + DST: lưu `dtstart` kèm `tzid`, mở rộng trong tz gốc; test cố định qua đổi giờ (America/New_York).
- Graph phình vì OA theo sự kiện → chỉ tạo khi có khách/cuộc gọi; xoá sự kiện xoá OA/UA.
