## 1. Proto Definition & Codegen Pipeline

- [x] 1.1 Create `proto/messaging/ws.proto` — define `ClientEnvelope` (oneof: auth, subscribe, unsubscribe, typing) and `ServerEnvelope` (oneof: auth_response, chat_message, typing_event, notification, unread_count, thread_reply, asset_updated, error)
- [x] 1.2 Run `make proto` — verify Go codegen produces `ws.pb.go` in `proto/messaging/`
- [x] 1.3 Install `@protobuf-ts/plugin` and `@protobuf-ts/runtime` in frontend
- [x] 1.4 Add `proto:gen` npm script — run protobuf-ts plugin to generate TypeScript into `frontend/src/generated/`
- [x] 1.5 Run `npm run proto:gen` — verify TypeScript codegen produces typed `ClientEnvelope`, `ServerEnvelope`, etc.
- [x] 1.6 Update `backend/Makefile` `proto` target — add TypeScript generation step alongside Go generation

## 2. Backend Hub — Binary Protocol

- [x] 2.1 Add auth state machine to `Client` struct — `authenticated bool`, `authTimeout` timer (5s). Reject non-auth messages before authenticated.
- [x] 2.2 Rewrite `readPump()` — detect `TextMessage` (JSON legacy) vs `BinaryMessage` (protobuf). For binary: `proto.Unmarshal` into `ClientEnvelope`, switch on `payload` oneof case
- [x] 2.3 Implement `handleAuth()` — validate JWT from `AuthRequest.token`, set client fields (userID, username, ngacNodeID), respond with `ServerEnvelope{auth_response}`, register in hub user tracking
- [x] 2.4 Rewrite `BroadcastToChannel()` — convert `pb.Message` to `ChatMessage`, wrap in `ServerEnvelope{chat_message}`, `proto.Marshal` to bytes, send as `BinaryMessage`
- [x] 2.5 Rewrite `broadcastTyping()` — wrap in `ServerEnvelope{typing_event}`, marshal and send binary
- [x] 2.6 Rewrite `SendNotification()` — wrap in `ServerEnvelope{notification}`, marshal and send binary
- [x] 2.7 Rewrite `writePump()` — change `WriteMessage` from `TextMessage` to `BinaryMessage`
- [x] 2.8 Update `HandleWebSocket()` — remove token-from-URL auth. Upgrade unconditionally, start auth timeout goroutine
- [x] 2.9 Add `sendError()` helper — wrap error code+message in `ServerEnvelope{error}`, marshal, send

## 3. Backend Hub — Redis Dual-Mode

- [x] 3.1 Update Redis publish — send protobuf bytes instead of JSON bytes
- [x] 3.2 Update `subscribeRedis()` — detect incoming format (first byte `{` = JSON legacy, else = protobuf). Decode accordingly. Forward to local clients as protobuf binary.
- [x] 3.3 Build and verify: `go build ./cmd/` for messaging service

## 4. Frontend — Protobuf WebSocket Store

- [x] 4.1 Rewrite `websocket.store.ts` — set `ws.binaryType = 'arraybuffer'`, import generated protobuf types
- [x] 4.2 Implement `connect()` — after WebSocket open, send `ClientEnvelope{auth: {token}}` as first message. Wait for `auth_response` before sending other messages. Queue messages during auth handshake.
- [x] 4.3 Implement `onmessage` handler — decode `ServerEnvelope.fromBinary(new Uint8Array(event.data))`, switch on `payload.oneofKind`
- [x] 4.4 Route decoded events to TanStack Query invalidation — same logic as current `handleWsMessage` but using typed proto fields instead of `any`
- [x] 4.5 Rewrite `sendTyping()` — encode `ClientEnvelope{typing: {channelId}}` via `ClientEnvelope.toBinary()`, send as binary
- [x] 4.6 Add `sendSubscribe(channelId)` and `sendUnsubscribe(channelId)` — encode and send as binary
- [x] 4.7 Add debug mode: if `WS_DEBUG` localStorage flag set, console.log decoded protobuf messages for DevTools visibility
- [x] 4.8 Remove token from WebSocket URL — `new WebSocket(wsUrl)` without `?token=` query param

## 5. Integration & Verification

- [x] 5.1 Build check: `go build ./cmd/` for messaging service — zero errors
- [x] 5.2 Build check: `npm run build` for frontend — zero TypeScript errors
- [x] 5.3 Lint check: `npm run lint` — zero ESLint errors
- [ ] 5.4 End-to-end test: connect WebSocket, send auth message, receive auth_response
- [ ] 5.5 End-to-end test: subscribe to channel, send message in channel, receive ChatMessage event
- [ ] 5.6 End-to-end test: typing indicator sends and receives correctly
- [ ] 5.7 End-to-end test: notifications push through WebSocket as protobuf
- [ ] 5.8 Verify Redis pub/sub: message broadcast works across simulated multi-instance
