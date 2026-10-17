---
phase: 9
title: "Frontend: trình soạn, tìm giờ, lặp, chia sẻ, điện thoại"
status: completed
priority: P1
effort: "2d"
dependencies: [6, 8]
---

# Phase 09: Trình soạn, tìm giờ, lặp, chia sẻ, điện thoại

## Requirements (mockup §3, 4, 5, 8, 9, 11, 12, 13)
- `EventEditor` (route `calendar.event.$eventId` và `calendar.new`, toàn vùng nội dung; tab Chi tiết · Tìm giờ): giờ +
  Múi giờ, Cả ngày, Select lặp (tính từ ngày đang chọn) + `RecurrenceDialog`, cuộc gọi video, địa điểm, nhiều nhắc, lịch,
  Bận/Rảnh, Riêng tư, mô tả, Khách = Picker người/nhóm (tái dùng picker hiện có) có trạng thái rảnh/bận, cờ "Mời người
  khác" / "Xem danh sách khách", hỏi "Bỏ thay đổi?", Ctrl+S. Đính kèm tài liệu: picker thư mục có sẵn nếu tái dùng được, nếu
  không thì chèn liên kết tài liệu vào mô tả (ghi rõ trong spec).
- `FindTime`: cột mỗi khách, dải chọn kéo được, chip giờ gợi ý.
- `RecurrenceScopeDialog` (this/following/all) cho sửa, kéo, xoá.
- `ShareCalendarDialog`: Picker + 3 mức + hàng cố định "Mọi người trong <workspace> · Thấy bận/rảnh".
- Thêm Lịch khác bằng Picker người; khối bận sọc cho lịch chỉ freebusy.
- Tìm sự kiện (ô tìm / phím /) → kết quả kiểu Lịch biểu.
- Cài đặt lịch: múi giờ, giờ làm việc, nhắc mặc định.
- < 1024: vỏ điện thoại (viên thuốc "Tìm sự kiện", ☰), mặc định Lịch biểu + dải tuần; 3 ngày; sheet chi tiết; nút + nổi;
  trình soạn toàn màn. 768–1023: 3 ngày, sheet phải.

## Success Criteria
- [x] Vitest: editor validate giờ; rrule gửi đúng cho mỗi lựa chọn lặp; scope dialog; share đổi mức gọi API; điện thoại
  render Lịch biểu.
- [x] Không ô nhập ID; không UUID trên màn.
