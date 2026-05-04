# Sidebar Integration — Approvals Entry with Badge

## Proto Mapping
- `GetPending` → `GET /api/approval/pending` — exists ✅ (reuse response `total` for badge count)

## User Stories

### Story 1: See Approvals in Sidebar
As a **workspace member**, I want to see "Approvals" in the sidebar navigation, so that I can access the approval module easily.

**Acceptance Criteria:**
- [ ] "Approvals" entry appears in AppSidebar between existing module entries
- [ ] Entry has an appropriate icon (CheckSquare or similar from lucide-react)
- [ ] Clicking navigates to `/_workspace/approval`
- [ ] Active state highlights when on approval route
- [ ] Entry visible on both desktop sidebar and mobile bottom nav

### Story 2: See Pending Count Badge
As an **approver**, I want to see a badge showing the number of pending approvals, so that I know when items need my attention.

**Acceptance Criteria:**
- [ ] Badge shows on the Approvals sidebar entry when pending count > 0
- [ ] Badge shows the count number (e.g., "3")
- [ ] Badge is hidden when count is 0
- [ ] Badge uses the existing Badge primitive component
- [ ] Count refreshes when navigating to/from approval module

## Flows

### Flow: Navigate to Approvals
1. User sees "Approvals" in sidebar with badge "3"
2. User clicks "Approvals"
3. Route changes to `/_workspace/approval`
4. Approval module renders with Pending tab active
5. Sidebar entry shows active state

### Flow: Mobile Navigation
1. User sees "Approvals" icon in MobileNav bottom bar
2. User taps icon
3. Route changes to approval module
4. If pending count > 0, badge dot visible on icon
