# Approval Redesign v2 — Specs Overview

## Module: Approval Workflow Engine (Frontend Redesign + Dynamic Form Fields)

## Proto Mapping Summary

### Existing RPCs (no change needed)
| RPC | Purpose | Used by |
|-----|---------|---------|
| `CreateTemplate` | Create approval template | Template Management |
| `GetTemplate` | Get template detail | Template Management |
| `ListTemplates` | List templates by entity_type | Template Management |
| `UpdateTemplate` | Update template | Template Management |
| `CreateRequest` | Submit approval request | Create Request |
| `Approve` | Approve a request step | Approval Actions |
| `Reject` | Reject a request | Approval Actions |
| `BatchApprove` | Batch approve requests | Approval Actions |
| `GetPending` | Query pending tab | Approval List |
| `GetHistory` | Query history tab | Approval List |
| `GetMyRequests` | Query my requests tab | Approval List |
| `GetDepartmentRequests` | Query department tab | Approval List |
| `GetAuditLog` | Get audit trail | Approval Detail |

### Proto Changes Required
| Change | Why |
|--------|-----|
| Add `FormFieldDefinition` message | Templates need to define dynamic form fields |
| Add `repeated FormFieldDefinition form_fields` to `ApprovalTemplate` | Associate fields with templates |
| Add `repeated FormFieldDefinition form_fields` to `CreateTemplateRequest` | Send fields when creating |
| Add `repeated FormFieldDefinition form_fields` to `UpdateTemplateRequest` | Send fields when updating |
| Add `string form_data_json` to `ApprovalRequest` | Store submitted form data |
| Add `string form_data_json` to `CreateApprovalRequestReq` | Send form data when creating request |
| Add `DeleteTemplate` RPC | CRUD compliance — deactivate template |
| Add `CancelRequest` RPC | Creator can cancel own pending request |
| Add `GetRequest` RPC | Get single request detail by ID |

### DB Changes Required
| Change | Table | Why |
|--------|-------|-----|
| Add `form_fields JSONB` column | `approval_templates` | Store field definitions |
| Add `form_data JSONB` column | `approval_requests` | Store submitted values |

## Capabilities (5 specs)

| # | Capability | Spec Path | Stories | Backend Work |
|---|-----------|-----------|---------|-------------|
| 1 | Dynamic Form Fields | `dynamic-form-fields/spec.md` | 4 | Proto + Backend + Frontend |
| 2 | Template Management | `template-management/spec.md` | 5 | Frontend only (REST exists) |
| 3 | Create Request | `create-request/spec.md` | 3 | Minor backend (form validation) |
| 4 | Approval List | `approval-list/spec.md` | 4 | Frontend only (REST exists) |
| 5 | Approval Detail | `approval-detail/spec.md` | 4 | Frontend only |

**Total: 20 user stories, ~60 acceptance criteria**

## Dependencies
- Auth service: ✅ JWT with ngac_node_id
- Workspace layout: ✅ AppSidebar + TopBar shell
- Design system: ✅ Primitives + Composites
- Approval backend: ✅ 18 REST endpoints
- Proto: ⚠️ Needs extension (form fields, cancel, get single)
