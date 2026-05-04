## Context

Policy service hiện tại dùng singleton `Graph` (graph.go) load toàn bộ nodes/edges từ DB vào RAM khi khởi động (`store.go:LoadGraph`). Kiến trúc này hoạt động tốt với vài chục tenants nhưng đạt giới hạn ~62GB RAM ở 50K workspaces.

**Trạng thái hiện tại:**
- `Graph` struct: in-memory DAG với `map[string]*NGACNode`, adjacency indexes
- `DecisionEngine`: BFS traversal → CTE fallback (broken) → prohibition check
- `findCommonPC`: single-PC intersection (vi phạm NIST spec khi Multi-PC)
- `CTEEvaluator`: gọi `ngac_check_access` SQL function — **function chưa tồn tại trong DB**
- PC lifecycle: chỉ system code tạo, không có metadata `tenant_id` trên secondary PCs

## Goals / Non-Goals

**Goals:**
- Giảm memory footprint từ ~62GB → ~1.5GB cho 50K tenants (lazy-load top 1000 shards)
- Giữ latency BFS ≤ 1ms cho hot workspaces (trong RAM)
- CTE SQL fallback hoạt động đúng cho cold workspaces (~3-10ms)
- Fix PC intersection algorithm đúng NIST NGAC spec (ALL-intersection)
- Hỗ trợ Multi-PC per-tenant (Confidential, PCI) — tenant admin tạo secondary PCs
- Backward-compatible: callers không truyền workspace_id vẫn hoạt động (fallback CTE)

**Non-Goals:**
- Distributed sharding across multiple instances (single-instance LRU đủ cho phase 1)
- Cross-tenant graph merging tại runtime
- Auto-rebalancing shards giữa các instances
- UI cho tenant admin tạo/quản lý secondary PCs (chỉ API layer)

## Decisions

### D1: Shard key = Tenant PC ID

**Decision**: Mỗi shard là 1 `*Graph` instance chứa tất cả nodes/edges thuộc 1 workspace + secondary PCs + PC_Global.

**Rationale**: Mỗi tenant request chỉ cần graph của workspace mình. PC là system-controlled, bounded, predictable. Tenant PC là 1:1 với workspace — shard key tự nhiên.

**Alternatives considered**:
- Shard by user: User thuộc nhiều workspace → duplicate → phức tạp
- Shard by department: Quá nhỏ, cross-dept requests cần merge
- Global graph with lazy-load regions: Ranh giới mơ hồ, khó evict

### D2: LRU Cache với max 1000 shards

**Decision**: Dùng LRU cache giữ max ~1000 shards đã load. Khi cache đầy, evict shard ít dùng nhất.

**Rationale**: Pareto principle — top 1000 active workspaces xử lý >95% traffic. Mỗi shard ~1.5MB → 1000 shards = ~1.5GB RAM. Cold workspaces dùng CTE SQL (~3-10ms).

**Alternatives considered**:
- TTL-based eviction: Không tốt — 1 workspace idle 30 phút bị evict, request tiếp theo chậm
- LFU: Phức tạp hơn LRU, marginal benefit cho use case này
- Unlimited cache: Quay về bài toán cũ khi tenant tăng

### D3: CTE SQL Fallback cho cold path

**Decision**: Tạo SQL function `ngac_check_access(user_id, object_id, operation)` dùng recursive CTE. Cold workspaces dùng path này trong khi shard đang lazy-load.

**Rationale**: CTE SQL ~3-10ms chấp nhận được cho cold path. Sau khi trả response, trigger async `LoadShard()` để lần sau nhanh hơn (auto-promote cold → hot).

**Flow**:
```
Request → resolve workspace_id → check LRU cache
  ├── HIT  → BFS in-memory (~0.1ms) → response
  └── MISS → CTE SQL (~3-10ms) → response
             + async LoadShard() → promote to LRU
```

### D4: ALL-intersection thay vì single-PC

**Decision**: Đổi `findCommonPC` → `allPCsSatisfied`: `objectPCs ⊆ userPCs`.

**Rationale**: NIST NGAC spec yêu cầu user phải thuộc TẤT CẢ PCs mà object thuộc. Code hiện tại chỉ check "ít nhất 1 PC chung" — sai khi object thuộc 2+ PCs.

**Backward compatibility**: Với single-PC (hiện tại), behavior giống hệt. Chỉ khác khi object thuộc nhiều PCs — case này chưa xảy ra trong production.

### D5: Workspace routing qua proto field

**Decision**: Thêm `optional string workspace_id` vào `CheckAccessRequest` proto.

**Rationale**: Request cần biết workspace nào để route tới đúng shard. Optional field giữ backward-compatible — nếu thiếu thì fallback CTE.

**Alternatives considered**:
- Resolve workspace từ node_id: Cần DB lookup mỗi request → latency
- Metadata trong context: Không standard, khó trace

### D6: PC metadata dùng properties JSONB

**Decision**: Dùng cột `properties` JSONB hiện có để lưu `{"scope":"tenant","tenant_id":"ws-xxx"}` cho PC nodes. Không thêm column mới.

**Rationale**: Schema đã có `properties JSONB DEFAULT '{}'`. Không cần migration. Shard loader query `properties->>'tenant_id' = $1` để tìm secondary PCs.

### D7: ShardManager interface tách biệt Graph

**Decision**: Tạo `ShardManager` bọc logic LRU + LoadShard + eviction. `DecisionEngine` chỉ gọi `ShardManager.GetGraph(workspaceID)`.

**Rationale**: Separation of concerns — Graph giữ nguyên BFS logic, ShardManager quản lý lifecycle. Testable: mock ShardManager cho unit tests.

```
DecisionEngine
  └── ShardManager.GetGraph(workspaceID)
        ├── LRU hit → return *Graph
        └── LRU miss → LoadShard() → return *Graph
```

## Risks / Trade-offs

| Risk | Impact | Mitigation |
|---|---|---|
| CTE SQL chậm hơn BFS 30-100x | Cold requests ~3-10ms thay vì ~0.1ms | Chấp nhận cho cold path. Auto-promote sau. |
| Shard load time ~50-200ms | First request tới cold workspace chậm | CTE trả response trước, load shard async |
| Stale shard (graph bị modify sau khi load) | Decision dựa trên data cũ | Invalidation qua Redis pub/sub (đã có) reload shard |
| LRU thrashing nếu >1000 distinct workspaces active cùng lúc | Continuous eviction/load | Monitor cache hit rate. Tăng max_shards nếu cần. |
| PC metadata thiếu consistency | Secondary PC không được load vào shard | Shard loader query TẤT CẢ PCs cùng tenant_id |

## Migration Plan

1. **Phase 1 — Safety net**: Tạo CTE SQL function + fix findCommonPC. Zero behavior change cho existing callers.
2. **Phase 2 — Sharding**: Thêm ShardManager, refactor DecisionEngine. LoadGraph() chuyển thành LoadShard(). Old path vẫn hoạt động.
3. **Phase 3 — Routing**: Thêm workspace_id vào proto. Update callers dần dần. Callers cũ fallback CTE.

**Rollback**: Revert ShardManager, dùng lại LoadGraph(). Tất cả changes đều additive — không xóa code cũ cho đến khi stable.
