## Why

WebSocket là điểm duy nhất trong hệ thống NGAC còn dùng JSON trên wire. Tất cả internal communication (gRPC giữa services) đã dùng protobuf. Chuyển WebSocket sang protobuf binary mang lại 3 lợi ích:

1. **Architectural consistency** — Cùng proto definitions cho cả gRPC và WebSocket. Một source of truth cho message schemas.
2. **Type safety end-to-end** — Proto codegen sinh typed structs cho Go backend và typed classes cho TypeScript frontend. Không còn `JSON.parse` rồi cast `any`.
3. **Security fix kèm theo** — Chuyển JWT auth từ URL query param (`?token=xxx`) sang first-message auth pattern. Token không còn xuất hiện trong URL/access logs.

Bonus: payload size giảm ~30-50% (JSON 200-500B → protobuf 100-300B), Redis pub/sub payload cũng nhỏ hơn tương ứng.

## What Changes

- **New proto file** `proto/messaging/ws.proto` — Định nghĩa `ClientEnvelope` (client→server) và `ServerEnvelope` (server→client) với `oneof` payload discriminator
- **Backend Hub rewrite** — `hub.go` chuyển từ `json.Marshal`/`TextMessage` sang `proto.Marshal`/`BinaryMessage`. Auth flow mới: upgrade trước, validate JWT qua first message
- **Frontend WebSocket store rewrite** — `websocket.store.ts` chuyển từ `JSON.parse` sang protobuf-ts decode. `ws.binaryType = 'arraybuffer'`
- **Frontend proto codegen** — Thêm `protobuf-ts` dependency và build script generate TypeScript từ `.proto`
- **Makefile update** — `make proto` generate cả Go và TypeScript code

## Capabilities

### New Capabilities
- `ws-binary-protocol`: WebSocket communication dùng protobuf binary frames thay JSON text frames. Typed envelope pattern với oneof discriminator.
- `ws-auth-handshake`: JWT authentication qua first-message sau WebSocket upgrade, thay vì token-in-URL.

### Modified Capabilities
- `realtime-websocket`: Chuyển wire format từ JSON sang protobuf. Giữ nguyên message types và semantics.
- `messaging-system`: Hub broadcast dùng proto.Marshal. Redis pub/sub payload là protobuf bytes.

## Impact

- **New file**: `proto/messaging/ws.proto` — envelope definitions
- **New directory**: `frontend/src/generated/` — protobuf-ts generated code
- **New dependency (frontend)**: `@protobuf-ts/runtime` (~15KB gzipped)
- **New dev dependency**: `@protobuf-ts/plugin` — codegen plugin
- **Modified file**: `backend/services/messaging/internal/grpc/hub.go` — marshal/unmarshal, binary frames, auth handshake
- **Modified file**: `frontend/src/stores/websocket.store.ts` — binary decode, typed handlers
- **Modified file**: `backend/Makefile` — thêm TypeScript proto gen target
- **Modified file**: `frontend/package.json` — thêm protobuf-ts deps + gen script
- **No database changes**
- **No API contract changes** (REST endpoints unchanged)
