# Approval Module — Overview

## BA Review
CEO proposal **ACCEPTED**. Backend is 100% ready. All RPCs map to existing REST endpoints. Frontend-only implementation.

## Proto → REST Mapping

| RPC | REST Endpoint | Method | Frontend Use |
|-----|---------------|--------|-------------|
| `GetPending` | `/api/approval/pending` | GET | Pending tab |
| `GetHistory` | `/api/approval/history?cursor=&limit=` | GET | History tab |
| `GetMyRequests` | `/api/approval/my-requests?cursor=&limit=` | GET | My Requests tab |
| `GetDepartmentRequests` | `/api/approval/department-requests?cursor=&limit=` | GET | Department tab |
| `Approve` | `/api/approval/approve` | POST | Approve action |
| `Reject` | `/api/approval/reject` | POST | Reject action |
| `BatchApprove` | `/api/approval/batch-approve` | POST | Batch approve |
| `GetAuditLog` | `/api/approval/requests/:id/audit` | GET | Detail panel timeline |

## Response Shapes (from REST handler)

**List endpoints** return:
```json
{ "items": [...], "total": N }              // pending (no pagination)
{ "items": [...], "next_cursor": "..." }    // history, my-requests, department
```

**Each item** (ApprovalRequest):
```json
{
  "id": "uuid",
  "entity_type": "purchase_order",
  "entity_id": "uuid",
  "template_name": "PO Approval",
  "current_step": 2,
  "status": "pending|approved|rejected|cancelled",
  "department_id": "dept-id",
  "created_by": "user-node-id",
  "created_at": "2026-04-30T...",
  "completed_at": null,
  "assignments": [
    {
      "id": "uuid",
      "step_order": 1,
      "step_name": "Manager Review",
      "user_node_id": "approver-node-id",
      "grant_source": "role:Manager",
      "status": "approved",
      "acted_at": "2026-04-30T...",
      "comment": "Looks good"
    }
  ]
}
```

## Dependencies
- Auth (JWT with ngac_node_id) — ✅ exists
- Workspace layout (sidebar, routes) — ✅ exists
- Primitives (Button, Badge, Spinner, Text, Avatar) — ✅ exists
- Composites (Tabs, DataTable, PeekPanel, Timeline) — ✅ exists

## Capabilities
1. **approval-list** — Tab-based list views (4 tabs)
2. **approval-actions** — Approve/reject/batch actions
3. **approval-detail** — Side panel with request details and step timeline
4. **sidebar-integration** — Add approvals to app sidebar with badge
