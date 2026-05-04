# Template Management (Admin CRUD)

## Proto Mapping
- `CreateTemplate` — ✅ exists
- `GetTemplate` — ✅ exists
- `ListTemplates` — ✅ exists
- `UpdateTemplate` — ✅ exists
- `DeleteTemplate` — ❌ NEW (deactivate via `is_active = false`)

## User Stories

### Story 1: View template list (Admin)
As an **Admin**, I want to see a list of all approval templates, so that I can manage which templates are available for my organization.

**Acceptance Criteria:**
- [ ] Templates tab visible only for admin/manager roles
- [ ] DataTable columns: Name, Entity Type, Steps (count), Status (Active/Inactive), Created date
- [ ] Active templates show green Badge, inactive show gray
- [ ] Click row opens template detail in PeekPanel
- [ ] "Create Template" button in header area
- [ ] Empty state: icon + "No templates yet" + "Create your first template" CTA

**States:**
- Empty: EmptyState with clipboard icon, "No templates yet" title
- Loading: LoadingState (spinner)
- Loaded: DataTable with template rows
- Error: ErrorState with retry button

### Story 2: Create template (Admin)
As an **Admin**, I want to create a new approval template with steps and form fields, so that employees can submit structured approval requests.

**Acceptance Criteria:**
- [ ] Click "Create Template" opens Modal (desktop) or full-screen (mobile)
- [ ] Form has sections: Basic Info, Form Fields, Approval Steps
- [ ] Basic Info: Name (required), Entity Type (select: purchase/expense/leave/hiring/contract/custom)
- [ ] Form Fields: Dynamic list — add/remove/reorder fields (see dynamic-form-fields spec)
- [ ] Approval Steps: Ordered list — step name, approver type (specific user/role/department/creator's manager), required count
- [ ] "Add Step" button appends new step with auto-incremented order
- [ ] Save button calls CreateTemplate RPC
- [ ] Success: toast + redirect to template list
- [ ] Validation: Name required, at least 1 step required

**States:**
- Initial: Empty form with all sections
- Filling: Validation runs on blur
- Submitting: Save button shows spinner, form disabled
- Error: Toast with error message, form re-enabled

### Story 3: Edit template (Admin)
As an **Admin**, I want to edit an existing template, so that I can adjust the workflow without creating a new template.

**Acceptance Criteria:**
- [ ] Edit button in template detail PeekPanel
- [ ] Opens same form as create, pre-populated with existing data
- [ ] Can modify name, entity type, form fields, steps
- [ ] Save calls UpdateTemplate RPC
- [ ] Success: toast + updated detail view

### Story 4: Deactivate/Reactivate template (Admin)
As an **Admin**, I want to deactivate a template, so that employees can no longer submit new requests using it, while keeping existing requests valid.

**Acceptance Criteria:**
- [ ] Toggle switch or "Deactivate" button in template detail
- [ ] ConfirmDialog: "Are you sure? Existing requests will not be affected."
- [ ] After deactivate: template shows gray "Inactive" badge in list
- [ ] Inactive templates do NOT appear in template selector when creating requests
- [ ] "Reactivate" button available on inactive templates
- [ ] No hard delete — data preserved

### Story 5: View template detail (Admin)
As an **Admin**, I want to see full template details including form fields and approval steps, so that I can understand the workflow.

**Acceptance Criteria:**
- [ ] PeekPanel shows: name, entity type, status, created by, created date
- [ ] "Form Fields" section lists all defined fields with type icons
- [ ] "Approval Steps" section shows ordered steps with approver info
- [ ] "Edit" and "Deactivate" action buttons in panel header

**States:**
- Loading: Skeleton in panel
- Loaded: Full template detail

## Flows

### Flow: Create template
1. Admin navigates to `/approval`
2. Admin clicks "Templates" tab
3. Admin clicks "Create Template" button
4. Modal opens with empty form
5. Admin fills Basic Info (name, entity type)
6. Admin adds Form Fields (label, type, required)
7. Admin adds Approval Steps (name, approver, count)
8. Admin clicks "Save"
9. System creates template via API
10. Success toast, modal closes, list refreshes

### Flow: Deactivate template
1. Admin clicks template row
2. PeekPanel opens with detail
3. Admin clicks "Deactivate"
4. ConfirmDialog appears
5. Admin confirms
6. System calls UpdateTemplate with `is_active = false`
7. Template badge changes to "Inactive"
