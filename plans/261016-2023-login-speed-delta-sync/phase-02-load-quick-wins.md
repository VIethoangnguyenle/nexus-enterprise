---
phase: 2
title: "Tải trang: gzip, bỏ Manrope, bỏ preload gọi video"
status: completed
priority: P1
effort: "done"
dependencies: []
---

# Phase 02: Quick-win tải trang

## Đã làm (agent, nhánh `worktree-agent-a89cd6daaa0c043c8`, commit `99f774b`, chưa review xong)
- `deploy/nginx/default.conf` (production) + `frontend/nginx.conf`: gzip level 6, min 1024, vary, proxied any, types
  text/js/css/json/manifest/svg/font; giữ nguyên quy tắc cache.
- `frontend/index.html`: bỏ Manrope (grep: không dùng ở đâu).
- `CallProvider.tsx` import `EMPTY_SNAPSHOT` từ `lib/call-room` kéo cả LiveKit vào entry → chuyển hằng sang
  `lib/call-model.ts`; build không còn modulepreload livekit/call-room.
- Agent báo: `npm test` 2800 pass, lint sạch, `nginx -t` đạt cả hai config.

## Còn lại
- Controller review diff + chạy lại test (phase 10). Sau deploy đo `Content-Encoding` và kích thước tải thật.

## Đo sau deploy (2026-10-16 22:49, run 38926146050)
- `Content-Encoding: gzip`; `/assets/index-CZYLMTF4.js` tải 663 459 B (trước 2 428 120 B).
- `index.html` không còn Manrope, không còn modulepreload.
