---
phase: 4
title: "Vỏ điện thoại: ô tìm, menu ☰, sheet tài khoản, thanh đáy Chat, nút +"
status: completed
priority: P0
effort: "2d"
dependencies: [1]
---

# Phase 04: Vỏ điện thoại

## Overview
Thay thanh trên "Nexus · workspace" và thanh đáy theo module bằng vỏ mới (DESIGN.md §5 "Điều hướng di động", mockup
§1, §5, §6). Áp dụng dưới 1024px (`useBottomBarLayout`), mọi module.

## Requirements
- **PhoneSearchBar** (thay `MobileTopBar`): ☰ · nút gợi ý theo module · avatar. Gợi ý: Chat "Tìm trong chat", Tài liệu
  "Tìm tài liệu", Phê duyệt "Tìm đề nghị", Tài sản "Tìm tài sản", Danh bạ "Tìm người", Cài đặt/Quản trị "Tìm cài đặt"
  (bảng ánh xạ một chỗ). Chạm: Chat mở màn Tìm (phase 06; trước đó rơi về `[data-module-search]` như hiện nay); module
  khác đi tới ô tìm của màn qua `[data-module-search]` (giữ logic MutationObserver hiện có); màn không có ô tìm riêng:
  phần giữa hiện tên module (không bấm được) để không có nút chết.
  Ẩn trong màn chi tiết (cuộc trò chuyện, tài liệu, đề nghị) như hiện nay (`isConversationPath` + tương tự).
  Chấm trên ☰ khi có đề nghị chờ bạn (workspace tổ chức) hoặc thông báo chưa đọc.
- **MainMenuDrawer** (thay `MobileMoreSheet`): theo DESIGN.md — logo, khối workspace với danh sách workspace tại chỗ
  (dùng `useSwitchWorkspace`, `useActiveWorkspace`, "Tạo workspace" như sidebar), Chat · Tài liệu · Phê duyệt · Tài sản ·
  Danh bạ | Thông báo · Quản trị · Cài đặt, tên · vai trò cuối. Cùng luật ẩn với `AppSidebar` (cá nhân: không Phê duyệt,
  Tài sản; Quản trị theo đúng luật sidebar đang dùng). Số: Chat chưa đọc (`unreadByLevel`), Phê duyệt "N chờ bạn",
  Thông báo chưa đọc. Thông báo mở `NotificationPanel` toàn màn có Quay lại. Modal đầy đủ, vuốt trái đóng, giảm chuyển
  động = fade.
- **Danh sách điểm đến dùng chung:** tách `mainNavItems`/`footNavItems` + luật ẩn của `AppSidebar.tsx` ra
  `frontend/src/lib/nav-destinations.ts`; sidebar và ngăn kéo cùng đọc (đổi tên một chỗ). `AppSidebar.test.tsx` phải
  xanh không sửa kỳ vọng.
- **AccountSheet**: avatar 52, tên, chức danh · workspace, trạng thái; Tạm dừng thông báo (nếu tính năng có sẵn trong
  Cài đặt → thông báo thì mở đó; không tự chế), Cài đặt tài khoản, Cài đặt, Đăng xuất (cùng đường logout duy nhất).
- **ChatTabBar** (viết lại `MobileNav.tsx`): chỉ trên `/channels` (danh sách), không trong cuộc trò chuyện, không ở module
  khác. 4 link icon-only tới `/channels?tab=home|direct|sections|more`, `aria-label` có số; Trang chủ chấm khi có chưa
  đọc; Trực tiếp số DM chưa đọc; Mục số cuộc nhóm chưa đọc (theo mức); Thêm chấm khi `useMentionsUnreadCount` > 0.
- **NewChatButton** (nút + 64): sheet Nhắn tin trực tiếp · Tạo nhóm · Duyệt nhóm · Tạo mục — dùng lại hộp thoại có sẵn
  (`StartChatDialog`, `CreateSpaceDialog`, duyệt nhóm, `overlays` tạo mục của `HomeView`).
- `_workspace.tsx`: gắn PhoneSearchBar thay MobileTopBar; đệm đáy 108 + safe-area chỉ khi ChatTabBar hiện; toast cách
  thanh đáy 16 (hoặc đáy 16 + safe-area).
- Search param `tab` trên `routes/_workspace/channels.index.tsx` (`validateSearch`, mặc định `home`, giá trị lạ → `home`);
  máy tính bỏ qua.

## Related Code Files
- Create: `frontend/src/lib/nav-destinations.ts` (+ test), `components/patterns/PhoneSearchBar.tsx`,
  `MainMenuDrawer.tsx`, `AccountSheet.tsx`, `components/spaces/NewChatButton.tsx` (+ test mỗi file)
- Modify: `components/patterns/MobileNav.tsx` (+ test) → thanh đáy Chat; `AppSidebar.tsx` (đọc danh sách chung);
  `routes/_workspace.tsx`; `routes/_workspace/channels.index.tsx`; `components/patterns/NotificationPanel.tsx` (biến thể
  toàn màn nếu cần)
- Delete: `MobileTopBar.tsx`, `MobileMoreSheet.tsx` và test của chúng — sau khi mọi chỗ dùng đã chuyển
- Modify: `docs/specs/lark-sidebar-layout/spec.md` (requirement "Phone Navigation And Workspace Switching" viết lại)

## Implementation Steps
1. Test trước: điểm đến theo loại workspace; ngăn kéo (mở/đóng/Esc/focus trả về ☰, aria-current, số, chọn workspace);
   thanh đáy (ẩn trong cuộc trò chuyện và module khác, aria-label có số, `aria-current`); ô tìm (gợi ý theo module,
   chạm tới `[data-module-search]`); `tab` param.
2. Tách `nav-destinations`, chuyển `AppSidebar` sang dùng — chạy `AppSidebar.test.tsx`.
3. Làm các component, gắn vào `_workspace.tsx`, xoá hai component cũ.
4. Cập nhật spec; `npm test`, `npm run lint`, typecheck, `npm run build`.

## Success Criteria
- [ ] Mọi module dưới 1024 có ô tìm + ☰; ☰ tới được mọi điểm đến; workspace cá nhân thiếu đúng 3 mục.
- [ ] Thanh đáy chỉ ở danh sách Chat; ở 390px hàng cuối không bị che (ảnh Playwright).
- [ ] Gate frontend xanh; `AppSidebar.test.tsx` không đổi kỳ vọng.

## Risk Assessment
- Đường từ module khác về Chat dài hơn 1 chạm — chấp nhận (người dùng chọn); ô tìm luôn có ☰.
- Xoá `MobileMoreSheet` có thể làm mất một luồng (đổi workspace, đăng xuất, thông báo) → test bao từng luồng trước khi xoá.
