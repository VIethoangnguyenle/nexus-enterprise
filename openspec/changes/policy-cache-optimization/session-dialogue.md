# Session Dialogue: policy-cache-optimization

> Evidence-based audit trail. Write-only: Orchestrator. Read-only: Human.
> KHÔNG narrative. KHÔNG roleplay. CHỈ structured reasoning.

---

## Phase 1: CEO

### D-001: Scope — Backend-only targeted invalidation

Quyết định:
- Scope = backend refactor, targeted Redis L1 invalidation + metrics. Không UI, không proto change.
- Bằng chứng: 
  - file: `policy/internal/grpc/server.go`, vị trí: L271-287, quan sát: `invalidateCache()` dùng SCAN wildcard flush toàn bộ `ngac:access:*`
  - file: `policy/internal/grpc/write_server.go`, vị trí: L178-197, quan sát: `invalidateRedisCache()` cùng pattern SCAN + DEL toàn bộ
  - file: `policy/internal/grpc/server.go`, vị trí: L242-243, quan sát: key format `ngac:access:{userID}:{objectID}:{op}` — đã có structured key cho phép targeted delete
- Xác nhận: đã verify 2 server implementations cùng có full-flush bug, key format đã có prefix per-user per-object
- Trạng thái: VERIFIED
- Độ tin cậy: HIGH

Phương án loại bỏ: Event-driven invalidation (Redpanda) — quá phức tạp cho scale hiện tại (200 users). Sẽ defer.

---

## Phase 3: User Checkpoint

Chuyển giao:
- BA → User: specs/ (2 specs, 7 stories, 15 AC). User approved với "process".

---

## Phase 4: SA

### D-002: Flat keys + prefix delete thay vì Redis Hash

Quyết định:
- Giữ flat key format `ngac:access:{user}:{obj}:{op}`, dùng prefix-based DEL thay vì SCAN wildcard
- Bằng chứng:
  - file: `policy/internal/grpc/server.go`, vị trí: L242-243, quan sát: `accessCacheKey` đã format `ngac:access:{userID}:{objectID}:{op}` — prefix per-user đã có sẵn
  - file: `policy/internal/ngac/graph.go`, vị trí: L240-263, quan sát: `GetDescendants()` BFS có sẵn, dùng để resolve affected U nodes từ UA
- Xác nhận: key structure cho phép `DEL ngac:access:{userID}:*` — xóa chỉ keys của 1 user, không ảnh hưởng users khác
- Trạng thái: VERIFIED
- Độ tin cậy: HIGH

Phương án loại bỏ: Redis Hash per user — yêu cầu thay đổi read path + Hash fields không có individual TTL. Over-engineering.

### D-003: PC change → full flush fallback

Quyết định:
- PolicyClass thay đổi = full flush. Vì PC change ảnh hưởng toàn bộ graph, không thể targeted.
- Bằng chứng:
  - file: `policy/internal/ngac/models.go`, vị trí: L78-80, quan sát: PC là root — U, UA, OA đều assign lên PC. Thay đổi PC = thay đổi mọi permission path.
- Xác nhận: PC change chỉ xảy ra khi tạo workspace mới (~hiếm). Full flush acceptable.
- Trạng thái: VERIFIED
- Độ tin cậy: HIGH

---

## Phase 5: UX
SKIP — Backend-only change, no UI.

---

## Phase 6: DEV

### Files created:
- `policy/internal/ngac/cache_invalidator.go` — NEW: Targeted invalidation logic
- `policy/internal/metrics/metrics.go` — NEW: Prometheus metric definitions

### Files modified:
- `policy/internal/grpc/write_server.go` — Replaced full-flush with CacheInvalidator
- `policy/internal/grpc/server.go` — Replaced full-flush with CacheInvalidator
- `policy/internal/grpc/read_server.go` — Added L1/L2/L3 metrics instrumentation
- `policy/cmd/main.go` — Wired CacheInvalidator + Prometheus HTTP
- `policy/cmd/policy-read/main.go` — Added Prometheus HTTP
- `services/policy/go.mod` — Added prometheus/client_golang dependency

### Build: ✅ PASS (`go build ./...`)
### Tests: ✅ PASS (`go test ./...` — all 14 tests pass)
### Vet: ✅ PASS (`go vet ./...`)

---

## Phase 7: SA Verify
- Architecture integrity: ✅ CONFIRMED
- Node type resolution: ✅ U→direct, UA→BFS descendants, OA→direct+descendants, PC→full flush
- Both server impls share CacheInvalidator: ✅ VERIFIED
- LoadGraph still uses full flush: ✅ VERIFIED
- No SCAN wildcard in mutation path: ✅ Only scoped prefix in collectKeys()

---

## Phase 8: QA
- Policy service build: ✅ PASS
- Policy service tests: ✅ 14/14 PASS
- Policy service vet: ✅ PASS
- Cross-service builds: ✅ All 7 services build (drive, messaging, approval, workspace, auth, asset, document)
- No regressions: ✅ CONFIRMED

---

## Phase 9: DONE
Pipeline complete. All gates passed.
