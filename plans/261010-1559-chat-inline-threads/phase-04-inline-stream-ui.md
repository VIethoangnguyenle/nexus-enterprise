---
phase: 4
title: "UI luồng in-line, panel Thread"
status: pending
priority: P1
effort: "2-3d"
dependencies: [1, 2, 3]
---

# Phase 04: UI luồng in-line, panel Thread

## Overview
Thay `TopicStream` (thẻ chủ đề) bằng luồng in-line theo mockup đã duyệt ở phase 01, cho cả nhóm và DM.

## Requirements
- `ConversationStream`: theo thời gian, gom tin cùng người ≤ 5 phút, vạch ngày, vạch "Tin mới" (từ
  `last_read_message_id` của kênh), nút nổi nhảy tới chưa đọc / mới nhất, tải thêm khi cuộn lên (`before=`).
- `ThreadSummary` dưới tin: avatar người trả lời (PersonChip/Avatar, không ID), "N trả lời", giờ cuối, "M mới".
- `ThreadPanel`: nút Theo dõi/Đang theo dõi, vạch "Tin mới" trong thread, mở panel → `POST .../thread/read`.
- `ThreadsPanel` (nút "Thread" ở header nhóm): lọc Tất cả / Đang theo dõi / Chưa đọc, phân trang, chọn → mở thread.
- Giữ: gửi, cảm xúc, ghim, nhắc tên, emoji, typing, đánh dấu đã đọc kênh, wash realtime (người khác), tải lên tệp.
- Xoá `TopicStream` và test của nó; cập nhật hoặc đánh dấu thay thế spec `lark-messaging-layout`.
- A11y: luồng là `role="log"` có `aria-live` lịch sự; phím: ↑ chỉnh tin (đợt 3), Esc đóng panel.

## Related Code Files
- Create: `frontend/src/components/spaces/ConversationStream.tsx`, `ThreadSummary.tsx`, `ThreadsPanel.tsx` (+ test)
- Modify: `frontend/src/components/spaces/SpaceView.tsx`, `MessageBlock.tsx`, `frontend/src/components/chat/ThreadPanel.tsx`
- Modify: `frontend/src/api/messaging.ts`, `frontend/src/hooks/useMessages.ts`, `frontend/src/hooks/keys/*`
- Delete: `frontend/src/components/spaces/TopicStream.tsx`, `TopicStream.test.tsx`
- Modify: `docs/specs/lark-messaging-layout/spec.md`, `docs/specs/chat-threads/spec.md`

## Implementation Steps
1. Test đỏ (vitest): gom tin; vạch "Tin mới" đúng vị trí; tóm tắt hiện "M mới" chỉ khi đang theo dõi; panel Thread lọc đúng; không chuỗi khớp regex UUID.
2. Hiện thực component theo mockup; nối hook/API; xoá TopicStream.
3. Chụp màn (Playwright, dữ liệu giả lập + 1 lần với stack thật nếu có) so với mockup; sửa lệch.
4. Gates: `npm test`, `npm run lint`, `npm run typecheck:diff`, bundle production.

## Success Criteria
- [ ] Ảnh chụp khớp mockup ở 1440px, 1100px, 390px, light + dark.
- [ ] Mọi gate frontend xanh; không còn import `TopicStream`.

## Risk Assessment
- Cuộn ngược + vạch tin mới dễ nhảy layout → giữ vị trí cuộn khi chèn trang cũ (anchor); tín hiệu: test cuộn thất bại → dùng `overflow-anchor` + đo lại.
