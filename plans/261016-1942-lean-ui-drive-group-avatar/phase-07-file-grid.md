---
phase: 7
title: "Bộ hiển thị tệp dùng chung"
status: pending
priority: P1
effort: "1d"
dependencies: [1, 6]
---

# Phase 07: `components/files/` — lưới, thẻ, thumbnail

## Overview
Các component dùng chung cho Drive (08) và tab Tệp (09), theo `drive-gallery.html`.

## Requirements
- `FileThumb`: tải `thumbnail?size=` (320 cho thẻ, 640 cho panel; `srcset` theo DPR), skeleton tới khi có ảnh,
  404/503/lỗi → ô icon theo `kindOf` (`drive/drive-model.ts`), không nhảy layout (khung 16:10 cố định), `loading="lazy"`.
  Auth theo cách `api/drive.ts` đang lấy ảnh (presigned hoặc fetch có cookie — theo preview hiện có).
- `FileCard` (thẻ tệp, ⋯ menu, chọn, nhấn giữ → sheet), `FolderTile`, `FileGrid` (lưới auto-fill: điện thoại 2 cột,
  máy tính bảng 3–4, web auto-fill min 200 px; ←/→/↑/↓ di chuyển theo lưới; Enter/Space = mở nhanh qua callback).
- `useFileViewMode` (Zustand persist trong `stores/`, khoá theo màn: `drive` | `space-files`): `grid` | `list`.
- Nhóm theo thời gian (`lib/file-groups.ts`): Hôm nay / Tuần này / Tháng này / Cũ hơn, theo múi giờ người dùng.
- Không UUID: thẻ hiện tên tệp, người bằng tên + avatar.

## Implementation Steps
1. Test trước: `file-groups` (biên ngày/tuần/tháng), `FileThumb` (skeleton → ảnh; 404 → icon; 503 → icon), `FileGrid`
   phím mũi tên + Enter/Space gọi `onOpen(index)`, `useFileViewMode` nhớ qua reload.
2. Viết component theo mockup; tokens từ `tokens.css`.
3. `npm test`, `npm run lint`.

## Related Code Files
- Create: `frontend/src/components/files/{FileThumb,FileCard,FolderTile,FileGrid}.tsx` (+ test), `frontend/src/lib/file-groups.ts` (+ test), `frontend/src/stores/file-view.store.ts`
- Modify: `frontend/src/api/drive.ts` (thumbnail URL), `frontend/src/hooks/keys/drive.ts` nếu cần

## Success Criteria
- [ ] Component có test; không có fake data; fallback icon hoạt động khi server không sinh được.

## Risk Assessment
- Lưới phím mũi tên với số cột thay đổi theo khổ → đọc số cột từ `getComputedStyle(grid).gridTemplateColumns`.
