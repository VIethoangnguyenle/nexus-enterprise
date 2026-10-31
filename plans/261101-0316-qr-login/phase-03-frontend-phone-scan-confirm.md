# Phase 03 — Frontend: phone confirm route + scanner

Design: `design/mockups/qr-login.html` sections 3, 4, 5.

## Routes

- `frontend/src/routes/qr.$token.tsx` (new, outside `_workspace` and `_auth` so both camera-app
  and in-app entry land here). Not signed in → the "Đăng nhập trên điện thoại trước" screen,
  whose button goes to `/login` with a `redirect` back to `/qr/<token>`. Check how
  `lib/auth-guards.ts` and the OTP/Google completion paths honour a redirect today; if they do
  not, add a same-origin-only `redirect` search param (reject anything not starting with `/`
  or starting with `//`). Wait for the boot refresh in `__root.tsx` before deciding "signed out".
- Signed in → call `scan` once on mount, show device card (browser + OS, "Tạo mã lúc HH:mm"),
  account card (avatar, display name, tenant name, role), the permanent warning, Đăng nhập /
  Huỷ. × and Huỷ both call `decline`. Success screen → Xong returns to the previous route or
  Trang chủ. `qr_unavailable` → "Mã này không còn dùng được"; `tenant_access_denied` →
  "Không đăng nhập được vào <tenant>" with "Đổi tổ chức".
- `frontend/src/routes/_workspace/scan.tsx` (new) — full-screen scanner (dark camera surface,
  documented exemption). `qr-scanner` (npm; uses `BarcodeDetector` where available, worker
  fallback otherwise): start on mount, stop on unmount and on `visibilitychange` hidden.
  Torch button only when `hasFlash()`. "Chọn ảnh có mã" uses `QrScanner.scanImage(file)`.
  Decoded value accepted only if `new URL(v).origin === location.origin` and the path matches
  `/qr/<token>`; then `navigate` to it. Otherwise show the inline "Đây không phải mã đăng nhập
  Nexus" line and keep scanning (vibrate 30 ms where supported). Permission denied →
  the "Chưa có quyền dùng camera" screen with Thử lại / Chọn ảnh có mã.
  `routeTree.gen.ts` is generated — regenerate, never hand-edit.

## Entry point

Add "Quét mã QR · Đăng nhập Nexus trên máy tính" to the + sheet (the "Trò chuyện mới" sheet in
the phone shell; locate it from `components/patterns/PhoneSearchBar.tsx` / `MobileNav.tsx`).
Also add the same row to the account sheet (`components/patterns/AccountSheet.tsx`, after
Tạm dừng thông báo; decided by Zane 2026-11-01). Shown only below 1024 px.

## API

`qrApi.scan/approve/decline` in `frontend/src/api/auth.ts` via `apiFetch` (JWT).

## Tests (vitest)

- confirm route: renders device/account from the scan response, never the token; approve and
  decline call the right endpoints; error codes map to the two error screens.
- scanner URL filter: accepts same-origin `/qr/<token>`, rejects other origins, `javascript:`,
  `//evil`, and plain text.
- redirect param: only same-origin paths survive.

## Done when

`npm test`, `npm run lint`, `npm run build` green; real-phone check on the deployed build
(camera needs HTTPS; decided by Zane 2026-11-01: test on the deploy, not a tunnel).
