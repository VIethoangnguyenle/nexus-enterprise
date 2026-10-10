---
phase: 3
title: "Tối ưu giao diện mobile"
status: pending
priority: P0
effort: "2-3d"
dependencies: [1]
---

# Phase 03: Tối ưu giao diện mobile

## Overview
Rà và sửa mọi màn ở 390px và 768px theo DESIGN.md §5 (thanh đáy 3 tab + Thêm, sheet, safe-area).

## Requirements
- Rà tự động: Playwright chụp mọi route ở 390 × 844 và 768 × 1024, light/dark; script báo phần tử tràn ngang
  (`scrollWidth > clientWidth`) và vùng chạm < 44px.
- Sửa: safe-area (`env(safe-area-inset-*)`) cho thanh trên/đáy và ô soạn; ô soạn dính đáy, không bị bàn phím ảo
  che (`visualViewport`); detail panel thành sheet; bảng thành danh sách thẻ; menu thành sheet đáy; chữ ô nhập ≥ 16px
  (tránh iOS zoom).
- Màn không phải chat (Trang chủ, Tài liệu, Phê duyệt, Tài sản, Danh bạ, Cài đặt, Quản trị, đăng nhập): rà + sửa
  **ngay**, không chờ Chat.
- Màn chat mới (luồng hai bên, thread, thẻ) làm mobile-first ngay trong chat phase 04 theo mockup mobile của phase 01.
- Thanh đáy: workspace cá nhân thay Phê duyệt bằng Danh bạ (đồng bộ plan personal workspace).

## Related Code Files
- Modify: `frontend/src/components/**` theo kết quả rà (ghi danh sách vào report của phase)
- Create: `frontend/e2e/mobile-audit.spec.ts` (hoặc script trong `scripts/`)

## Success Criteria
- [ ] 0 màn tràn ngang ở 390px; mọi nút ≥ 44px; ảnh chụp khớp mockup; gates xanh.
