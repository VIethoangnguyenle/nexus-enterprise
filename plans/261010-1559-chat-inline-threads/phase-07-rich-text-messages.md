---
phase: 7
title: "Soạn tin định dạng HTML (đợt 2)"
status: pending
priority: P1
effort: "1.5d"
dependencies: [5]
---

# Phase 07: Soạn tin định dạng HTML (đợt 2)

## Overview
Composer (TipTap StarterKit trong `components/chat/ChatEditor.tsx`) đã sinh HTML nhưng không có thanh định dạng,
thiếu gạch chân và link; server lưu HTML **không lọc** (chỉ client lọc khi render bằng DOMPurify). Đưa về cơ chế
Google Chat và lọc ở server.

## Requirements
- Thanh định dạng bật/tắt bằng nút "A" (như Google Chat): **Đậm, Nghiêng, Gạch chân, Gạch ngang, Danh sách
  chấm, Danh sách số, Link, Code, Khối code, Trích dẫn**. Trạng thái nút phản ánh vùng chọn.
- Phím tắt: Ctrl/Cmd+B/I/U, Ctrl/Cmd+Shift+X (gạch ngang), Ctrl/Cmd+K (link); cú pháp gõ nhanh `*đậm*`,
  `_nghiêng_`, `~gạch~`, `` `code` ``, ` ``` ` khối code, `- ` / `1. ` danh sách.
- Enter gửi, Shift+Enter xuống dòng; trong danh sách/khối code Enter tạo dòng mới.
- Link: hộp nhập văn bản + URL; chỉ http(s)/mailto; mở tab mới `rel="noopener noreferrer"`.
- Gửi `content_format: "html"`; **server lọc allowlist** (Go, ví dụ bluemonday): thẻ `p, br, strong, b, em, i, u,
  s, del, code, pre, blockquote, ul, ol, li, a[href], span[data-mention]`; bỏ mọi thứ khác; giới hạn độ dài.
  Đồng thời lưu bản **văn bản thuần** (strip tags) cho tìm kiếm, thông báo, xem trước Trang chủ.
- Nhắc tên giữ là node có cấu trúc (`data-mention`), không dựa regex `@\w+` trên HTML.
- Hiển thị: giữ DOMPurify ở client (hai lớp); kiểu chữ theo DESIGN.md (không màu tuỳ ý, không font tuỳ ý).
- Test **deny**: gửi `<script>`, `onerror`, `javascript:`, `style` → server lưu đã lọc; client vẫn lọc.

## Related Code Files
- Modify: `frontend/src/components/chat/ChatEditor.tsx`; Create: `FormattingToolbar.tsx`, `LinkDialog.tsx` (+ test)
- Add deps: `@tiptap/extension-underline`, `@tiptap/extension-link` (phiên bản khớp TipTap đang dùng)
- Modify: `frontend/src/components/chat/MessageContent.tsx` (mention node, bỏ regex highlight trên HTML)
- Modify: `backend/services/messaging/internal/domain/*` (sanitize + plain text), `go.mod` của messaging
- Create: `data/migrations/039_message_plain_text.sql` (cột `content_text`, backfill strip tags)
- Modify: `SearchMessages` dùng `content_text`
- Spec: `docs/specs/chat-threads/spec.md` (mục định dạng) hoặc spec riêng `chat-rich-text`

## Implementation Steps
1. Mockup (phase 01): thanh định dạng mở/đóng, trạng thái nút, hộp link — xác nhận đã duyệt.
2. Test đỏ server: allowlist giữ định dạng hợp lệ, bỏ thẻ/thuộc tính nguy hiểm; `content_text` đúng.
3. Test đỏ client: phím tắt, cú pháp gõ nhanh, toolbar phản ánh vùng chọn, link chỉ http(s)/mailto.
4. Backup DB → migration → `make db-migrate` → kiểm chứng; hiện thực; gates.

## Success Criteria
- [ ] Gửi đủ các định dạng trong danh sách, người nhận thấy đúng.
- [ ] HTML độc gửi thẳng qua API được lọc ở server (test deny); tìm kiếm khớp chữ, không khớp thẻ.
- [ ] Mọi gate xanh.

## Risk Assessment
- Tin cũ dạng markdown → giữ đường render markdown hiện có theo `content_format`; không chuyển đổi dữ liệu.
