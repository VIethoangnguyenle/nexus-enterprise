# Data Flow Trace — Reject Request

## Mô tả

User từ chối yêu cầu phê duyệt. Hệ thống dừng toàn bộ quy trình ngay lập tức — skip tất cả assignment, đánh dấu request "rejected".

---

## Step 1 — API

**Endpoint:** `POST /api/approval/reject`

**File:** `services/approval/internal/rest/handler.go`
**Function:** `Handler.RejectAction()` (line 350)

**Input body:**
```json
{
  "request_id": "UUID",
  "comment": "Không đủ ngân sách"
}
```

---

## Step 2 — Service (Domain)

**File:** `services/approval/internal/domain/execution.go`
**Function:** `Service.Reject()` (line 158)

### Flow chi tiết

1. **Load request** → `store.GetRequest(ctx, requestID)` → kiểm tra `status == "pending"`
2. **Load assignment** → `store.GetAssignment(ctx, requestID, userNodeID)` → kiểm tra `stepOrder == currentStep`
3. **NGAC double-check** (nếu `GrantSource != "direct"`) → `verifyApproveAccess(ctx, userNodeID, scopeOAID)`
4. **Update assignment status** → `store.UpdateAssignmentStatus(ctx, assignment.ID, "rejected", comment)`
5. **Skip ALL remaining** → `store.SkipAllPendingAssignments(ctx, requestID)` — khác với Approve (chỉ skip bước hiện tại)
6. **Complete request** → `store.CompleteRequest(ctx, requestID, "rejected")`
7. **Audit** → `logAudit(ctx, requestID, "rejected", userNodeID, currentStep, {comment})`
8. **Publish event** → Kafka `approval-events`

---

## Step 3 — Repository (Store)

### 3a. `UpdateAssignmentStatus()` (line 337)

```sql
UPDATE approval_assignments SET status = $2, acted_at = NOW(), comment = $3
WHERE id = $1
```

**Bảng:** `{schema}.approval_assignments`
| Field | Trước | Sau |
|---|---|---|
| status | `pending` | `rejected` |
| acted_at | NULL | NOW() |
| comment | NULL | "Không đủ ngân sách" |

### 3b. `SkipAllPendingAssignments()` (line 394)

```sql
UPDATE approval_assignments SET status = 'skipped'
WHERE request_id = $1 AND status = 'pending'
```

**Bảng:** `{schema}.approval_assignments` — UPDATE (TẤT CẢ bước, không chỉ bước hiện tại)
| Field | Trước | Sau |
|---|---|---|
| status | `pending` | `skipped` |

### 3c. `CompleteRequest()` (line 430)

```sql
UPDATE approval_requests SET status = $2, completed_at = NOW() WHERE id = $1
```

**Bảng:** `{schema}.approval_requests`
| Field | Trước | Sau |
|---|---|---|
| status | `pending` | `rejected` |
| completed_at | NULL | NOW() |

### 3d. `InsertAuditEntry()` (line 633)

**Bảng:** `{schema}.approval_audit_log` — INSERT
| Field | Giá trị |
|---|---|
| action | `rejected` |
| actor_node_id | rejector NGAC node |
| step_order | current step |
| detail | `{"comment": "Không đủ ngân sách"}` |

---

## Khác biệt Approve vs Reject

| Aspect | Approve | Reject |
|---|---|---|
| Assignment status | `approved` | `rejected` |
| Skip assignments | Chỉ bước hiện tại (nếu đủ required_count) | **Tất cả bước** (ngay lập tức) |
| Request status | `approved` (chỉ nếu bước cuối) | `rejected` (ngay lập tức) |
| checkStepCompletion | Có | **Không** (skip thẳng) |
| Terminal | Có thể tiếp tục (advance step) | **Luôn terminal** |

---

## Database Impact Tổng Kết

| Schema | Bảng | Thao tác | Luôn/Điều kiện |
|---|---|---|---|
| `tenant_*` | `approval_assignments` | UPDATE (1 record → rejected) | Luôn luôn |
| `tenant_*` | `approval_assignments` | UPDATE (N records → skipped) | Luôn luôn |
| `tenant_*` | `approval_requests` | UPDATE (rejected + completed_at) | Luôn luôn |
| `tenant_*` | `approval_audit_log` | INSERT | Luôn luôn |
