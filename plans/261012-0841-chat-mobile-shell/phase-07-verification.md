---
phase: 7
title: "Kiểm chứng"
status: completed
priority: P0
effort: "0.5-1d"
dependencies: [5, 6]
---

# Phase 07: Kiểm chứng

## Overview
Chứng minh acceptance của plan trên stack thật trước khi gọi là xong.

## Implementation Steps
1. Gate: `make build-check`, `make check-layering`, `make check-ngac`, `make test s=messaging`; `cd frontend && npm test`,
   `npm run lint`, typecheck, `npm run build`.
2. `make dev` (theo dõi PID, dừng khi xong): đăng nhập 2 người (OTP 999999), tạo nhóm + tin + trả lời thread.
   - Mở app lạnh ở 390px: Trang chủ có xem trước và dòng thread.
   - Gỡ người B khỏi nhóm → `/api/channels/activity`, `/api/messages/search`, `/api/threads/followed` của B không còn
     nhóm đó (curl + UI).
3. Playwright 390×844, sáng/tối: Trang chủ (+ lọc chưa đọc), Trực tiếp, Mục, Thêm, ngăn kéo (tổ chức, cá nhân, danh
   sách workspace), một module khác (Phê duyệt), Tìm, nút +, sheet tài khoản, màn Thread → `plans/reports/chat-mobile-shell-screens/`;
   so với mockup; kiểm không cuộn ngang, hàng cuối không bị che, safe-area (giả lập iPhone).
4. Màn ≥ 1024: ảnh trước/sau giống nhau (không đổi máy tính).
5. Code review (`ak:code-review`), sửa phát hiện; cập nhật trạng thái mọi phase, plan, `docs/specs/README.md`; viết report
   `plans/reports/` và handoff nếu dừng giữa chừng.

## Success Criteria
- [ ] Mọi gate xanh; ảnh khớp mockup; deny kiểm trên stack thật có ghi lại.
- [ ] Không còn tham chiếu tới `MobileTopBar`/`MobileMoreSheet`.
