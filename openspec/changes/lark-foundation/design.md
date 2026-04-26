## Context

NGAC Platform hiện tại có 6 microservices (Auth, Policy, Workspace, Document, Messaging, Gateway) giao tiếp qua gRPC, với PostgreSQL shared database, Redis cache, và Redpanda (Kafka) cho event streaming. Messaging Service đã có foundation cho channels, DMs, và WebSocket nhưng chưa có notification system hay thread support. Asset management là domain hoàn toàn mới.

Platform phục vụ SME nhưng được thiết kế với enterprise-grade architecture để học các patterns hệ thống lớn. Reference Lark cho UX/feature direction, không cạnh tranh.

### Current Service Map

| Service | Port | Vai trò hiện tại |
|---------|------|-------------------|
| Policy | 50051 | NGAC PDP — graph engine, access decisions |
| Auth | 50052 | JWT auth, user management, Redis token blacklist |
| Workspace | 50053 | Workspace CRUD, NGAC scaffolding |
| Document | 50054 | Document upload/download, workflow |
| Messaging | 50055 + WS:8081 | Channels, DMs, WebSocket broadcast |
| Gateway | 8080 | REST → gRPC proxy, JWT validation |

### Constraints

- Policy Service là PDP duy nhất — không service nào tự quyết access
- Services giao tiếp nội bộ qua gRPC — không HTTP internal
- Gateway là cửa duy nhất ra ngoài (trừ Messaging WebSocket)
- JWT chứa `ngac_node_id` — downstream dùng node ID này gọi Policy Service
- Mỗi service có Go module riêng, share code chỉ qua proto

## Goals / Non-Goals

**Goals:**
- Thêm Asset Service mới tuân thủ kiến trúc microservice hiện có
- Asset type system generic — admin define types, fields, lifecycle tại runtime
- Tích hợp event-driven giữa Asset và Messaging qua Kafka
- Notification center trong messaging cho asset events
- Thread support gắn discussion với asset cụ thể
- NGAC graph model cho asset access control (categories, types, per-asset)
- Học được: event-driven architecture, state machines, saga pattern, CQRS-lite

**Non-Goals:**
- Không build rich-text document editor (giữ upload/download)
- Không build mobile app — web responsive là đủ
- Không build video/audio calling
- Không build calendar/scheduling
- Không cạnh tranh feature parity với Lark
- Không implement full CQRS (event sourcing) — chỉ CQRS-lite (separate read cache)
- Không build bot platform hay extension system ở phase này

## Decisions

### Decision 1: Asset Service là microservice riêng (không merge vào Document)

**Chosen**: Tạo `services/asset/` với Go module riêng, port 50056

**Alternatives considered**:
- *Merge vào Document Service*: Đơn giản hơn nhưng vi phạm Single Responsibility. Document là file storage + workflow, Asset là inventory tracking + lifecycle. Hai bounded contexts khác nhau.
- *Shared library*: Vi phạm rule "không share code giữa services trừ proto"

**Rationale**: Tách service cho phép scale independently. Asset Service có thể heavy on reads (dashboard, reports) trong khi Document Service heavy on writes (uploads). Cũng là cơ hội học service decomposition pattern.

### Decision 2: Generic Asset Type System dùng JSON Schema

**Chosen**: Asset types defined tại runtime, custom fields validated bằng JSON Schema, stored as JSONB trong PostgreSQL

**Alternatives considered**:
- *Hardcoded types*: Nhanh hơn nhưng không generic — thêm type = thay code + deploy
- *EAV model (Entity-Attribute-Value)*: Flexible nhưng query performance rất tệ, schema phức tạp
- *Separate tables per type*: Performance tốt nhưng không dynamic — thêm type = migration

**Rationale**: JSON Schema + JSONB cho phép:
- Admin tạo asset type mới qua UI (không cần developer)
- Validation ở application layer (Go: `santhosh-tekuri/jsonschema`)
- PostgreSQL JSONB hỗ trợ indexing trên custom fields khi cần
- Schema evolution dễ (thêm field = update JSON Schema)

### Decision 3: State Machine Engine cho Asset Lifecycle

**Chosen**: Configurable state machine per asset type, transitions stored trong DB, mỗi transition map tới NGAC operation

**Design**:
```
State Machine Definition (per AssetType):
{
  "states": ["requested", "approved", "assigned", "in_use", "returned", "disposed"],
  "initial": "requested",
  "transitions": [
    {"from": "requested", "to": "approved", "operation": "approve", "ngac_permission": "approve"},
    {"from": "approved", "to": "assigned", "operation": "assign", "ngac_permission": "assign"},
    {"from": "assigned", "to": "in_use", "operation": "activate", "ngac_permission": "manage"},
    {"from": "in_use", "to": "returned", "operation": "return", "ngac_permission": "manage"},
    {"from": "returned", "to": "disposed", "operation": "dispose", "ngac_permission": "dispose"}
  ]
}
```

**Rationale**: Mỗi transition require NGAC permission check → access control granular đến từng bước lifecycle. Admin có thể customize lifecycle per type (software license có lifecycle khác laptop).

### Decision 4: Event-Driven Integration qua Kafka (không gRPC sync)

