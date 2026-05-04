# Dynamic Form Fields

## Proto Mapping
- Message: `FormFieldDefinition` — **NEW** (not in proto)
- Field in template: `repeated FormFieldDefinition form_fields` — **NEW**
- Field in request: `string form_data_json` — **NEW**
- Backend: Validate form data against template fields — **NEW logic**

## User Stories

### Story 1: Define form fields in template (Admin)
As an **Admin**, I want to add dynamic form fields when creating an approval template, so that each template type collects the right information from submitters.

**Acceptance Criteria:**
- [ ] Template create form shows "Form Fields" section
- [ ] Admin can add field with: label, field type (text/number/currency/date/select/textarea), required flag
- [ ] Admin can add select field with options list (comma-separated or add-one-by-one)
- [ ] Admin can reorder fields via drag or up/down arrows
- [ ] Admin can remove a field
- [ ] Saving template persists form field definitions to backend
- [ ] At least these field types are supported: text, number, currency, date, select, textarea

**States:**
- Empty: No fields added yet — "Add your first field" button
- Has fields: List of field cards with type icon, label, required badge
- Error: Backend save fails — toast with retry

### Story 2: Edit form fields in existing template (Admin)
As an **Admin**, I want to edit form fields on an existing template, so that I can adjust what data is collected without creating a new template.

**Acceptance Criteria:**
- [ ] Edit template pre-populates existing form fields
- [ ] Admin can add, remove, reorder fields
- [ ] Saving updates persists changes
- [ ] Existing requests are NOT affected (they store snapshot)

**States:**
- Loading: Skeleton while fetching template
- Loaded: Pre-populated form fields

### Story 3: Render dynamic form on request creation (Employee)
As an **Employee**, I want to fill in a dynamic form when creating an approval request, so that I provide all required information for the approval process.

**Acceptance Criteria:**
- [ ] After selecting template type, the form fields render dynamically
- [ ] Text field → renders `<Input>`
- [ ] Number field → renders `<Input type="number">`
- [ ] Currency field → renders `<Input>` with currency prefix (VND)
- [ ] Date field → renders `<Input type="date">`
- [ ] Select field → renders `<Select>` with defined options
- [ ] Textarea field → renders `<Textarea>`
- [ ] Required fields show asterisk (*) and validation error if empty
- [ ] Submit is disabled until all required fields are filled
- [ ] Form data is sent as structured JSON to backend

**States:**
- No template selected: Template selector visible, form area empty
- Template selected: Dynamic form renders with all defined fields
- Validation error: Red border + error message on invalid fields
- Submitting: Submit button shows spinner, fields disabled

### Story 4: Display form data in approval detail (Manager)
As a **Manager**, I want to see the filled form data when reviewing an approval request, so that I can make an informed decision.

**Acceptance Criteria:**
- [ ] Detail panel shows "Request Details" section with all submitted field values
- [ ] Each field displays: label and value in a consistent layout
- [ ] Currency values show formatted with VND
- [ ] Date values show formatted (DD/MM/YYYY)
- [ ] Empty optional fields show "—"

**States:**
- No form data: Section shows "No additional details"
- Has form data: Renders label/value pairs in vertical layout

## Flows

### Flow: Define form fields in template
1. Admin navigates to Approvals page
2. Admin clicks "Templates" tab (admin only)
3. Admin clicks "Create Template"
4. Admin fills template name, entity type
5. Admin scrolls to "Form Fields" section
6. Admin clicks "Add Field"
7. Admin selects field type, enters label, sets required
8. Admin repeats for all needed fields
9. Admin adds approval steps
10. Admin clicks "Save Template"
11. System saves template with form fields to backend
12. Admin sees success toast, returns to template list

Error flow:
- If save fails → Error toast with "Failed to save template. Retry?"
- If field label is empty → Inline validation error
