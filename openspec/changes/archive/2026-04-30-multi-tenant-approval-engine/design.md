# Design: Multi-Tenant Approval Workflow Engine

## Overview

Hybrid NGAC + Approval Engine architecture. NGAC xử lý structural access (ai thuộc department nào, role gì). Approval Engine xử lý per-item dynamic access (ai duyệt giao dịch nào) với denormalized tables cho fast query. Schema-per-tenant cho data isolation.

---

## Part 1: Schema-per-Tenant

### Cấu trúc database

```
PostgreSQL
├── public                     ← shared across all tenants
│   ├── tenants                   tenant registry
│   ├── users                     user accounts + ngac_node mapping
│   └── billing                   subscription/billing
│
├── tenant_{uuid_short}        ← isolated per tenant
│   ├── transactions              business data
│   ├── documents                 files/drive items
│   ├── approval_templates        workflow definitions
│   ├── approval_conditions       template match conditions
│   ├── approval_steps            step definitions
│   ├── approval_requests         execution instances
│   ├── approval_assignments      denormalized user→item access
│   └── approval_audit_log        append-only audit trail
```

### Tenant routing

Mỗi request sau JWT auth → resolve tenant schema từ claims → `SET search_path`:

```go
func TenantSchemaMiddleware(next echo.HandlerFunc) echo.HandlerFunc {
    return func(c echo.Context) error {
        claims := httputil.GetClaims(c)
        schema := fmt.Sprintf("tenant_%s", claims.TenantID[:8])
        conn := getConnFromPool()
        conn.Exec(ctx, "SET search_path TO "+pgx.Identifier{schema}.Sanitize()+", public")
        return next(c)
    }
}
```

### Tenant provisioning

Khi tenant mới đăng ký → background job:
1. `CREATE SCHEMA tenant_{id}`
2. Run migration scripts tạo tables
3. Seed default approval templates (optional)

---

## Part 2: NGAC Graph Refactor — Structural Only

### Thay đổi cốt lõi

Graph chỉ chứa structural nodes (UA, OA, PC, associations). **KHÔNG load O nodes** (transactions, documents) vào graph. O nodes chỉ cần biết parent OA → column `scope_oa_id` trong data table.

### OA Hierarchy — phản ánh cấu trúc tổ chức với regional grouping

OA hierarchy phải hỗ trợ **regional nodes** để Manager Miền Nam chỉ thấy data miền Nam, Manager Miền Bắc chỉ thấy miền Bắc, CEO thấy tất cả:

```
PC_TenantX
└── OA: Root_t1
    ├── OA: HoiSo_Docs
    │   └── OA: KeToan_HoiSo_Docs
    │
    ├── OA: MienNam_Region              ← REGIONAL NODE
    │   ├── OA: CN_HCM_Docs
    │   └── OA: CN_CanTho_Docs
    │
    └── OA: MienBac_Region              ← REGIONAL NODE
        ├── OA: CN_HaNoi_Docs
        └── OA: CN_HaiPhong_Docs
```

### UA Hierarchy — roles với regional scope

```
PC_TenantX
├── UA: HoiSo_Directors              ← CEO, CFO (toàn quyền)
├── UA: HoiSo_MienNam_Managers       ← Manager miền Nam
├── UA: HoiSo_MienBac_Managers       ← Manager miền Bắc
├── UA: CN_HCM_Manager
├── UA: CN_HCM_Staff
├── UA: CN_HaNoi_Manager
├── UA: CN_HaNoi_Staff
├── UA: KeToan_HoiSo_Chief
└── UA: KeToan_HoiSo_Staff
```

### Associations — ai thấy data của ai

