# Approval Detail & Actions

## Proto Mapping
- `GetRequest` — ❌ NEW (get single request by ID)
- `Approve` — ✅ exists
- `Reject` — ✅ exists
- `GetAuditLog` — ✅ exists

## User Stories

### Story 1: View request detail (All roles)
As a **user**, I want to see full request details when I click a row, so that I can understand what's being requested.

**Acceptance Criteria:**
- [ ] Click row in any tab → PeekPanel opens on right (desktop) or full-screen overlay (mobile)
- [ ] Uses `<PeekPanel>` composite (NOT custom panel)
- [ ] Panel shows sections: Header (template name + status badge), Request Details (form data), Approval Steps (timeline), Audit Log
- [ ] Header has close button (IconButton with X)
- [ ] "Request Details" section renders submitted form field values (label: value pairs)
- [ ] "Approval Steps" section uses `<Timeline>` composite showing step progression
- [ ] Each timeline item: step name, approver, status (pending/approved/rejected), timestamp
- [ ] Completed steps show green check, rejected show red X, pending show gray clock

**States:**
- Loading: Skeleton lines in panel
- Loaded: Full detail with sections
- Error: Inline error with retry button

### Story 2: Approve request with comment (Manager)
As a **Manager**, I want to approve a request with an optional comment, so that I can move the workflow forward.

**Acceptance Criteria:**
- [ ] "Approve" button visible in detail panel for pending items assigned to current user
- [ ] Button uses `<Button variant="primary">` with green styling
- [ ] Click shows comment textarea (optional) + "Confirm Approve" button
- [ ] After approve: status updates, timeline refreshes, panel stays open with updated data
- [ ] Success toast: "Request approved"
- [ ] If this was the last step: request status becomes "approved"

**States:**
- Ready: Approve button enabled
- Confirming: Comment textarea expanded
- Processing: Button spinner
- Success: Updated status + toast

### Story 3: Reject request with reason (Manager)
As a **Manager**, I want to reject a request with a required reason, so that the submitter knows why.

**Acceptance Criteria:**
- [ ] "Reject" button visible next to Approve button
- [ ] Button uses `<Button variant="error">`
- [ ] Click shows comment textarea (REQUIRED for rejection) + "Confirm Reject" button
- [ ] Submit disabled if comment is empty
- [ ] After reject: request status becomes "rejected", timeline shows rejection
- [ ] Success toast: "Request rejected"

**States:**
- Ready: Reject button enabled
- Confirming: Comment textarea expanded, validation enforced
- Processing: Button spinner
- Error: Toast if API fails

### Story 4: View audit trail (All roles)
As a **user**, I want to see the complete audit trail of a request, so that I can see who did what and when.

**Acceptance Criteria:**
- [ ] "Audit Log" section in detail panel (collapsible, expanded by default)
- [ ] Shows chronological list: timestamp + actor + action + detail
- [ ] Actions include: created, assigned, approved, rejected, cancelled
- [ ] Actor shows user identifier (node_id for now, name in future)
- [ ] Detail shows comment if present

**States:**
- Loading: Skeleton lines
- Empty: "No audit entries"
- Loaded: Chronological list

## Flows

### Flow: Approve request
1. Manager clicks pending request row
2. PeekPanel opens with full detail
3. Manager reviews form data and step timeline
4. Manager clicks "Approve"
5. Comment textarea appears (optional)
6. Manager types "Approved. Budget confirmed." (optional)
7. Manager clicks "Confirm Approve"
8. Spinner on button
9. System calls Approve API
10. Timeline updates: current step shows green check
11. Toast: "Request approved"
12. If last step: request status changes to "approved"

### Flow: Reject request
1. Manager clicks pending request row
2. PeekPanel opens
3. Manager clicks "Reject"
4. Comment textarea appears (required)
5. Manager types "Budget exceeded quarterly limit. Please resubmit with revised amount."
6. Manager clicks "Confirm Reject"
7. System calls Reject API
8. Timeline shows red X on current step
9. Request status changes to "rejected"
10. Toast: "Request rejected"
