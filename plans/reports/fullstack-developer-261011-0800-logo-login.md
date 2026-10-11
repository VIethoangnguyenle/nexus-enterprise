# Logo Nexus + đăng nhập có chuyển động: báo cáo triển khai

Không commit, không đụng git index. Chỉ sửa trong danh sách cho phép, cộng hai ngoại lệ ghi ở mục Deviations.

## Files

Mới:
- `frontend/src/components/brand/NexusMark.tsx`, `NexusLogo.tsx`, `nexus-mark.css`, `NexusMark.test.tsx`: dấu hướng A (props `size`, `assemble`, `tone`, `decorative`; ≤24px dùng bản nhỏ; `role="img"` tên "Nexus" khi đứng một mình).
- `frontend/src/components/auth/auth-motion.ts` (+ test): preset trượt bước 16/12px, cờ sessionStorage `nexus:logo-assembled` (`useLogoAssembly`), `GRAPH_SETTLE_SECONDS = 30`.
- `auth-motion.css`: form vào lần lượt, nền đồ thị, hàng hoạt động + lớp phủ 2400ms, flyer email.
- `AuthGraph.tsx` (nền đồ thị: trôi 1 lần 30s rồi đứng, dừng khi tab ẩn, không vẽ khi giảm chuyển động), `AddressFlight.tsx` (email bay, FLIP bằng WAAPI, 1 bản duy nhất).
- `frontend/public/`: `favicon.svg` (đổi màu theo tab sáng/tối), `favicon-32.png`, `favicon-16.png`, `apple-touch-icon.png`, `icon-192/512.png`, `icon-maskable-192/512.png` (Playwright, nền trong suốt ở góc bo).

Sửa:
- `AuthShell.tsx`: dùng `NexusLogo`, đồ thị, "Nexus" thay "Nexus Hub", logo tự lắp một lần mỗi phiên, `data-reduced`.
- `LoginScreen.tsx`: trượt ±16px theo hướng (variants + `custom`), email bay, tiêu đề "Đăng nhập vào Nexus".
- `CodeEntry.tsx`: "Đã xác minh." trong vùng aria-live, giữ 700ms (240ms giảm chuyển động) rồi mới gọi `onVerified`; chỉ chế độ đăng nhập; huỷ timer khi unmount.
- `primitives/OtpInput.tsx`: pop khi gõ, rung 1 lần khi sai, gom ô + huy hiệu ✓ khi đúng.
- `frontend/index.html`: title "Nexus", link favicon 32/16 và apple-touch-icon.
- `DESIGN.md`: tên "Nexus"; mục "Logo và tên" (dấu, lockup, khoảng trống, màu, quy tắc cỡ nhỏ, a11y, icon); §7 thêm ngoại lệ màn đăng nhập.
- `docs/specs/sign-in-screens/spec.md`: một câu (xem Deviations).

## Tests

- Mới: `NexusMark.test.tsx` (10), `auth-motion.test.ts` (6), thêm vào `AuthShell.test.tsx` (tên, logo lần đầu/sau, đồ thị có/không/tạm dừng/30s), `OtpInput.test.tsx` (rung, giảm chuyển động, pop, huy hiệu), `CodeEntry.test.tsx` (thông báo, giữ rồi gọi, không gọi nếu rời màn), `LoginScreen.test.tsx` (tiêu đề, dùng ngay được, email bay và hạ cánh, giảm chuyển động không bay).
- Test cũ "error: fade, no transform" của OtpInput đổi theo thiết kế đã duyệt.
- `npx vitest run src/components/auth src/components/brand src/components/primitives`: 247 pass, 0 fail.
- Toàn bộ `npx vitest run`: chỉ fail ngoài phạm vi: AssetsScreen (đã biết), cộng `websocket.store.test.ts` và `SpaceView.test.tsx` (code chat đang sửa song song, dao động giữa các lần chạy).
- `npm run lint`: 0 lỗi ở các file tôi sửa (một warning ở `ThreadPanel.tsx`, của người khác).
- `npm run typecheck:diff`: lỗi mới còn lại chỉ ở `generated/proto/messaging/ws.ts` và `websocket.store.test.ts` (chat, không phải của tôi).

## Ảnh

