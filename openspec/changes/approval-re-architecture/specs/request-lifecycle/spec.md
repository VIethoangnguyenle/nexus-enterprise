# Approval: Request Lifecycle

## User Stories

### US-4: Member submits approval request

As a **workspace member**,
I want to submit an approval request by selecting a template and filling in required form data,
so that my request enters the approval workflow.

Acceptance Criteria:
- [ ] Click "New Request" opens CreateRequestModal
- [ ] Modal shows template selector (only active templates)
- [ ] After selecting template, form fields render dynamically based on template's form_fields
- [ ] Required fields are enforced before submit
- [ ] Submit calls `POST /api/approval/requests` with entity_type, entity_id, form_data_json, scope_oa_id, department_id
- [ ] On success: modal closes, "My Requests" tab refreshes, new request appears at top
- [ ] On error: inline error shown

Proto mapping: `CreateRequest` RPC
Backend status: EXISTS — domain has template matching (`matcher.go`) and execution logic (`execution.go`)

---

### US-5: Approver approves a pending request

As an **assigned approver**,
I want to approve a pending request from my "Pending" tab,
so that the request advances to the next step or completes.

Acceptance Criteria:
- [ ] Pending tab shows only requests assigned to current user with status "pending"
- [ ] Click row opens ApprovalDetailPanel showing: template name, entity type, status, step info, assignments, form data
- [ ] "Approve" button calls `POST /api/approval/approve` with request_id and optional comment
- [ ] On success: request removed from pending (optimistic), history refreshes
- [ ] Inline approve button in table row (desktop only) works without opening panel

Proto mapping: `Approve` RPC
Backend status: EXISTS

---

### US-6: Approver rejects a pending request

As an **assigned approver**,
I want to reject a pending request with a mandatory comment,
so that the requester knows why their request was denied.

Acceptance Criteria:
- [ ] "Reject" button in detail panel opens comment input
- [ ] Comment is required — cannot reject without reason
- [ ] Submit calls `POST /api/approval/reject` with request_id and comment
- [ ] On success: request removed from pending, history refreshes
- [ ] Request status changes to "rejected"

Proto mapping: `Reject` RPC
Backend status: EXISTS

---

### US-7: Approver batch approves multiple requests

As an **assigned approver**,
I want to select multiple pending requests and approve them at once,
so that I can process bulk approvals efficiently.

Acceptance Criteria:
- [ ] Checkbox selection on pending tab rows
- [ ] BatchActionBar appears when 1+ items selected, showing count + "Batch Approve" button
- [ ] Batch approve calls `POST /api/approval/batch-approve` with request_ids array
- [ ] On success: all approved items removed from pending (optimistic)
- [ ] "Clear" button deselects all

Proto mapping: `BatchApprove` RPC
Backend status: EXISTS

---

## Flow: Submit → Approve → Complete

```
1. Member navigates to /workspace/approval
2. Clicks "New Request" → CreateRequestModal opens
3. Selects template → form fields render
4. Fills form → clicks Submit
5. System: CreateRequest → MatchTemplate → CreateAssignments → Insert audit
6. Assigned approver sees request in Pending tab
7. Approver clicks row → ApprovalDetailPanel
8. Approver clicks "Approve" (with optional comment)
9. System: Approve → UpdateAssignment → CheckStepComplete → AdvanceOrComplete → InsertAudit
10. If last step: request status → "approved", appears in History
```

Error flows:
- No matching template → "No matching approval template" error
- User not assigned → "access denied" error
- Request already completed → "request already completed" error

---

## States

| State | Pending Tab | History Tab | My Requests Tab | Department Tab |
|-------|-------------|-------------|-----------------|----------------|
| Empty | "No pending approvals" | "No approval history" | "No requests submitted" | "No department requests" |
| Loading | Centered spinner | Centered spinner | Centered spinner | Centered spinner |
| Loaded | Request rows with inline actions | Request rows (read-only) | Request rows (read-only) | Request rows (read-only) |
| Error | ErrorState + retry | ErrorState + retry | ErrorState + retry | ErrorState + retry |
| Not provisioned | "Approvals not configured" | Same | Same | Same |
