---
phase: 9
title: "Mức thông báo, Trang chủ theo thread (đợt 4)"
status: pending
priority: P3
effort: "1.5d"
dependencies: [8]
---

# Phase 09: Mức thông báo, Trang chủ theo thread (đợt 4)

## Overview
Mỗi người chọn mức thông báo cho từng nhóm như Google Chat; Trang chủ ưu tiên thread đang theo dõi có tin mới.

## Requirements
- Mức thông báo theo (người, kênh): **Tất cả** / **Tin chính và thread đang theo dõi** (mặc định) / **Tắt**.
  Áp vào: thông báo (`notifications`), badge chưa đọc, sự kiện WS gửi riêng.
- Trang chủ: mục "Thread có tin mới" phía trên danh sách hội thoại; bấm → mở nhóm + panel thread.
- `GET/PUT /api/channels/:chId/notification-level`; quyền `read` trên OA kênh; test deny.
- Cập nhật spec `notifications`; mockup + DESIGN.md trước khi code (menu mức thông báo ở header nhóm).

## Success Criteria
- [ ] Đặt "Tắt" → không tăng badge/không thông báo từ nhóm đó; "Tất cả" → mọi tin đều tính.
- [ ] Trang chủ hiện thread có tin mới đúng thứ tự `last_reply_at`.

## Risk Assessment
- Thông báo email/đẩy chưa có kênh gửi → chỉ áp cho in-app; ghi rõ trong spec.
