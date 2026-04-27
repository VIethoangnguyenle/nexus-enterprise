## Context

NGAC Messaging Service expose WebSocket endpoint tại `:8081`, proxied qua Nginx. Backend dùng `gorilla/websocket` với `Hub` pattern (channel subscriptions, user tracking, Redis pub/sub cho cross-instance). Frontend dùng Zustand store wrapping native `WebSocket` API.

Hiện tại: `json.Marshal` → `TextMessage` trên backend, `JSON.parse(event.data)` trên frontend. 6 server→client event types, 3 client→server event types. Token truyền qua URL query param.

### Constraints

- Proto file mới phải nằm cùng package `messaging` — reuse existing `go_package`
- Không thay đổi semantics của bất kỳ event nào — chỉ đổi wire format
- Frontend codegen output vào `src/generated/` — gitignore-able nhưng commit để CI không cần protoc
- Giữ backward compatibility trong transition: backend accept cả text (JSON) và binary (protobuf) messages cho migration period
- gorilla/websocket hỗ trợ `BinaryMessage` type natively — không cần library mới ở Go side

## Goals / Non-Goals

**Goals:**
- Protobuf binary wire format cho toàn bộ WebSocket communication
- Typed envelope pattern: `ClientEnvelope` (oneof) + `ServerEnvelope` (oneof)
- Auth handshake qua first message thay vì URL query param
- Proto codegen cho cả Go (protoc-gen-go) và TypeScript (protobuf-ts)
- Redis pub/sub cũng chuyển sang protobuf bytes

**Non-Goals:**
- Không implement WebSocket compression (per-message deflate) — separate concern
- Không thêm message batching — chưa có nhu cầu ở scale hiện tại
- Không thêm protocol version negotiation — YAGNI
- Không implement gRPC-Web — WebSocket và gRPC là 2 channels riêng

## Decisions

### Decision 1: Envelope pattern với `oneof` thay vì `type` field + `bytes` payload

**Chosen**: `ClientEnvelope { oneof payload { ... } }` — mỗi message type là một field trong oneof

**Alternatives considered**:
- *Type enum + bytes payload*: `{ type: EventType; data: bytes }` — flexible hơn nhưng mất type safety. Client phải decode 2 lần (envelope + payload). Pattern này giống Cap'n Proto RPC nhưng overkill cho use case đơn giản.
- *Separate proto message per direction*: Quá nhiều top-level messages, khó manage imports.

**Rationale**: Oneof được compiler enforce — không thể set 2 payload cùng lúc. Generated code có discriminator (`oneofKind` trong protobuf-ts, type switch trong Go). Zero runtime type-string matching.

### Decision 2: protobuf-ts cho frontend codegen

**Chosen**: `@protobuf-ts/plugin` generate TypeScript code từ `.proto` files

**Alternatives considered**:
- *@bufbuild/protobuf (Buf)*: Tốt nhưng heavier ecosystem. Cần buf CLI, buf.yaml config. Overkill khi đã có protoc pipeline.
- *google-protobuf*: Legacy API, 40KB+ bundle, không tree-shakeable.
- *protobufjs*: Popular nhưng runtime reflection-based, larger bundle.

**Rationale**: protobuf-ts sinh static code (không reflection), tree-shakeable, runtime chỉ ~15KB gzipped. DX tốt nhất: full TypeScript types, no `any`. Compatible với standard `.proto` files — cùng protoc pipeline hiện có.

### Decision 3: Auth handshake qua first message (fix security issue)

**Chosen**: WebSocket upgrade không cần token. Sau khi connected, client gửi `ClientEnvelope{auth: {token: "eyJ..."}}`. Server validate, respond `ServerEnvelope{auth_response: {ok: true}}`. Messages trước auth bị reject.

**Rationale**: Token trong URL = token trong server access logs, Nginx logs, browser history, referrer headers. First-message pattern là industry standard (Slack, Discord đều dùng).

**Backend state machine**:
```
CONNECTED ──auth msg──► AUTHENTICATED ──normal msgs──►
     │                                                
     │──non-auth msg──► send ErrorEvent, close         
     │──timeout 5s──► close                            
```

### Decision 4: Flat WebSocket-specific messages thay vì reuse gRPC proto messages

**Chosen**: `ChatMessage` (cho WS) tách biệt với `Message` (cho gRPC). ChatMessage chỉ chứa fields frontend cần render.

**Rationale**: gRPC `Message` có fields chỉ dùng internal (`linked_entity_type/id`, `message_type`). WebSocket chỉ cần display fields. Flat messages = smaller payload + không leak internal schema ra client.

Backend convert: `pb.Message` → `ChatMessage` trước khi broadcast.

### Decision 5: Dual-mode transition period

**Chosen**: Trong quá trình migration, backend detect message type bằng WebSocket frame type:
- `TextMessage` → JSON (legacy path)
- `BinaryMessage` → Protobuf (new path)

**Rationale**: Cho phép deploy backend trước, frontend sau. Không cần big-bang deploy. Remove JSON path sau khi frontend đã chuyển hoàn toàn.

## Risks / Trade-offs

| Risk | Impact | Mitigation |
|------|--------|------------|
| DevTools không đọc được binary frames | Debug khó hơn | Thêm `WS_DEBUG=1` env toggle trên frontend — khi bật, log decoded messages ra console |
| protobuf-ts version mismatch với protoc | Build fail | Pin version trong package.json, document exact protoc version |
| Auth handshake thêm 1 round-trip | Connect chậm hơn ~50ms | Acceptable — chỉ xảy ra lần đầu connect. Reconnect có thể cache auth state |
| Proto file thay đổi = rebuild cả Go + TS | Dev workflow phức tạp hơn | `make proto` đã handle Go. Thêm `npm run proto:gen` cho TS. Document trong README |
| Redis pub/sub chuyển sang binary = rolling deploy phức tạp | Instance cũ không decode được message instance mới | Dual-mode: cũ gửi JSON, mới gửi binary. Detect bằng first byte (`{` = JSON, else = protobuf) |
