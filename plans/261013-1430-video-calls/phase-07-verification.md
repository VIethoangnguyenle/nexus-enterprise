---
phase: 7
title: "Kiểm chứng: thiết bị thật, mạng chặn UDP, tải 20 người, deny"
status: pending
priority: P1
effort: "1d"
dependencies: [6]
---

# Phase 07: Kiểm chứng

## Implementation Steps
1. Gate: `make build-check`, `make check-layering`, `make check-ngac`, `make test s=messaging`, `npm test`, lint,
   typecheck, build, `deploy/check-compose-env.sh`.
2. Stack thật (`make dev`, theo dõi PID): 2–3 người (OTP 999999). Deny bằng curl: người ngoài nhóm `POST
   /api/channels/:id/calls` và `/api/calls/:id/token` → 403/404; gỡ người đang trong cuộc gọi → rời phòng ≤ 5s (đo).
3. Thiết bị: Chrome máy tính ↔ Safari iPhone (PWA đã cài) ↔ Chrome Android: gọi DM, trả lời từ thông báo đẩy, nhóm 3
   người, trình bày từ máy tính, chạm để bật âm thanh trên iPhone. Ghi kết quả từng cặp.
4. Mạng chặn UDP (máy tính: chặn UDP ra ngoài bằng firewall cục bộ) → vẫn kết nối qua TURN/TLS 443 (xem
   `chrome://webrtc-internals` candidate relay).
5. Tải: `lk load-test` 20 người (camera giả, simulcast) lên LiveKit trên VPS (sau deploy) hoặc máy cùng cấu hình; ghi
   CPU, RAM, băng thông ra/vào, chất lượng → `plans/reports/video-call-load-test.md`; quyết định giới hạn camera nếu cần.
6. Playwright máy tính + 390px sáng/tối (fake media) so mockup → `plans/reports/video-call-screens/`.
7. `ak:code-review` (nhấn mạnh webhook, cấp vé, đá người), sửa; cập nhật status mọi phase, specs README.

## Success Criteria
- [ ] Mọi acceptance trong `plan.md` có bằng chứng ghi lại.
