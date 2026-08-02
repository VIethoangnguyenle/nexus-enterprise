# Approve-Request Flow — End-to-End Trace

> **Service:** Approval Service (`backend/services/approval/`)
> **Task:** C-1 | **Generated:** 2026-07-12 | **Revised:** 2026-08-02
> **Confidence:** HIGH — all claims verified by current source

---

## 1. Overview

The approve-request flow handles a user approving a pending approval step. It spans
four layers: **REST handler → Domain logic → Store (PostgreSQL) → Events (Kafka)**.
The system supports multi-step approval templates, where each step may require
multiple approvals (quorum). An individual `Approve` action may trigger step
advancement or full request completion.

### Entry Points

| Protocol | Endpoint | Handler | Status |
|----------|----------|---------|--------|
| **REST** | `POST /api/approval/approve` | `rest.Handler.ApproveAction` | ✅ Implemented |
| **gRPC** | `rpc Approve(ApproveRequest)` | — | ❌ Not implemented (proto defined, server stub only) |
| **REST** | `POST /api/approval/batch-approve` | `rest.Handler.BatchApproveAction` | ✅ Implemented |

> **Note:** gRPC `server.go` only implements query RPCs (GetPending, GetHistory,
> GetMyRequests, GetDepartmentRequests, GetAuditLog). All lifecycle mutations
> (Create, Approve, Reject, BatchApprove) are REST-only.

> **BatchApprove is a loop over `Approve`, not a bulk UPDATE.** It used to be a
> single statement that matched on `user_node_id` and `status='pending'` alone,
> which skipped the request-status guard, the current-step guard and the NGAC
> re-check in §3.3 — so an approver whose role had been revoked could approve
> through this endpoint what `/approve` denied them. Requests the caller may not
> approve are now skipped and the response carries only the set that succeeded.

---

## 2. Sequence Diagram

```
Client                REST Handler              Domain Service           Store (PG)              Policy Service     Kafka
  │                       │                          │                      │                        │               │
  │ POST /approve         │                          │                      │                        │               │
  │──────────────────────>│                          │                      │                        │               │
  │                       │ JWT + Tenant Middleware   │                      │                        │               │
  │                       │ Extract claims.NGACNodeID │                      │                        │               │
  │                       │ Parse {request_id,comment}│                      │                        │               │
  │                       │                          │                      │                        │               │
  │                       │ svc.Approve(input)       │                      │                        │               │
  │                       │─────────────────────────>│                      │                        │               │
  │                       │                          │ GetRequest(id)       │                        │               │
  │                       │                          │─────────────────────>│                        │               │
  │                       │                          │ <── Request          │                        │               │
  │                       │                          │                      │                        │               │
  │                       │                          │ [status != pending?] │                        │               │
  │                       │                          │ → ErrRequestCompleted│                        │               │
  │                       │                          │                      │                        │               │
  │                       │                          │ GetAssignment(req,user)                       │               │
  │                       │                          │─────────────────────>│                        │               │
  │                       │                          │ <── Assignment       │                        │               │
  │                       │                          │                      │                        │               │
  │                       │                          │ [step != current?]   │                        │               │
  │                       │                          │ → ErrStepNotActive   │                        │               │
  │                       │                          │                      │                        │               │
  │                       │                          │ [grant != "direct"?] │                        │               │
  │                       │                          │ CheckAccess(user,scope,"approve")             │               │
  │                       │                          │──────────────────────────────────────────────>│               │
  │                       │                          │ <── allow/deny       │                        │               │
  │                       │                          │                      │                        │               │
  │                       │                          │ UpdateAssignmentStatus("approved")            │               │
  │                       │                          │─────────────────────>│                        │               │
  │                       │                          │                      │ UPDATE assignment      │               │
  │                       │                          │                      │ SET status='approved', │               │
  │                       │                          │                      │     acted_at=NOW()     │               │
  │                       │                          │                      │                        │               │
  │                       │                          │ InsertAuditEntry("approved")                  │               │
  │                       │                          │─────────────────────>│                        │               │
  │                       │                          │                      │ INSERT audit_log       │               │
  │                       │                          │                      │                        │               │
  │                       │                          │ checkStepCompletion()│                        │               │
  │                       │                          │─────────────────────>│                        │               │
  │                       │                          │  (see §4 below)      │                        │               │
  │                       │                          │                      │                        │               │
  │                       │ <────────────── nil/err  │                      │                        │               │
  │                       │                          │                      │                        │               │
  │                       │ producer.Publish(event)  │                      │                        │               │
  │                       │─────────────────────────────────────────────────────────────────────────>│
  │                       │                          │                      │                        │ → approval.events
  │                       │                          │                      │                        │               │
  │ <── 200 {status:approved}                        │                      │                        │               │
```

---

## 3. Layer-by-Layer Trace

### 3.1 REST Handler — `ApproveAction`

**File:** `internal/rest/handler.go` L318–L347

**Middleware chain** (applied to all `/api/approval/*` routes):

