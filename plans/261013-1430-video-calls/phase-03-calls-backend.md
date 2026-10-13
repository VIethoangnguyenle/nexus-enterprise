---
phase: 3
title: "Backend cuộc gọi: dữ liệu, quyền, vé, webhook, thẻ"
status: pending
priority: P1
effort: "2-2.5d"
dependencies: [1]
---

# Phase 03: Backend cuộc gọi (service `messaging`)

## Overview
Cuộc gọi thuộc kênh, nên nằm trong `messaging` (đã có kênh, quyền, thẻ, WS, đẩy). Thêm vòng đời cuộc gọi, cấp vé
LiveKit sau kiểm quyền, nhận webhook, thẻ cuộc gọi trong luồng, đá người mất quyền.

## Requirements
- **Migration** `data/migrations/052_calls.sql` (sau `051_message_search_simple.sql`; đổi số nếu main có migration mới hơn): `calls` (id TEXT PK, channel_id FK
  channels ON DELETE CASCADE, kind `audio|video`, status `ringing|active|ended|missed|declined|cancelled`, started_by FK
  users, message_id FK messages (thẻ), created_at, started_at, ended_at, ring_expires_at); unique partial index một cuộc
  gọi `ringing|active` mỗi kênh; `call_participants` (call_id, user_id, joined_at, left_at, PK (call_id, user_id,
  joined_at)). Backup → `make db-migrate` → kiểm `\d calls`.
- **REST** (JWT như các route khác):
  - `POST /api/channels/:chId/calls` body `{kind}` — `authorizeChannel(ngac.OpWrite)`; đã có cuộc gọi đang mở → trả
    cuộc gọi đó (idempotent, không tạo mới); DM → status `ringing`, `ring_expires_at = now+45s`; nhóm → `active`. Tạo
    phòng LiveKit (`RoomService.CreateRoom`, `name = call id`, `max_participants 20`, `empty_timeout 30`). Tạo thẻ =
    tin hệ thống `message_type = "call"`, `linked_entity_type = "call"`, `linked_entity_id = call id` (đường thẻ của
    chat-cards). Trả `{call, token, url}` cho người gọi.
  - `POST /api/calls/:callId/token` — kiểm lại `OpWrite` trên kênh của cuộc gọi; cuộc gọi phải `ringing|active`; phòng
    đủ 20 → 409 `call_full`. Vé: identity = user id, name = tên hiển thị, grant `RoomJoin` + `Room` + `CanPublish`,
    `CanPublishData`, `CanSubscribe`, `SetValidFor(10m)`; trả `{token, url: LIVEKIT_PUBLIC_URL}`.
  - `GET /api/calls/:callId` — trạng thái + người đang trong (tên, avatar từ users), cho thẻ và kiểm tra trước.
  - `POST /api/calls/:callId/decline` (người được gọi trong DM) và `/cancel` (người gọi, khi còn `ringing`).
  - Route mới `/api/calls` → `frontend/vite.config.js` (8183) và nginx regex `^/api/(channels|messages|dms|threads|
    notifications|polls|tasks|calls)`; cập nhật danh sách route trong `docs/deployment.md`.
- **Webhook** `POST /internal/livekit/webhook` (không dưới `/api`, không JWT, không route công khai): xác thực bằng
  `webhook.ReceiveWebhookEvent` (`livekit/protocol/webhook` + auth provider với key/secret); sai chữ ký → 401.
  - `participant_joined`: **kiểm lại quyền** người đó trên kênh; mất quyền → `RemoveParticipant`; được → ghi
    `call_participants`, DM `ringing` → `active`.
  - `participant_left`: ghi `left_at`.
  - `room_finished`: `ended`, `ended_at`; cập nhật thẻ (thời lượng, số người) qua đường sửa tin/WS hiện có.
- **Hết giờ đổ chuông:** vòng lặp nền trong `messaging` mỗi 5s: `UPDATE calls SET status='missed' WHERE
  status='ringing' AND ring_expires_at < now() RETURNING ...` (an toàn khi nhiều bản chạy) → xoá phòng, cập nhật thẻ
  "nhỡ", báo realtime (phase 04 nối).
- **Đá người mất quyền:** trong consumer realtime hiện có (`internal/events/realtime_consumer.go`): `KindMemberRemoved`
  của kênh hoặc workspace → tìm cuộc gọi đang mở của người đó trong các kênh liên quan → `RemoveParticipant`.
- **Xem trước:** `ConversationActivity` (kind) thêm `call` → frontend hiện "📹 Cuộc gọi video · 32 phút" / "Cuộc gọi nhỡ".
- **Proto:** `Call`, `CallParticipant` trong `messaging.proto`; `make proto` + `cd frontend && npm run proto:gen`.
- **Client LiveKit:** gói `internal/livekit` (bọc `server-sdk-go/v2` `RoomServiceClient` sau interface để test bằng fake).
- Không ID trên UI: room name = call id chỉ nội bộ; tên hiển thị lấy từ users.

## Related Code Files
- Create: `data/migrations/052_calls.sql`; `backend/services/messaging/internal/store/calls.go` (+ DB test);
  `domain/calls.go` (+ test), `domain/calls_webhook.go` (+ test); `rest/calls.go` (+ test);
  `internal/livekit/client.go` (+ fake)
- Modify: `rest/handler.go` (routes, webhook ngoài nhóm `/api`), `cmd/main.go` (cấu hình LiveKit, vòng lặp hết giờ),
  `internal/events/realtime_consumer.go` (đá người), `domain/activity.go` (kind call), `backend/proto/messaging/messaging.proto`,
  `backend/services/messaging/go.mod` (deps pin), `frontend/vite.config.js`, `deploy/nginx/default.conf`
- Create spec `docs/specs/chat-calls/spec.md`; modify `chat-cards`, `chat-activity-feed`; README index

## Implementation Steps
1. Migration (backup trước), store + DB test (một cuộc gọi mở mỗi kênh, hết giờ idempotent).
2. Domain test trước: deny (không thành viên bắt đầu / lấy vé; bị gỡ rồi lấy vé; cuộc gọi `ended` → 409; đầy 20 → 409),
   allow; webhook: chữ ký sai, `participant_joined` của người mất quyền → RemoveParticipant được gọi (fake).
3. REST + proxy + nginx. 4. Consumer đá người. 5. Spec.

## Success Criteria
- [ ] `make test s=messaging`, `make check-layering`, `make check-ngac`, `make build-check` xanh; test deny đủ danh sách.
- [ ] Trên dev stack (LiveKit phase 02): tạo cuộc gọi, lấy vé, webhook ghi người tham gia, phòng đóng → thẻ "kết thúc".

## Risk Assessment
- Webhook tới chậm/mất → trạng thái lệch: khi lấy vé/GET, đối chiếu `RoomService.ListParticipants` nếu phòng rỗng quá
  `empty_timeout` thì coi là kết thúc.
- Nhiều thiết bị một người: identity = user id → thiết bị sau đẩy thiết bị trước ra (hành vi LiveKit) — chấp nhận, ghi
  vào spec ("một người một chỗ trong cuộc gọi").
