# Rà mobile — PWA phase 03 (2026-10-11)

Công cụ: `frontend/scripts/mobile-audit.mjs` (Playwright, đăng nhập một lần qua mã OTP thử nghiệm, đổi route phía
client). Đo mỗi màn: tràn ngang (phần tử vượt mép ngoài vùng cuộn ngang), vùng chạm < 44px (tính cả `::after`
mở rộng), ô nhập < 16px. 18 route × 390×844 / 768×1024 × sáng / tối = 72 trường hợp, 0 lỗi JS.

## Trước khi sửa
- Tràn ngang: 0 màn (layout mobile đã có: thanh đáy, bảng thành thẻ, thanh tab cuộn trong vùng của nó).
- Ô nhập < 16px: 14 ô trên 8 màn (đăng nhập, tìm kiếm Danh bạ/Tài liệu/Drive/Quản trị/Tài sản, form Cài đặt,
  yêu cầu tài sản) → iOS phóng to khi chạm.
- Vùng chạm < 44px: chip lọc 32, tab 40, IconButton 28–32, nút 30–36 trên mọi màn.
- Bàn phím ảo: khung app `h-dvh` — iOS không co `dvh` theo bàn phím → ô soạn bị che.

## Đã sửa (theo DESIGN.md §6: Input 16px, IconButton vùng chạm ≥ 44 trên touch, touch target ≥ 44)
- `index.css` `@media (pointer: coarse)`: ô nhập 16px; `button`, `[role=button|tab|menuitem|switch]`, `summary`
  có `::after` vô hình nới vùng chạm tới 44 mỗi chiều (`:where()` → component tự có `::after` vẫn thắng). Không đổi
  giao diện nhìn thấy.
- `lib/app-height.ts` + `h-app`: khung workspace cao bằng `visualViewport` (co theo bàn phím ảo); `index.html` thêm
  `interactive-widget=resizes-content` cho Android.

## Sau khi sửa
- 0 tràn ngang, 0 ô nhập < 16px; vùng chạm < 44 còn 1: tên thư mục trong bảng Drive (21px, nằm trong hàng 44 cả hàng
  bấm được).
- Safe-area: đã có sẵn (`pt-safe`, `pb-bar`, `pb-tabbar`, `pb-sheet`) trên thanh trên/đáy, sheet, toast.

## Còn lại
- Màn chat mới (luồng hai bên, thread, thẻ) làm mobile-first trong chat phase 04 (theo plan).
- Kiểm trên thiết bị thật (iOS Safari bàn phím, Android Chrome) ở phase 06.
- Nút TanStack Query devtools (chỉ dev) đè góc thanh đáy — không có ở bản build.
