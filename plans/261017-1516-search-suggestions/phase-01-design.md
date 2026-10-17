---
phase: 1
title: "Thiết kế: DESIGN.md + mockup"
status: completed
priority: P1
effort: "3h"
dependencies: []
---

# Phase 01: Thiết kế

## Overview
Thiết kế panel gợi ý dùng chung cho 5 module trên điện thoại, máy tính bảng và máy tính, theo hướng "Tín hiệu". Zane
duyệt rồi mới sang phase 02.

## Requirements
- Phải chốt ba điều trong thiết kế:
  1. Trên máy tính, panel là dropdown neo dưới ô tìm hay chiếm cả cột danh sách.
  2. Ô tìm của Phê duyệt đặt ở đâu và tìm theo gì (tiêu đề, người gửi, mẫu).
  3. Mỗi nhóm hiện tối đa bao nhiêu mục.
- Trạng thái: đang tải (skeleton), trống (không có lịch sử, không có gợi ý thì ẩn panel hay hiện một câu), lỗi tải
  (bỏ nhóm đó, không báo lỗi), giảm chuyển động.
- Trên điện thoại giữ nguyên "Tìm gần đây" hiện có của màn Tìm trong chat. Pattern mới phải trông giống nó.

## Related Code Files
- Modify: `DESIGN.md` (thêm mục "Gợi ý trong ô tìm"; mục "Tìm trong chat" trỏ sang mục mới)
- Create: `design/mockups/search-suggestions.html` (dùng `tokens.css`)

## Implementation Steps
1. Đọc `DESIGN.md` (thanh trên điện thoại, Picker, Tìm trong chat) và `design/mockups/chat-mobile-home.html` để nối tiếp
   cách làm đang có.
2. Vẽ mockup: điện thoại 390 và máy tính 1280 cho từng module; bàn phím/focus; trạng thái trống và đang tải.
3. Ghi đặc tả vào DESIGN.md: kích thước, token, hàng, nhãn nhóm, hành vi.
4. Gửi Zane duyệt (`/browse` để chụp ảnh). Ghi ngày duyệt vào DESIGN.md.

## Success Criteria
- [ ] Mockup + DESIGN.md đã được Zane duyệt.
- [ ] Đã chốt ba điều ở mục Requirements.

## Risk Assessment
Nếu Zane muốn ô tìm của Phê duyệt tìm phía máy chủ thì phase 05 phải đổi thành việc backend. Khi đó dừng lại và lên
plan riêng, không nhồi vào plan này.
