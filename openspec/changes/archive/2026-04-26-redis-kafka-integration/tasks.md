## 1. Infrastructure — Xóa NATS, Thêm Redis + Redpanda

- [x] 1.1 Xóa NATS container khỏi `docker-compose.yml`
- [x] 1.2 Xóa `NATS_URL` env vars khỏi workspace, document, messaging services trong `docker-compose.yml`
- [x] 1.3 Xóa NATS dependency (`depends_on: nats`) khỏi workspace, document, messaging
- [x] 1.4 Thêm Redis container (redis:7-alpine) với maxmemory 256MB, healthcheck `redis-cli ping`, volume `redisdata`
- [x] 1.5 Thêm Redpanda container (redpandadata/redpanda:latest) single node, ports 9092 (Kafka API) + 9644 (Admin), log.retention.ms=604800000 (7 days)
- [x] 1.6 Thêm `REDIS_URL` env vars: policy=DB0, auth=DB1, messaging=DB2
- [x] 1.7 Thêm `KAFKA_BROKERS` env var cho policy + messaging services
- [x] 1.8 Thêm volume `redisdata` và `redpandadata`
- [x] 1.9 Verify: `docker-compose config` valid, `docker-compose up` starts all containers healthy

## 2. Go Dependencies

- [x] 2.1 `go get github.com/redis/go-redis/v9` cho services: policy, auth, messaging
- [x] 2.2 `go get github.com/twmb/franz-go` cho services: policy, messaging
- [x] 2.3 `go mod tidy` cho mỗi service
- [x] 2.4 Verify: `go build ./cmd/` pass cho tất cả 6 services

## 3. Redis Access Cache — Policy Service

- [x] 3.1 Thêm `connectRedis()` helper vào `services/policy/cmd/main.go` (parse URL, ping, return client)
- [x] 3.2 Update `NewPolicyServer()` nhận `*redis.Client` (nil = no cache)
- [x] 3.3 Implement `getFromCache(ctx, key)` — Redis GET, JSON unmarshal AccessDecision
- [x] 3.4 Implement `setCache(ctx, key, resp)` — JSON marshal, Redis SET with 30s TTL
- [x] 3.5 Implement `invalidateCache(ctx)` — SCAN ngac:access:* + DEL batch
- [x] 3.6 Update `CheckAccess()` — cache check → miss → compute → store
- [x] 3.7 Update `CreateAssignment()`, `RemoveAssignment()`, `CreateAssociation()`, `RemoveAssociation()`, `DeleteNode()`, `LoadGraph()` — call `invalidateCache()` after mutation
- [x] 3.8 Cache key format: `ngac:access:{userNodeId}:{objectNodeId}:{operation}`
- [x] 3.9 Verify: CheckAccess returns cached result on repeat call, cache flushed after CreateAssignment

## 4. JWT Blacklist — Auth Service

- [x] 4.1 Thêm `connectRedis()` helper vào `services/auth/cmd/main.go`
- [x] 4.2 Update Auth gRPC server nhận `*redis.Client`
- [x] 4.3 Thêm `jti` (JWT ID) claim vào JWT generation (uuid per token)
- [x] 4.4 Implement `RevokeToken` RPC — `SET jwt:blacklist:{jti} 1 EX {remaining_seconds}`
- [x] 4.5 Implement `IsTokenRevoked(ctx, jti)` — `EXISTS jwt:blacklist:{jti}`
- [x] 4.6 Update proto `auth.proto` — thêm `RevokeTokenRequest`/`RevokeTokenResponse`, thêm `jti` field vào token claims
- [x] 4.7 Make proto: regenerate Go code
- [x] 4.8 Gateway: update JWT validation middleware — sau parse JWT, check `IsTokenRevoked` trước forward
- [x] 4.9 Verify: Login → get token → RevokeToken → request với revoked token bị reject

## 5. Redis Pub/Sub — Messaging Hub

- [x] 5.1 Thêm `connectRedis()` helper vào `services/messaging/cmd/main.go`
- [x] 5.2 Refactor `NewHub()` nhận `*redis.Client` (nil = local-only)
- [x] 5.3 Implement `subscribeRedis()` goroutine — `PSUBSCRIBE channel:*`, deliver messages to local clients
- [x] 5.4 Update `BroadcastToChannel()` — if Redis available: `PUBLISH channel:{id} payload`, else: local broadcast
- [x] 5.5 Update typing indicator broadcast qua Redis pub/sub
- [x] 5.6 Add `Close()` method trên Hub — cancel Redis subscription context
- [x] 5.7 Wire Redis vào Hub trong `messaging/cmd/main.go`
- [x] 5.8 Verify: 2 WebSocket clients connect, send message, both receive via Redis pub/sub path

## 6. Kafka Event Streaming — Producers

- [x] 6.1 Tạo `services/policy/internal/events/producer.go` — Kafka producer wrapper với `PublishAccessChecked()` và `PublishGraphMutated()`
- [x] 6.2 Define event structs: `AccessCheckedEvent{UserID, ObjectID, Operation, Decision, Timestamp}`, `GraphMutatedEvent{MutationType, NodeIDs, OperatorID, Timestamp}`
- [x] 6.3 Update `PolicyServer` nhận Kafka producer, publish sau mỗi CheckAccess và mutation
- [x] 6.4 Wire Kafka producer vào `policy/cmd/main.go`
- [x] 6.5 Tạo `services/messaging/internal/events/producer.go` — `PublishMessageSent()` 
- [x] 6.6 Define event struct: `MessageSentEvent{ChannelID, SenderID, Timestamp, ContentHash}`
- [x] 6.7 Update `MessagingServer.SendMessage()` — publish Kafka event sau insert
- [x] 6.8 Wire Kafka producer vào `messaging/cmd/main.go`
- [x] 6.9 Verify: Check Redpanda Admin UI hoặc `rpk topic consume` thấy events đúng format

## 7. Graceful Degradation + Cleanup

- [x] 7.1 All Redis/Kafka connections gracefully degrade (nil checks, slog.Warn, no os.Exit)
- [x] 7.2 Remove any remaining NATS import hoặc reference trong Go code
- [x] 7.3 Update all service Dockerfiles nếu cần
- [x] 7.4 Full `docker-compose build` pass
- [x] 7.5 Full `docker-compose up` — all containers healthy
- [x] 7.6 E2E: Login → create workspace → send message → verify Redis cache + Kafka events present

## 8. Git

- [x] 8.1 Commit `feat: add redis and redpanda to infrastructure, remove nats` (docker-compose + deps)
- [x] 8.2 Commit `feat(policy): add redis access decision cache with invalidation`
- [ ] 8.3 Commit `feat(auth): add jwt blacklist via redis`
- [ ] 8.4 Commit `feat(messaging): add redis pub/sub for websocket horizontal scaling`
- [ ] 8.5 Commit `feat(policy,messaging): add kafka event producers`
- [ ] 8.6 Merge branch → main, push
