---
phase: 3
title: "Chat + Danh bạ"
status: completed
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

## Thực tế khi làm (2026-10-17)
- Trên máy tính, Chat không có ô tìm chung cho cả module: `SpaceSearchPanel` chỉ tìm trong một cuộc trò chuyện, nên
  "Trò chuyện gần đây" không có nghĩa ở đó. Chat chỉ gắn vào màn Tìm trong chat trên điện thoại (`ChatSearchScreen`),
  khớp với mockup (Chat chỉ được vẽ ở bản điện thoại). `SpaceSearchPanel` giữ nguyên.
- Thêm `hooks/useRecentSearches.ts`: nhóm "Tìm gần đây" dùng chung cho mọi module.
- Danh bạ trên điện thoại: nút "Quay lại danh sách" đặt cạnh ô tìm; ô không đổi sang viên thuốc 52 viền accent như
  mockup, vì đổi wrapper sẽ mount lại input và làm mất focus. Nếu muốn đúng mockup thì phải sửa `SearchField` để
  dùng một cây DOM cho cả hai kiểu; để quyết ở phase 06.
- Trên máy tính, ô tìm Danh bạ giờ có role `combobox` (trước là `searchbox`); test đã đổi theo.
- Review: `reports/code-review-261017-phase-03.md`. Đã sửa:
  - M1: Tìm trong chat hiện gợi ý ngay khi mở (`initiallyOpen`), không chờ focus (iOS có thể không trao focus).
  - M2: Danh bạ không dùng `useConversations` nữa. `useRecentDirects` chỉ lấy DM + activity, không đăng ký WS, và
    chỉ tải khi ô tìm được focus lần đầu.
  - M3: trên điện thoại, gợi ý phủ lên danh sách nên giữ vị trí cuộn; Esc từ bàn phím rời đóng gợi ý.
  - Nhỏ: ẩn bộ lọc khi gợi ý che danh sách; Enter lúc bộ gõ đang ghép chữ không lưu lần tìm.
- Chưa làm, để quyết ở phase 06:
  - Nút back của Android trên Danh bạ rời khỏi màn, thay vì đóng gợi ý. Cần một mục lịch sử riêng như `?search=1`
    của Chat; việc này đụng router.
  - Focus không quay về chỗ cũ sau khi đóng hồ sơ mở từ gợi ý trên điện thoại.
  - "Trò chuyện gần đây" không có skeleton lúc đang tải.
- Kiểm tra: vitest 3094/3094, lint ok, `typecheck:diff` không tăng lỗi, `vite build` ok.
