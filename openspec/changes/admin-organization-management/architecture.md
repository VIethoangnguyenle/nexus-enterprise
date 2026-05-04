# Architecture: Admin Organization Management

## Overview

Admin module extends the existing **workspace service** with department management. It reuses the NGAC permission model for department hierarchy and the existing role/member APIs.

**Principle:** No new service. Department is a workspace-level concept — same as roles and members.

---

## Service Boundaries

```
┌─────────────────────────────────────────────────────────┐
│                      Frontend (Vite + React)            │
│  /admin → AdminLayout → [Departments | Users | Roles]  │
│  TanStack Router + Query + Zustand                     │
└───────┬────────────────────────────┬────────────────────┘
        │ REST /api/workspaces/:id/* │
        ▼                            ▼
┌───────────────┐           ┌────────────────┐
│  Workspace    │  gRPC     │  Auth Service  │
│  Service      │◄─────────►│  (user data)   │
│  (admin APIs) │           └────────────────┘
└───────┬───────┘
        │ gRPC
        ▼
┌───────────────┐
│ Policy Service│
│ (NGAC graph)  │
└───────────────┘
```

### Why extend workspace service (not new service):
- Workspace service already owns: roles (CreateRole/ListRoles/DeleteRole), members (InviteMember/ListMembers/UpdateMemberRoles), folders, permissions
- Department is same abstraction level as role — a UA node under workspace PC
- Adding a new service would duplicate policy client, member enrichment, workspace resolution

---

## Data Flow

### Department CRUD

```
Frontend                 Workspace REST           Workspace Domain         Policy Service (gRPC)        DB
   │                          │                         │                         │                      │
   │─POST /departments───────►│                         │                         │                      │
   │                          │──CreateDepartment──────►│                         │                      │
   │                          │                         │──CreateNode(UA)────────►│                      │
   │                          │                         │◄──NGACNode─────────────│                      │
   │                          │                         │──CreateAssignment──────►│                      │
   │                          │                         │  (dept → parent/PC)     │                      │
   │                          │                         │◄──Assignment────────────│                      │
   │                          │                         │──INSERT departments────►│──────────────────────►│
   │                          │◄──Department────────────│                         │                      │
   │◄──201 + department──────│                         │                         │                      │
```

### User-Department Assignment

```
Frontend                 Workspace REST           Workspace Domain         Policy Service (gRPC)
   │                          │                         │                         │
   │─PUT /members/:id/dept──►│                         │                         │
   │                          │──UpdateMemberDept──────►│                         │
   │                          │                         │──RemoveAssignment──────►│  (old dept UA)
   │                          │                         │──CreateAssignment──────►│  (new dept UA)
   │                          │                         │──UPDATE departments ──►│  (user dept_id)
   │                          │◄──OK────────────────────│                         │
   │◄──200 OK────────────────│                         │                         │
```

---

## NGAC Permission Model

### Department as UA Hierarchy

```
PC_<workspace>
├── <workspace>_Owners (UA)        ← existing
├── <workspace>_Members (UA)       ← existing
├── Dept_Engineering (UA)          ← NEW: department UA node
│   ├── Dept_Frontend (UA)         ← child department
│   │   └── user_node_123 (U)     ← user assigned to Frontend dept
│   └── Dept_Backend (UA)          ← child department
│       └── user_node_456 (U)
├── Dept_Marketing (UA)            ← root department
│   └── user_node_789 (U)
├── Role_ContentManager (UA)       ← existing role pattern
└── ...OA nodes...                 ← existing (Mgmt, Documents, Channels)
```

### Key NGAC operations:
1. **Create department** → `CreateNode(name="Dept_<name>", type=UA)` + `CreateAssignment(dept → parent_dept OR PC)`
2. **Assign user to dept** → `CreateAssignment(user_node → dept_UA)`
3. **Move department** → `RemoveAssignment(dept → old_parent)` + `CreateAssignment(dept → new_parent)`
4. **Delete department** → reassign children to parent, then `DeleteNode(dept_UA)`

### Department naming convention (ngac_ops.go):
```go
func DeptUAName(name string) string { return fmt.Sprintf("Dept_%s", name) }
```

---

## Database Changes

### New migration: `011_departments.sql`

```sql
CREATE TABLE IF NOT EXISTS departments (
    id              TEXT PRIMARY KEY,
    workspace_id    TEXT NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    name            TEXT NOT NULL,
    parent_id       TEXT REFERENCES departments(id),
    ngac_ua_id      TEXT NOT NULL REFERENCES ngac_nodes(id),
    sort_order      INT NOT NULL DEFAULT 0,
    created_at      TIMESTAMPTZ DEFAULT NOW(),
    updated_at      TIMESTAMPTZ DEFAULT NOW(),
    UNIQUE(workspace_id, name, parent_id)
);

CREATE INDEX IF NOT EXISTS idx_departments_workspace ON departments(workspace_id);
CREATE INDEX IF NOT EXISTS idx_departments_parent ON departments(parent_id);

-- Link users to departments
ALTER TABLE tenant_users ADD COLUMN IF NOT EXISTS department_id TEXT REFERENCES departments(id);
CREATE INDEX IF NOT EXISTS idx_tenant_users_department ON tenant_users(department_id) WHERE department_id IS NOT NULL;
```

---

## API Design (REST)