**Chosen**: Asset Service produce Kafka events → Messaging Service consume → tạo notifications

**Alternatives considered**:
- *Sync gRPC call từ Asset → Messaging*: Simple nhưng tight coupling. Asset Service phải biết Messaging API. Nếu Messaging down, asset operations fail.
- *Webhook/HTTP callback*: Vi phạm rule "không HTTP internal"
- *Shared database polling*: Anti-pattern, poor latency

**Rationale**: Kafka decouples hoàn toàn. Asset Service chỉ cần biết produce events, không care ai consume. Messaging Service subscribe topics nó quan tâm. Thêm consumers sau (audit log, analytics) không cần thay đổi producer. Redpanda đã có trong stack.

**Topic design**:
```
asset.lifecycle    → {asset_id, type, from_state, to_state, actor_id, workspace_id, timestamp}
asset.request      → {request_id, asset_type, requester_id, workspace_id, timestamp}
asset.assignment   → {asset_id, from_user_id, to_user_id, actor_id, workspace_id, timestamp}
```

### Decision 5: NGAC Graph Model cho Assets

**Chosen**: Mỗi Asset Category → OA, mỗi Asset Type → child OA, mỗi Asset Instance → Object (O). Tất cả dưới PC_AssetManagement.

**Graph structure**:
```
PC_AssetManagement
├── OA: {workspace}_Assets                    ← workspace-scoped
│   ├── OA: {workspace}_IT_Equipment          ← category OA
│   │   ├── OA: {workspace}_Laptops           ← type OA (auto-created khi define type)
│   │   │   ├── O: asset_001                  ← asset instance
│   │   │   └── O: asset_002
│   │   └── OA: {workspace}_Monitors
│   └── OA: {workspace}_Vehicles
│       └── OA: {workspace}_Cars
```

**Associations**:
```
UA: {workspace}_IT_Admin  ──[read,write,approve,assign,dispose]──► OA: {workspace}_IT_Equipment
UA: {workspace}_Engineers ──[read,request]──────────────────────► OA: {workspace}_IT_Equipment
UA: {workspace}_Fleet_Mgr ──[read,write,approve]────────────────► OA: {workspace}_Vehicles
```

**Rationale**: Tận dụng NGAC inheritance — assign permission trên category OA tự động áp dụng cho tất cả types và assets bên dưới. Department access tự nhiên qua UA hierarchy. Cross-workspace isolation qua workspace-scoped OA.

### Decision 6: Thread Model cho Messaging

**Chosen**: Threads là messages có `parent_message_id` + optional `linked_entity_type` + `linked_entity_id`

**Schema addition**:
```sql
ALTER TABLE messages ADD COLUMN parent_message_id UUID REFERENCES messages(id);
ALTER TABLE messages ADD COLUMN linked_entity_type VARCHAR(50); -- 'asset', 'document', etc.
ALTER TABLE messages ADD COLUMN linked_entity_id UUID;
CREATE INDEX idx_messages_thread ON messages(parent_message_id) WHERE parent_message_id IS NOT NULL;
CREATE INDEX idx_messages_entity ON messages(linked_entity_type, linked_entity_id);
```

**Rationale**: Simple extension — không cần bảng riêng cho threads. `linked_entity_type/id` là generic, support liên kết với asset, document, hoặc bất kỳ entity nào sau này. Polymorphic nhưng explicit.

### Decision 7: Notification System Architecture

**Chosen**: Notifications stored trong DB + pushed qua WebSocket. Kafka consumer trong Messaging Service tạo notification records từ asset events.

**Flow**:
```
Asset Event (Kafka) → Messaging Consumer → Determine recipients (via NGAC) 
→ Create notification records → Push via WebSocket to online users
→ Offline users see on next load (unread count badge)
```

**Notification types**: `asset_requested`, `asset_approved`, `asset_assigned`, `asset_returned`, `mention`, `thread_reply`

**Rationale**: Tách notification khỏi chat messages — notifications có riêng bảng, riêng API, riêng UI (bell icon + dropdown). Chat messages vẫn riêng. Tương tự Lark pattern: notification center tách biệt khỏi chat.

## Risks / Trade-offs

| Risk | Impact | Mitigation |
|------|--------|------------|
| JSON Schema validation performance | Mỗi asset create/update validate JSON Schema | Cache compiled schemas per type trong memory. Schema thay đổi ít, assets thay đổi nhiều |
| Kafka consumer lag → stale notifications | Notifications đến chậm | Monitor consumer lag. Notification không critical path — acceptable delay 1-2s |
| NGAC graph complexity tăng | Mỗi asset type tạo thêm OA nodes, mỗi asset tạo O node | Pagination + lazy loading graph. PostgreSQL index trên ngac_assignments |
| State machine misconfiguration | Admin define invalid lifecycle → assets stuck | Validate state machine at definition time (reachability check, no orphan states) |
| Cross-service data consistency | Asset approved nhưng notification fail | Kafka at-least-once delivery. Idempotent notification creation. Accept eventual consistency |
| Schema migration complexity | Nhiều bảng mới, foreign keys | Phased migration: tables first, indexes after, constraints last. Rollback script per phase |
