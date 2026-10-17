---
phase: 4
title: "Ảnh nhóm: backend messaging"
status: pending
priority: P1
effort: "1d"
dependencies: [3]
---

# Phase 04: Ảnh nhóm — backend

## Overview
Nhóm (kênh, không phải DM) có ảnh riêng: tải lên, bỏ, phục vụ, báo realtime. Mẫu y như ảnh người dùng (migration 055).

## Requirements
- Migration `data/migrations/056_channel_avatars.sql`: bảng `channel_avatars` (token mờ, 3 cỡ JPEG, `channel_id` FK,
  `created_at`) + cột `channels.avatar_url TEXT DEFAULT ''`, ghi cùng transaction như `user_avatars`. Bảng ở schema
  của `channels` (kiểm `init.sql` — channels ở `public` hay schema tenant; theo đúng chỗ đó).
- REST (messaging, flat `/api/channels` đã proxy tới 8183 trong `vite.config.js:103`):
  `PUT /api/channels/:chId/avatar` (multipart, ≤ 5 MB, JPG/PNG), `DELETE /api/channels/:chId/avatar`,
  `GET /api/channels/avatars/:token[/:size]` (công khai theo token mờ như auth, có limiter).
- Quyền: `manage` trên OA nội dung kênh — cùng kiểm tra với `UpdateChannel` (`domain/channels.go:248`), qua
  `policyclient`, fail-closed. DM → `ErrInvalidInput` ("Tin nhắn trực tiếp không có ảnh nhóm").
- Rate limit + cổng đồng thời như auth (`takeAvatarUpload`, `acquireAvatarGate`) — dùng Redis key riêng cho kênh.
- Realtime: phát cùng sự kiện đổi kênh mà rename đang phát (tìm trong `domain/channels.go` UpdateChannel → realtime),
  payload có `avatar_url`. Không tạo loại sự kiện mới nếu loại hiện có đủ.
- `avatar_url` có trong mọi chỗ trả kênh (list, get, search, conversations) — proto `Channel` thêm trường nếu cần →
  `make proto` **và** `cd frontend && npm run proto:gen`.
- Lỗi theo `httputil.MapDomainError`; 500 không lộ nguyên nhân.

## Implementation Steps
1. Backup DB dev; viết migration; `make db-migrate`; `psql` kiểm bảng + cột.
2. Test trước (domain): allow (`manage`) → lưu, `avatar_url` đổi; **deny** (member chỉ read/write) → 403, không ghi;
   DM → 400; ảnh hỏng/quá lớn → mã lỗi như auth; bỏ ảnh → `avatar_url` rỗng, token cũ 404.
3. Test REST: route, multipart, limiter, mã lỗi (`avatar_not_image`, `avatar_too_large`, `rate_limited`).
4. Store + domain + REST; gọi `backend/pkg/imagenorm`.
5. Realtime + test sự kiện có `avatar_url`.
6. Spec mới `docs/specs/chat-channel-avatar/spec.md`.
7. `make build-check`, `make check-layering`, `make check-ngac`, `make test s=messaging`.

## Related Code Files
- Create: `data/migrations/056_channel_avatars.sql`, `backend/services/messaging/internal/{store,domain,rest}/channel_avatar*.go` (+ test), `docs/specs/chat-channel-avatar/spec.md`
- Modify: `backend/services/messaging/internal/rest/handler.go` (routes), `domain/channels.go` (trả `avatar_url`), proto nếu cần

## Success Criteria
- [ ] Migration đã áp và quan sát trên DB đang chạy.
- [ ] Test deny xanh; người không có `manage` không đổi được.
- [ ] Sự kiện realtime mang `avatar_url`; proto hai phía đã sinh lại (nếu đổi).

## Risk Assessment
- Số migration 056 có thể bị chiếm nếu nhánh khác thêm trước → lấy số kế tiếp lúc làm.
- Trường mới trong proto quên sinh phía frontend → kiểm `git diff frontend/src/generated`.
