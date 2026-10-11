---
phase: 2
title: "Nền tảng PWA"
status: completed
priority: P1
effort: "1-1.5d"
dependencies: [1]
---

# Phase 02: Nền tảng PWA

## Overview
App cài được vào màn hình chính và có khung app khi mất mạng.

## Requirements
- `vite-plugin-pwa` (chế độ `injectManifest`, SW tự viết để gộp OneSignal ở phase 04) hoặc SW thủ công nếu plugin
  không hợp Vite 8 — kiểm khi bắt đầu.
- `manifest.webmanifest`: `name` "Nexus", `short_name`, `start_url` "/", `display` "standalone", `theme_color` /
  `background_color` theo token, icons 192/512/maskable; `frontend/index.html` thêm `link rel=manifest`,
  `apple-touch-icon`, `meta theme-color` (light/dark), `apple-mobile-web-app-capable`.
- Precache khung app (HTML/JS/CSS/fonts); **không** cache phản hồi `/api/*` hay WebSocket; điều hướng khi mất mạng →
  khung app + thanh "Mất kết nối" (đọc `navigator.onLine` + lỗi WS).
- Cập nhật: SW mới chờ → thanh "Có bản mới — Tải lại" (không tự reload giữa lúc đang soạn).
- Header deploy: `Service-Worker-Allowed`, không cache `sw.js` ở Traefik/nginx (`docs/deployment.md`).

## Related Code Files
- Modify: `frontend/package.json`, `frontend/vite.config.js`, `frontend/index.html`
- Create: `frontend/src/sw.ts`, `frontend/public/icons/*`, `frontend/src/components/app/{OfflineBar,UpdateBar}.tsx` (+ test)
- Create: `docs/specs/pwa-install/spec.md`

## Implementation Steps
1. Test đỏ (vitest): OfflineBar hiện khi offline; UpdateBar gọi `skipWaiting` khi bấm.
2. Hiện thực; `npm run build`; Lighthouse PWA trên bản build (Playwright + Chromium có sẵn).
3. Gates: test, lint, typecheck, bundle.

## Success Criteria
- [x] Lighthouse "installable" đạt; mất mạng vẫn mở được khung app; không cache dữ liệu API.
