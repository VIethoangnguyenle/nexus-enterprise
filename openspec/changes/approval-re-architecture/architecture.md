# Architecture: Approval Re-Architecture

## System Overview

```
┌──────────────────────┐
│  Frontend (Vite)     │  approval.tsx → useApproval.ts → approvalApi.ts
│  Proxy: /api/approval│
└──────────┬───────────┘
           │ HTTP
┌──────────▼───────────┐
│  Traefik             │  PathPrefix(`/api/approval`) → approval:8080
│  :80                 │
└──────────┬───────────┘
           │
┌──────────▼───────────┐
│  Approval Service    │  REST :8186 (dev) + gRPC :50058
│  cmd/main.go         │  Echo REST + gRPC Health
├──────────────────────┤
│  rest/handler.go     │  Parse → Validate → Delegate → Respond
│  grpc/server.go      │  Service-to-service (not used by frontend)
│  domain/             │  Business logic (template, execution, queries)
│  store/store.go      │  Tenant-schema-scoped SQL (pgx)
├──────────────────────┤
│  Events              │  Kafka reconciliation consumer (optional)
└──────────┬───────────┘
           │ gRPC
┌──────────▼───────────┐
│  Policy Service      │  CheckAccess, ResolveAccessibleScopes
│  :50051              │
└──────────┬───────────┘
           │
┌──────────▼───────────┐
│  PostgreSQL          │  tenant_schemas registry
│  :5432               │  {schema}.approval_templates
│                      │  {schema}.approval_conditions
│                      │  {schema}.approval_steps
│                      │  {schema}.approval_requests
│                      │  {schema}.approval_assignments
│                      │  {schema}.approval_audit_log
└──────────────────────┘
```

## Critical Architecture Observations

### 1. Tenant Schema Isolation (RISK: HIGH)
- Approval uses schema-per-tenant pattern (`007_tenant_schema_approval.sql`)
- `httputil.TenantSchemaResolver` resolves `tenant_id → schema_name`
- Every REST endpoint goes through `tenantSchemaMiddleware()` 
- Store queries must use `httputil.GetTenantSchema(ctx)` to get schema prefix
- **UNVERIFIED**: Does provisioning actually work? Does schema resolution work?

### 2. Policy Integration (RISK: MEDIUM)
- `policyGRPCAdapter` in main.go wraps `PolicyReadServiceClient`
- Domain uses `CheckAccess` for approve/reject permission verification
- Domain uses `ResolveAccessibleScopes` for department tab + role-based assignment
- **UNVERIFIED**: Does the policy service return correct scopes for approval operations?

### 3. Domain Execution Logic (RISK: MEDIUM)
- Template matching: `matcher.go` (has unit tests)
- Request creation: `execution.go:CreateApprovalRequest` (5-step orchestration)
- Step advancement: `execution.go:checkStepCompletion` (template snapshot + step counting)
- NGAC double-check: `verifyApproveAccess` before every approve/reject
- **Has unit tests** in `execution_test.go`, `matcher_test.go`, `queries_test.go`

### 4. REST Handler Layer (RISK: LOW)
- Follows thin handler pattern (parse → validate → delegate → respond)
- Uses `httputil.MapDomainError()` for error translation
- JWT + tenant middleware properly stacked
- **Structurally sound** but untested against real DB

### 5. Frontend Integration (RISK: MEDIUM)
- API layer (`approval.ts`) maps correctly to REST endpoints
- Hooks use TanStack Query with proper key management
- Optimistic updates for approve/reject/batch
- **UNVERIFIED**: Does CreateRequestModal actually work with real template data?

## Validation Plan

### Phase 1: Service Boot + Provisioning
1. Start infra (Docker: Postgres, Redis, Redpanda)
2. Start policy service (required dependency)
3. Build and start approval service
4. Verify health check responds
5. Provision a tenant schema via `POST /api/admin/tenants/:id/provision`
6. Verify schema created in Postgres

### Phase 2: Template CRUD via REST
7. Create template: `POST /api/approval/templates` (with JWT + tenant_id)
8. List templates: `GET /api/approval/templates`
9. Get template: `GET /api/approval/templates/:id`
10. Update template: `PUT /api/approval/templates/:id`

### Phase 3: Request Lifecycle via REST
11. Create request: `POST /api/approval/requests` (triggers template matching)
12. Get pending: `GET /api/approval/pending` (assigned approver)
13. Approve: `POST /api/approval/approve`
14. Verify step advancement / completion
15. Get history: `GET /api/approval/history`

### Phase 4: Frontend Integration
16. Navigate to /workspace/approval
17. Verify tabs load data
18. Test CreateTemplateModal
19. Test CreateRequestModal
20. Test approve/reject flow in UI

## Decision: Validation-First Architecture

```
For each layer:
├── Test exists + passes? → Layer likely correct
├── Test doesn't exist? → Must validate via execution
├── Validation passes? → KEEP
├── Validation fails? → REWRITE that specific layer
└── Cannot validate? → REWRITE (conservative)
```

## Boundaries

- Approval service owns: template CRUD, request lifecycle, assignment management, audit log
- Policy service owns: permission decisions (CheckAccess, ResolveAccessibleScopes)
- Workspace service owns: tenant context (tenant_id in JWT)
- Frontend owns: UI state, optimistic updates, tab management