1. `httputil.JWTMiddleware(jwtSecret)` — validates JWT, extracts claims
2. `httputil.TenantMiddleware()` — extracts `tenant_id` from claims
3. `tenantSchemaMiddleware()` — resolves `tenant_id → schema_name` via
   `TenantSchemaResolver` (cached), stores schema in `context.Context`

**Handler logic:**

```
1. RequireClaims(c) → extract claims.NGACNodeID (the approver's NGAC node ID)
2. Bind JSON body: { request_id: string, comment: string }
3. Call domain: svc.Approve(ctx, ApproveInput{RequestID, UserNodeID, Comment})
4. On error → mapDomainError (ErrStepNotActive→409, ErrRequestCompleted→409, etc.)
5. On success → publish Kafka event (fire-and-forget)
6. Return 200 { "status": "approved" }
```

### 3.2 Domain Logic — `Service.Approve`

**File:** `internal/domain/execution.go` L114–L155

```go
func (s *Service) Approve(ctx context.Context, in ApproveInput) error
```

**Step-by-step:**

| # | Action | Error Path |
|---|--------|------------|
| 1 | Validate `RequestID` and `UserNodeID` non-empty | `ErrInvalidInput` |
| 2 | `store.GetRequest(ctx, in.RequestID)` — load the approval request | `ErrNotFound` (wrapped) |
| 3 | Check `req.Status == "pending"` | `ErrRequestCompleted` |
| 4 | `store.GetAssignment(ctx, requestID, userNodeID)` — find the user's pending assignment | `ErrNotFound` (no pending assignment) |
| 5 | Check `assignment.StepOrder == req.CurrentStep` | `ErrStepNotActive` |
| 6 | **NGAC double-check** (only if `assignment.GrantSource != "direct"`): call `policy.CheckAccess(userNodeID, scopeOAID, "approve")` | `ErrAccessDenied` |
| 7 | `store.UpdateAssignmentStatus(assignment.ID, "approved", comment)` | DB error (wrapped) |
| 8 | `logAudit(requestID, "approved", userNodeID, currentStep, {comment})` | Swallowed (fire-and-forget) |
| 9 | `checkStepCompletion(ctx, req)` → see §4 | Various (wrapped) |

### 3.3 NGAC Access Check — `verifyApproveAccess`

**File:** `internal/domain/execution.go` L384–L396

This check prevents **stale role exploitation**: a user whose role was revoked
after their assignment was created cannot approve.

- **Skipped for:** `GrantSource == "direct"` (specific_user assignments are self-authorizing)
- **Runs for:** `GrantSource` matching `role:*` or `department:*`
- **Calls:** `policy.CheckAccess(userNodeID, scopeOAID, "approve")` via gRPC to the Policy Service
- **On deny:** returns `ErrAccessDenied`

### 3.4 Store (DB Writes) — Approve Path

**File:** `internal/store/store.go`

All queries run on the tenant's PostgreSQL schema (set via `SET search_path`
through `httputil.TenantConn`).

| Method | SQL | Table | Line |
|--------|-----|-------|------|
| `GetRequest` | `SELECT … FROM approval_requests WHERE id = $1` | `approval_requests` | L276 |
| `GetAssignment` | `SELECT … FROM approval_assignments WHERE request_id = $1 AND user_node_id = $2 AND status = 'pending'` | `approval_assignments` | L322 |
| `UpdateAssignmentStatus` | `UPDATE approval_assignments SET status = $2, acted_at = NOW(), comment = $3 WHERE id = $1` | `approval_assignments` | L344 |
| `InsertAuditEntry` | `INSERT INTO approval_audit_log (id, request_id, action, actor_node_id, step_order, detail, ip_address, created_at)` | `approval_audit_log` | L650 |

---

## 4. Step Completion & Advancement

**File:** `internal/domain/execution.go` L241–L306

After each approval, `checkStepCompletion` determines whether the current step
has collected enough approvals:

```
1. Unmarshal template from req.TemplateSnapshot (frozen at request creation)
2. Find the current step definition by StepOrder
3. COUNT approved assignments for (request_id, step_order)
4. If approvedCount < step.RequiredCount → return nil (not enough yet)
5. If quorum met:
   a. SkipRemainingAssignments for this step (mark all pending as 'skipped')
   b. Log audit: "step_advanced"
   c. Look for next step (StepOrder + 1) in template
   d. If NO next step → CompleteRequest(status="approved"), log "completed"
   e. If next step exists → AdvanceStep(currentStep → nextStep) as a
      compare-and-swap; only the caller that wins it runs assignStep()
```

### DB Writes During Step Completion

