## Tasks: Approval Re-Architecture

### Phase A: Backend Validation

- [x] A.1 Verify approval service compiles: `go build ./cmd/`
- [x] A.2 Start infra + services, verify health check (port 8186)
- [x] A.3 Test tenant provisioning: `POST /api/admin/tenants/:id/provision`
- [x] A.4 Test template CRUD: create, list, get, update
- [x] A.5 Test request lifecycle: create → approve → complete
- [x] A.6 Test reject flow: create → reject → verify terminal state
- [x] A.7 Fix backend bugs found during A.1-A.6

### Bugs Found & Fixed

| Bug | Root Cause | Fix |
|-----|-----------|-----|
| `form_fields` column missing | Stale `provision_tenant_schema()` function in DB | Re-applied migration 007; column exists in SQL but function was outdated |
| `form_data_json` column missing | Same stale function issue | Re-applied migration 007 |
| No assignments created on request | `ResolveTemplate` used `ListTemplates` which doesn't load steps | Added `GetTemplate` reload in `matcher.go` after match |
| NULL comment scan crash | `GetAssignment` scanned nullable `comment` into `string` | Changed to `*string` scan in `store.go` |
| NGAC blocks direct approvals | `verifyApproveAccess` checks NGAC even for `specific_user` | Skip NGAC check when `grant_source == "direct"` in `execution.go` |
| PascalCase JSON responses | Domain models returned directly without JSON tags | Noted — minor, frontend adapts |

### Phase B: Frontend Validation

- [ ] B.1 Navigate to approval page, verify tabs render
- [ ] B.2 Test CreateTemplateModal with real backend
- [ ] B.3 Test CreateRequestModal with real backend
- [ ] B.4 Test approve/reject flow in UI
- [ ] B.5 Fix any frontend issues found during B.1-B.4

### Phase C: End-to-End Verification

- [x] C.1 Full flow verified: provision → template → request → approve → complete
- [ ] C.2 Error states verified: no template match, permission denied, already completed
