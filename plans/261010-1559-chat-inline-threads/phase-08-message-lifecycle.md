---
phase: 8
title: "Vòng đời tin nhắn (đợt 2)"
status: in-progress
priority: P2
effort: "2d"
dependencies: [5, 7]
---

# Phase 08: Vòng đời tin nhắn (đợt 2)

Người dùng yêu cầu sửa/xoá tin (2026-10-10): chuyển từ đợt 3 lên đợt 2. Thiết kế: DESIGN.md "Sửa tin", "Xoá tin";
mockup `spaces.html` §2 (nút Demo sửa/xoá) và §3 ("Xoá tin có thread").

## Overview
Sửa, xoá, trích dẫn khi trả lời, đánh dấu chưa đọc — như Google Chat. Hiện không có `PATCH`/`DELETE /messages/:id`.

## Requirements
- Sửa tin của mình: `PATCH /api/messages/:msgId` → `edited_at`; UI nhãn "Đã chỉnh sửa"; phím ↑ sửa tin cuối.
  Quyền: người gửi + `write` trên OA kênh.
- Xoá: `DELETE /api/messages/:msgId` → soft delete (`deleted_at`, nội dung xoá khỏi phản hồi); UI dòng "Tin nhắn đã
  bị xoá" chỉ cho tin có thread, tin không có thread biến mất khỏi luồng; luôn có hộp xác nhận. Quyền: người gửi
  (`write`) hoặc người có `manage` trên OA kênh; xoá tin người khác ghi nhật ký nhóm (ai xoá, tin của ai, lúc nào).
- Sửa chữ rỗng → client hỏi xoá; server từ chối `content` rỗng khi PATCH. Thẻ (poll, task…) không PATCH được.
- Trích dẫn khi trả lời: `quoted_message_id`; hiển thị khối trích dẫn (tên người + đoạn đầu), bấm → cuộn tới tin.
- Đánh dấu chưa đọc: đặt read receipt kênh/thread lùi về trước tin đó.
- WS: `MessageEditedEvent`, `MessageDeletedEvent` (proto + regenerate cả hai phía).
- Spec mới `chat-message-lifecycle`; migration `040_message_lifecycle.sql` (sau 037 thread, 038 attachments, 039 plain text).

## Success Criteria
- [ ] Test deny: sửa tin người khác → 403; xoá tin người khác không có `manage` → 403.
- [ ] Người đang xem thấy sửa/xoá ≤ 1s; không UUID trên UI; mọi gate xanh.

## Risk Assessment
- Tìm kiếm còn trả nội dung tin đã xoá → lọc `deleted_at IS NULL` trong `SearchMessages`; có test.

## Tiến độ
- Backend xong 2026-10-11: PATCH/DELETE `/api/messages/:id`, `POST /api/messages/:id/unread`, trích dẫn `quoted_message_id`, migration 040, WS `message_edited`/`message_deleted`; quản lý xoá tin người khác → dòng hệ thống trong nhóm; test deny. Spec `chat-message-lifecycle`. **Còn:** UI (nhãn Đã chỉnh sửa, hộp xác nhận, khối trích dẫn, ↑ sửa tin cuối).
- Review: `plans/reports/code-reviewer-261011-0830-chat-wave2-backend.md` (C1, H1, H2, M1–M5, M8 đã sửa; M6 bằng dòng hệ thống; M7 — sửa tin không đổi danh sách nhắc tên — giữ nguyên, ghi vào câu hỏi mở).