```
CEO thấy tất cả:    HoiSo_Directors ──[read,approve]──> Root_t1
Mgr MN thấy MN:     MienNam_Managers ──[read]──────────> MienNam_Region
Mgr MB thấy MB:     MienBac_Managers ──[read]──────────> MienBac_Region
CN HCM tự quản:     CN_HCM_Manager ──[read,write,approve]──> CN_HCM_Docs
                    CN_HCM_Staff ──[read,write]────────────> CN_HCM_Docs
Kế toán HQ:         KeToan_Chief ──[read,write,approve]──> KeToan_HoiSo_Docs
                    KeToan_Staff ──[read,write]───────────> KeToan_HoiSo_Docs
```

**Key insight**: Gán association cho OA cha (VD: `MienNam_Region`) → tự động có quyền tới TẤT CẢ OA con (`CN_HCM_Docs`, `CN_CanTho_Docs`) nhờ NGAC upward BFS.

### scope_oa_id — cầu nối universal giữa NGAC và data queries

Mọi entity trong tenant đều có `scope_oa_id`:

```
transactions.scope_oa_id      → "giao dịch thuộc department nào?"
documents.scope_oa_id         → "tài liệu thuộc department nào?"
approval_requests.scope_oa_id → "lệnh duyệt thuộc department nào?"
drive_items.scope_oa_id       → "file thuộc department nào?"
```

Database chứa millions of rows — KHÔNG load vào graph.

### Scope-based filtering (thay thế per-item CheckAccess)

Khi list giao dịch, thay vì loop CheckAccess:

```go
// TRƯỚC: O(N × gRPC) — bottleneck
for _, item := range items {
    resp, _ := policyRead.CheckAccess(ctx, userNodeID, item.NGACNodeID, "read")
    if resp.Decision == "ALLOW" { visible = append(visible, item) }
}

// SAU: O(1) — resolve scopes 1 lần, SQL filter
scopes := policyRead.ResolveAccessibleScopes(ctx, userNodeID, "read")
// → returns: ["oa_cn1_docs", "oa_ketoan_docs"]
items := store.ListByScopes(ctx, scopes, pagination)
// → SQL: WHERE scope_oa_id = ANY($1) ORDER BY created_at DESC LIMIT 20
```

### Mới: ResolveAccessibleScopes RPC

```protobuf
// Thêm vào policy_read.proto
rpc ResolveAccessibleScopes(ResolveAccessibleScopesRequest) 
    returns (AccessibleScopesResponse);

message ResolveAccessibleScopesRequest {
    string user_node_id = 1;
    string operation = 2;     // "read", "approve"
}

message AccessibleScopesResponse {
    repeated string scope_oa_ids = 1;  // OA nodes user can access
}
```

Implementation (3 bước):
1. BFS upward từ user node → tìm tất cả UA ancestors
2. Tìm associations từ các UA đó matching operation → collect target OA IDs
3. **GetDescendants cho mỗi target OA** → flatten thành leaf OA IDs

Bước 3 rất quan trọng: khi Manager MN có association tới `MienNam_Region`, resolve phải trả về `[CN_HCM_Docs, CN_CanTho_Docs]` (leaf nodes), không phải `[MienNam_Region]` (parent node). Vì data rows lưu `scope_oa_id = CN_HCM_Docs` (leaf), không phải regional parent.

Kết quả cache trong Redis (TTL 60s, invalidate on policy change).

**Ví dụ resolve cho từng persona:**

| User | UA | Association target | Descendants | Final scopes |
|------|----|--------------------|-------------|-------------|
| CEO | HoiSo_Directors | Root_t1 | ALL | [HoiSo_Docs, KeToan_HoiSo_Docs, CN_HCM_Docs, CN_CanTho_Docs, CN_HaNoi_Docs, CN_HaiPhong_Docs] |
| Mgr MN | MienNam_Managers | MienNam_Region | CN_HCM, CN_CanTho | [CN_HCM_Docs, CN_CanTho_Docs] |
| Mgr MB | MienBac_Managers | MienBac_Region | CN_HN, CN_HP | [CN_HaNoi_Docs, CN_HaiPhong_Docs] |
| NV HCM | CN_HCM_Staff | CN_HCM_Docs | (leaf) | [CN_HCM_Docs] |

