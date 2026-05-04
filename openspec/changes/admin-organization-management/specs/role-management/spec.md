# Role Management

## Proto Contract Analysis

**Existing RPCs:**
- `CreateRole(CreateRoleRequest) returns (Role)` — creates UA node under workspace PC
- `ListRoles(ListRolesRequest) returns (RoleList)` — lists all UA roles
- `DeleteRole(DeleteRoleRequest)` — removes UA node
- `CreatePermission(CreatePermissionRequest)` — creates association (UA → OA, operations)
- `ListPermissions(ListPermissionsRequest)` — lists all associations
- `DeletePermission(DeletePermissionRequest)` — removes association

**Required NEW RPCs:**
- `UpdateRole(UpdateRoleRequest) returns (Role)` — rename role
- `GetRoleDetail(GetRoleDetailRequest) returns (RoleDetail)` — role + permissions + member count

**No DB changes needed** — roles are NGAC UA nodes, permissions are NGAC associations.

## User Stories

### Story 1: View Role List
As an **organization admin**,
I want to see all roles in my organization,
so that I understand the permission structure.

**Acceptance Criteria:**
- [ ] Role list shows as cards or table rows
- [ ] Each role shows: name, member count, permission count summary
- [ ] Built-in roles (Owners, Members) are marked as "System" and cannot be deleted
- [ ] No internal IDs visible
- [ ] Empty state: "No custom roles. Create your first role." with CTA
- [ ] Loading state: skeleton cards

**Proto mapping:** EXISTING — ListRoles + enrichment

### Story 2: Create Role
As an **organization admin**,
I want to create a new role,
so that I can define custom permission groups.

**Acceptance Criteria:**
- [ ] "New Role" button at top of roles section
- [ ] Modal opens with: role name input
- [ ] Name is required, max 50 characters
- [ ] Duplicate name rejected with clear error
- [ ] After creation, role appears in list
- [ ] New role starts with no permissions (admin adds them after)

**Proto mapping:** EXISTING — CreateRole RPC

### Story 3: View Role Permissions
As an **organization admin**,
I want to see what permissions a role has,
so that I understand what the role can do.

**Acceptance Criteria:**
- [ ] Clicking a role opens a detail panel
- [ ] Shows: role name, member count, permission matrix
- [ ] Permission matrix shows: resource type (Documents, Channels, etc.) × operations (read, write, manage, etc.)
- [ ] Permissions displayed as checkboxes (read-only in view mode)
- [ ] All displayed in human-readable format: "Documents — Read, Write" not "oa-123 → read, write"
- [ ] System roles (Owners, Members) show their fixed permissions with "System" label

**Proto mapping:** EXISTING — ListPermissions + enrichment with node names

### Story 4: Edit Role Permissions
As an **organization admin**,
I want to change what permissions a role has,
so that I can control access.

**Acceptance Criteria:**
- [ ] "Edit Permissions" button on role detail panel
- [ ] Permission matrix becomes editable (checkboxes)
- [ ] Toggling a checkbox adds/removes the operation
- [ ] Save button applies all changes
- [ ] Cancel reverts to saved state
- [ ] System roles (Owners, Members) cannot be edited — "Edit" button hidden
- [ ] Changes take effect immediately for all users with this role

**Proto mapping:** EXISTING — CreatePermission / DeletePermission RPCs

### Story 5: Delete Role
As an **organization admin**,
I want to delete a custom role,
so that I can clean up unused roles.

**Acceptance Criteria:**
- [ ] Delete button visible on role detail (only for custom roles)
- [ ] Confirmation: "Delete [Role Name]? [N] users will lose this role's permissions."
- [ ] After deletion, users are NOT removed from organization — they just lose the role
- [ ] System roles cannot be deleted
- [ ] Role disappears from list after deletion

**Proto mapping:** EXISTING — DeleteRole RPC

### Story 6: View Role Members
As an **organization admin**,
I want to see which users have a specific role,
so that I can audit access.

**Acceptance Criteria:**
- [ ] Role detail panel shows "Members" section
- [ ] Lists all users assigned to this role
- [ ] Each user shows: display name, email, avatar
- [ ] Link to user detail from member list
- [ ] Count shown in section header: "Members (5)"

**Proto mapping:** NEW — uses GetChildren on role UA node + enrich with user data

## Flows

### Flow: Create Role and Add Permissions
1. Admin clicks "New Role"
2. Modal: enters "Content Manager"
3. Clicks "Create"
4. Role appears in list
5. Admin clicks "Content Manager"
6. Detail panel opens (empty permissions)
7. Admin clicks "Edit Permissions"
8. Checks: Documents → Read, Write; Channels → Read
9. Clicks "Save"
10. Permissions matrix updates

### Flow: Delete Role
1. Admin selects "Old Role" (2 members)
2. Clicks "Delete"
3. Confirmation: "Delete Old Role? 2 users will lose this role."
4. Admin confirms
5. Role removed from list
6. Users remain in organization with other roles

## States

| Screen | Empty | Loading | Loaded | Error |
|--------|-------|---------|--------|-------|
| Role list | "No custom roles" + CTA | Skeleton cards | Card/table list | "Failed to load" + retry |
| Role detail | n/a | Spinner | Role info + permissions | "Failed to load" |
| Create modal | Empty form | Submit spinner | n/a | Inline errors |
| Permission editor | All unchecked | Saving spinner | Checkbox matrix | "Failed to save" toast |
