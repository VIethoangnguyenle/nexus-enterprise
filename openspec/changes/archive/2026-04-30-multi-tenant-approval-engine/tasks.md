## Tasks

### Phase 0: Approval Service Scaffolding (first — foundation)
- [x] Create service directory structure: `backend/services/approval/{cmd,internal/{rest,grpc,domain,store}}`
- [x] `go.mod` with replace directive `ngac-platform => ../..`
- [x] Dockerfile
- [x] Add to `docker-compose.yml` with Traefik routing (`PathPrefix: /api/approval`)
- [x] Add proto: `backend/proto/approval/approval.proto`
- [x] Add to `backend/Makefile` (build, proto gen targets)
- [x] Wire empty `cmd/main.go` with Echo REST + gRPC servers
- [x] Verify: `go build ./cmd/` passes

### Phase 1: Schema-per-Tenant Infrastructure
- [x] Create `public.tenant_schemas` registry table (tenant_id, schema_name, status, version)
- [x] `tenant_id` already in JWT claims (migration 005) — no change needed
- [x] Implement `TenantSchemaMiddleware` for Echo (resolves tenant_id → schema_name, stores in ctx)
- [x] Create `TenantSchemaResolver` (cached lookup) + `TenantConn` (SET search_path per connection)
- [x] Create tenant schema provisioning SQL function `provision_tenant_schema()`
- [x] Rewrote store layer to use tenant-scoped connections via `conn(ctx)`
- [x] Implement tenant provisioning endpoint (POST /api/admin/tenants/:id/provision)
- [x] Integration test: 2 tenants, verify data isolation (tenant A cannot see tenant B data)

### Phase 2: NGAC Graph Refactor — Structural + Regional Hierarchy
- [x] Refactor graph to only load structural nodes (UA, OA, PC) — skip O nodes in LoadGraph()
- [x] Regional OA grouping is supported inherently by graph structure (Root → Region → Department)
- [x] Add `ResolveAccessibleScopes` RPC to `policy_read.proto`
- [x] Implement ResolveAccessibleScopes in ReadServer:
  - Step 1: BFS upward from user → find UA ancestors
  - Step 2: Find associations matching operation → collect target OA IDs
  - Step 3: GetDescendants for each target OA → flatten to leaf OA IDs
- [x] Add Redis caching for scope resolution (TTL 60s, key: `scopes:{user_node}:{operation}`)
- [x] Invalidate scope cache on policy write events (assignment/association create/delete)
- [x] Add `scope_oa_id` column to `drive_items` table
- [x] Populate `scope_oa_id` on item creation (inherit from parent folder's OA)
- [x] Refactor drive `ListFolder` to use `WHERE scope_oa_id = ANY($1)` instead of per-item CheckAccess loop
- [x] Unit test: ResolveAccessibleScopes returns correct leaf OAs for regional hierarchy
- [x] Performance test: ListFolder with 10K items < 50ms

### Phase 3: Approval Template Engine
- [x] Template tables already created in migration 007 (provision_tenant_schema)
- [x] Implement template store: InsertTemplate, ListTemplates, GetTemplate, UpdateTemplate
- [x] Implement condition matching engine (evaluate JSONB conditions: gt, gte, lt, lte, eq, in, between)
- [x] Implement template resolution: entity fields → best matching template (by priority)
- [x] Implement `{creator_dept}` placeholder resolution in approver_value
- [x] REST API: CRUD for approval templates (admin-only endpoints)
- [x] Unit tests: condition matching, priority resolution, placeholder substitution

### Phase 4: Approval Execution Runtime
- [x] Execution tables already created in migration 007 (provision_tenant_schema)
- [x] Indexes created in migration 007 (idx_aa_pending, idx_aa_history, idx_ar_scope, idx_ar_creator)
- [x] Implement approver resolver (specific_user, role_in_dept, department)
- [x] Implement `CreateApprovalRequest`: match template → snapshot → resolve approvers → insert assignments → audit
- [x] Implement `Approve`: validate → update assignment → check required_count → advance/complete → skip remaining → audit
- [x] Implement `Reject`: update assignment → mark rejected → skip ALL remaining → audit
- [x] Implement `BatchApprove`: atomic multi-item approve with RETURNING
- [x] Implement NGAC double-check on write path: verify user still has role before executing approve/reject
- [x] REST API: POST /approve, POST /reject, POST /batch-approve
- [x] Unit tests: step advancement, reject=terminal, batch approve, required_count > 1, skipped status

### Phase 5: Query APIs — 4 Tabs
- [x] Tab 1 "Chờ duyệt": load all (no paging), `WHERE user_node_id AND status = 'pending' AND current_step = step_order`
- [x] Tab 2 "Đã duyệt": cursor-based on `acted_at`
- [x] Tab 3 "Lệnh tôi tạo": cursor-based on `created_at`
- [x] Tab 4 "Tất cả lệnh department": scope-based via ResolveAccessibleScopes → `WHERE scope_oa_id = ANY($1)`
- [x] REST API: GET /pending, GET /history, GET /my-requests, GET /department-requests
- [x] Performance test: pending < 5ms, history cursor < 10ms, scope query < 10ms

### Phase 6: Audit Trail
- [x] Implement append-only audit log insert on all actions (fire-and-forget in domain)
- [x] Actions: created, assigned, approved, rejected, step_advanced, completed
- [x] Policy change actions: reassigned_policy_change, revoked_policy_change (Phase 7)
- [x] REST API: GET /requests/:id/audit
- [x] Verify: no UPDATE/DELETE on audit_log table (append-only constraint via trigger)

### Phase 7: Policy Change Reconciliation
- [x] Publish policy change events to Redpanda from PolicyWriteService (on assignment/association CRUD)
- [x] Implement reconciliation consumer in Approval Service
- [x] Handle user added to UA:
  - Find pending assignments matching `grant_source LIKE 'department:{UA}%'`
  - INSERT new assignments for all matching pending requests
  - Audit: 'reassigned_policy_change'
- [x] Handle user removed from UA:
  - Find user's pending assignments matching `grant_source`
  - Mark as 'revoked'
  - Check: step has other approvers? If orphaned → alert admin
  - Audit: 'revoked_policy_change'
- [x] Invalidate Redis scope cache for affected users
- [x] Integration test: role change → assignments updated within 10s

### Verification
- [x] All services build: `go build ./cmd/` for policy, drive, approval
- [x] End-to-end: create tenant → setup departments (with regional grouping) → create roles → assign users → create template → create transaction → 3-step approval flow → complete
- [x] Visibility: CEO sees all, Manager MN sees only MN, Manager MB sees only MB, NV sees only own dept
- [x] Performance: 50K assignments/tenant, all queries within SLA
- [x] Policy change: role change reconciles pending assignments
- [x] Audit: full trace from creation to completion (every action logged)
- [x] Schema isolation: tenant A cannot see tenant B data
- [x] Batch approve: 50 items in single transaction
