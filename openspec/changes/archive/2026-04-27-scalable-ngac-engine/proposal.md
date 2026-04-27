## Why

Policy Service hiện tại có 3 vấn đề nghiêm trọng ở scale lớn:

1. **Full graph in-memory** — `LoadGraph()` đọc toàn bộ nodes, assignments, associations vào Go maps. Với 100K+ nodes → RAM vượt ngưỡng, OOM risk.
2. **Single PDP bottleneck** — 6 services (Auth, Workspace, Document, Messaging, Asset × 3 servers) đều gRPC tới 1 Policy Service. Policy down = toàn bộ hệ thống deny.
3. **Thuật toán không hiệu quả** — DFS đệ quy × 4 lần traversal cho mỗi CheckAccess, không early termination, O(N) linear scan cho FindNodeByName, visited set tạo mới mỗi vòng lặp.

Ngoài ra, cross-workspace sharing (Share document từ WS_A sang user WS_B qua PC_Global) yêu cầu traverse 3 scope đồng thời — partition-by-workspace không giải quyết được.

## What Changes

### Phase 1: Algorithm Optimization (không thay đổi kiến trúc)
- Refactor `CheckAccess` từ 4×DFS sang 2×BFS iterative với early termination
- Thêm `nameTypeIndex` cho O(1) `FindNodeByName`
- Merge UAs/OAs/PCs collection vào single-pass BFS
- Loại bỏ đệ quy → iterative queue-based traversal

### Phase 2: CQRS Read/Write Split + DB-Driven Evaluation
- Tách `PolicyService` proto thành `PolicyReadService` + `PolicyWriteService`
- Read service: stateless, 3-layer cache (Redis L1 → Materialized L2 → SQL CTE L3)
- Write service: singleton, persist + publish Kafka events + invalidate cache
- SQL recursive CTE thay thế in-memory graph cho read operations
- Sub-problem caching: cache ancestors/associations riêng, targeted invalidation
- Graph version tracking thay vì flush toàn bộ Redis cache

## Capabilities

### Modified Capabilities
- `ngac-engine`: Thuật toán graph traversal tối ưu 3-7× qua BFS + early termination
- `ngac-engine`: CheckAccess có thể chạy DB-only (zero in-memory graph requirement)

### New Capabilities
- `policy-cqrs`: Read/Write separation cho Policy Service — reads scale horizontal
- `materialized-access`: Pre-computed access decisions cached tại DB layer
- `sub-problem-cache`: Granular caching (per-user ancestors, per-object ancestors) thay vì all-or-nothing
- `graph-versioning`: Version-based cache consistency thay vì full flush

## Impact

**Backend (Policy Service — major refactor)**:
- `services/policy/internal/ngac/access.go` — rewrite toàn bộ CheckAccess
- `services/policy/internal/ngac/graph.go` — thêm BFS methods, nameType index
- `backend/proto/policy/policy.proto` — tách thành read/write services
- Thêm SQL CTE functions cho DB-driven evaluation
- Thêm materialized access table

**Database**:
- Thêm table `ngac_materialized_access` (pre-computed decisions)
- Thêm table `ngac_graph_version` (version tracking)
- Thêm indexes cho recursive CTE performance

**Other Services (minor)**:
- Services update gRPC import từ `PolicyServiceClient` → `PolicyReadServiceClient` + `PolicyWriteServiceClient`
- Không thay đổi business logic — chỉ thay đổi client target

**Infrastructure**:
- Docker Compose: thêm N policy-read replicas
- PostgreSQL: cấu hình read replicas (optional, Phase 3)
