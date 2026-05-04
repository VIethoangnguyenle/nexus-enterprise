# Data Flow Trace — Create Approval Request

## Mô tả

User tạo yêu cầu phê duyệt. Hệ thống match template, snapshot cấu hình, resolve approvers qua NGAC graph, và publish event.

---

## Step 1 — API

**Endpoint:** `POST /api/approval/requests`

**File:** `services/approval/internal/rest/handler.go`
**Function:** `Handler.CreateRequest()` (line 272)

**Input body:**
```json
{
  "entity_type": "purchase_order",
  "entity_id": "UUID",
  "entity_fields": {"amount": "5000000", "category": "office"},
  "form_data_json": "{\"reason\":\"Mua văn phòng phẩm\"}",
  "scope_oa_id": "oa-ke-toan-mgmt",
  "department_id": "dept-ke-toan"
}
```

**Middleware chain:**
1. `httputil.JWTMiddleware` → extract JWT claims
2. `httputil.TenantMiddleware` → set tenant_id
3. `tenantSchemaMiddleware` → resolve tenant schema

**Handler logic:**
- Parse body → `domain.CreateRequestInput{..., CreatedBy: claims.NGACNodeID}`
- Gọi `svc.CreateApprovalRequest(ctx, input)`
- Sau DB commit → `producer.Publish()` event (line 304)

---

## Step 2 — Service (Domain)

**File:** `services/approval/internal/domain/execution.go`
**Function:** `Service.CreateApprovalRequest()` (line 50)

### 2a. Match template

**File:** `services/approval/internal/domain/matcher.go`
**Function:** `Service.ResolveTemplate()` → `MatchTemplate()`

- Gọi `store.ListTemplates(ctx, entityType, true)` → lấy tất cả active templates
- Duyệt từng template theo priority (cao → thấp)
- Kiểm tra conditions: field == value, field >= value, field in [values]
- Trả về template đầu tiên match
- Nếu không match → `ErrNoMatchingTemplate`

### 2b. Snapshot template

```go
snapshot, _ := json.Marshal(tmpl)
```

- Serialize toàn bộ template + steps + conditions thành JSON
- Lưu vào `template_snapshot` → đảm bảo logic phê duyệt không bị ảnh hưởng nếu template bị sửa sau

### 2c. Build request object

```go
req := &Request{
    ID:               uuid.New().String(),
    EntityType:       in.EntityType,
    EntityID:         in.EntityID,
    TemplateID:       tmpl.ID,
    TemplateName:     tmpl.Name,
    TemplateSnapshot: string(snapshot),
    FormDataJSON:     in.FormDataJSON,
    CurrentStep:      1,
    Status:           "pending",
    ScopeOAID:        in.ScopeOAID,
    DepartmentID:     in.DepartmentID,
    CreatedBy:        in.CreatedBy,
    CreatedAt:        time.Now(),
}
```

### 2d. Insert request
- Gọi `store.InsertRequest(ctx, req)`

### 2e. Resolve approvers cho Step 1

**File:** `services/approval/internal/domain/execution.go`
**Function:** `Service.assignStep()` (line 309)

Tùy `step.ApproverType`:

| ApproverType | Logic | GrantSource |
|---|---|---|
| `specific_user` | Gán trực tiếp cho user node | `"direct"` |
| `role_in_dept` | Gọi `policy.ResolveAccessibleScopes(approverValue, "approve")` → tìm tất cả user có quyền approve qua NGAC traversal | `"role:{approverValue}"` |
| `department` | Gán cho tất cả thành viên department UA | `"department:{approverValue}"` |

**NGAC traversal cho `role_in_dept`:**
- Policy service: tìm ngược từ OA → Association có "approve" → UA → User
- Trả về danh sách user node IDs

### 2f. Insert assignments
- Gọi `store.InsertAssignments(ctx, assignments)` → batch insert

### 2g. Audit log
- `logAudit(ctx, req.ID, "created", createdBy, 0, {entity_type, entity_id, template})`
- `logAudit(ctx, req.ID, "assigned", userNodeID, stepOrder, {grant_source})` — mỗi approver

---

## Step 3 — Repository (Store)

**File:** `services/approval/internal/store/store.go`

### 3a. `ListTemplates()` (line 165) — for matching

