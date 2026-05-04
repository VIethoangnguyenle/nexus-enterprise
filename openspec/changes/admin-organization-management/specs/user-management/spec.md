# User Management

## Proto Contract Analysis

**Existing RPCs:**
- `ListMembers(ListMembersRequest) returns (MemberList)` — lists all workspace members with roles
- `InviteMember(InviteMemberRequest)` — adds user to workspace
- `RemoveMember(RemoveMemberRequest)` — removes user from workspace
- `UpdateMemberRoles(UpdateMemberRolesRequest)` — assigns roles to user

**Required NEW RPCs** (workspace.proto):
- `UpdateMemberDepartment(UpdateMemberDepartmentRequest) returns (Empty)` — assign user to department
- `GetMemberDetail(GetMemberDetailRequest) returns (MemberDetail)` — enriched member info with department, roles

**Existing DB:**
- `users` table: id, username, email, display_name, title, department (TEXT), location, avatar_url
- `tenant_users` table: tenant_id, user_id, role, status, open_id, ngac_node_id

## User Stories

### Story 1: View User List
As an **organization admin**,
I want to see all users in my organization,
so that I can manage team membership.

**Acceptance Criteria:**
- [ ] User list shows in a table/list format
- [ ] Each row shows: display name (or username), email, department name, role names
- [ ] No UUIDs or internal IDs visible
- [ ] Avatar shown next to name (if available)
- [ ] Search/filter by name, email, department
- [ ] Sort by name (default), department, role
- [ ] Empty state: "No users in this organization" 
- [ ] Loading state: skeleton table rows

**Proto mapping:** EXISTING — ListMembers + enrichment with auth GetUserByID

### Story 2: View User Detail
As an **organization admin**,
I want to view a user's details,
so that I can see their department and role assignments.

**Acceptance Criteria:**
- [ ] Clicking a user opens a detail panel (slide-over or side panel)
- [ ] Shows: display name, email, title, department, location, role list
- [ ] Department shown as human-readable name (not ID)
- [ ] Roles shown as tag/badges with human-readable names
- [ ] "Edit" action available for department and role assignment

**Proto mapping:** NEW — GetMemberDetail (enriched view)

### Story 3: Assign User to Department
As an **organization admin**,
I want to assign a user to a department,
so that the user belongs to the correct part of the organization.

**Acceptance Criteria:**
- [ ] From user detail panel, "Change Department" button opens a department picker
- [ ] Department picker shows department tree (searchable)
- [ ] Selecting a department and confirming updates the user's department
- [ ] Change is reflected immediately in user list and detail panel
- [ ] User can be moved from one department to another
- [ ] User can be unassigned from a department (set to "None")

**Proto mapping:** NEW — UpdateMemberDepartment RPC

### Story 4: Assign Roles to User
As an **organization admin**,
I want to assign one or more roles to a user,
so that they get the correct permissions.

**Acceptance Criteria:**
- [ ] From user detail panel, "Edit Roles" button opens a role picker
- [ ] Role picker shows checkboxes for all available roles
- [ ] Current roles are pre-checked
- [ ] Saving updates the user's role assignments
- [ ] Change is reflected immediately in user list and detail panel
- [ ] At least one role must remain (Members role is always assigned)

**Proto mapping:** EXISTING — UpdateMemberRoles RPC

### Story 5: Search and Filter Users
As an **organization admin**,
I want to search and filter the user list,
so that I can quickly find specific users.

**Acceptance Criteria:**
- [ ] Search bar at top of user list
- [ ] Searches by display name and email
- [ ] Filter dropdown for department
- [ ] Filter dropdown for role
- [ ] Filters can be combined (department + role + search text)
- [ ] "Clear filters" button when any filter is active
- [ ] Results update in real-time as user types

**Proto mapping:** Frontend-only — client-side filtering of ListMembers result

## Flows

### Flow: Assign User to Department
1. Admin navigates to Users tab
2. Admin clicks a user row
3. Detail panel slides open showing user info
4. Admin clicks "Change Department"
5. Department picker modal opens
6. Admin selects "Engineering" department
7. Admin clicks "Save"
8. System calls UpdateMemberDepartment RPC
9. Detail panel updates with "Engineering" department
10. User list row updates department column

## States

| Screen | Empty | Loading | Loaded | Error |
|--------|-------|---------|--------|-------|
| User list | "No users" | Skeleton table | Table with data | "Failed to load" + retry |
| User detail | n/a | Spinner in panel | User info + actions | "Failed to load" + retry |
| Department picker | "No departments" | Spinner | Tree selector | "Failed to load" |
| Role picker | "No roles" | Spinner | Checkbox list | "Failed to load" |
