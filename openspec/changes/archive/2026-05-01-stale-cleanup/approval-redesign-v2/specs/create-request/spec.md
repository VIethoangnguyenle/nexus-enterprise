# Create Approval Request (Employee)

## Proto Mapping
- `CreateRequest` (`CreateApprovalRequestReq`) — ✅ exists (needs `form_data_json` field)
- `ListTemplates` — ✅ exists (for template selector)
- `GetTemplate` — ✅ exists (for loading form fields)

## User Stories

### Story 1: Select template and fill form (Employee)
As an **Employee**, I want to select an approval template and fill in the required form, so that I can submit a request for approval.

**Acceptance Criteria:**
- [ ] "New Request" button visible on Approval page header (all roles)
- [ ] Click opens Modal (desktop) or full-screen (mobile)
- [ ] Step 1: Template selector dropdown showing active templates grouped by entity_type
- [ ] After selecting template, form fields render dynamically below selector
- [ ] Form validates required fields on blur and on submit
- [ ] Submit button enabled only when all required fields are filled
- [ ] Submit calls CreateRequest API with form_data_json
- [ ] Success: toast + redirect to "My Requests" tab

**States:**
- Template list loading: Spinner in dropdown
- Template list empty: "No templates available" message
- Template selected: Dynamic form renders
- Validation error: Red borders + error messages on invalid fields
- Submitting: Button spinner, form disabled
- Success: Toast + auto-switch to My Requests tab
- Error: Toast with "Submission failed" + retry

### Story 2: Preview approval steps before submitting (Employee)
As an **Employee**, I want to see who will approve my request before I submit it, so that I know the workflow.

**Acceptance Criteria:**
- [ ] After selecting template, "Approval Steps" preview shows below form
- [ ] Steps displayed as Timeline component: Step 1 → Step 2 → ...
- [ ] Each step shows: step name, approver type description
- [ ] If approver_type is "creator_manager" → show "Your direct manager"
- [ ] If approver_type is "specific_user" → show user name (if available)
- [ ] Preview is read-only — informational only

### Story 3: Cancel own pending request (Employee)
As an **Employee**, I want to cancel my own pending request, so that I can withdraw a request I no longer need.

**Acceptance Criteria:**
- [ ] "Cancel" button visible on own pending requests in "My Requests" tab
- [ ] Click shows ConfirmDialog: "Cancel this request? This cannot be undone."
- [ ] After cancel: request status changes to "cancelled" in UI
- [ ] Cancelled request shows gray "Cancelled" badge
- [ ] Cannot cancel already approved/rejected requests

**States:**
- Cancelling: Button shows spinner
- Success: Toast + status updates inline
- Error: Toast with error message

## Flows

### Flow: Submit approval request
1. Employee navigates to `/approval`
2. Employee clicks "New Request" button
3. Modal opens with template selector
4. Employee selects "Chi tiền mua vật phẩm" template
5. Form fields render: Tên sản phẩm, Số lượng, Đơn giá, Tổng tiền, Lý do
6. Employee fills all required fields
7. Approval steps preview shows: Step 1: Trưởng phòng KD → Step 2: CFO (if > 10M)
8. Employee clicks "Submit"
9. System calls CreateRequest API
10. Success toast, modal closes, My Requests tab active with new request

Error flow:
- If required field empty → Red border + "This field is required"
- If backend rejects → Toast: "Submission failed: [reason]"
- If no template matches conditions → Toast: "No matching approval workflow found"
