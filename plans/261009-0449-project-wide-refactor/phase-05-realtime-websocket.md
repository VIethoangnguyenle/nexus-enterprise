---
phase: 5
title: "Realtime over WebSocket"
status: completed
priority: P1
effort: 3-4d
dependencies: [2, 4]
---

# Phase 05 — Realtime over WebSocket

## Overview
Yêu cầu: mọi thay đổi do người khác gây ra phải hiện trên màn hình đang mở **mà không cần
refresh**, qua WebSocket (`:8081`, hub trong messaging, wire type `backend/proto/messaging/ws.proto`).
Hiện chỉ chat là realtime thật; drive, admin, workspace, document gần như không. Phần bảo mật của
WebSocket (subscribe check, broadcast theo tenant) đã nằm ở phase 02 — phase này giả định nó xong.

## Key insights (đã kiểm chứng)
- **Drive realtime chưa từng chạy:** `DriveObjectEvent`/`DrivePermEvent` có trong proto (field 13,
  14) và FE có handler (`websocket.store.ts:381,401`), nhưng **không backend nào phát** — drive
  service không có producer, messaging không consume topic drive. Spec `drive-realtime-sync` ghi
  "Matches code" là sai → phải thêm `## Status` ghi divergence ngay khi bắt đầu phase.
- Messaging consumer chỉ nghe `asset.lifecycle`, `asset.request`, `asset.assignment`,
  `approval.events` (`messaging/internal/events/consumer.go:87`).
- Không có event cho: channel đổi tên/xoá, thành viên channel/workspace thêm/xoá, đổi role,
  department, document upload/xoá, asset type, notification read-state từ thiết bị khác.
- Fan-out hiện tại: theo channel (`channel:*`), theo user (`user:*`), hoặc **toàn bộ** (presence,
  approval). Không có scope theo workspace.
- FE invalidation dùng key literal và đã có 2 key sai (poll, drive root) — phase 04 đưa về key
  factory; phase này dựa vào factory đó.
- Reconnect có backoff + re-subscribe channel, nhưng spec `drive-realtime-sync` yêu cầu
  invalidate drive + permission cache khi reconnect — cần kiểm chứng; các domain khác không có
  resync sau khi rớt kết nối (event mất trong lúc offline).

## Requirements
- **Mô hình fan-out 3 cấp:** user, channel, workspace. Mọi event mang `tenant_id` +
  `workspace_id`; hub chỉ giao cho client cùng tenant đã subscribe workspace đó (subscribe
  workspace cũng phải qua policy check `read`).
- **Nguồn event:** mỗi service ghi state → produce lên Redpanda topic `<domain>.events` sau khi
  commit; messaging consumer → hub. Không gọi hub đồng bộ từ request path.
- **Event phải có:**
  - drive: object created/updated/deleted/moved (cả parent cũ lẫn mới), share created/revoked
  - channel: created/renamed/archived, member added/removed
  - workspace: member added/removed, role changed, department changed
  - document: uploaded/deleted/approved
  - approval: tới requester + approver liên quan (không broadcast)
  - asset: giữ event hiện có, scope theo workspace
  - permission: khi association/assignment của user đổi → event tới chính user đó để FE xoá
    permission cache (nối với EPP ở phase 03)
- **Payload chỉ mang ID + loại thay đổi**; FE invalidate query qua key factory rồi refetch, không
  vá cache bằng dữ liệu trong event (trừ chat message/reaction như hiện nay, để mượt).
- **Resync:** mỗi event có `seq` theo workspace; FE thấy hở seq hoặc reconnect → invalidate các
  query của workspace đang mở.
- **Presence** chỉ trong tenant, debounce đổi trạng thái.
- Proto đổi → `make proto` **và** `cd frontend && npm run proto:gen` (CLAUDE.md §5).
- Hiệu ứng khi dữ liệu đổi do người khác (hàng mới, hàng bị xoá) theo motion tokens ở phase 06.

## Related files
- modify `backend/proto/messaging/ws.proto` (+ generated Go và `frontend/src/generated/`)
- create `backend/services/{drive,workspace,document}/internal/events/producer.go`
- modify `backend/services/messaging/internal/events/consumer.go`, `backend/services/messaging/internal/grpc/hub.go`
- modify `backend/services/{approval,asset}/internal/events/producer.go` (thêm tenant/workspace)
- modify `frontend/src/stores/websocket.store.ts` (tách reducer thành map event → key factory)
- modify `docs/specs/drive-realtime-sync/spec.md`; create `docs/specs/realtime-event-delivery/spec.md`

## Implementation steps
1. Ghi divergence vào `drive-realtime-sync`; viết spec `realtime-event-delivery` (fan-out, scope,
   seq, resync).
2. Test hub (Go): client tenant A không nhận event tenant B; client chưa subscribe workspace không
   nhận event workspace; seq tăng đơn điệu.
3. Proto + regenerate cả hai phía.
4. Producer từng service, test produce sau commit (không produce khi rollback).
5. FE: reducer thuần (event → danh sách key cần invalidate), test vitest cho từng loại event.
6. E2E 2 trình duyệt (Playwright): user A đổi tên folder / thêm member / gửi approval → màn hình
   user B cập nhật ≤ 1s, không refresh.

## Success criteria
- [~] Mỗi màn hình trong `frontend/src/routes/` phản ánh thay đổi của user khác không cần refresh. Đã chạy thật: drive, văn bản, danh bạ/admin (người), phê duyệt, chat. Chưa có event: trạng thái đã đọc thông báo từ thiết bị khác, asset-type riêng lẻ (xem report).
- [x] Không event nào vượt tenant (test).
- [x] Rớt mạng 30s rồi nối lại → dữ liệu đúng (test resync).

## Risks
- Nhiều event → bão refetch. Gộp invalidation theo frame (debounce ~100ms) trong reducer.
- Producer sau commit nhưng service chết trước khi produce → mất event; chấp nhận nhờ resync
  theo seq, hoặc outbox nếu cần (quyết định khi làm).
