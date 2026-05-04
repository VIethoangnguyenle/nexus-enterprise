# Approval List (All Roles)

## Proto Mapping
- `GetPending` — ✅ exists
- `GetHistory` — ✅ exists (cursor pagination)
- `GetMyRequests` — ✅ exists (cursor pagination)
- `GetDepartmentRequests` — ✅ exists (cursor pagination)
- `BatchApprove` — ✅ exists

## User Stories

### Story 1: View pending approvals (Manager)
As a **Manager**, I want to see a list of pending approvals assigned to me, so that I can act on them efficiently.

**Acceptance Criteria:**
- [ ] "Pending" tab is default active tab (shows count badge)
- [ ] Uses `<DataTable>` composite with columns: Request (template name + entity type), Requester, Status, Step (current/total), Date
- [ ] Mobile (375px): Shows only Request + Status columns, no Step/Date
- [ ] Tablet (768px): Shows Request + Status + Date, no Step
- [ ] Desktop (1280px): All columns visible
- [ ] Click row opens PeekPanel with request detail
- [ ] Pending count badge shows on Pending tab
- [ ] Auto-refreshes on tab switch

**States:**
- Empty: EmptyState with clipboard icon, "No pending approvals", "Items assigned to you will appear here"
- Loading: LoadingState spinner
- Loaded: DataTable with rows
- Error (not provisioned): EmptyState "Approvals not configured for this workspace"
- Error (API): ErrorState with retry

### Story 2: View approval history (Manager)
As a **Manager**, I want to see items I've previously approved or rejected, so that I can track my decision history.

**Acceptance Criteria:**
- [ ] "History" tab shows items user has acted on
- [ ] DataTable columns: Request, Status (Approved/Rejected badge), Date acted
- [ ] Cursor-based pagination with "Load More" button
- [ ] Click row opens detail in PeekPanel

### Story 3: View my submitted requests (Employee)
As an **Employee**, I want to see all requests I've submitted, so that I can track their status.

**Acceptance Criteria:**
- [ ] "My Requests" tab shows requests created by current user
- [ ] DataTable columns: Request, Status, Current Step, Submitted Date
- [ ] Pending requests show "Cancel" action (inline icon button)
- [ ] Cursor-based pagination with "Load More" button
- [ ] After submitting new request, this tab auto-activates

### Story 4: Batch approve (Manager)
As a **Manager**, I want to select multiple pending requests and approve them at once, so that I can handle high volumes efficiently.

**Acceptance Criteria:**
- [ ] Checkboxes visible on each row in Pending tab only
- [ ] Select-all checkbox in header
- [ ] When 1+ items selected: floating BatchActionBar appears at bottom
- [ ] BatchActionBar shows: "[N] selected" + "Approve All" button + "Clear" button
- [ ] "Approve All" calls BatchApprove API
- [ ] Success: toast with "N requests approved" + list refreshes
- [ ] BatchActionBar positioned above MobileNav on mobile

**States:**
- None selected: No BatchActionBar
- Items selected: BatchActionBar visible
- Processing: "Approve All" button shows spinner
- Partial failure: Toast listing failed IDs

## Flows

### Flow: Batch approve
1. Manager navigates to Pending tab
2. Manager clicks checkboxes on rows 1, 3, 5
3. BatchActionBar appears: "3 selected | Approve All | Clear"
4. Manager clicks "Approve All"
5. Spinner on button
6. System calls BatchApprove API
7. Success toast: "3 requests approved"
8. BatchActionBar disappears, list refreshes
