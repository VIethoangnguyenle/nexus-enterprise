# Approval: Template Management

## Proposal Evaluation
ACCEPT — CEO correctly identified that the system exists and needs audit-fix-verify. Proto has `CreateTemplate`, `GetTemplate`, `ListTemplates`, `UpdateTemplate`. DB has `approval_templates`, `approval_conditions`, `approval_steps`. All implementable.

---

## User Stories

### US-1: Admin creates approval template

As a **workspace admin**,
I want to create an approval template with name, entity type, steps, and optional form fields,
so that approval workflows are configured for my workspace.

Acceptance Criteria:
- [ ] Click "New Template" opens CreateTemplateModal
- [ ] Form requires: name (text), entity_type (select), at least 1 step
- [ ] Each step has: step_order, name, approver_type (select), approver_value, required_count
- [ ] Optional: form_fields can be added (label, field_type, required, options)
- [ ] Submit calls `POST /api/approval/templates`
- [ ] On success: modal closes, templates list refreshes
- [ ] On error: inline error message shown, modal stays open
- [ ] Empty name → validation error before submit

Proto mapping: `CreateTemplate` RPC
Backend status: EXISTS — `rest/handler.go:CreateTemplate`, `domain/template.go`

---

### US-2: Admin views template list

As a **workspace admin**,
I want to see all configured templates with name, entity type, status, step count, and field count,
so that I can manage approval workflows.

Acceptance Criteria:
- [ ] Templates tab shows table with columns: Name, Type, Fields, Steps, Status, Actions
- [ ] Active templates show green "Active" badge
- [ ] Inactive templates show neutral "Inactive" badge
- [ ] Click row opens TemplateDetailPanel
- [ ] Edit button opens CreateTemplateModal in edit mode

Proto mapping: `ListTemplates` RPC
Backend status: EXISTS

---

### US-3: Admin updates template

As a **workspace admin**,
I want to edit a template's name, status, steps, conditions, and form fields,
so that I can adjust workflows as requirements change.

Acceptance Criteria:
- [ ] CreateTemplateModal populates with existing template data when editing
- [ ] Changes save via `PUT /api/approval/templates/:id`
- [ ] On success: modal closes, template list and detail panel refresh
- [ ] Toggle is_active to deactivate without deleting

Proto mapping: `UpdateTemplate` RPC
Backend status: EXISTS

---

## States

| State | Template Tab |
|-------|-------------|
| Empty | "No templates configured" + "Create Template" CTA |
| Loading | Centered spinner |
| Loaded | DataTable with template rows |
| Error (generic) | ErrorState with retry |
| Error (not provisioned) | "Approvals not configured" empty state |
