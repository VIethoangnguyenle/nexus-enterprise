# Session Dialogue: approval-realtime-events

> Audit trail dựa trên evidence. Chỉ Orchestrator ghi. Human đọc.
> KHÔNG narrative. KHÔNG roleplay. CHỈ structured reasoning.

---

## Phase 1: CEO — Phạm vi

Quyết định:
- Tái sử dụng hạ tầng hiện có (Redpanda + messaging WS hub) thay vì xây mới
- Bằng chứng: file: `messaging/internal/grpc/hub.go`, vị trí: L507-533, quan sát: `BroadcastAssetUpdated()` đã tồn tại với pattern broadcast qua WS cho tất cả user
- Bằng chứng: file: `messaging/internal/events/consumer.go`, vị trí: L62-82, quan sát: consumer đã subscribe 3 Kafka topics, chỉ cần thêm 1 topic mới
- Xác nhận: đã kiểm tra `hub.go` và `consumer.go` — pattern tồn tại và hoạt động cho asset module, kết luận: xác nhận có thể tái sử dụng
- Trạng thái: VERIFIED
- Độ tin cậy: HIGH
- Phương án loại bỏ: WebSocket server riêng cho approval — vi phạm kiến trúc single-hub, thêm complexity không cần thiết

Quyết định:
- Phạm vi giới hạn: 3 file backend + 1 file frontend, không service mới
- Bằng chứng: file: `architecture.md`, vị trí: Files Changed table, quan sát: chỉ cần modify 6 file + 1 file mới
- Xác nhận: đã đếm số file cần thay đổi dựa trên kiến trúc, kết luận: xác nhận scope hợp lý
- Trạng thái: VERIFIED
- Độ tin cậy: HIGH

Chuyển giao:
- CEO → BA: proposal.md với ranh giới phạm vi, đã validate scope vs hạ tầng hiện có

---

## Phase 2: BA — Đặc tả

Quyết định:
- 3 specs: event-publishing, event-consumption, frontend-realtime
- Bằng chứng: file: `architecture.md`, vị trí: Data Flow diagram, quan sát: data flow có đúng 3 tầng: producer → consumer → UI
- Xác nhận: đã kiểm tra data flow diagram — mỗi spec map 1:1 với 1 tầng, kết luận: xác nhận
- Trạng thái: VERIFIED
- Độ tin cậy: HIGH

Quyết định:
- Sử dụng pattern fire-and-forget cho producer (không cần delivery guarantee)
- Bằng chứng: file: `frontend/src/stores/websocket.store.ts`, vị trí: L437-451 (`resyncAfterReconnect`), quan sát: reconnect đã invalidate tất cả query keys — event bị mất sẽ được phục hồi
- Xác nhận: đã kiểm tra `invalidateQueries` là idempotent (TanStack Query built-in), kết luận: xác nhận
- Trạng thái: VERIFIED
- Độ tin cậy: HIGH
- Phương án loại bỏ: transactional outbox — over-engineering cho cache invalidation

Chuyển giao:
- BA → User Checkpoint: specs đã lock (hash: `04c1fcd8...`), đã validate 3 specs cover toàn bộ data flow

---

## Phase 3: User Checkpoint

- User phê duyệt specs ✅

---

## Phase 4: SA — Kiến trúc

Quyết định:
- Event publish ở tầng REST handler (SAU KHI domain commit), không trong domain service
- Bằng chứng: file: `approval/internal/rest/handler.go`, vị trí: L290-301, quan sát: `CreateApprovalRequest` trả về `*domain.Request` đầy đủ field (ID, TemplateName, Status, ScopeOAID)
- Bằng chứng: file: `approval/internal/domain/execution.go`, vị trí: L50-111, quan sát: domain layer thuần logic, không có side-effect I/O
- Xác nhận: đã kiểm tra domain không import Kafka package — xác nhận tầng domain thuần khiết
- Trạng thái: VERIFIED
- Độ tin cậy: HIGH
- Phương án loại bỏ: publish trong domain service — vi phạm clean architecture (domain không nên có I/O side-effect)

Quyết định:
- Broadcast tới TẤT CẢ user đang kết nối (không target cụ thể)
- Bằng chứng: file: `messaging/internal/grpc/hub.go`, vị trí: L507-533, quan sát: `BroadcastAssetUpdated` dùng pattern broadcast tới tất cả user — cùng use case
- Xác nhận: đã kiểm tra — approval ảnh hưởng nhiều role (approver, requester, department), không có cách xác định chính xác ai cần nhận mà không query thêm
- Trạng thái: VERIFIED
- Độ tin cậy: MEDIUM (có thể tối ưu sau bằng targeted broadcast, nhưng hiện tại acceptable)

