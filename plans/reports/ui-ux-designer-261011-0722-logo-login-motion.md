# Logo Nexus + trang đăng nhập có chuyển động (mockup chờ duyệt)

Mockup: `design/mockups/logo.html`, `design/mockups/auth-motion.html`. Ảnh kiểm tra: `plans/reports/design-261011-0722-logo-login/` (390×844 và 1440×900, sáng/tối, cộng các trạng thái: bước mã, mã sai, đang gõ, mã đúng, chọn workspace, chạy hết luồng ở chế độ giảm chuyển động).
Chưa sửa code, DESIGN.md hay mockup cũ.

## 1. Ba hướng logo (lưới 48, SVG inline, chỉ màu token)

| Hướng | Ý | Ưu | Nhược |
|---|---|---|---|
| **A · Lối đi (đề xuất)** | Chữ N là một đường đi qua 4 nút; nút cuối (đích) màu cam đất `--color-person-1` | Nói đúng NGAC (đi được tới là có quyền); vẫn đọc là chữ N ở 16px; animation tự nhiên; đổi nhẹ từ icon chữ N hiện tại | Hai màu chỉ đẹp trên nền trung tính; trên nền accent thành một màu |
| B · Giao điểm | Hai vòng (người, tài nguyên), phần giao màu accent = nguyên tắc giao nhau | Trừu tượng, hiện đại | Không gợi chữ N; dễ trùng Mastercard và các ví điện tử |
| C · Tín hiệu | Chữ N hình học, thân phải ngắn, chấm màu người phía trên | Gần icon hiện tại nhất; đọc rõ nhất ở 16px | Mất ý đồ thị quyền |

Mỗi hướng có: lockup ngang trên 4 nền (sáng/tối × trung tính/accent), icon 512 và 192, maskable (tràn viền + vòng an toàn 80%, cắt tròn, cắt squircle), favicon 32/16 trong thanh tab sáng và tối, dải cỡ 48/32/24/16. Bản ≤ 24px dùng symbol riêng (nét dày hơn; A bỏ ba nút đầu, giữ nút đích). Hướng A có thêm bản dựng hình và quy tắc dùng.

## 2. Trang đăng nhập (`auth-motion.html`)

Luồng: email/số điện thoại → mã 6 số → "Đã xác minh" → Chọn workspace. Khung máy tính 1200×720 dùng bố cục chia đôi của AuthShell; khung điện thoại 390. Có nút chuyển sáng/tối, công tắc Giảm chuyển động, công tắc nền động, và 7 nút phát lại (vào màn, logo, xung, sang bước mã, mã sai, mã đúng, quay lại). Gõ mã tay vào ô thì luôn đúng.

| Chuyển động | Thời lượng / easing (token §7) | Giảm chuyển động |
|---|---|---|
| Logo tự lắp | nút `scale(.6)→1` 160 ease-out; nét `scaleX 0→1` 160 ease-out; nút đích 280 expo; lệch 40 đến 60ms, tổng ~840ms | fade 120ms |
| Form vào lần lượt | `translateY(8px)` + fade 220 ease-out, lệch 40ms, bắt đầu 200ms | fade 120ms |
| Hoạt động ở brand pane | chèn hàng 220; hàng mới phủ màu người 2400ms + "vừa tải lên" (đúng mẫu realtime §7) | hiện ngay, lớp phủ vẫn có |
| Nền đồ thị | fade 280 expo; 2 lớp trôi ≤16px, 24s/36s; dừng khi tab ẩn, khung ra khỏi màn, hoặc tắt | đứng yên |
| Xung trên cạnh | chấm màu người chạy theo cạnh 900ms, vòng loé 900ms, mỗi 3,2s | không có |
| Nút Nhận mã | nhấn `scale(.98)` 100; spinner 700ms | không lún, không quay |
| Bước 1 ↔ 2 | vào `translateX(±16px)` + fade 280 expo; ra `translateX(∓12px)` + fade 210 ease-in; email bay (FLIP) 280 expo | fade 120ms |
| Gõ số | ô `scale(.92)→1`, số `translateY(6px)` + fade, 160 ease-out | hiện ngay |
| Mã sai | rung ±6px 280 ease-out một lần, viền danger, "Mã chưa đúng. Còn 4 lần thử."; xoá số phải→trái 120 ease-in lệch 40 | chỉ đổi màu |
| Mã đúng | phủ success-wash lệch 40 (160); ô gom vào giữa 220 ease-in; huy hiệu ✓ 280 expo; nút đích logo loé 900ms | đổi màu + fade 120ms |
| Bàn giao | content fade 160 (chuyển route), hàng workspace lệch 40, 220 ease-out | fade 120ms |

Chỉ animate `transform`/`opacity`. Mọi animation đi qua một hàm duy nhất: khi giảm chuyển động thì bỏ transform, opacity thành fade ≤120ms, không độ trễ. Đã kiểm luồng đầy đủ ở chế độ giảm chuyển động: vẫn tới Chọn workspace.

## 3. Kiểm tra

Playwright 1.62, cả 2 file × 390/1440 × sáng/tối: 0 lỗi console, 0 page error, không tràn ngang ở 390 (`scrollWidth == clientWidth`). Font tải từ Google Fonts như các mockup khác.

## 4. Sẽ đổi gì trong code (sau khi duyệt)

- `DESIGN.md`: thêm mục logo (hướng chọn, quy tắc dùng); §7 thêm các dòng ngoại lệ cho auth: trượt 16px giữa các bước, nền ambient, logo tự lắp.
- `frontend/src/components/auth/AuthShell.tsx`: thay `Mark` chữ N bằng component logo SVG, thêm lớp nền đồ thị cho brand pane và dải trên mobile, chuỗi chèn hoạt động.
- `frontend/src/components/auth/LoginScreen.tsx`: preset trượt theo hướng thay `m.route`, email bay (shared `layoutId` của motion).
- `frontend/src/components/primitives` (OtpInput) + `CodeEntry.tsx`: pop khi gõ, rung, gom khi đúng, huy hiệu ✓.
- `frontend/src/lib/motion.ts`: preset mới (step, stagger 40ms, logo assemble, cell pop), đều có nhánh reduced.
- Component logo mới (vd `frontend/src/components/brand/NexusMark.tsx`); `frontend/public/` favicon.svg, icon 192/512/maskable/180 xuất từ mark; `index.html` và manifest PWA.
- Test: `AuthShell.test.tsx`, `LoginScreen.test.tsx`, `CodeEntry.test.tsx` cập nhật theo markup mới.

## Câu hỏi còn mở

1. Chọn logo nào: A (đề xuất), B hay C? Có giữ nút đích cam đất (hai màu) không, hay chỉ một màu accent?
2. Tên trên logo và tiêu đề là "Nexus" hay "Nexus Hub"? Mockup dùng "Nexus" (khớp tên PWA), app hiện ghi "Nexus Hub".
3. Có muốn nền đồ thị chuyển động không? Nếu có, giữ chạy liên tục (dừng khi ẩn) hay chỉ chạy 1 lần khoảng 30 giây rồi đứng yên?
4. Đồng ý cho bước đăng nhập trượt 16px (ngoại lệ so với quy tắc chuyển route chỉ fade ở §7)?
5. Logo tự lắp mỗi lần mở trang, hay chỉ lần đầu trong phiên (mockup đề xuất chỉ lần đầu)?
