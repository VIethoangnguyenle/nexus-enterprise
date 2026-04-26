## Why

Hệ thống NGAC hiện tại có 3 điểm yếu về infrastructure khiến nó chưa production-ready:

1. **NATS là dead infrastructure**: Khai báo trong docker-compose.yml, 3 services phụ thuộc NATS trong config, nhưng **không có dòng code Go nào import hay sử dụng NATS**. Đây là tech debt gây nhầm lẫn.

2. **CheckAccess là hot path không có cache**: Mỗi user action (đọc document, gửi message, list channels) tạo gRPC round-trip tới Policy Service → graph traversal in-memory. Với 9 call sites trên 3 services, volume tăng tuyến tính theo users. Không có caching layer.

3. **WebSocket hub không scale được**: Messaging Hub lưu client connections trong memory (`sync.RWMutex + map`). Chạy 2 instances → 2 hub riêng biệt → user A ở instance 1 không nhận message từ user B ở instance 2.

4. **Không có JWT revocation**: Khi user logout hoặc bị ban, JWT vẫn valid cho đến khi hết hạn. Không có blacklist mechanism.

5. **Không có event streaming**: Các sự kiện quan trọng (message.sent, member.invited, document.approved) không được track. Không có audit trail, không có replay capability.

## What Changes

- **BREAKING**: Xóa NATS khỏi docker-compose.yml và tất cả env references
- Add Redis (redis:7-alpine) cho 3 use cases:
  - DB0: Access decision cache cho Policy Service (TTL 30s, flush on graph mutation)
  - DB1: JWT blacklist cho Auth Service (TTL = token expiry)
  - DB2: Pub/Sub cho Messaging Service WebSocket hub (cross-instance broadcast)
- Add Kafka (Redpanda — Kafka-compatible, single binary) cho durable event streaming:
  - Topic `ngac.access.checked`: Audit trail cho mọi access decision
  - Topic `ngac.graph.mutated`: Track graph mutations (CreateAssignment, CreateAssociation, etc.)
  - Topic `ngac.messages`: Message events cho analytics/search indexing
- Update service entrypoints để inject Redis/Kafka clients
- Graceful degradation: Redis/Kafka down → fallback to direct gRPC/local broadcast

## Capabilities

### New Capabilities
- `redis-access-cache`: Cache CheckAccess results trong Redis DB0 với TTL 30s. Invalidate toàn bộ khi graph mutation xảy ra. Giảm Policy Service load ~80% cho repeated access checks.
- `redis-jwt-blacklist`: JWT blacklist trong Redis DB1. Auth Service thêm RPC `RevokeToken` để invalidate JWT trước expiry. Gateway kiểm tra blacklist trước khi forward request.
- `redis-ws-pubsub`: Messaging Hub publish messages/typing qua Redis DB2 pub/sub. Mọi instance subscribe pattern `channel:*` và broadcast tới local clients. Enable horizontal scaling.
- `kafka-event-streaming`: Redpanda (Kafka-compatible) cho durable event streaming. Producers ở Policy/Messaging services. Consumers có thể thêm sau cho audit log, analytics, search indexing.

### Modified Capabilities
- `microservice-architecture`: Thay NATS bằng Redis + Kafka trong infrastructure stack. Docker Compose updated.
- `ngac-engine`: Policy Service thêm Redis cache layer xung quanh CheckAccess. Graph mutation RPCs thêm cache invalidation + Kafka event publishing.
- `messaging-system`: Hub refactored từ local-only thành Redis pub/sub-backed. Broadcast flow: gRPC SendMessage → Redis publish → all instances subscribe → local WebSocket delivery.
- `user-auth`: Auth Service thêm JWT blacklist check + RevokeToken RPC.

## Impact

**Infrastructure (major addition)**:
- Thêm Redis container (redis:7-alpine, ~30MB, maxmemory 256MB)
- Thêm Redpanda container (Kafka-compatible, ~150MB, single binary — no ZooKeeper)
- Xóa NATS container
- Net change: +1 container (NATS → Redis + Redpanda)

**Services affected**:
- `policy/cmd/main.go`: +Redis client injection, +Kafka producer
- `policy/internal/grpc/server.go`: +cache get/set trên CheckAccess, +cache invalidation trên mutation RPCs, +Kafka publish trên mutations
- `auth/cmd/main.go`: +Redis client injection
- `auth/internal/grpc/server.go` hoặc domain mới: +RevokeToken RPC, +blacklist check
- `messaging/cmd/main.go`: +Redis client injection
- `messaging/internal/grpc/hub.go`: Refactor Hub với Redis pub/sub backend
- `gateway/cmd/main.go`: +JWT blacklist check trước forwarding
- `docker-compose.yml`: Xóa NATS, thêm Redis + Redpanda, update env vars

**Dependencies added (Go modules)**:
- `github.com/redis/go-redis/v9` — Redis client (45K stars, actively maintained)
- `github.com/twmb/franz-go` — Kafka client cho Go (pure Go, no CGO, Redpanda recommended)

**Database**: Không thay đổi. Cache và events hoàn toàn nằm ngoài PostgreSQL.

**Frontend**: Không thay đổi. Redis/Kafka là backend-only infrastructure.

## Non-Goals

- Redis Cluster / Redis Sentinel (single instance đủ cho VPS hiện tại)
- Kafka consumer services (chỉ tạo producers, consumers thêm sau)
- Rate limiting qua Redis (sẽ chuyển sang Traefik ở change tiếp theo)
- Redis-based session store (JWT stateless là đủ)
- Schema Registry cho Kafka (dùng JSON encoding đơn giản trước)
