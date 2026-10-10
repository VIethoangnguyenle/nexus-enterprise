---
phase: 1
title: "Thiết kế Chat (in-line threading, đính kèm, định dạng)"
status: completed
priority: P1
effort: "0.5-1d"
dependencies: []
---

# Phase 01: Thiết kế Chat (in-line threading, đính kèm, định dạng)

## Overview
Cập nhật nguồn thiết kế trước khi code (CLAUDE.md §3): DESIGN.md và `design/mockups/spaces.html` chuyển từ thẻ
chủ đề sang luồng in-line như Google Chat. Người dùng duyệt mockup là cổng vào phase 04.

## Requirements
- Luồng chính: tin theo thời gian, **gom** tin liên tiếp cùng người trong ≤ 5 phút (ẩn avatar/tên dòng sau),
  vạch ngày, vạch **"Tin mới"** tại tin chưa đọc đầu tiên, nút nổi "Nhảy tới tin chưa đọc" / "Tin mới nhất".
- Dưới mỗi tin có thread: hàng tóm tắt — avatar tối đa 3 người trả lời gần nhất, "N trả lời", giờ trả lời cuối,
  pill "M mới" khi thread đang theo dõi có tin chưa đọc.
- Hover actions trên tin: thả cảm xúc, **Trả lời trong thread**, ghim, thêm (menu). Đợt 3 bổ sung sửa/xoá/trích dẫn.
- Panel thread (phải): tin gốc, vạch "Tin mới" trong thread, nút **Theo dõi / Đang theo dõi**, composer "Trả lời".
- Panel **Thread** của nhóm (nút ở header): danh sách thread, lọc **Tất cả / Đang theo dõi / Chưa đọc**, mỗi mục:
  tin gốc rút gọn, người trả lời, số trả lời, giờ cuối, số mới.
- DM dùng cùng mô hình.
- **Ô soạn tin (cho phase 06, 07):** nút đính kèm (menu Tải lên / Chọn từ Tài liệu), vùng thả tệp khi kéo vào,
  hàng chờ tệp (chip + thumbnail, tiến trình, bỏ, thử lại), nút "A" mở thanh định dạng (Đậm, Nghiêng, Gạch chân,
  Gạch ngang, danh sách chấm/số, Link, Code, Khối code, Trích dẫn), hộp chèn link.
- **Hiển thị đính kèm:** 1 ảnh lớn, 2-4 ảnh lưới, thẻ tệp; trình xem ảnh (trái/phải, tải xuống, Esc).
- Trạng thái: đang tải, rỗng ("Chưa có thread nào"), lỗi; giảm chuyển động; light + dark.
- Motion: thread mới / trả lời mới của người khác dùng wash màu người gửi (DESIGN.md §7), không đổi token.

## Related Code Files
- Modify: `DESIGN.md` (mục chat/realtime: luồng in-line, tóm tắt thread, vạch tin mới, panel Thread)
- Modify: `design/mockups/spaces.html` (§2 viết lại; thêm §Thread panel; demo realtime: trả lời thread đang theo dõi
  tăng "mới", vạch tin mới, nhảy tới chưa đọc)
- Modify: `design/mockups/tokens.css` chỉ khi cần token mới (tránh)

## Implementation Steps
1. Đọc DESIGN.md, `spaces.html`, `components/spaces/TopicStream.tsx`, `MessageBlock.tsx`, `ThreadPanel.tsx` để giữ thứ đã duyệt.
2. Sửa DESIGN.md: thay mô tả "topic card" bằng in-line; thêm quy tắc gom tin, vạch tin mới, tóm tắt thread, panel Thread.
3. Viết lại mockup §2 + thêm panel Thread; demo bấm được; kiểm tra light/dark, reduced motion, 390px.
4. Publish mockup (Artifact) để người dùng duyệt; ghi nhận góp ý; chốt.

## Success Criteria
- [x] DESIGN.md mô tả đủ hành vi in-line, không còn mô tả topic card.
- [x] Mockup có luồng in-line, panel thread có Theo dõi, panel Thread có 3 bộ lọc, vạch tin mới + nút nhảy.
- [x] Mockup có ô soạn với hàng chờ tệp, thanh định dạng, hộp link; lưới ảnh và trình xem.
- [x] Người dùng duyệt mockup (2026-10-10, phiên bản 7 của artifact `spaces.html`).

## Kết quả
Mockup `design/mockups/spaces.html` (§1–§9) và DESIGN.md "Chat: …" gồm cả phần thêm theo yêu cầu trong lúc duyệt:
tin hai bên (bong bóng), mục trong cột điều hướng (phase 10), menu chính thu gọn (rail 64px), tạo trong chat —
bình chọn/công việc/tài liệu/phê duyệt (phase 11), sửa/xoá tin (phase 08, chuyển lên đợt 2).

Quyết định thiết kế chốt cùng lần duyệt (theo đề xuất, người dùng duyệt không đổi):
1. Trả lời trong thread không theo dõi: không phủ màu, không "mới", chỉ đổi số.
2. Header chật khi mở panel: "Thêm người" thu thành icon.
3. Tệp tải lỗi khoá nút Gửi tới khi thử lại hoặc bỏ tệp.
4. Trình xem ảnh theo theme (không thêm token nền tối).
5. Không có "Đồng thời gửi vào nhóm".

## Risk Assessment
- Người dùng muốn giữ thẻ chủ đề cho một số nhóm → tín hiệu: góp ý ở bước duyệt; phản ứng: dừng, hỏi lại trước khi phase 02.
