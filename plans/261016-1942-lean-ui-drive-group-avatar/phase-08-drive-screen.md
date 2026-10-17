---
phase: 8
title: "Màn Tài liệu: lưới / danh sách / panel / phím tắt"
status: pending
priority: P1
effort: "1.5d"
dependencies: [7]
---

# Phase 08: Màn Tài liệu

## Overview
`DriveScreen` dùng `FileGrid` (mặc định) hoặc `DriveTable` (danh sách); panel chi tiết có preview; 3 khổ màn.

## Hiện trạng (đã kiểm)
- `components/drive/DriveScreen.tsx` (427 dòng), `DriveBody`, `DriveTable` (`RowActions`, `onPreview`), `DriveTree`,
  `DriveDetailPanel`, `DriveBreadcrumbs`; URL giữ folder (`?folder=`); `FileViewer` đã gắn.

## Requirements
- Thanh công cụ: tìm (hiện có) + chip loại + toggle Lưới/Danh sách + sắp xếp (tên / sửa gần nhất / dung lượng).
- Lưới: thư mục trước (`FolderTile` là `Link` như hàng thư mục hiện tại), tệp sau. Danh sách: `DriveTable` giữ nguyên
  (đã hairline) nhưng thêm ô thumbnail 24 trước tên.
- Panel chi tiết (`DriveDetailPanel`): preview 640 + thông tin + Mở / Tải xuống / Chia sẻ; máy tính bảng sheet phải,
  điện thoại bottom sheet.
- Phím: Space/Enter trên thẻ/hàng tệp đã chọn mở `FileViewer` với danh sách tệp của thư mục (←/→ do FileViewer lo).
- Máy tính bảng: `DriveTree` thành ngăn kéo (nút ☰ cạnh breadcrumb). Điện thoại: nút + nổi (Tải lên · Thư mục mới).
- Upload, chia sẻ, di chuyển, đổi tên, xoá: chỉ đổi chỗ đặt nút (⋯ / chuột phải / nhấn giữ), luồng giữ nguyên.
- Realtime arrivals (`useArrivals`), quyền theo `permsOf` giữ nguyên trong lưới.
- Split file nếu `DriveScreen.tsx` vượt quá cỡ hợp lý (tách toolbar và hook chọn).

## Implementation Steps
1. Cập nhật `DriveScreen.test.tsx` trước: toggle chế độ + nhớ; lưới thư mục trước; Space mở viewer; panel có preview;
   3 khổ (mock `matchMedia`): ngăn kéo cây ở md, bottom sheet ở sm; ⋯ luôn hiện trên pointer-coarse.
2. Sửa màn; giữ test hiện có (728 dòng) xanh — test nào đổi do bố cục mới thì sửa có lý do ghi trong PR.
3. Spec `docs/specs/drive-context-panel/spec.md` (panel preview), `drive-tree-navigation` (ngăn kéo).

## Related Code Files
- Modify: `frontend/src/components/drive/{DriveScreen,DriveBody,DriveTable,DriveDetailPanel,DriveTree,DriveBreadcrumbs}.tsx`, tests
- Create (nếu tách): `frontend/src/components/drive/DriveToolbar.tsx`

## Success Criteria
- [ ] Đủ acceptance 2, 3, 8 của brainstorm cho Drive; xem tận mắt 390 / 820 / 1440.

## Risk Assessment
- Thay đổi lớn ở màn nhiều test → làm theo từng bước nhỏ, chạy `vitest src/components/drive` sau mỗi bước.
