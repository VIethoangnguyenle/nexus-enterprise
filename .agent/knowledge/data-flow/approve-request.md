# Data Flow Trace — Approve Request

## Mô tả

User duyệt một yêu cầu phê duyệt. Hệ thống kiểm tra quyền NGAC, cập nhật trạng thái assignment, kiểm tra bước hoàn thành, và publish event.

---

## Step 1 — API

**Endpoint:** `POST /api/approval/approve`

**File:** `services/approval/internal/rest/handler.go`
**Function:** `Handler.ApproveAction()` (line 319)

**Input body:**
```json
{
  "request_id": "UUID",
  "comment": "Đồng ý"
}
```

**Middleware chain:**
1. `httputil.JWTMiddleware` → extract JWT claims (UserID, NGACNodeID, TenantID)
2. `httputil.TenantMiddleware` → set tenant_id in context
3. `tenantSchemaMiddleware` → resolve tenant schema name, set in context

**Handler logic:**
- Parse body → tạo `domain.ApproveInput{RequestID, UserNodeID: claims.NGACNodeID, Comment}`
- Gọi `svc.Approve(ctx, input)`
- Sau khi Approve thành công → `producer.Publish()` event

---

## Step 2 — Service (Domain)

**File:** `services/approval/internal/domain/execution.go`
**Function:** `Service.Approve()` (line 114)

### 2a. Load request
- Gọi `store.GetRequest(ctx, requestID)` → SELECT từ `approval_requests`
- Kiểm tra `req.Status == "pending"` → nếu không → `ErrRequestCompleted`

### 2b. Load assignment
- Gọi `store.GetAssignment(ctx, requestID, userNodeID)` → SELECT từ `approval_assignments`
- Kiểm tra `assignment.StepOrder == req.CurrentStep` → nếu không → `ErrStepNotActive`

### 2c. NGAC double-check (CRITICAL)
- **Chỉ khi** `assignment.GrantSource != "direct"` (role-based assignment)
- Gọi `verifyApproveAccess(ctx, userNodeID, req.ScopeOAID)`

**File:** `services/approval/internal/domain/execution.go`
**Function:** `Service.verifyApproveAccess()` (line 387)

```
s.policy.CheckAccess(ctx, userNodeID, scopeOAID, "approve")
```
- Gọi Policy service qua gRPC → `CheckAccessRequest{UserNodeId, ObjectNodeId, Operation: "approve"}`
- Policy service chạy in-memory BFS:
  1. Tìm tất cả UA mà user đạt được
  2. Tìm tất cả OA mà scopeOAID đạt được
  3. Tìm PC chung
  4. Tìm Association có operation "approve"
- Nếu `!allowed` → return `ErrAccessDenied`

### 2d. Update assignment status
- Gọi `store.UpdateAssignmentStatus(ctx, assignment.ID, "approved", comment)`

### 2e. Audit log
- Gọi `logAudit(ctx, requestID, "approved", userNodeID, currentStep, {comment})`

### 2f. Check step completion
- Gọi `checkStepCompletion(ctx, req)`

**File:** `services/approval/internal/domain/execution.go`
**Function:** `Service.checkStepCompletion()` (line 241)

- Lấy template snapshot → tìm step definition cho `req.CurrentStep`
- Đếm số approved: `store.CountApprovedForStep(ctx, requestID, stepOrder)`
- Nếu `approvedCount >= step.RequiredCount`:
  - Skip remaining assignments: `store.SkipRemainingAssignments(ctx, requestID, stepOrder)`
  - Nếu còn bước tiếp theo:
    - `store.AdvanceStep(ctx, requestID, nextStepOrder)`
    - `assignStep(ctx, requestID, nextStep, deptID)` → tạo assignments cho bước mới
  - Nếu bước cuối:
    - `store.CompleteRequest(ctx, requestID, "approved")`

---

## Step 3 — Repository (Store)

**File:** `services/approval/internal/store/store.go`

Tất cả query chạy trên **tenant schema** (ví dụ `tenant_abc12345`), resolve qua `httputil.TenantConn()`.

### 3a. `GetRequest()` (line 267)

```sql
SELECT id, entity_type, entity_id, template_id, template_name, template_snapshot,
       form_data_json, current_step, status, scope_oa_id, department_id,
       created_by, created_at, completed_at
FROM approval_requests WHERE id = $1
```

### 3b. `GetAssignment()` (line 313)