---

## Part 3: Approval Workflow Engine

### 3.1 Template Definitions (admin cấu hình)

```sql
CREATE TABLE approval_templates (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name            TEXT NOT NULL,
    entity_type     TEXT NOT NULL,        -- 'transaction', 'purchase_order', 'document'
    is_active       BOOLEAN DEFAULT true,
    priority        INT DEFAULT 0,       -- higher priority matched first
    created_by      TEXT NOT NULL,
    created_at      TIMESTAMPTZ DEFAULT NOW(),
    updated_at      TIMESTAMPTZ DEFAULT NOW()
);

CREATE TABLE approval_conditions (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    template_id     UUID REFERENCES approval_templates(id) ON DELETE CASCADE,
    field           TEXT NOT NULL,        -- 'amount', 'service_type', 'department_id', 'category'
    operator        TEXT NOT NULL,        -- 'gt', 'lt', 'eq', 'in', 'between', 'any'
    value           JSONB NOT NULL       -- {"threshold": 100000000} or ["transfer", "loan"]
);

CREATE TABLE approval_steps (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    template_id     UUID REFERENCES approval_templates(id) ON DELETE CASCADE,
    step_order      INT NOT NULL,
    name            TEXT NOT NULL,        -- "Trưởng phòng duyệt"
    approver_type   TEXT NOT NULL,        -- 'specific_user', 'role_in_dept', 'department', 'creator_manager'
    approver_value  TEXT,                 -- UA name or user_node_id or "{creator_dept}" placeholder
    required_count  INT DEFAULT 1,       -- cần bao nhiêu người duyệt ở step này
    timeout_hours   INT,                 -- NULL = no timeout
    UNIQUE(template_id, step_order)
);
```

**Approver types và resolution:**

| `approver_type` | `approver_value` | Resolution via NGAC |
|---|---|---|
| `specific_user` | `user_node_id` | Direct — 1 user |
| `role_in_dept` | `KeToan_Chief` | `GetDescendants(KeToan_Chief_UA)` → users with that role |
| `department` | `KeToan_Dept` | `GetDescendants(KeToan_Dept_UA)` → ALL department members |
| `creator_manager` | `{creator_dept}_Manager` | Resolve creator's dept from NGAC → find manager UA |

### 3.2 Execution Runtime

```sql
CREATE TABLE approval_requests (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    entity_type     TEXT NOT NULL,
    entity_id       UUID NOT NULL,
    template_id     UUID REFERENCES approval_templates(id),
    template_snapshot JSONB NOT NULL,   -- frozen copy of template at creation time
    current_step    INT DEFAULT 1,
    status          TEXT DEFAULT 'pending'
                    CHECK (status IN ('pending','approved','rejected','cancelled')),
    -- NGAC scope: link request tới department hierarchy
    scope_oa_id     TEXT NOT NULL,      -- OA node của department tạo lệnh
    department_id   TEXT NOT NULL,      -- human-readable department reference
    created_by      TEXT NOT NULL,      -- creator's ngac_node_id
    created_at      TIMESTAMPTZ DEFAULT NOW(),
    completed_at    TIMESTAMPTZ
);

-- Index cho Tab 4: "Tất cả lệnh department" (scope-based query)
CREATE INDEX idx_ar_scope ON approval_requests(scope_oa_id, status, created_at DESC);

-- THE MONEY TABLE — denormalized for fast queries
CREATE TABLE approval_assignments (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    request_id      UUID REFERENCES approval_requests(id) ON DELETE CASCADE,
    step_order      INT NOT NULL,
    user_node_id    TEXT NOT NULL,
    grant_source    TEXT NOT NULL,       -- 'direct', 'role:KeToan_Chief', 'department:KeToan_Dept'
    status          TEXT DEFAULT 'pending'
                    CHECK (status IN ('pending','approved','rejected','skipped','revoked')),
    acted_at        TIMESTAMPTZ,
    comment         TEXT
);

-- Indexes cho 2 query patterns chính
CREATE INDEX idx_aa_pending ON approval_assignments(user_node_id, status)
    WHERE status = 'pending';
CREATE INDEX idx_aa_history ON approval_assignments(user_node_id, status, acted_at DESC)
    WHERE status IN ('approved', 'rejected');
```

