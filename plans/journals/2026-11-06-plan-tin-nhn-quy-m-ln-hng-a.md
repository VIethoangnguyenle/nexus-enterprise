---
title: "Plan: tin nhắn ở quy mô lớn (hướng A)"
date: 2026-11-06
summary: "Lập plan unread O(1) theo seq, badge realtime, phân trang seq; phát hiện badge channel khác không realtime"
---

# Plan: tin nhắn ở quy mô lớn (hướng A)

## What happened
Nghiên cứu cơ chế tải tin nhắn cho bài toán hàng nghìn tenant
(`plans/reports/brainstorm-261106-2130-message-loading-at-scale.md`), rồi lập plan
`plans/261106-2127-message-unread-seq-at-scale/` (7 phase, ~45h).

Phát hiện khi lập plan:
- Client chỉ subscribe realtime channel đang mở (`SpaceView.tsx`, `CallChat.tsx`). Bão `unreadCounts` là N người đang mở channel, không phải N thành viên online. Brainstorm đã được đính chính.
- `UnreadCountEvent` có trong `ws.proto` và `Hub.SendUnreadCount` tồn tại, nhưng không có chỗ nào gọi. Badge thanh bên của channel không mở chỉ làm mới khi mở app hoặc reconnect.
- `GetUnreadCounts` có nhánh CTE `followed` quét `messages` theo `sender_id` không giới hạn tenant; bảng không có index `sender_id`.

## Decision
Hướng A, đã chốt với người dùng: số chưa đọc chính xác (trừ tin xoá), badge realtime bằng `ConversationActivityEvent`
chỉ gửi cho thành viên, benchmark quy mô Mid trên DB local `ngac_bench`.

## Next steps
`/ak:cook --tdd` bắt đầu từ phase 1 (benchmark nền) để xác nhận các suy luận về planner trước khi đổi schema.

> Historical work record — not durable authority. Prefer docs/specs/ADRs for current decisions.