Quyết định:
- `ApprovalEvent` là field 16 trong `ServerEnvelope` oneof
- Bằng chứng: file: `proto/messaging/ws.proto`, vị trí: L42-60, quan sát: field cuối hiện tại là `ErrorEvent` = 15, field 16 là slot kế tiếp
- Xác nhận: đã kiểm tra không có field 16 trước đó — xác nhận không conflict
- Trạng thái: VERIFIED
- Độ tin cậy: HIGH

Chuyển giao:
- SA → DEV: architecture.md đã validate, đã kiểm tra tất cả integration points match codebase thực tế

---

## Phase 5: UX — Bỏ qua

Quyết định:
- Không cần thay đổi UI
- Bằng chứng: file: `architecture.md`, vị trí: Files Changed table, quan sát: frontend chỉ modify `websocket.store.ts` (cache invalidation code, không phải UI component)
- Xác nhận: đã kiểm tra không có screen mới, không component mới trong scope
- Trạng thái: VERIFIED
- Độ tin cậy: HIGH

---

## Phase 6: DEV — Triển khai

Quyết định:
- Producer đặt file riêng `events/producer.go` (không gộp vào `consumer.go`)
- Bằng chứng: file: `approval/internal/events/consumer.go`, vị trí: L1-4, quan sát: package comment nói "Kafka consumer for policy change reconciliation" — producer có purpose khác
- Xác nhận: đã kiểm tra consumer có ReconciliationStore dependency, producer không cần — xác nhận tách riêng hợp lý
- Trạng thái: VERIFIED
- Độ tin cậy: HIGH

Quyết định:
- Thay đổi signature `NewHandler` nhận `*events.Producer` param thứ 3
- Bằng chứng: file: `approval/cmd/main.go`, vị trí: L82-86, quan sát: producer khởi tạo cạnh consumer, inject qua constructor — pattern DI hiện có
- Xác nhận: đã kiểm tra `NewConsumer(brokers, store)` dùng cùng pattern constructor injection
- Trạng thái: VERIFIED
- Độ tin cậy: HIGH

Quyết định:
- Thêm interface `ApprovalBroadcaster` trong messaging consumer
- Bằng chứng: file: `messaging/internal/events/consumer.go`, vị trí: L50-53, quan sát: `NotificationCreator` interface đã dùng pattern interface segregation cho hub
- Xác nhận: đã kiểm tra Hub implement cả `NotificationCreator` và sẽ implement `ApprovalBroadcaster` — xác nhận pattern nhất quán
- Trạng thái: VERIFIED
- Độ tin cậy: HIGH

Quyết định:
- `handleApprovalEvent` tạo notification VÀ broadcast WS event
- Bằng chứng: file: `messaging/internal/events/consumer.go`, vị trí: L139-167 (`handleRequestEvent`), quan sát: asset request handler cũng tạo notification cho requester — cùng pattern
- Xác nhận: đã kiểm tra notification lưu DB (offline), WS event cho online — hai kênh bổ sung nhau
- Trạng thái: VERIFIED
- Độ tin cậy: HIGH
- Phương án loại bỏ: chỉ notification (không realtime), chỉ WS (mất offline) — cả hai đều thiếu

Quyết định:
- `resyncAfterReconnect()` thêm `['approval']`
- Bằng chứng: file: `frontend/src/stores/websocket.store.ts`, vị trí: L437-451, quan sát: tất cả module khác (messages, drive, tasks) đều có trong resync list
- Xác nhận: đã kiểm tra — thiếu approval trong resync sẽ gây stale data sau reconnect
- Trạng thái: VERIFIED
- Độ tin cậy: HIGH

### Xác nhận build
- Approval service: ✅ `go build -o /tmp/approval-test ./cmd/` — pass, 0 lỗi
- Messaging service: ✅ `go build -o /tmp/messaging-test ./cmd/` — pass, 0 lỗi
- Frontend: ✅ `npm run build` — 2322 modules, 0 lỗi source code

---

## Deviations

(không có)

---

## Red Flags

| # | Loại | Mô tả | Vị trí |
|---|------|-------|--------|
| — | — | Không phát hiện red flag | — |
