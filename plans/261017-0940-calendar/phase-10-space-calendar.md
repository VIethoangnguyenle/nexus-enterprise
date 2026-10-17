---
phase: 10
title: "Lịch nhóm chat + thẻ sự kiện"
status: pending
priority: P2
effort: "1d"
dependencies: [7, 9]
---

# Phase 10: Lịch nhóm chat

## Requirements (mockup §10)
- Tab **Lịch** cạnh Trò chuyện · Tệp · Công việc ở nhóm (không DM): Sắp tới + Tháng, "Tạo sự kiện" (lịch mặc định = lịch
  nhóm, tạo lười), "Mở trong Lịch".
- Sự kiện mới trong lịch nhóm → messaging (consumer `calendar.events` `event.created` có `channelId`) đăng tin hệ thống của
  người tạo mang thẻ sự kiện (theo cơ chế thẻ trong chat, `docs/specs/chat-cards`); thẻ đọc dữ liệu sống từ calendar API
  (giờ, số trả lời, RSVP của tôi); đổi giờ → thẻ cập nhật + tin hệ thống nhỏ; huỷ → thẻ "Đã huỷ".
- "Tạo trong chat" (menu +) thêm mục Sự kiện → QuickCreate với lịch nhóm.
- Sự kiện lịch nhóm có cuộc gọi → Tham gia dùng cuộc gọi của nhóm.

## Success Criteria
- [ ] Test consumer tạo tin có thẻ; test frontend tab Lịch render + RSVP từ thẻ.
- [ ] Rời nhóm → không thấy lịch nhóm (deny ở phase 02/04).
