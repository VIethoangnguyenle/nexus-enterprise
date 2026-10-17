---
phase: 2
title: "Nền: lịch sử tìm theo module + panel gợi ý"
status: completed
priority: P1
effort: "4h"
dependencies: [1]
---

# Phase 02: Nền

## Overview
Đưa lịch sử tìm từ riêng Chat sang dùng được cho mọi module, và làm một pattern `SearchSuggestions` để các module chỉ
cần truyền vào danh sách nhóm gợi ý.

## Requirements
- `recent-searches.ts`:
  - Có tham số `module: 'chat' | 'drive' | 'contacts' | 'assets' | 'approval'`.
  - Key mới: `ngac-recent-searches:<module>:<user>`.
  - Lần đầu đọc Chat thì chuyển key cũ `ngac-chat-recent-searches:<user>` sang key mới rồi xoá key cũ.
  - `clearAllRecent` xoá cả hai tiền tố.
- `SearchSuggestions` (patterns):
  - Nhận `groups: { label, items: { key, icon?, title, meta?, onSelect, onRemove? }[] }[]`.
  - Render listbox: ↑/↓ đi giữa các mục, Enter chọn, Esc đóng và giữ focus ở ô tìm.
  - Nhóm nào không có mục thì không hiện.
- Hook `useSuggestionPanel(value)`: mở khi ô được focus và trống; đóng khi gõ, khi blur ra ngoài panel, hoặc khi Esc.
- Bố cục theo DESIGN.md đã duyệt ở phase 01. Không đổi `SearchField` cho 25 chỗ đang gọi: panel là component riêng đặt
  cạnh ô tìm.

## Related Code Files
- Modify: `frontend/src/lib/recent-searches.ts`, `frontend/src/lib/recent-searches.test.ts`
- Modify: `frontend/src/components/spaces/ChatSearchScreen.tsx` (truyền `'chat'`)
- Create: `frontend/src/components/patterns/SearchSuggestions.tsx` + `.test.tsx`
- Create: `frontend/src/hooks/useSuggestionPanel.ts` (test qua harness trong `SearchSuggestions.test.tsx`)

## Cách các module gắn vào (cho phase 03–05)
`const panel = useSuggestionPanel({ value, groups, variant, anchorRef })`, trải `panel.inputProps` lên `SearchField`
(ref của ô = `anchorRef`), đặt `<SearchSuggestions panel={panel} emptyText=… />`. `variant`: `'popover'` khi
`!useBottomBarLayout()`, `'screen'` trên điện thoại. Màn có `onKeyDown`/`onFocus`/`onBlur` riêng thì gọi của hook trước,
bỏ qua sự kiện đã `defaultPrevented`. Nhóm "Tìm gần đây" dùng `recent: true`, item có `onRemove`.

## Implementation Steps (TDD)
1. Test fail: hai module không lẫn lịch sử của nhau; key Chat cũ được chuyển sang; `clearAllRecent` xoá cả hai tiền tố;
   storage bị chặn thì trả `[]`.
2. Sửa `recent-searches.ts` cho test xanh. `ChatSearchScreen.test.tsx` vẫn phải xanh.
3. Test fail cho `SearchSuggestions`: điều hướng bàn phím, xoá một mục, nhóm rỗng bị ẩn, role/aria đúng.
4. Viết component và hook.

## Success Criteria
- [x] Test mới và test cũ của Chat đều xanh.
- [x] Không còn chỗ nào dùng thẳng `ngac-chat-recent-searches` ngoài nhánh chuyển key.

## Risk Assessment
Nếu chuyển key bị lỗi thì người dùng mất lịch sử Chat. Phải có test riêng cho việc chuyển key, và chỉ xoá key cũ sau khi
ghi key mới thành công.

## Kết quả (2026-10-17)
- Review: `reports/code-review-261017-phase-02.md`. Đã sửa:
  - H1: dữ liệu cũ của Chat hỏng làm mất lịch sử mới.
  - M1: điện thoại dùng hàng nút, ✕ có tên riêng; máy tính dùng Delete, ✕ chỉ cho chuột.
  - M2: trên điện thoại, cất bàn phím không đóng gợi ý; đóng bằng `panel.close()` khi bấm Quay lại.
  - M3: popover không hẹp hơn ô, không tràn mép, cao không quá khoảng trống bên dưới.
  - Nhỏ: hàng đang chọn theo key; key nhóm không trùng; chỉ có `aria-controls` khi panel mở.
- Để lại, kiểm ở phase 06 trên trình duyệt thật:
  - Popover dùng `z-dropdown` (10) như `Popover` primitive, thấp hơn `z-sticky` (20). Nếu thấy phần tử sticky đè lên
    thì nâng tầng cho cả hai, cùng một lần sửa.
  - Popover không lật lên trên khi thiếu chỗ bên dưới.
  - Popover không đi theo khi layout dịch chuyển mà không có scroll hay resize.
- Kiểm tra: vitest 3071/3071, `npm run lint` ok, `typecheck:diff` không tăng lỗi, `vite build` ok.
