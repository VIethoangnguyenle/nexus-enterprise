## Context

NGAC backend gồm 8 Go microservices giao tiếp qua gRPC, fronted bởi Traefik (edge proxy) + Gateway (REST→gRPC BFF). Tổng cộng ~10,274 dòng Go production code.

Hiện trạng code quality:
- **Messaging** (`server.go` 524 dòng): Mix handler + business logic + SQL. Không có store layer. Hàm `CreateChannel` 137 dòng. `findExistingDM` gây N+1 gRPC calls (1000 DM channels → 1000 calls).
- **Drive** (`server.go` 622 dòng): Có store layer nhưng handler vẫn gọi `s.db` trực tiếp ở 2 chỗ. Không có domain layer.
- **Gateway** (`main.go` 576 dòng): CORS duplicate với Traefik. WS proxy thừa (Traefik route trực tiếp). Zero input validation.
- **Hardcoded strings**: `"read"`, `"write"`, `"PC_Global"`, `"PublicUsers"`, `"%s_Channels"` rải trong 9 files.

Kiến trúc chuẩn mỗi service (từ AGENTS.md): `cmd/` → `internal/grpc/` → `internal/domain/` → `internal/store/`. Hiện chỉ asset service follow đúng pattern này.

## Goals / Non-Goals

**Goals:**
- Tạo shared NGAC constants package — 1 nơi duy nhất cho operations, node names, naming conventions
- Messaging service follow chuẩn 3-layer: handler → domain → store
- Drive service hoàn thiện layer separation
- Gateway bỏ redundancy với Traefik, thêm input validation
- Fix N+1 DM lookup performance issue
- Giữ nguyên API contract (REST endpoints + gRPC proto không đổi)

**Non-Goals:**
- Thêm features mới cho Chat hoặc Drive (Lark parity là change riêng)
- Refactor workspace, auth, asset, policy services (scope giới hạn ở messaging + drive + gateway)
- Thay đổi database schema ngoài table `channel_members` mới
- Thay đổi proto definitions

## Decisions

### 1. Shared `ngac` package ở root module, không per-service

**Decision**: Tạo `backend/ngac/` package trong root module `ngac-platform`, import bằng `ngac-platform/ngac`.

**Alternatives considered**:
- Per-service copy: Mỗi service có file constants riêng → drift, inconsistency
- Shared proto enum: Thêm operations vào proto → thay đổi proto contract, quá nặng cho string constants

**Rationale**: Root module `ngac-platform` đã được mọi service `replace` tới qua `go.mod`. Package nhẹ (1 file), zero dependencies, chỉ chứa constants và format functions.

### 2. Messaging: Tách store trước, domain sau

**Decision**: Tạo `messaging/internal/store/` chứa SQL operations, rồi `messaging/internal/domain/` chứa business logic orchestration.

**Rationale**: Store layer dễ tách nhất (extract SQL query strings). Domain layer cần store để inject. Làm store trước cho phép domain import ngay.

### 3. DM lookup: DB-level join thay vì gRPC N+1

**Decision**: Thêm table `channel_members(channel_id, ngac_node_id)` và dùng `INTERSECT` query thay vì scan tất cả DM channels + gọi `GetChildren` cho mỗi channel.

**Alternatives considered**:
- Cache NGAC graph locally: Phức tạp, consistency risk
- Denormalize member IDs vào channels table: JSON array khó query, schema pollution

**Rationale**: `channel_members` là denormalization nhẹ — chỉ track membership cho DM lookup purpose. NGAC graph vẫn là source of truth cho access control.

### 4. Gateway: Giữ BFF pattern, bỏ redundancy

**Decision**: Giữ Gateway service (không chuyển sang grpc-gateway). Bỏ CORS + WS proxy (Traefik handle). Thêm `decodeAndValidate` helper.

**Alternatives considered**:
- grpc-gateway plugin: Bỏ Gateway service hoàn toàn, mỗi gRPC service tự expose REST → proto phức tạp hơn, mỗi service cần HTTP server
- Connect protocol: gRPC-Web compatible → chưa mature đủ cho production

**Rationale**: Gateway đã tồn tại, hoạt động ổn. Chỉ cần cleanup, không cần đập xây lại. Risk thấp nhất.

## Risks / Trade-offs

| Risk | Impact | Mitigation |
|------|--------|------------|
| Refactor phá test hiện tại | Medium | Chạy `go test` sau mỗi service thay đổi. Tests hiện tại validate behavior, không structure |
| `channel_members` table out of sync với NGAC graph | Low | Insert khi add member, nhưng NGAC graph vẫn là source of truth cho access checks. `channel_members` chỉ dùng cho DM discovery |
| Import cycle khi thêm `ngac` package | Low | Package `ngac` chỉ chứa constants, zero import từ service code |
| Gateway CORS removal break client | Low | Traefik đã set CORS trước Gateway. Gateway CORS là redundant layer |
