# Approval Detail — Side Panel with Timeline

## Proto Mapping
- `GetAuditLog` → `GET /api/approval/requests/:id/audit` — exists ✅
- Request data comes from the list item already loaded (no separate GetRequest RPC needed)

## User Stories

### Story 1: View Request Details
As a **workspace member**, I want to see full details of an approval request in a side panel, so that I can understand what's being approved and its current status.

**Acceptance Criteria:**
- [ ] Clicking a row in any tab opens a PeekPanel on the right
- [ ] Panel header shows: template name + status badge
- [ ] Panel body shows: requester, entity type, entity ID, created date, department
- [ ] Panel shows step timeline: each approval step with approver, status, date, comment
- [ ] Current active step is visually highlighted
- [ ] Completed steps show green checkmark, rejected show red X, pending show gray circle
- [ ] Panel has close button (X) and clicking outside closes it

**States:**
- Loading: Skeleton in panel while audit data loads
- Loaded: Full detail with timeline
- Error: "Failed to load details" with retry

### Story 2: View Step Timeline
As a **workspace member**, I want to see the step-by-step approval timeline, so that I can understand where in the process a request is.

**Acceptance Criteria:**
- [ ] Timeline shows all steps from the request's assignments array
- [ ] Each step shows: step name, approver (user_node_id), status badge, acted_at date
- [ ] Steps with comments show the comment text below the step
- [ ] Steps are ordered by step_order ascending
- [ ] Future steps (not yet reached) show as grayed out placeholders
- [ ] Timeline uses the existing Timeline composite component

## Flows

### Flow: View Detail
1. User clicks a row in any tab
2. PeekPanel slides in from right
3. Panel header: template name + status badge
4. Panel loads audit log: `GET /api/approval/requests/:id/audit`
5. Timeline renders with step progression
6. If item is pending and user is an approver → Approve/Reject buttons shown at bottom
7. User clicks X or outside panel → panel closes

### Flow: Approve from Detail Panel
1. User opens detail panel for a pending item
2. Approve/Reject buttons visible at panel bottom
3. User clicks "Approve"
4. Inline confirmation with comment field (within panel, not separate modal)
5. User confirms → API call → success → panel closes, list refreshes
