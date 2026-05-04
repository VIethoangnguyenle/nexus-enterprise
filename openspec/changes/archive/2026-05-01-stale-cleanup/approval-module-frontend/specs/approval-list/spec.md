# Approval List — Tab-Based Views

## Proto Mapping
- `GetPending` → `GET /api/approval/pending` — exists ✅
- `GetHistory` → `GET /api/approval/history` — exists ✅
- `GetMyRequests` → `GET /api/approval/my-requests` — exists ✅
- `GetDepartmentRequests` → `GET /api/approval/department-requests` — exists ✅

## User Stories

### Story 1: View Pending Approvals
As a **workspace member with approver role**, I want to see a list of items waiting for my approval, so that I can act on them promptly.

**Acceptance Criteria:**
- [ ] Navigating to `/approval` shows the Pending tab as default
- [ ] Each item shows: template name, requester info, created date, current step, status badge
- [ ] Items are sorted by created_at descending (newest first)
- [ ] Pending count is visible on the tab label (e.g., "Pending (3)")
- [ ] Empty state shows when no pending items: icon + "No pending approvals" message

**States:**
- Empty: Centered icon + "No pending approvals" + "Items assigned to you will appear here" subtext
- Loading: Spinner centered in content area
- Error: Error message + Retry button
- Loaded: List of approval request rows

### Story 2: View Approval History
As a **workspace member**, I want to see items I've previously approved or rejected, so that I can reference past decisions.

**Acceptance Criteria:**
- [ ] Clicking "History" tab loads acted-on items
- [ ] Each item shows: template name, requester, my action (approved/rejected badge), acted date
- [ ] List supports cursor-based pagination (load more button or infinite scroll)
- [ ] Empty state: "No approval history yet"

**States:**
- Empty: "No approval history yet" centered message
- Loading: Spinner
- Error: Error message + Retry
- Loaded: List with pagination

### Story 3: View My Requests
As a **workspace member**, I want to see approval requests I've submitted, so that I can track their progress.

**Acceptance Criteria:**
- [ ] Clicking "My Requests" tab loads items I created
- [ ] Each item shows: template name, status (pending/approved/rejected), created date, current step
- [ ] Pending items show which step they're waiting on
- [ ] Cursor-based pagination
- [ ] Empty state: "No requests submitted yet"

### Story 4: View Department Requests
As a **department manager**, I want to see all approval requests within my department, so that I have visibility into team activities.

**Acceptance Criteria:**
- [ ] Clicking "Department" tab loads department-scoped items
- [ ] Shows all statuses (pending, approved, rejected)
- [ ] Cursor-based pagination
- [ ] Empty state: "No department requests"

## Flows

### Flow: Tab Navigation
1. User navigates to `/_workspace/approval`
2. Pending tab is active by default, API call `GET /api/approval/pending`
3. User sees list of pending items (or empty state)
4. User clicks "History" tab
5. API call `GET /api/approval/history?limit=20`
6. User sees history items
7. User scrolls to bottom → "Load More" appears if `next_cursor` exists
8. User clicks "Load More" → API call with cursor → new items appended

### Error Flow
- If API call fails → Show error state with "Something went wrong" + Retry button
- If Retry clicked → Re-fetch current tab data