| Method | SQL | Table |
|--------|-----|-------|
| `CountApprovedForStep` | `SELECT COUNT(*) FROM approval_assignments WHERE … AND status = 'approved'` | `approval_assignments` |
| `SkipRemainingAssignments` | `UPDATE approval_assignments SET status = 'skipped' WHERE … AND status = 'pending'` | `approval_assignments` |
| `AdvanceStep` | `UPDATE approval_requests SET current_step = $3 WHERE id = $1 AND current_step = $2 AND status = 'pending'` — returns whether it won | `approval_requests` |
| `CompleteRequest` | `UPDATE approval_requests SET status = $2, completed_at = NOW() WHERE id = $1 AND status = 'pending'` — returns whether it won | `approval_requests` |
| `InsertAssignments` | `INSERT INTO approval_assignments (id, request_id, step_order, user_node_id, grant_source, status)` | `approval_assignments` |
| `InsertAuditEntry` | (same as above — logged for step_advanced, completed, assigned) | `approval_audit_log` |

### Approver Resolution for Next Step — `assignStep`

**File:** `internal/domain/execution.go` L309–L382

When advancing to the next step, approvers are resolved based on `step.ApproverType`:

| ApproverType | Resolution | GrantSource |
|-------------|------------|-------------|
| `specific_user` | Direct assignment to `approver_value` | `"direct"` |
| `role_in_dept` | `policy.ResolveAccessibleScopes(approverValue, "approve")` → one assignment per scope member | `"role:{approverValue}"` |
| `department` | Direct assignment to department UA node | `"department:{approverValue}"` |

Placeholder `{creator_dept}` in `approver_value` is resolved via `ResolvePlaceholder()`.

---

## 5. Events (Kafka)

### 5.1 Producer — After Approve

**File:** `internal/events/producer.go`

After `domain.Approve` returns success, the REST handler publishes to Kafka:

- **Topic:** `approval.events`
- **Key:** `request_id`
- **Payload:**
  ```json
  {
    "request_id": "...",
    "action": "approved",
    "actor_node_id": "...",
    "timestamp": 1720780000
  }
  ```
- **Delivery:** Fire-and-forget (async callback logs errors but doesn't fail the HTTP response)

### 5.2 Consumer — Policy Change Reconciliation

**File:** `internal/events/consumer.go`

A separate consumer listens to `ngac.graph.mutated` for policy graph changes:

- `create_assignment` (U→UA): Creates new pending assignments for users added to roles
- `remove_assignment` (U→UA): Revokes pending assignments for users removed from roles

This ensures that NGAC policy changes are reflected in active approval workflows.

---

## 6. Database Schema (Inferred from SQL)

| Table | Purpose | Key Columns |
|-------|---------|-------------|
| `approval_templates` | Workflow template definitions | id, name, entity_type, is_active, priority, form_fields (JSONB) |
| `approval_conditions` | Template matching conditions | id, template_id, field, operator, value |
| `approval_steps` | Step definitions within templates | id, template_id, step_order, approver_type, approver_value, required_count |
| `approval_requests` | Running approval instances | id, entity_type, entity_id, template_id, template_snapshot, form_data_json, current_step, status, scope_oa_id |
| `approval_assignments` | Per-user approval assignments | id, request_id, step_order, user_node_id, grant_source, status, acted_at, comment |
| `approval_audit_log` | Append-only audit trail | id, request_id, action, actor_node_id, step_order, detail (JSONB), ip_address (INET) |

All tables are tenant-scoped (each tenant has its own PostgreSQL schema).

---

## 7. Error Handling Summary

| Error | HTTP | gRPC | When |
|-------|------|------|------|
| `ErrInvalidInput` | 400 | `INVALID_ARGUMENT` | Missing request_id or user_node_id |
| `ErrNotFound` | 404 | `NOT_FOUND` | Request or assignment not found |
| `ErrAccessDenied` | 403 | `PERMISSION_DENIED` | NGAC check failed (role revoked) |
| `ErrStepNotActive` | 409 | `FAILED_PRECONDITION` | Assignment's step ≠ request's current step |
| `ErrRequestCompleted` | 409 | `FAILED_PRECONDITION` | Request status ≠ "pending" |
| Internal errors | 500 | `INTERNAL` | DB failures, marshaling errors |

---

## 8. Architectural Notes

1. **No transaction wrapping, guarded by compare-and-swap:** The approve flow
   executes multiple store calls without an explicit DB transaction. Each store
   method acquires and releases its own connection. What makes that safe:
   - Assignment status update is idempotent (`WHERE status='pending'`)
   - `AdvanceStep` and `CompleteRequest` are compare-and-swaps that report
     whether they won, and only the winner advances the workflow
   - Audit entries are append-only and fire-and-forget

   Step completion is **not** re-entrant on its own. Two approvals that satisfy
   the same quorum concurrently both observe the count as met and both attempt
   to advance; before the compare-and-swap, both succeeded and each went on to
   create a full set of assignments for the next step.

2. **Template snapshot:** At request creation time, the entire template is
   JSON-serialized into `template_snapshot`. Step completion uses this frozen
   copy — template changes after request creation do not affect running requests.

3. **Dual API surface:** Proto defines all lifecycle RPCs but gRPC only
   implements query RPCs. All mutations go through REST with Echo framework.

4. **Tenant isolation:** Every DB query is scoped to the tenant's schema via
   `SET search_path` in `httputil.TenantConn`. The `TenantSchemaResolver`
   caches `tenant_id → schema_name` mappings.
