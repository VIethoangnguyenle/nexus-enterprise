---
phase: 10
title: "Mục trong cột điều hướng (như Google Chat)"
status: pending
priority: P2
effort: "1.5d"
dependencies: [1, 5]
---

# Phase 10: Mục trong cột điều hướng

## Overview
Người dùng tự tạo mục để gom nhóm và tin nhắn trực tiếp trong cột điều hướng, như Google Chat. Thiết kế:
DESIGN.md "Chat: cột điều hướng và mục" + mockup `spaces.html` §1.

## Requirements
- Mục là của riêng từng người trong từng workspace; tên 1–40 ký tự, không trùng (không phân biệt hoa thường).
- Một cuộc trò chuyện ở tối đa một mục; thứ tự mục do người dùng chọn (lên/xuống).
- CRUD mục; chuyển cuộc trò chuyện vào/ra mục; xoá mục trả cuộc trò chuyện về mục mặc định; Hoàn tác khôi phục.
- Quyền: chỉ thao tác mục của chính mình; chuyển kênh vào mục cần `read` trên OA kênh (op từ `backend/ngac`).
  Mục không phải node NGAC (sở thích cá nhân, không cấp quyền gì). Test **deny**: mục của người khác, kênh
  không đọc được, kênh workspace khác.
- Danh sách mục trả kèm khi tải cột điều hướng; kênh rời/bị xoá tự rơi khỏi mục (FK `ON DELETE CASCADE`).

## Related Code Files
- Create: `data/migrations/0NN_chat_sections.sql` (`chat_sections(id, workspace_id, user_id, name, position)`,
  `chat_section_items(section_id, channel_id, user_id, UNIQUE(user_id, channel_id))`)
- Modify: `backend/services/messaging/internal/{store,domain,rest}/*` — routes dưới `/api/workspaces/:id/chat-sections`
- Modify: `frontend/vite.config.js` (khối regex `/api/workspaces`), `frontend/src/components/spaces/ChatNavigator.tsx`
- Create: `docs/specs/chat-sections/spec.md`

## Success Criteria
- [ ] Tạo, đổi tên, sắp xếp, xoá mục; chuyển/bỏ cuộc trò chuyện; Hoàn tác — khớp mockup.
- [ ] Test deny đủ ba trường hợp trên; migration áp dụng bằng `make db-migrate` và kiểm chứng.
- [ ] Mọi gate xanh; không ID nào lên màn hình.
