---
phase: 4
title: "OneSignal phía frontend"
status: pending
priority: P1
effort: "1d"
dependencies: [2]
---

# Phase 04: OneSignal phía frontend

## Overview
Gắn OneSignal Web SDK v16, liên kết thiết bị với người dùng đăng nhập, xin quyền theo thiết kế.

## Requirements
- Cấu hình: `VITE_ONESIGNAL_APP_ID` (công khai); không có App ID → tắt toàn bộ push UI (dev, test).
- Khởi tạo SDK một lần (`react-onesignal` hoặc script v16), `serviceWorkerPath` trỏ SW của app; SW app
  `importScripts('https://cdn.onesignal.com/sdks/web/v16/OneSignalSDK.sw.js')`.
- Đăng nhập: `OneSignal.login(users.id)`; đăng xuất / đổi workspace: `OneSignal.logout()`.
- Xin quyền chỉ từ cú chạm của người dùng (thẻ "Nhận thông báo…" → `Notifications.requestPermission()`).
- Bấm thông báo: payload mang đường dẫn nội bộ (`/messages/<kênh>?thread=<tin>`, không hiện ID) → SW focus tab đang
  mở hoặc mở mới tới đúng route.
- Cài đặt → Thông báo: bật/tắt trên thiết bị (`optIn/optOut`), loại sự kiện + giờ yên lặng gọi API phase 05.
- CSP/Traefik cho phép `cdn.onesignal.com`, `api.onesignal.com`, `onesignal.com`.

## Related Code Files
- Create: `frontend/src/lib/push.ts`, `frontend/src/components/settings/NotificationSettings.tsx`, `PushPrompt.tsx` (+ test)
- Modify: `frontend/src/sw.ts`, nơi đăng nhập/đăng xuất (`stores`/auth), trang Cài đặt
- Spec: `docs/specs/web-push/spec.md`

## Success Criteria
- [ ] Thiết bị đăng ký hiện trong OneSignal dashboard với `external_id` đúng; đăng xuất thì gỡ liên kết.
- [ ] Không App ID → không tải SDK, không lỗi console; gates xanh.
