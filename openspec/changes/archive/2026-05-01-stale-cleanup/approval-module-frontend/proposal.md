# Approval Module Frontend

## Evidence Summary

- **Backend**: ✅ **COMPLETE** — Full approval service with domain logic (template matching, multi-step execution, batch approve), store (tenant-scoped schema), REST handler (13 endpoints), gRPC server, event consumer, and comprehensive tests (unit, integration, performance, benchmarks)
- **Frontend**: ❌ **MISSING** — Zero components, zero routes, zero hooks, zero API client code for approvals
- **Proto**: ✅ **COMPLETE** — `approval.proto` with 12 RPCs: template CRUD, request lifecycle (create/approve/reject/batch), 4 query tabs (pending/history/my-requests/department), audit log
- **DB**: ✅ **COMPLETE** — Migration 007 creates tenant-scoped schema with 6 tables (templates, conditions, steps, requests, assignments, audit_log) plus indexes for all 4 query patterns
- **Dependencies**:
  - Auth service: ✅ Working (JWT with ngac_node_id)
  - Workspace layout: ✅ Working (sidebar + list panel + content area)
  - Design system: ✅ Working (primitives, composites, design tokens)
  - Tenant provisioning: ⚠️ Backend exists but not auto-triggered from frontend

## Product Assessment

- **Size**: **L** — New frontend module requiring: route, API client, hooks, 5+ components, sidebar integration
- **Risk**: **Low** — Backend is 100% complete with tests. Frontend follows existing patterns (Drive, Messaging modules). No proto changes needed. REST API is already defined and mounted.
- **Target user**: Workspace members who submit, review, or manage approval requests (Employees submit → Managers/Admins approve)
- **Core action**: View pending approvals, approve/reject with comments, track own request history

## Scope

### In scope
1. **Approval route** (`_workspace/approval.tsx`) — Main approval module page
2. **Tab navigation** — 4 query tabs matching REST endpoints:
   - Pending (items awaiting my approval)
   - History (items I've acted on)
   - My Requests (items I submitted)
   - Department (items in my department scope)
3. **Approval list view** — Table/list of approval requests with status, requester, timestamp
4. **Approval detail panel** — Side panel showing request details, step timeline, assignments
5. **Approve/Reject actions** — Action buttons with comment input
6. **Batch approve** — Select multiple pending items and approve at once
7. **API client + hooks** — `api/approval.ts` + `hooks/useApproval.ts`
8. **Sidebar integration** — Add "Approvals" entry to AppSidebar with pending count badge
9. **Empty/Loading/Error states** — For all views

### Out of scope
- **Template management UI** — Admin-only, complex form builder. Deferred to separate change.
- **Create approval request UI** — Requests are created by other modules (e.g., asset requests). No standalone "submit approval" form in this change.
- **Audit log detail page** — Data available via API but not a priority for first release. Will show inline in detail panel.
- **Real-time updates** — WebSocket push for approval status changes. Deferred.
- **Department management** — Requires its own module. Approval frontend works with existing department_id data.

### Deferred
- Template CRUD UI (admin feature, separate change)
- WebSocket real-time approval notifications
- Department management module
- Approval analytics/dashboard
- Mobile-specific approval actions (swipe to approve)

## Success Criteria
- User can view 4 tabs of approval data from REST API
- User can approve/reject a pending item with comment
- User can batch-approve multiple items
- User can see step-by-step timeline in detail panel
- Approval count badge shows in sidebar
- All views responsive at 375px, 768px, 1280px
- Empty states show when no data
- Loading spinners during API calls
- Error states with retry on API failure
