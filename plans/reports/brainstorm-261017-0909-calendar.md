---
type: brainstorm
date: 2026-10-17
branch: feat/calendar
status: accepted
---

# Brainstorm — Calendar (Google-Calendar-like)

## Summary
Mỗi workspace có Lịch kiểu Google Calendar. Service Go thứ 9 `calendar`; mỗi calendar là một OA; event là row Postgres FK tới OA của calendar. Chọn hướng A.

## Outcome
- View ngày / tuần / tháng (+ lịch nhỏ bên trái), kéo thả để tạo và đổi giờ, chồng nhiều lịch có màu riêng.
- Event: tiêu đề, giờ (có timezone), cả ngày, mô tả, khách, link gọi video (`chat-calls`), lời nhắc.
- Mời khách bằng picker; khách RSVP Có / Không / Có thể; xem free/busy khi chọn giờ.
- Lặp lại theo RRULE (RFC 5545) + ngoại lệ; sửa/xoá theo "lần này / lần này và sau / tất cả".
- Lời nhắc qua `notifications` + `web-push`.
- 3 loại lịch: cá nhân (chia sẻ được), workspace, theo kênh.

## Constraints
- Phân quyền chỉ qua NGAC. Event không phải node; check trên OA của calendar.
- Không thêm op mới: `read` xem, `write` tạo/sửa event, `share` chia sẻ lịch, `manage` quản lý lịch.
- Node name/op từ `backend/ngac` (helper nhận ID type, vd `ngac.CalendarID`); ghi graph qua EPP.
- Lịch kênh: quyền suy ra từ Content OA của kênh (thành viên kênh thấy lịch).
- Khách không có quyền trên lịch: OA "khách" theo event, chỉ tạo khi event có khách → khách được `read` (+ RSVP).
- Route mới khai trong `frontend/vite.config.js` (khối regex nếu nằm dưới `/api/workspaces/:id/`).
- UI không lộ ID; ai cũng chọn bằng picker; audit ghi tên người làm.
- Thiết kế trước: `DESIGN.md` + `design/mockups/calendar.html` được duyệt rồi mới code.
- Schema: migration mới (057+), chạy `make db-migrate` và kiểm trên DB đang chạy.

## Non-goals
- Đồng bộ với Google Calendar thật, import/export ICS.
- Đặt phòng họp / tài nguyên, lịch công khai ra internet, Tasks.

## Acceptance criteria
- Tạo/sửa/xoá event đơn và event lặp, đủ 3 kiểu sửa lặp; test mở rộng RRULE qua đổi DST/timezone.
- Matrix PDP: chủ lịch, người được chia sẻ (read vs write), thành viên kênh, khách được mời, người lạ → test cả nhánh **deny**.
- Bỏ khách khỏi event → khách mất quyền ngay (EPP invalidation, có test).
- RSVP cập nhật realtime cho người tổ chức.
- Nhắc giờ đến đúng hạn qua notification + push.
- Free/busy chỉ trả bận/rảnh, không lộ tiêu đề event mà người xem không có `read`.
- Mockup được duyệt; spec mới `docs/specs/calendar/spec.md` (+ cập nhật spec NGAC liên quan).

## Options considered
| | Hướng | Giả định chịu lực | Hỏng trước khi |
|---|---|---|---|
| **A ✔** | Service `calendar`, calendar = OA, OA khách theo event | OA theo event hợp mô hình (giống Content OA của kênh) | Event có khách quá nhiều làm graph phình → chỉ tạo khi có khách |
| B | Gộp vào `workspace`, quyền khách từ bảng `event_attendees` | Chấp nhận quyền ngoài NGAC | Vi phạm nguyên tắc cốt lõi — loại |
| C | Event là card trong kênh, quyền theo kênh | Không cần lịch cá nhân | Mất lịch cá nhân/free-busy → không "giống Google" — loại |

## Recommendation
Hướng A. Thứ tự đề xuất cho `ak:plan`: (1) mockup + DESIGN.md → (2) spec + model NGAC + test vector → (3) schema + service `calendar` (CRUD, RRULE, RSVP) → (4) free/busy + nhắc giờ → (5) frontend views + kéo thả → (6) lịch kênh + link gọi video.

## Unresolved questions
- Free/busy: xem được của mọi thành viên workspace, hay chỉ người đã chia sẻ lịch? (đề xuất: mọi thành viên, chỉ bận/rảnh)
- Mức chia sẻ lịch cá nhân: chỉ "xem bận/rảnh / xem chi tiết / sửa" như Google? (`read` không tách được bận/rảnh với chi tiết → có thể cần OA phụ)
- Khách ngoài workspace (email): không hỗ trợ đợt này?
- Lịch mở rộng RRULE: thư viện Go (`teambition/rrule-go`) hay tự viết — quyết ở plan.