```sql
SELECT t.id, t.name, t.entity_type, t.is_active, t.priority, t.form_fields,
       t.created_by, t.created_at, t.updated_at,
       (SELECT COUNT(*) FROM approval_steps s WHERE s.template_id = t.id) AS step_count,
       (SELECT COUNT(*) FROM approval_conditions c WHERE c.template_id = t.id) AS condition_count
FROM approval_templates t
WHERE t.entity_type = $1 AND t.is_active = true
ORDER BY t.priority DESC, t.created_at DESC
```

### 3b. `InsertRequest()` (line 243)

```sql
INSERT INTO approval_requests (id, entity_type, entity_id, template_id, template_name,
  template_snapshot, form_data_json, current_step, status, scope_oa_id, department_id,
  created_by, created_at)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13)
```

**Bảng:** `{schema}.approval_requests` — INSERT
| Field | Giá trị |
|---|---|
| id | UUID mới |
| entity_type | "purchase_order" |
| entity_id | entity UUID |
| template_id | matched template UUID |
| template_name | "Mua sắm > 1M" |
| template_snapshot | JSON (full template + steps + conditions) |
| form_data_json | `{"reason":"Mua văn phòng phẩm"}` |
| current_step | 1 |
| status | `pending` |
| scope_oa_id | "oa-ke-toan-mgmt" |
| department_id | "dept-ke-toan" |
| created_by | user NGAC node ID |
| created_at | NOW() |

### 3c. `InsertAssignments()` (line 292)

```sql
INSERT INTO approval_assignments (id, request_id, step_order, user_node_id, grant_source, status)
VALUES ($1, $2, $3, $4, $5, $6)
```

**Bảng:** `{schema}.approval_assignments` — INSERT (N records, 1 per approver)
| Field | Giá trị |
|---|---|
| id | UUID mới |
| request_id | request UUID |
| step_order | 1 |
| user_node_id | approver NGAC node ID |
| grant_source | `"role:KeToan_Chief"` hoặc `"direct"` |
| status | `pending` |

### 3d. `InsertAuditEntry()` (line 633) — 2+ entries

**Bảng:** `{schema}.approval_audit_log` — INSERT

Entry 1 (created):
| Field | Giá trị |
|---|---|
| action | `created` |
| actor_node_id | creator NGAC node |
| step_order | 0 |
| detail | `{"entity_type":"purchase_order","template":"Mua sắm > 1M"}` |

Entry 2+ (assigned, mỗi approver):
| Field | Giá trị |
|---|---|
| action | `assigned` |
| actor_node_id | approver NGAC node |
| step_order | 1 |
| detail | `{"grant_source":"role:KeToan_Chief"}` |

---

## Step 4 — Event

**File:** `services/approval/internal/rest/handler.go` (line 304)

```go
h.producer.Publish(ctx, events.ApprovalEventPayload{
    RequestID:    req.ID,
    TemplateName: req.TemplateName,
    EntityType:   req.EntityType,
    Status:       req.Status,
    Action:       "created",
    ActorNodeID:  claims.NGACNodeID,
    CreatedBy:    claims.NGACNodeID,
    ScopeOaID:    req.ScopeOAID,
})
```

- Topic: `approval-events`
- Consumer: Messaging service → tạo notification cho approvers

---

## Step 5 — Frontend Update

- WebSocket broadcast → approvers nhận notification
- TanStack Query invalidation → pending list refresh
- Approver thấy request mới trong tab "Chờ duyệt"

---

## Database Impact Tổng Kết

| Schema | Bảng | Thao tác | Số records |
|---|---|---|---|
| `tenant_*` | `approval_templates` | SELECT | N templates (matching) |
| `tenant_*` | `approval_conditions` | SELECT | M conditions (matching) |
| `tenant_*` | `approval_requests` | INSERT | 1 |
| `tenant_*` | `approval_assignments` | INSERT | N (1 per approver) |
| `tenant_*` | `approval_audit_log` | INSERT | N+1 (created + assigned per approver) |

---

## NGAC Check

| Điểm kiểm tra | File | Function | Logic |
|---|---|---|---|
| Resolve approvers | `execution.go:327` | `assignStep()` | `policy.ResolveAccessibleScopes(approverValue, "approve")` |
| NGAC graph traversal | Policy service | `ResolveAccessibleScopes()` | Traverse OA → Association [approve] → UA → User |
| Tạo request | **Không có NGAC check** | `CreateApprovalRequest()` | Mọi user có thể tạo yêu cầu |