`plans/reports/login-motion-261011/`: `login-{390,1440}-{light,dark}-{0-assembling,1-rest}.png`, `code-*-{2-step,3-wrong,4-success}.png`, `after-*-5-next.png` (Chọn workspace), `login-1440-light-reduced.png` (giảm chuyển động: không có đồ thị, hàng hoạt động hiện ngay).
Kiểm bằng Playwright thật qua dev stack (email smoke1791570521@example.org, mã dev lấy từ biến môi trường, không in):
- 390 và 1440, sáng/tối: không tràn ngang (`scrollWidth - clientWidth = 0`, cả màn đầu và bước mã).
- Ô email có focus ngay (autocomplete=username), gõ được trong lúc form còn đang vào.
- Mã sai: "Mã chưa đúng. Còn 4 lần thử." trong vùng aria-live; mã đúng sau đó: "Đã xác minh." rồi vào `/workspace-select`.
- Console: 0 lỗi, trừ một dòng 401 là phản hồi mã sai của server (trình duyệt tự ghi). Bỏ qua `ERR_NETWORK_CHANGED` của font Google do mạng máy.
- Cờ phiên: tải lại trong cùng phiên thì logo không còn `data-assemble`.
- Giảm chuyển động (OS): `[data-auth-graph]` có 0 phần tử.
So với mockup: bố cục, màu, cỡ chữ lockup (19px), đồ thị, hàng hoạt động và lớp phủ "vừa tải lên" khớp. Ảnh tĩnh không chứng minh được thời lượng; các giá trị kiểm bằng test đơn vị.

## Deviations so với mockup / yêu cầu

1. Chạm hai file ngoài danh sách: `docs/specs/sign-in-screens/spec.md` (câu "wrong code = fade, no transform" mâu thuẫn với thiết kế đã duyệt; CLAUDE.md bắt cập nhật spec cùng lúc) và không đổi guard `auth-source.test.ts` (không dùng chữ "shake" trong code nên guard vẫn đúng; DESIGN.md §7 mô tả rung là ngoại lệ).
2. Không làm: dải đồ thị mờ trên điện thoại (bạn chỉ yêu cầu brand pane desktop), xung chạy trên cạnh mỗi 3,2s (không có trong quyết định), vòng loé nút đích logo khi mã đúng, số rơi từ phải sang trái sau mã sai (ô vẫn xoá ngay như trước, giữ hành vi cũ).
3. Nhãn "vừa tải lên" đặt sau giờ ("Vừa xong  vừa tải lên") thay vì sau tên, vì nhãn mờ đi vẫn chiếm chỗ và để lại khoảng trống giữa dòng.
4. Chuyển bước dùng `AnimatePresence mode="wait"` (cũ ra rồi mới vào), không chồng nhau như mockup; email bay xuất phát khi bước mã mount, nên có khoảng ngắn không thấy chữ trong lúc bước cũ thoát.
5. "Crossfade sang màn sau": sau khi giữ 700ms thì chuyển route, màn sau fade 160ms theo layout `_auth` hiện có; màn mã không tự fade ra.
6. Mã đúng giữ ~700ms trước khi đi tiếp (để huy hiệu kịp hạ cánh): đăng nhập chậm hơn ~0,7s so với trước.
7. Không thêm `manifest.webmanifest` (chưa có trong repo, ngoài phạm vi); icon maskable và 192/512 đã sẵn để nối vào khi làm PWA.
8. Chưa đổi: "Nexus Hub" trong `ProfileTab.tsx` ("Thoát khỏi Nexus Hub trên trình duyệt này.") và comment trong `index.css`/chuỗi ngoài màn đăng nhập (không thuộc danh sách được sửa).
9. `lang` của index.html vẫn "en" (không yêu cầu).

Status: DONE_WITH_CONCERNS
Summary: Logo Nexus (hướng A), favicon/icon, đăng nhập có chuyển động (logo lắp một lần mỗi phiên, trượt 16px, email bay, ô mã pop/rung/gom thành ✓, nền đồ thị 30s rồi đứng) đã làm theo TDD; test auth/brand/primitives xanh, lint sạch, ảnh Playwright không tràn ngang và không lỗi console.
Concerns/Blockers: Sửa thêm `docs/specs/sign-in-screens/spec.md` (ngoài danh sách, theo luật spec của repo); vài mục mockup bỏ có chủ đích (mục Deviations 2 đến 5); test fail ở websocket.store, SpaceView, AssetsScreen thuộc code chat song song hoặc lỗi cũ đã biết, không phải của thay đổi này.