### 3.3 Query Patterns — 4 Tabs

**Tab 1: "Chờ duyệt"** — load all, no paging (dataset nhỏ per user):

```sql
SELECT ar.id, ar.entity_type, ar.entity_id, ar.status,
       ar.template_snapshot->>'name' AS workflow_name,
       ar.created_at, aa.step_order, aa.grant_source
FROM approval_assignments aa
JOIN approval_requests ar ON aa.request_id = ar.id
WHERE aa.user_node_id = $1
  AND aa.status = 'pending'
  AND ar.current_step = aa.step_order
ORDER BY ar.created_at ASC;
```

**Tab 2: "Đã duyệt"** — cursor paging, chỉ người đã action thấy:

```sql
SELECT ar.id, ar.entity_type, ar.entity_id, ar.status AS request_status,
       aa.status AS my_action, aa.acted_at, aa.comment, aa.step_order
FROM approval_assignments aa
JOIN approval_requests ar ON aa.request_id = ar.id
WHERE aa.user_node_id = $1
  AND aa.status IN ('approved', 'rejected')
  AND aa.acted_at < $2              -- cursor
ORDER BY aa.acted_at DESC
LIMIT 20;
```

**Tab 3: "Lệnh tôi tạo"** — cursor paging:

```sql
SELECT ar.* FROM approval_requests ar
WHERE ar.created_by = $1
  AND ar.created_at < $2            -- cursor
ORDER BY ar.created_at DESC
LIMIT 20;
```

**Tab 4: "Tất cả lệnh department"** — scope-based, cursor paging:

Đây là tab cho HoiSo/Manager xem lệnh thuộc các department họ quản lý.
Không cần là approver — chỉ cần có structural read access qua NGAC.

```sql
-- Step 1: Resolve scopes (cached in Redis)
-- ResolveAccessibleScopes(user_node, 'read') → scope_oa_ids[]

-- Step 2: Pure index scan
SELECT ar.* FROM approval_requests ar
WHERE ar.scope_oa_id = ANY($1)      -- $1 = user's accessible scopes
  AND ar.created_at < $2             -- cursor
ORDER BY ar.created_at DESC
LIMIT 20;

-- CEO:        scopes = ALL → thấy mọi lệnh
-- Mgr MN:     scopes = [CN_HCM, CN_CanTho] → chỉ thấy miền Nam
-- Mgr MB:     scopes = [CN_HaNoi, CN_HaiPhong] → chỉ thấy miền Bắc
-- NV CN HCM:  scopes = [CN_HCM] → chỉ thấy CN HCM
```

**Batch approve:**

```sql
UPDATE approval_assignments
SET status = 'approved', acted_at = NOW(), comment = $3
WHERE user_node_id = $1
  AND request_id = ANY($2)
  AND status = 'pending'
RETURNING request_id, step_order;
```

### Query Performance Summary

| Tab | Method | Index | Cost |
|-----|--------|-------|------|
| Chờ duyệt | approval_assignments scan | `(user_node_id, status) WHERE pending` | < 5ms, no paging |
| Đã duyệt | approval_assignments cursor | `(user_node_id, status, acted_at)` | < 10ms |
| Lệnh tôi tạo | approval_requests cursor | `(created_by, created_at)` | < 10ms |
| Tất cả dept | approval_requests scope | `(scope_oa_id, status, created_at)` | < 10ms |
| Batch approve | approval_assignments update | `(user_node_id, status)` | < 100ms for 50 items |

