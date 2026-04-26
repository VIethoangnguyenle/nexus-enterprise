## Context

Hệ thống NGAC hiện tại gồm 6 Go microservices giao tiếp qua gRPC, PostgreSQL 16 shared, và React SPA. NATS được khai báo trong docker-compose nhưng không service nào thực sự sử dụng — nó là dead infrastructure.

### Hiện trạng data flow

```
Client → Gateway → gRPC → Service → PolicyService.CheckAccess() → Graph Traversal (in-memory)
                                                                     ↓
                                                               PostgreSQL (persistence)
```

Mỗi CheckAccess là một synchronous gRPC call. Messaging Hub broadcast hoàn toàn in-memory (không cross-instance).

### Target data flow

```
Client → Gateway → gRPC → Service ─→ Redis Cache ─HIT──→ Response
          │                   │              │
          │                   │            MISS
          │                   │              ↓
          │                   │     PolicyService.CheckAccess()
          │                   │              │
          │                   │        ┌─────┴─────┐
          │                   │        ↓           ↓
          │                   │   Redis SET    Kafka Publish
          │                   │   (cache)      (audit event)
          │                   │
          │                   └─→ Kafka Produce (domain events)
          │
          └── JWT Blacklist Check (Redis DB1) ← trước khi forward
```

## Goals / Non-Goals

**Goals:**
- Xóa hoàn toàn NATS từ codebase và infrastructure
- Cache CheckAccess results trong Redis với invalidation khi graph thay đổi
- JWT blacklist cho phép revoke token trước expiry
- Redis pub/sub cho WebSocket horizontal scaling
- Kafka producers cho audit trail và event streaming
- Graceful degradation — Redis/Kafka down không crash services

**Non-Goals:**
- Kafka consumers (chỉ tạo producers, consumers là change riêng)
- Redis Cluster / HA (single instance)
- End-to-end event sourcing / CQRS
- Message replay UI
- Rate limiting (Traefik sẽ xử lý)

## Decisions

### D1: Redis 3 Databases trên 1 Instance

**Choice: Dùng Redis DB0/DB1/DB2 để tách concern logic trên cùng instance**

| DB | Service | Use | Justification |
|----|---------|-----|---------------|
| DB0 | Policy | Access cache | TTL 30s, flush-all on mutation. Read-heavy, write-rare. |
| DB1 | Auth + Gateway | JWT blacklist | `SET jwt:blacklist:{jti} 1 EX {remaining_ttl}`. Check trước forward. |
| DB2 | Messaging | Pub/Sub | `PUBLISH channel:{id} payload`. `PSUBSCRIBE channel:*`. |

**Why:**
- 3 DB numbers trên 1 instance = zero ops overhead, logical separation
- Mỗi service chỉ connect tới DB number của mình → không vô tình đọc data service khác
- VPS có 10GB RAM, Redis maxmemory 256MB là dư dả
- Nếu sau này cần tách, chỉ đổi REDIS_URL env var

**Alternatives considered:**
- 3 Redis instances riêng: Tốn resource, phức tạp ops, chưa cần
- 1 Redis DB cho tất cả: Namespace collision risk, không tách được data khi scale

### D2: Redpanda thay vì Apache Kafka

**Choice: Redpanda (Kafka-compatible, single binary, no JVM/ZooKeeper)**

**Why:**
- API 100% compatible với Apache Kafka — dùng bất kỳ Kafka client nào
- Single binary ~150MB vs Kafka ~500MB + ZooKeeper ~200MB
- No JVM → khởi động nhanh, ít RAM (256MB đủ vs Kafka 1-2GB)
- Tự quản lý — không cần ZooKeeper/KRaft controller
- Chuyển sang Kafka cluster thật chỉ cần đổi broker address

**Alternatives considered:**
- Apache Kafka: Heavyweight, cần JVM + ZK/KRaft, overkill cho single node
- Redis Streams: Đủ cho pub/sub nhưng thiếu consumer groups, compaction, replay mạnh
- NATS JetStream: Đang xóa NATS, không muốn giữ lại

