---
phase: 5
title: "Ảnh nhóm: frontend"
status: pending
priority: P1
effort: "6h"
dependencies: [3, 4]
---

# Phase 05: Ảnh nhóm — frontend

## Overview
Người có `manage` đổi/bỏ ảnh nhóm bằng `AvatarEdit` trong panel thông tin nhóm; mọi chỗ vẽ icon nhóm hiện ảnh.

## Requirements
- `api/messaging.ts`: `uploadChannelAvatar(chId, file)`, `removeChannelAvatar(chId)` (dùng `apiUpload` như `api/profile.ts`).
- Hook `useChannelAvatar` (TanStack Query mutation) cập nhật cache kênh/conversations; sự kiện realtime cập nhật cache
  ở máy thành viên khác (theo cách rename đang làm).
- `SpaceInfoPanel.tsx`: `AvatarEdit shape="square"`, `canEdit` = quyền `manage` (dùng nguồn quyền kênh hiện có, không
  tự đoán từ vai trò); DM không có.
- Icon nhóm nhận `src`: tìm mọi chỗ vẽ icon nhóm (`FeedRow`, `FeedParts`, `SpaceView` header, `ChatSearchScreen`,
  cột điều hướng, thông báo, `WorkspaceSelectScreen` nếu là kênh) — gom về một component nếu chưa có.
- Không có ảnh → ô chữ cái như cũ.

## Implementation Steps
1. Test trước: panel hiện AvatarEdit chỉ khi `manage`; upload gọi API và cache đổi; sự kiện realtime đổi ảnh ở danh
   sách; DM không có; lỗi API hiện dưới ảnh (`avatarErrorMessage`).
2. API + hook + panel.
3. Icon nhóm có `src` ở mọi chỗ; test snapshot nhẹ cho 2–3 chỗ chính.
4. `npm test`, `npm run lint`, `npm run build`.

## Related Code Files
- Modify: `frontend/src/api/messaging.ts`, `frontend/src/hooks/` (kênh), `frontend/src/components/spaces/SpaceInfoPanel.tsx`,
  các component icon nhóm nêu trên, handler realtime kênh

## Success Criteria
- [ ] Đổi ảnh ở máy A → máy B thấy trong vài giây, không tải lại trang.
- [ ] Thành viên thường không thấy lớp phủ đổi ảnh.
- [ ] Không UUID trên UI; alt/nhãn dùng tên nhóm.

## Risk Assessment
- Icon nhóm vẽ rải rác nhiều nơi → bỏ sót một chỗ. Grep `space-icon`/hueKey nhóm trước khi sửa, liệt kê trong PR.
