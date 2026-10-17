---
phase: 1
title: "Thiết kế: DESIGN.md + nav"
status: pending
priority: P1
effort: "0.5d"
dependencies: []
---

# Phase 01: Thiết kế

## Overview
Ghi thiết kế Lịch đã có ở mockup vào `DESIGN.md` (nguồn thiết kế duy nhất) trước khi code.

## Requirements
- DESIGN.md §5: menu chính thêm **Lịch** sau Tin nhắn (rail, drawer ☰). Workspace cá nhân vẫn có Lịch (lịch cá nhân).
- DESIGN.md §6 mục mới "Lịch (mockup `design/mockups/calendar.html`, duyệt 2026-10-17)": thanh trên kiểu Google, cột lịch,
  lưới giờ (1 giờ = 48px, tuần từ Thứ Hai, 24h), khối sự kiện **tô đặc màu lịch + chữ `--color-on-person`**, đã qua = wash +
  chữ ink, chưa trả lời = viền màu lịch, đã từ chối = gạch ngang; vạch giờ hiện tại `--color-danger` (ngoại lệ có chủ đích:
  giống Google); nút pill trong Lịch; màu lịch = 8 person hue (lịch cá nhân mặc định hue của chủ), lịch workspace accent;
  popover chi tiết; tạo nhanh; trình soạn; tìm giờ; lặp; chia sẻ 3 mức; nhắc giờ; lịch nhóm; tablet/điện thoại; trạng thái;
  phím tắt D W M A X T J K C E / Delete.
- §7 thêm dòng chuyển động: khối dời (FLIP 280ms), lớp phủ realtime người khác.

## Related Code Files
- Modify: `DESIGN.md`, `design/mockups/calendar.html` (đã cập nhật quyết định)

## Implementation Steps
1. Viết mục §6 "Lịch" ngắn gọn theo các ghi chú trong mockup (không chép lại toàn bộ).
2. Sửa §5 danh sách mục chính (5 → 6 mục) ở cả rail và drawer.

## Success Criteria
- [ ] DESIGN.md mô tả đủ để code render không cần đoán; ghi rõ ngoại lệ (đỏ, pill, khối đặc) và lý do.
