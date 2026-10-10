---
phase: 6
title: "Đính kèm như Google Chat (đợt 2)"
status: pending
priority: P1
effort: "2-2.5d"
dependencies: [5]
---

# Phase 06: Đính kèm như Google Chat (đợt 2)

## Overview
Hiện mỗi tệp được tải lên ngay rồi tự gửi thành một tin riêng "📎 tên" với một `linked_entity` duy nhất
(`components/spaces/SpaceView.tsx` `handleFileUpload`). Chuyển sang mô hình Google Chat: tệp **chờ trong ô soạn**,
gửi **cùng** nội dung trong **một** tin, nhiều tệp mỗi tin, ảnh hiện thumbnail.

## Requirements
- Thêm tệp: nút đính kèm (menu: "Tải lên từ máy", "Chọn từ Tài liệu"), **kéo-thả** vào khung hội thoại hoặc
  panel thread, **dán** ảnh (Ctrl/Cmd+V).
- Hàng chờ trong composer: chip/thumbnail mỗi tệp, tiến trình tải, huỷ/bỏ từng tệp, thử lại khi lỗi; nút Gửi bị khoá
  khi còn tệp đang tải; gửi được tệp không kèm chữ, hoặc chữ + tệp trong **một** tin.
- Giới hạn: tối đa 20 tệp mỗi tin; kích thước theo hạn mức drive hiện có; thông báo lỗi nói rõ tệp nào, vì sao.
- Hiển thị trong tin: ảnh → thumbnail (1 ảnh lớn, 2-4 ảnh lưới), bấm mở **trình xem** (lightbox: trái/phải, tải
  xuống, Esc đóng); tệp khác → thẻ (icon theo loại, tên, loại · dung lượng), bấm mở xem trước/tải. Không UUID.
- Áp dụng cả ở luồng chính và trong thread; tab **Tệp** của nhóm liệt kê đúng các tệp này.
- Lưu trữ: tệp vào drive của kênh như hiện nay (giữ đường phân quyền drive).
- Dữ liệu: bảng `message_attachments(message_id, drive_file_id, position, file_name, mime_type, size_bytes,
  width, height)`; proto `Message.attachments` (repeated). Tin cũ có `linked_entity_type='drive_file'` được đọc
  như 1 attachment (không migrate dữ liệu, chỉ ánh xạ khi trả về).
- Quyền (server): gửi tin cần `write` trên OA kênh; mỗi `drive_file_id` đính kèm **phải nằm trong drive của chính
  kênh đó** và đã confirm; sai → 400/403. Test **deny**: đính tệp của kênh khác / workspace khác / tệp chưa confirm.
- Ảnh: kích thước/thumbnail lấy từ ảnh khi tải (client đo `width/height`); không thêm dịch vụ xử lý ảnh mới.

## Related Code Files
- Create: `data/migrations/038_message_attachments.sql`
- Modify: `backend/proto/messaging/messaging.proto` (+ `ws.proto` nếu `ChatMessage` cần attachments) → `make proto` + `npm run proto:gen`
- Modify: `backend/services/messaging/internal/{store,domain,grpc,rest}/*` (send với attachments, kiểm tệp thuộc drive kênh)
- Modify/Create: `frontend/src/components/chat/ChatEditor.tsx`, new `AttachmentTray.tsx`, `AttachmentGrid.tsx`,
  `ImageViewer.tsx`; `frontend/src/components/spaces/MessageBlock.tsx`, `SpaceView.tsx`, `SpaceFiles.tsx`
- Modify: `docs/specs/chat-threads/spec.md` hoặc tạo `docs/specs/chat-attachments/spec.md`

## Implementation Steps
1. Mockup (đã nằm trong phase 01): composer có hàng chờ, lưới ảnh, trình xem — xác nhận đã duyệt.
2. Backup DB → migration → `make db-migrate` → kiểm chứng.
3. Test đỏ backend: gửi 3 tệp + chữ = 1 tin; đính tệp ngoài drive kênh → deny; tin cũ `drive_file` trả 1 attachment.
4. Test đỏ frontend: thêm/bỏ tệp trong hàng chờ; Gửi khoá khi đang tải; dán ảnh tạo chip; lưới 1/2/4 ảnh; trình xem phím trái/phải/Esc.
5. Hiện thực; chụp màn so mockup; gates.

## Success Criteria
- [ ] Gửi chữ + nhiều tệp → đúng 1 tin; người khác thấy realtime kèm thumbnail.
- [ ] Đính tệp không thuộc drive của kênh → bị từ chối (test deny).
- [ ] Tab Tệp hiện tệp từ tin; không UUID; mọi gate xanh.

## Risk Assessment
- Upload thất bại giữa chừng để lại tệp mồ côi trong drive kênh → tệp chưa gửi được xoá khi bỏ khỏi hàng chờ; tệp
  mồ côi do đóng tab: giữ nguyên (drive vẫn quản lý), ghi nhận trong spec.
