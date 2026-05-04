# Approval Actions — Approve, Reject, Batch

## Proto Mapping
- `Approve` → `POST /api/approval/approve` — exists ✅
- `Reject` → `POST /api/approval/reject` — exists ✅
- `BatchApprove` → `POST /api/approval/batch-approve` — exists ✅

## User Stories

### Story 1: Approve a Single Request
As an **approver**, I want to approve a pending request with an optional comment, so that the workflow advances to the next step.

**Acceptance Criteria:**
- [ ] Approve button is visible on pending items (in detail panel or list row)
- [ ] Clicking Approve opens a confirmation with optional comment textarea
- [ ] After confirming, API call `POST /api/approval/approve` with request_id and comment
- [ ] On success: item moves from Pending to History, success feedback shown
- [ ] On failure: error toast, item remains in Pending

**States:**
- Default: Approve button enabled (green/primary)
- Submitting: Button shows spinner, disabled
- Success: Toast "Approved successfully", item removed from pending list
- Error: Toast "Failed to approve. Try again."

### Story 2: Reject a Single Request
As an **approver**, I want to reject a pending request with a required comment, so that the requester knows why it was denied.

**Acceptance Criteria:**
- [ ] Reject button is visible alongside Approve on pending items
- [ ] Clicking Reject opens a confirmation with required comment textarea
- [ ] Comment is required — Submit disabled if empty
- [ ] After confirming, API call `POST /api/approval/reject`
- [ ] On success: item moves from Pending to History with "rejected" status
- [ ] On failure: error toast

### Story 3: Batch Approve Multiple Requests
As an **approver with many pending items**, I want to select multiple items and approve them all at once, so that I can process approvals efficiently.

**Acceptance Criteria:**
- [ ] Checkbox appears on each pending item row
- [ ] Selecting 1+ items shows a floating action bar: "N selected — Batch Approve"
- [ ] Clicking "Batch Approve" opens confirmation with optional comment
- [ ] API call `POST /api/approval/batch-approve` with request_ids array
- [ ] On success: shows "N items approved" feedback, items removed from pending
- [ ] On partial failure: shows "N approved, M failed" with failed IDs

## Flows

### Flow: Single Approve
1. User selects a pending item (clicks row)
2. Detail panel opens on the right
3. User clicks "Approve" button in detail panel
4. Confirmation dialog appears with optional comment textarea
5. User types comment (optional) and clicks "Confirm"
6. API call fires, button shows loading state
7. Success → toast notification, item removed from list, detail panel closes
8. Pending tab count decrements

### Flow: Batch Approve
1. User is on Pending tab with multiple items
2. User checks 3 items via row checkboxes
3. Floating bar appears at bottom: "3 selected — Batch Approve"
4. User clicks "Batch Approve"
5. Confirmation dialog: "Approve 3 items?" with optional comment
6. User confirms → API call
7. Success → toast "3 items approved", items removed, selection cleared
