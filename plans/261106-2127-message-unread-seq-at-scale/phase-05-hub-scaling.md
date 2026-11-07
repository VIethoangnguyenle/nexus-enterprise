---
phase: 5
title: "Hub: subscribe Redis theo nhu cầu, reconnect có jitter"
status: pending
priority: P2
effort: "6h"
dependencies: [4]
---
# Phase 5: Hub subscribe theo nhu cầu, reconnect có jitter

## Mục tiêu
Một instance chỉ nhận từ Redis những channel/tenant nó có client; deploy không gây bão reconnect và resync.

## Files
- `backend/services/messaging/internal/grpc/hub_broadcast.go`, `hub.go`: thay `PSubscribe("channel:*", "user:*", …)`
  bằng một `PubSub` động — `Subscribe("channel:<id>")` khi client cục bộ đầu tiên subscribe channel đó,
  `Unsubscribe` khi client cuối rời; tương tự `user:<id>` và `activity:<tenant>` theo client đang kết nối. Giữ
  `PSubscribe` cho các prefix ít lưu lượng (revoke, permission, workspace events) nếu đo thấy không đáng.
- `frontend/src/stores/websocket.store.ts`: backoff reconnect có jitter ngẫu nhiên (0–5 s cho lần đầu, rồi lũy thừa,
  trần 30 s); resync sau reconnect dùng `revalidateChatState` (đã có, trả 304 khi không đổi) trước khi invalidate từng
  khoá; chỉ invalidate `messagesAll` của các channel đang mở, không phải mọi channel.

## Test (viết trước)
- Go: instance A có client ở channel X, instance B không → B không nhận frame của X (đếm message nhận ở subscriber);
  client cuối rời → A unsubscribe; reconnect Redis → subscribe lại đúng tập hiện có.
- Go benchmark (`hub_bench_test.go` từ phase 1): so sánh trước/sau với 2 instance mô phỏng.
- Vitest: lịch reconnect có jitter trong biên; resync không invalidate channel không mở.

## Validation
`make test s=messaging`; `cd frontend && npm test`; số liệu benchmark ghi vào `reports/after.md`.

## Rủi ro / rollback
Subscribe động có đường đua giữa subscribe Redis và tin đến đầu tiên: client đã có cơ chế lấp lỗ bằng sequence/resync,
test phải bao ca này. Rollback: cờ env quay về `PSubscribe` toàn cục.

## Quyết định 2026-11-06: phần backend để lại
Người dùng đồng ý **chưa làm** subscribe Redis theo nhu cầu. Lý do: prod chạy một instance messaging, nên mỗi instance
nhận mọi tin cũng chỉ là nhận tin của chính nó, không có lợi ích; benchmark hub cho thấy phát 1 tin tới 1.000 người xem
cục bộ mất ~125 µs; còn subscribe động mở ra khe mất tin giữa lúc SUBSCRIBE và tin đầu tiên. Làm lại khi chạy từ hai
instance trở lên. Phần client (reconnect có jitter) đã làm; resync sau reconnect vốn đã gọn (TanStack chỉ refetch
query đang hiển thị, chat state trả 304 khi không đổi), nên không đổi.