**Zero NGAC graph traversal cho mọi query.** Scope resolution cached trong Redis.

### 3.4 Step Advancement Logic

Khi approve action xảy ra:

```
1. UPDATE assignment → 'approved'
2. COUNT approved assignments cho step này
3. IF count >= required_count:
   a. UPDATE tất cả remaining assignments cho step → 'skipped'
   b. IF last step → request.status = 'approved', completed_at = NOW()
   c. ELSE → request.current_step += 1
4. Audit log: INSERT action record
```

Khi reject:

```
1. UPDATE assignment → 'rejected'  
2. request.status = 'rejected', completed_at = NOW()
3. UPDATE tất cả remaining assignments → 'skipped'
4. Audit log: INSERT action record
```

### 3.5 Audit Trail (append-only)

```sql
CREATE TABLE approval_audit_log (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    request_id      UUID NOT NULL,
    action          TEXT NOT NULL,
    -- actions: 'created', 'assigned', 'approved', 'rejected',
    --          'step_advanced', 'completed', 'cancelled',
    --          'reassigned_policy_change', 'revoked_policy_change'
    actor_node_id   TEXT NOT NULL,
    step_order      INT,
    detail          JSONB,             -- contextual data per action type
    ip_address      INET,
    created_at      TIMESTAMPTZ DEFAULT NOW()
);

CREATE INDEX idx_audit_request ON approval_audit_log(request_id, created_at);
```

### 3.6 Policy Change Reconciliation

Khi NGAC graph thay đổi (user joins/leaves department, role change):

```
Event: "User Tú added to KeToan_Dept_UA"
→ Reconciliation Worker:
  1. Find pending assignments WHERE grant_source LIKE 'department:KeToan_Dept%'
  2. INSERT new assignment cho Tú for each pending request
  3. Audit: 'reassigned_policy_change'

Event: "User Lan removed from KeToan_Chief_UA"  
→ Worker:
  1. Find Lan's pending assignments WHERE grant_source = 'role:KeToan_Chief'
  2. Mark as 'revoked'
  3. Check: step has other approvers? If not → alert admin
  4. Audit: 'revoked_policy_change'
```

**Consistency model**: Reads are eventually consistent (seconds delay). Writes always double-check via NGAC graph before executing approve/reject.

---

## Part 4: Service Architecture

### New service: Approval Service

```
backend/services/approval/
├── cmd/main.go
├── internal/
│   ├── rest/handler.go           POST /approve, POST /reject, POST /batch-approve
│   │                             GET /pending, GET /history
│   ├── grpc/server.go            CreateRequest, ResolveApprovers
│   ├── domain/
│   │   ├── service.go            Template matching, step advancement, reconciliation
│   │   ├── resolver.go           Approver resolution via NGAC
│   │   └── errors.go
│   └── store/
│       ├── store.go              CRUD for all approval tables
│       ├── models.go
│       └── migrations/           Schema creation SQL
├── go.mod
└── Dockerfile
```

### Inter-service communication

```
Client → REST → Approval Service
                    │
                    ├── gRPC → Policy Service (ResolveAccessibleScopes, GetDescendants)
                    │          (structural queries only, no per-item check)
                    │
                    └── Redpanda ← Policy Service (policy change events)
                                → Reconciliation worker
```

---

## Verification

1. **Unit tests**: Template matching, step advancement, batch approve, reject=terminal
2. **Integration tests**: Schema provisioning, cross-schema isolation
3. **Performance tests**: 
   - "Chờ duyệt" query < 5ms with 50K assignments/tenant
   - "Đã duyệt" cursor paging < 10ms
   - Batch approve 50 items < 100ms
4. **Policy change reconciliation**: User role change → assignments updated within 10s
5. **Build**: `go build ./cmd/` passes for approval service
