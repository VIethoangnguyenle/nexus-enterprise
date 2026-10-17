---
phase: 1
title: "Thiết kế: mockup + DESIGN.md"
status: pending
priority: P1
effort: "1d"
dependencies: []
---

# Phase 01: Thiết kế (duyệt trước code)

## Overview
Vẽ và ghi thiết kế cho mọi thứ plan đổi, theo hướng "Tín hiệu". Không có code UI nào bắt đầu trước khi người dùng duyệt.

## Requirements
- Mockup HTML tĩnh trong `design/mockups/`, dùng `tokens.css`, mỗi màn ở 3 khổ: 390 (điện thoại), 820 (máy tính bảng), 1440 (web).
- DESIGN.md sửa trong cùng PR với code của phần đó.

## Architecture / nội dung
1. **Nguyên tắc tinh gọn** (DESIGN.md §6 đầu mục Components): thao tác nằm trên chính đối tượng (chạm ảnh để đổi,
   ⋯ trên thẻ, menu chuột phải / nhấn giữ); không hàng nút phụ cạnh đối tượng; trên thiết bị không hover thì
   affordance luôn hiện.
2. **`design/mockups/avatar-edit.html`:** AvatarEdit 64 (Hồ sơ) và 64 (panel thông tin nhóm): nghỉ / hover (lớp phủ
   `--color-scrim` + icon máy ảnh 20) / focus ring / menu (Đổi ảnh · Bỏ ảnh) / đang tải (vòng tiến độ trên ảnh) / lỗi
   (dòng `--color-danger` dưới ảnh) / người không có quyền (không lớp phủ). Điện thoại: sheet Chụp ảnh · Chọn ảnh ·
   Bỏ ảnh. Icon nhóm có ảnh ở cột điều hướng, Trang chủ di động, header, tìm kiếm, thông báo (vuông bo 10/12 như hiện).
3. **Ô soạn khi bàn phím mở** (bổ sung vào `design/mockups/chat-mobile.html`): bàn phím đóng = ô bo 22 như hiện,
   cách đáy `max(12px, safe-area-inset-bottom)`; bàn phím mở = thanh tràn ngang, không bo, không margin, viền trên 1px
   `--color-line`, nền `--color-base`, padding 6 × 10. Chuyển trạng thái không animation layout (theo bàn phím).
4. **`design/mockups/drive-gallery.html`:** thanh công cụ (tìm, chip loại, Lưới/Danh sách, sắp xếp); lưới: hàng "Thư mục"
   (ô 48 cao, icon + tên) rồi "Tệp" (thẻ: thumbnail 16:10 radius 10 + ô loại 20 + tên cắt giữa + "Người · thời gian");
   danh sách = bảng hiện có; panel chi tiết (preview lớn, thông tin, hoạt động, Mở / Tải xuống / Chia sẻ); trạng thái
   skeleton, rỗng, lỗi, thumbnail lỗi (ô icon). Máy tính bảng: ngăn kéo cây ☰, lưới 3–4 cột, sheet phải. Điện thoại:
   lưới 2 cột, nút + nổi, bottom sheet, nhấn giữ.
5. **Tab Tệp chat** (cùng file mockup, mục riêng): chip Tất cả · Ảnh & video · Tài liệu · Khác; nhóm Hôm nay / Tuần
   này / Tháng này / Cũ hơn; ảnh/video lưới vuông, còn lại thẻ như Drive.
6. DESIGN.md: mục mới "Tài liệu: lưới, danh sách, panel", "AvatarEdit", sửa "Ô soạn" (điện thoại), "Gom tin" đã sửa.

## Related Code Files
- Create: `design/mockups/drive-gallery.html`, `design/mockups/avatar-edit.html`
- Modify: `DESIGN.md`
- Ghi chú: phần ô soạn khi bàn phím mở và lề phải luồng chat nằm ở `avatar-edit.html` §6–§7 (gom mọi thứ của PR 1
  vào một file để duyệt một lần), không sửa `chat-mobile.html`.

## Implementation Steps
1. Đọc DESIGN.md §2–§7 và mockup `core-screens.html` §2, `file-viewer.html`, `chat-mobile.html` để giữ ngôn ngữ.
2. Vẽ 3 mockup; mỗi trạng thái có chú thích token.
3. Viết các mục DESIGN.md.
4. Gửi người dùng duyệt (đường dẫn + ảnh chụp 3 khổ). Sửa theo góp ý.

## Success Criteria
- [x] Người dùng duyệt `avatar-edit.html` + mục ô soạn (mở khoá PR 1) — duyệt 2026-10-16.
- [x] Người dùng duyệt `drive-gallery.html` + tab Tệp (mở khoá PR 2) — duyệt 2026-10-16.
- [ ] Các mục DESIGN.md viết theo mockup đã duyệt (làm trong PR tương ứng).
- [ ] Không UUID/ID trong mockup; người hiện bằng tên + avatar.

## Risk Assessment
- Thiết kế bị đổi nhiều vòng → tách duyệt theo PR để PR 1 không chờ Drive.
