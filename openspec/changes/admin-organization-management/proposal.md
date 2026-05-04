# Admin Module — Organization Management

## Evidence Summary
- Backend: partial — workspace service has role CRUD + member management, NO department logic
- Frontend: missing — no `/admin` route, no admin components
- Proto: partial — `workspace.proto` has Role/Member RPCs, NO department RPCs
- DB: partial — `users.department` TEXT field exists (migration 009), NO departments table, NGAC graph has role UA nodes
- Dependencies: workspace service (exists), policy service (exists), auth service (exists)

## Product Assessment
- Size: **L** — backend needs department proto + handlers, frontend needs new admin module (3 sections, 5+ screens)
- Risk: **Medium** — backend role/member APIs exist but department hierarchy is new; NGAC modeling for department scope needs careful design
- Target user: **Organization Admin / Owner** — manages org structure, assigns departments and roles
- Core action: Create departments, manage users within departments, create roles with permissions

## Scope

### In scope (Phase 1 — MVP)
1. **Department management**: CRUD + parent-child hierarchy, tree view UI
2. **User management**: List users, assign to department, assign roles
3. **Role management**: Create/delete roles, assign permissions to roles
4. **Admin navigation**: `/admin` route with sidebar (Organization, Users, Roles)
5. **Permission display**: Show which operations a role grants (read-only view of NGAC associations)

### Out of scope
- Drag-and-drop department reordering (complex UX, not MVP)
- Custom permission templates (over-engineering for Phase 1)
- Audit log for admin actions (future Phase 2)
- Bulk user import/export
- Department-scoped data isolation (approval already handles via scope_oa_id)

### Deferred (Phase 2)
- Advanced hierarchy operations (merge, split departments)
- Permission refinement (granular per-resource permissions)
- Department head assignment
- Audit trail for admin operations
- RBAC template presets

## Success Criteria
- Admin can create a department hierarchy (at least 2 levels)
- Admin can assign users to departments
- Admin can create roles and assign them to users
- Admin can see role permissions (human-readable, no IDs)
- All admin pages are non-technical, easy to understand
- No UUID/internal IDs displayed on UI
