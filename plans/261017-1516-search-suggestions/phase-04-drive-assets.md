---
phase: 4
title: "Tài liệu + Tài sản"
status: pending
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
