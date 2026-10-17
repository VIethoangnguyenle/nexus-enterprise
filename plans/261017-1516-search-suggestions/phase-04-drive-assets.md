---
phase: 4
title: "Tài liệu + Tài sản"
status: completed
priority: P2
effort: "3h"
dependencies: [2]
---

# Phase 04: Tài liệu + Tài sản

## Overview
Gắn panel gợi ý vào ô tìm của Tài liệu (`DriveToolbar`) và danh sách Tài sản (`AssetListBody`).

## Requirements
- Tài liệu:
  - "Tìm gần đây".
  - "Sửa gần đây trong <thư mục>": tệp của thư mục đang mở, sắp theo thời gian sửa.
  - "Được chia sẻ với tôi": `/api/drive/shared-with-me`, chỉ tải khi panel mở lần đầu.
  - Chạm một tệp thì mở Trình xem tệp.
- Tài sản:
  - "Tìm gần đây" (chạy lại lần tìm qua `filterSearch` `q`).
  - "Tài sản bạn đang giữ".
  - "Hoạt động gần đây" (`/assets/activity`, hiện tên người làm).
  - Chạm một tài sản thì gọi `assetSearch`.

## Related Code Files
- Modify: `frontend/src/components/drive/DriveToolbar.tsx` (và màn chứa nó, để lấy danh sách tệp)
- Modify: `frontend/src/components/assets/AssetListBody.tsx`
- Create: `frontend/src/lib/suggestions/drive.ts`, `assets.ts` + tests

## Implementation Steps (TDD)
1. Test hàm thuần:
   - Sắp xếp và giới hạn.
   - Deny: tệp hay tài sản không có trong dữ liệu trả về thì không được gợi ý.
   - Thư mục không lọt vào nhóm "Sửa gần đây".
   - Không ID nào trong `title`/`meta`.
2. Test component: chỉ gọi `shared-with-me` khi panel mở; lỗi tải thì nhóm đó biến mất, không hiện toast.
3. Viết code. Query key mới đặt trong `hooks/keys/`.

## Success Criteria
- [ ] Test xanh; mở Tài liệu không phát sinh request thừa khi chưa focus ô tìm.

## Risk Assessment
Nhóm của Drive sẽ khá ít nếu thư mục đang mở có ít tệp. Đây là điều đã chấp nhận ở phương án A. Nếu Zane thấy chưa đủ
thì làm C (lưu ID tệp đã mở trên máy) trong một plan riêng.

## Thực tế khi làm (2026-10-17)
- Tài liệu: "Sửa gần đây trong <thư mục>" (chỉ tệp) và "Được chia sẻ với tôi" (tải khi ô được focus lần đầu; không
  có trong view Được chia sẻ). Tệp không nằm trong thư mục đang mở thì mở một mình trong Trình xem tệp (`loneFile`).
- Tài sản: "Bạn đang giữ" (`useAssets` với `assigned_to`, tải khi ô được focus lần đầu) và "Hoạt động gần đây"
  (bước mới nhất của mỗi tài sản, bỏ quyết định về yêu cầu; mô tả bằng tên người, ví dụ "Trần Minh Đức đã thu hồi
  từ Lê Quang Vinh").
- Cả hai: trên điện thoại, gợi ý phủ lên danh sách; danh sách vẫn được giữ trong DOM. Bộ lọc ẩn đi. Esc và Quay lại
  đóng gợi ý. Lần tìm được lưu khi bấm Enter (không tính Enter của bộ gõ) hoặc khi mở một kết quả.
- Hook: bấm vào ô (vẫn đang focus) sau khi chọn gợi ý hoặc bấm Esc thì gợi ý mở lại. `useRecentSearches.add` ổn
  định (`useCallback`).
- Fixture tài sản giờ lọc theo `assigned_to`, như máy chủ.
- Test cũ tìm ô theo nhãn (`getByLabelText`) thay vì role `searchbox`, vì trên máy tính ô là `combobox`.
- Kiểm tra: vitest 3111/3111, lint ok, `typecheck:diff` không tăng lỗi, `vite build` ok. Review gộp với phase 05.
