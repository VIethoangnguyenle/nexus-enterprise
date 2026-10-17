---
phase: 3
title: "Chat + Danh bạ"
status: pending
priority: P2
effort: "3h"
dependencies: [2]
---

# Phase 03: Chat + Danh bạ

## Overview
Gắn panel gợi ý vào ô tìm của Chat (máy tính: `SpaceSearchPanel`; điện thoại: `ChatSearchScreen`) và của Danh bạ.

## Requirements
- Chat:
  - "Tìm gần đây" (đang có) và "Trò chuyện gần đây": tối đa N cuộc trò chuyện có tin mới nhất, lấy từ danh sách
    conversation đã tải.
  - Chạm một cuộc trò chuyện thì mở nó.
- Danh bạ:
  - "Tìm gần đây" và "Nhắn gần đây": người có DM với mình mới nhất (`usePeople` + danh sách DM), không có chính mình.
  - Chạm một người thì mở hồ sơ của người đó trong Danh bạ.
- Lưu một lần tìm khi mở kết quả hoặc khi bấm Enter (như Chat đang làm).

## Related Code Files
- Modify: `frontend/src/components/spaces/SpaceSearchPanel.tsx`, `ChatSearchScreen.tsx`
- Modify: `frontend/src/components/contacts/ContactsScreen.tsx`
- Create: `frontend/src/lib/suggestions/chat.ts`, `contacts.ts` + tests (hàm thuần: dữ liệu vào, nhóm gợi ý ra)

## Implementation Steps (TDD)
1. Test hàm thuần:
   - Sắp theo thời gian tin cuối; giới hạn N.
   - Deny: cuộc trò chuyện không còn trong danh sách đọc được thì không bao giờ được gợi ý, kể cả khi trước đó đã mở.
   - Không có chính mình.
   - Không ID nào trong `title`/`meta`.
2. Test component: focus ô trống thì thấy các nhóm; gõ chữ thì panel tắt.
3. Viết code.

## Success Criteria
- [ ] Các test deny và test sắp xếp xanh. `ChatSearchScreen.test.tsx` không phải sửa kỳ vọng nào ngoài phần đã thiết kế lại.

## Risk Assessment
Danh sách DM có thể chưa tải lúc mở Danh bạ. Khi đó hiện skeleton, không chặn ô tìm.
