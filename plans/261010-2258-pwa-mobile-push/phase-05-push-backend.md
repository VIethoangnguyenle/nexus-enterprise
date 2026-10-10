---
phase: 5
title: "Gửi push từ backend"
status: pending
priority: P1
effort: "2d"
dependencies: [4]
---

# Phase 05: Gửi push từ backend

## Overview
Backend gửi push qua OneSignal REST API khi có sự kiện đáng báo, tôn trọng tuỳ chọn và quyền của người nhận.

## Requirements
- Gói dùng chung `backend/pkg/push` (client OneSignal: `POST https://api.onesignal.com/notifications`, header
  `Authorization: Key <ONESIGNAL_REST_API_KEY>`, `target_channel: "push"`, `include_aliases.external_id`), timeout,
  retry có giới hạn, log không chứa nội dung tin; không cấu hình → no-op.
- Điểm gửi: pipeline thông báo hiện có của messaging (`internal/events/consumer.go`, `notifications_from_events`) —
  mỗi thông báo trong app được tạo → cân nhắc push. Sự kiện: tin nhắn trực tiếp, nhắc tên, trả lời trong thread
  đang theo dõi (phase 02–03 plan chat), việc được giao, đề nghị cần bạn duyệt.
- Không gửi khi: người nhận đang kết nối WS và đang xem đúng kênh/thread (hub biết); trong giờ yên lặng; loại sự
  kiện bị tắt; **người nhận không còn `read` trên OA của kênh/đối tượng** (kiểm lại qua `policyclient` lúc gửi).
- Gộp: nhiều tin cùng kênh trong 30s → một push "3 tin mới từ …" (`collapse_id` theo kênh).
- Bảng `push_preferences(user_id, kind, enabled)` + `quiet_hours`; REST `GET/PUT /api/me/push-preferences`
  (thêm vào `vite.config.js`).
- Nội dung: tiêu đề = tên nhóm/người gửi, thân = đoạn đầu văn bản thuần (tuỳ câu hỏi mở trong plan.md).

## Related Code Files
- Create: `backend/pkg/push/*.go` (+ test với HTTP server giả), `data/migrations/0NN_push_preferences.sql`
- Modify: `backend/services/messaging/internal/{events,domain,store,rest}/*`, `cmd/main.go` (env), `docs/deployment.md`,
  `docker-compose*.yml` / env mẫu (chỉ tên biến), `docs/specs/notifications/spec.md`, `docs/specs/web-push/spec.md`

## Implementation Steps
1. Test đỏ: client gửi đúng request; no-op khi thiếu cấu hình; gộp 30s.
2. Test đỏ **deny**: người bị gỡ khỏi kênh không được push; loại bị tắt không push; đang xem kênh không push.
3. Backup DB → migration → `make db-migrate` → kiểm chứng; hiện thực; gates `make test s=messaging`, `make check-layering`.

## Success Criteria
- [ ] Push thật tới thiết bị test ≤ 10s; test deny xanh; không secret nào trong git.
