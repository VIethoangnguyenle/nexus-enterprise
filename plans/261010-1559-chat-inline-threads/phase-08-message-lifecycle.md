---
phase: 8
title: "Vòng đời tin nhắn (đợt 3)"
status: pending
priority: P2
effort: "2d"
dependencies: [5, 7]
---

# Phase 08: Vòng đời tin nhắn (đợt 3)

## Overview
Sửa, xoá, trích dẫn khi trả lời, đánh dấu chưa đọc — như Google Chat. Hiện không có `PATCH`/`DELETE /messages/:id`.

## Requirements
- Sửa tin của mình: `PATCH /api/messages/:msgId` → `edited_at`; UI nhãn "Đã chỉnh sửa"; phím ↑ sửa tin cuối.
  Quyền: người gửi + `write` trên OA kênh.
- Xoá: `DELETE /api/messages/:msgId` → soft delete (`deleted_at`, nội dung xoá khỏi phản hồi); UI dòng "Tin nhắn đã
  bị xoá"; tin gốc bị xoá vẫn giữ thread. Quyền: người gửi (`write`) hoặc người có `manage` trên OA kênh.
- Trích dẫn khi trả lời: `quoted_message_id`; hiển thị khối trích dẫn (tên người + đoạn đầu), bấm → cuộn tới tin.
- Đánh dấu chưa đọc: đặt read receipt kênh/thread lùi về trước tin đó.
- WS: `MessageEditedEvent`, `MessageDeletedEvent` (proto + regenerate cả hai phía).
- Spec mới `chat-message-lifecycle`; migration `040_message_lifecycle.sql` (sau 037 thread, 038 attachments, 039 plain text).

## Success Criteria
- [ ] Test deny: sửa tin người khác → 403; xoá tin người khác không có `manage` → 403.
- [ ] Người đang xem thấy sửa/xoá ≤ 1s; không UUID trên UI; mọi gate xanh.

## Risk Assessment
- Tìm kiếm còn trả nội dung tin đã xoá → lọc `deleted_at IS NULL` trong `SearchMessages`; có test.