### D3: Cache Invalidation Strategy — Flush All on Mutation

**Choice: Xóa toàn bộ ngac:access:* keys khi bất kỳ graph mutation nào xảy ra**

Graph mutations xảy ra qua các RPCs:
- `CreateAssignment` / `RemoveAssignment`
- `CreateAssociation` / `RemoveAssociation`
- `DeleteNode`
- `LoadGraph`

**Why:**
- Mutations hiếm (admin operations) — 1-10 lần/ngày vs reads hàng nghìn lần/giờ
- Targeted invalidation cần dependency analysis phức tạp (node A bị xóa → ảnh hưởng access path nào?)
- TTL 30s đã là safety net — stale data tồn tại tối đa 30s
- SCAN + DEL trên ~1000 keys takes <1ms

**Trade-off:** Burst cache miss ngay sau mutation. Chấp nhận được vì mutations hiếm.

### D4: Kafka Topic Design

**Choice: 3 topics phẳng, JSON encoding, no schema registry**

```
ngac.access.checked   — Mỗi CheckAccess result (user, object, op, decision, timestamp)
ngac.graph.mutated    — Mỗi graph mutation (type, node_ids, operator, timestamp)
ngac.messages         — Mỗi message sent (channel_id, sender_id, timestamp, content_hash)
```

**Why:**
- JSON cho simplicity — Kafka consumers parse dễ, debug dễ
- 3 topics theo domain boundary — consumers subscribe chỉ events cần
- Không partition key phức tạp — default round-robin, đủ cho single broker
- Schema Registry thêm sau khi có real consumers

**Alternatives considered:**
- Protobuf encoding: Type-safe nhưng thêm compile step, overkill khi chưa có consumers
- Single topic `ngac.events` với type field: Harder to filter, harder to manage retention per type
- Per-service topics: Too granular, no benefit at current scale

### D5: Go Kafka Client — franz-go

**Choice: `github.com/twmb/franz-go` thay vì `confluent-kafka-go` hoặc `segmentio/kafka-go`**

**Why:**
- Pure Go — no CGO dependency (confluent-kafka-go cần librdkafka)
- Redpanda team officially recommends franz-go
- Active development, strong community
- Supports all Kafka protocol versions
- Simple Producer API đủ cho use case hiện tại

### D6: Graceful Degradation Pattern

**Choice: Redis/Kafka unavailable → service hoạt động bình thường, chỉ mất cache/events**

```go
rdb, err := connectRedis(ctx, redisURL)
if err != nil {
    slog.Warn("redis unavailable, caching disabled", "error", err)
    // rdb remains nil — service continues without cache
}
```

Mọi function nhận `*redis.Client` hoặc `*kgo.Client` check nil trước khi dùng.

**Why:**
- Cache và events là optimization, không phải core logic
- CheckAccess vẫn đúng khi không có cache (chỉ chậm hơn)
- Messages vẫn gửi được khi Redis pub/sub down (local broadcast)
- Startup không bị block bởi Redis/Kafka unavailability

## Risks / Trade-offs

- **[Stale cache — sai quyền tối đa 30s]** → Sau graph mutation, có window 0-30s cache trả kết quả cũ. Mitigated bằng flush-all on mutation. Nếu flush fail, TTL 30s là safety net.
- **[Redis SPOF]** → Redis down = mất cache + mất pub/sub + mất blacklist. Mitigated: cache fallback to gRPC, pub/sub fallback to local, blacklist → tokens valid until expiry (acceptable risk).
- **[Redpanda disk usage]** → Events tích lũy trên disk. Mitigated: default retention 7 days, cấu hình trong docker-compose.
- **[VPS resource pressure]** → Thêm 2 containers trên VPS 10GB. Redis ~50MB, Redpanda ~256MB. Còn dư ~3GB headroom.
- **[Kafka producer latency]** → Publish event thêm ~1ms per request. Mitigated: async fire-and-forget publish, không block response.
