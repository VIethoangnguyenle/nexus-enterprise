# Department Management

## Proto Contract Analysis

**Existing RPCs:** None for departments.
**Required NEW RPCs** (workspace.proto):
- `CreateDepartment(CreateDepartmentRequest) returns (Department)` — creates NGAC UA node + DB record
- `ListDepartments(ListDepartmentsRequest) returns (DepartmentList)` — returns tree structure
- `UpdateDepartment(UpdateDepartmentRequest) returns (Department)` — rename
- `DeleteDepartment(DeleteDepartmentRequest) returns (Empty)` — remove + reassign children
- `MoveDepartment(MoveDepartmentRequest) returns (Department)` — change parent

**DB Changes Required:**
- New `departments` table: `id, workspace_id, name, parent_id, ngac_ua_id, sort_order, created_at, updated_at`
- The `users.department` TEXT field (migration 009) should be migrated to use `department_id` FK

**NGAC Model:**
- Each department = UA node under workspace PC
- Hierarchy = NGAC assignment chain: `Dept_Child → Dept_Parent → PC`
- Users assigned to department UA inherit department-scope permissions

## User Stories

### Story 1: View Department Tree
As an **organization admin**,
I want to see all departments in a tree structure,
so that I understand the organization hierarchy.

**Acceptance Criteria:**
- [ ] Department tree displays as an expandable/collapsible tree view
- [ ] Root departments show at top level, child departments nested underneath
- [ ] Each department shows: name, member count
- [ ] No internal IDs visible — only department names and member counts
- [ ] Empty state: "No departments yet. Create your first department." with a CTA button
- [ ] Tree supports at least 3 levels of nesting
- [ ] Clicking a department selects it and shows its details in a detail panel

**Proto mapping:** NEW — ListDepartments RPC

### Story 2: Create Department
As an **organization admin**,
I want to create a new department,
so that I can structure my organization.

**Acceptance Criteria:**
- [ ] "New Department" button visible at top of department tree
- [ ] Clicking opens a modal with: name input, parent department dropdown (optional)
- [ ] Parent dropdown shows existing departments in tree format (indented by level)
- [ ] If no parent selected → root-level department
- [ ] After creation, new department appears in tree without full reload
- [ ] Name is required, max 100 characters
- [ ] Duplicate name within same parent is rejected with clear error message

**Proto mapping:** NEW — CreateDepartment RPC

### Story 3: Edit Department
As an **organization admin**,
I want to rename a department,
so that I can keep the structure up to date.

**Acceptance Criteria:**
- [ ] Selected department shows an edit button (or inline edit on double-click)
- [ ] Editing opens inline rename input
- [ ] Save updates the name immediately in the tree
- [ ] Cancel reverts to original name
- [ ] Validation: name required, max 100 characters

**Proto mapping:** NEW — UpdateDepartment RPC

### Story 4: Delete Department
As an **organization admin**,
I want to delete a department,
so that I can clean up unused departments.

**Acceptance Criteria:**
- [ ] Delete button visible when department is selected
- [ ] Confirmation dialog shows: "Delete [Department Name]? Members will be moved to the parent department."
- [ ] If department has children → confirmation shows children count
- [ ] Children departments are reassigned to the deleted department's parent (or root)
- [ ] After deletion, tree updates without full reload
- [ ] Cannot delete if it's the only root department

**Proto mapping:** NEW — DeleteDepartment RPC

### Story 5: Move Department (Change Parent)
As an **organization admin**,
I want to move a department under a different parent,
so that I can reorganize the structure.

**Acceptance Criteria:**
- [ ] Right-click or action menu on department shows "Move to..."
- [ ] Opens a department picker excluding the department itself and its descendants
- [ ] After move, tree re-renders with the new hierarchy
- [ ] Cannot move a department to be its own child (circular dependency prevention)

**Proto mapping:** NEW — MoveDepartment RPC

## Flows

### Flow: Create Department
1. Admin clicks "New Department" button
2. Modal appears with name input and parent dropdown
3. Admin enters name "Engineering"
4. Admin optionally selects parent "Technology"
5. Admin clicks "Create"
6. System calls CreateDepartment RPC → creates NGAC UA node + DB row
7. Modal closes, tree updates with "Engineering" under "Technology"

### Flow: Delete Department with Children
1. Admin selects "Old Department" (which has 2 children)
2. Admin clicks "Delete"
3. Confirmation: "Delete Old Department? 2 sub-departments will be moved to root."
4. Admin confirms
5. System reassigns children to parent, then deletes
6. Tree updates — children move to new parent

## States

| Screen | Empty | Loading | Loaded | Error |
|--------|-------|---------|--------|-------|
| Department tree | "No departments yet" + CTA | Skeleton tree | Expandable tree view | "Failed to load" + retry |
| Create modal | Pre-filled empty form | Submit spinner | n/a | Inline validation errors |
| Delete confirm | n/a | Deleting spinner | n/a | "Failed to delete" toast |
