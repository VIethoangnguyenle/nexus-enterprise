# Spec: Cache Observability Metrics

## Overview
Thêm Prometheus metrics cho Policy service để monitor cache performance và detect degradation.

## Proto Mapping
- Không thay đổi proto — metrics expose qua HTTP `/metrics` endpoint (Prometheus standard)

## User Stories

### Story 1: Cache hit/miss metrics
**As a** ops engineer,
**I want** to see cache hit/miss rates per layer (L1/L2/L3),
**so that** I can detect cache degradation before it impacts users.

**Acceptance Criteria:**
- [ ] AC1: Metric `ngac_check_access_total{layer="L1|L2|L3"}` counter tăng mỗi checkAccess
- [ ] AC2: Metric `ngac_check_access_duration_seconds{layer="L1|L2|L3"}` histogram ghi latency
- [ ] AC3: Metrics accessible tại `/metrics` endpoint (hoặc cùng port gRPC)
- [ ] AC4: Build pass: `go build ./services/policy/...`

### Story 2: Cache invalidation metrics
**As a** ops engineer,
**I want** to track invalidation frequency and scope,
**so that** I can correlate mutations with cache misses.

**Acceptance Criteria:**
- [ ] AC1: Metric `ngac_cache_invalidation_total{scope="targeted|full"}` counter
- [ ] AC2: Metric `ngac_cache_keys_deleted_total` counter ghi số keys bị xóa mỗi lần
- [ ] AC3: Log structured: `slog.Info("cache_invalidated", "scope", "targeted", "keys_deleted", N, "affected_nodes", nodeIDs)`

### Story 3: Graph size metrics
**As a** ops engineer,
**I want** to monitor graph size (node count by type),
**so that** I can predict scaling needs.

**Acceptance Criteria:**
- [ ] AC1: Metric `ngac_graph_node_count{type="U|UA|OA|PC"}` gauge cập nhật sau mỗi LoadGraph
- [ ] AC2: Metric `ngac_graph_association_count` gauge

## Implementation Notes
- Dùng `github.com/prometheus/client_golang` (đã có trong go.mod hay chưa → cần check)
- Metrics package: `backend/services/policy/internal/metrics/`
- Register metrics vào default registry