```sql
SELECT id, request_id, step_order, user_node_id, grant_source, status, acted_at, comment
FROM approval_assignments
WHERE request_id = $1 AND user_node_id = $2 AND status = 'pending'
```

### 3c. `UpdateAssignmentStatus()` (line 337)

```sql
UPDATE approval_assignments
SET status = $2, acted_at = NOW(), comment = $3
WHERE id = $1
```

**Bảng:** `{schema}.approval_assignments`
| Field | Trước | Sau |
|---|---|---|
| status | `pending` | `approved` |
| acted_at | NULL | NOW() |
| comment | NULL | "Đồng ý" |

### 3d. `InsertAuditEntry()` (line 633)

```sql
INSERT INTO approval_audit_log (id, request_id, action, actor_node_id, step_order, detail, ip_address, created_at)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
```

**Bảng:** `{schema}.approval_audit_log` — INSERT
| Field | Giá trị |
|---|---|
| action | `approved` |
| actor_node_id | user NGAC node ID |
| step_order | current step |
| detail | `{"comment": "Đồng ý"}` |

### 3e. `CountApprovedForStep()` (line 355)

```sql
SELECT COUNT(*) FROM approval_assignments
WHERE request_id = $1 AND step_order = $2 AND status = 'approved'
```

### 3f. (Nếu đủ required_count) `SkipRemainingAssignments()` (line 375)

```sql
UPDATE approval_assignments SET status = 'skipped'
WHERE request_id = $1 AND step_order = $2 AND status = 'pending'
```

**Bảng:** `{schema}.approval_assignments` — UPDATE (các assignment còn lại)
| Field | Trước | Sau |
|---|---|---|
| status | `pending` | `skipped` |

### 3g. (Nếu còn bước tiếp) `AdvanceStep()` (line 412)

```sql
UPDATE approval_requests SET current_step = $2 WHERE id = $1
```

**Bảng:** `{schema}.approval_requests`
| Field | Trước | Sau |
|---|---|---|
| current_step | 1 | 2 |

### 3h. (Nếu bước cuối) `CompleteRequest()` (line 430)

```sql
UPDATE approval_requests SET status = $2, completed_at = NOW() WHERE id = $1
```

**Bảng:** `{schema}.approval_requests`
| Field | Trước | Sau |
|---|---|---|
| status | `pending` | `approved` |
| completed_at | NULL | NOW() |

---

## Step 4 — Event

**File:** `services/approval/internal/rest/handler.go` (line 340)
**File:** `services/approval/internal/events/producer.go`

```go
h.producer.Publish(ctx, events.ApprovalEventPayload{
    RequestID:   body.RequestID,
    Action:      "approved",
    ActorNodeID: claims.NGACNodeID,
})
```

- Topic: `approval-events`
- Consumer: Messaging service (`services/messaging/internal/events/consumer.go`)
- Hành động: tạo notification cho người tạo request

---

## Step 5 — Frontend Update

- WebSocket broadcast approval status change
- TanStack Query cache invalidation → re-fetch pending/history lists
- UI cập nhật: request biến mất khỏi tab Pending, xuất hiện trong History

---

## Database Impact Tổng Kết

| Schema | Bảng | Thao tác | Điều kiện |
|---|---|---|---|
| `tenant_*` | `approval_assignments` | UPDATE status=approved | Luôn luôn |
| `tenant_*` | `approval_audit_log` | INSERT | Luôn luôn |
| `tenant_*` | `approval_assignments` | UPDATE status=skipped | Nếu step đủ required_count |
| `tenant_*` | `approval_requests` | UPDATE current_step | Nếu còn bước tiếp |
| `tenant_*` | `approval_assignments` | INSERT (batch) | Nếu có bước tiếp (gán approvers) |
| `tenant_*` | `approval_requests` | UPDATE status=approved, completed_at | Nếu bước cuối |
| `tenant_*` | `approval_audit_log` | INSERT (thêm 1 entry "step_completed") | Nếu step hoàn thành |

---

## NGAC Check

| Điểm kiểm tra | File | Function | Logic |
|---|---|---|---|
| Approve access | `execution.go:387` | `verifyApproveAccess()` | `CheckAccess(userNodeID, scopeOAID, "approve")` |
| Chỉ khi role-based | `execution.go:137` | `Approve()` | `assignment.GrantSource != "direct"` |
| Policy service | `ngac/access.go` | `Graph.CheckAccess()` | BFS: User→UA→PC ∩ OA→PC, tìm Association |