### Department endpoints (workspace service REST handler)

| Method | Path | Description |
|--------|------|-------------|
| POST | `/api/workspaces/:id/departments` | Create department |
| GET | `/api/workspaces/:id/departments` | List departments (tree) |
| PUT | `/api/workspaces/:id/departments/:deptId` | Update department (rename) |
| DELETE | `/api/workspaces/:id/departments/:deptId` | Delete department |
| PUT | `/api/workspaces/:id/departments/:deptId/move` | Move department (change parent) |
| PUT | `/api/workspaces/:id/members/:nodeId/department` | Assign user to department |
| GET | `/api/workspaces/:id/members/:nodeId/detail` | Get enriched member detail |

### Proto additions (workspace.proto)

```protobuf
// Department management
rpc CreateDepartment(CreateDepartmentRequest) returns (Department);
rpc ListDepartments(ListDepartmentsRequest) returns (DepartmentList);
rpc UpdateDepartment(UpdateDepartmentRequest) returns (Department);
rpc DeleteDepartment(DeleteDepartmentRequest) returns (Empty);
rpc MoveDepartment(MoveDepartmentRequest) returns (Department);

// Member enrichment
rpc UpdateMemberDepartment(UpdateMemberDepartmentRequest) returns (Empty);
rpc GetMemberDetail(GetMemberDetailRequest) returns (MemberDetail);

message Department {
  string id = 1;
  string name = 2;
  string parent_id = 3;
  int32 member_count = 4;
  repeated Department children = 5;
}

message MemberDetail {
  string user_id = 1;
  string username = 2;
  string display_name = 3;
  string email = 4;
  string ngac_node_id = 5;
  string department_id = 6;
  string department_name = 7;
  repeated Role roles = 8;
  string title = 9;
  string avatar_url = 10;
}
```

---

## Frontend Architecture

### Route Structure
```
routes/
├── _workspace/
│   ├── admin.tsx              ← Admin layout (sidebar + outlet)
│   ├── admin/
│   │   ├── index.tsx          ← Organization (department tree) — default
│   │   ├── users.tsx          ← User management table
│   │   └── roles.tsx          ← Role management
```

### Component Hierarchy
```
AdminLayout
├── AdminSidebar (3 nav items: Organization, Users, Roles)
└── <Outlet />
    ├── AdminDepartments (default)
    │   ├── TreeView (reuse existing pattern)
    │   ├── DepartmentDetailPanel (slide-over)
    │   └── CreateDepartmentModal
    ├── AdminUsers
    │   ├── DataTable (reuse existing composite)
    │   ├── UserDetailPanel (slide-over)
    │   └── DepartmentPicker (tree selector)
    └── AdminRoles
        ├── RoleCard list
        ├── RoleDetailPanel (slide-over)
        └── PermissionMatrix (checkbox grid)
```

### Data Hooks
```
hooks/
├── useAdmin.ts               ← admin context (workspace resolution)
├── useDepartments.ts         ← department CRUD queries/mutations
├── useAdminMembers.ts        ← member list + detail queries
└── useAdminRoles.ts          ← role CRUD + permission queries
```

### Component Reuse Assessment
| Component | Exists? | Reuse plan |
|-----------|---------|------------|
| TreeView | ✅ `patterns/TreeView.tsx` | Direct reuse for department tree |
| DataTable | ✅ `composites/DataTable.tsx` | Reuse for user list |
| Modal | ✅ `composites/Modal.tsx` | Reuse for create forms |
| PeekPanel | ✅ `composites/PeekPanel.tsx` | Reuse for detail panels |
| Tabs | ✅ `composites/Tabs.tsx` | Reuse for admin sub-nav |
| ConfirmDialog | ✅ `composites/ConfirmDialog.tsx` | Reuse for delete confirmations |
| Avatar | ✅ `primitives/Avatar.tsx` | Reuse in user list |
| Badge | ✅ `primitives/Badge.tsx` | Reuse for role tags |
| Button, Input, Select | ✅ `primitives/` | Standard reuse |
| PermissionMatrix | ❌ NEW | Checkbox grid — new component |
| DepartmentPicker | ❌ NEW | Tree selector modal — new component |
| AdminSidebar | ❌ NEW | Simple nav list — new component |

---

## Access Control

Admin module access is gated by the **workspace owner** check:
- Owner = assigned to `<workspace>_Owners` UA → sees admin
- Member (non-owner) = assigned to `<workspace>_Members` UA → hidden admin

For MVP, we do NOT introduce a separate "admin" role. Owners manage the organization. This avoids over-engineering the permission model.

Future (Phase 2): dedicated `Admin` role UA with specific admin operations.

---

## Edge Cases

1. **Circular department**: MoveDepartment must check that target is NOT a descendant of the source
2. **Last root department**: Cannot delete if it's the only root — must have at least one root
3. **Department with users**: Deleting reassigns users to parent department (or unassigns)
4. **Concurrent edits**: Optimistic concurrency via `updated_at` timestamp
5. **Empty workspace**: First admin visit creates no departments — shows empty state with CTA

---

## Performance Considerations

- Department tree: workspace typically has 10-50 departments → single query, no pagination needed
- User list: workspace typically has 10-500 users → paginate at 50, with search
- Role list: typically 5-15 roles → no pagination needed
- NGAC graph queries: cached in memory by policy service → fast
