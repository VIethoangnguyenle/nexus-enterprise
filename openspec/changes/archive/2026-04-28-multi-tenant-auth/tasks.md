## 1. Database Migration

- [x] 1.1 Create `data/migrations/005_multi_tenant_auth.sql` — add `email`, `union_id`, `display_name` to users table
- [x] 1.2 Add `domain` column to workspaces table
- [x] 1.3 Create `tenant_users` table with composite PK `(tenant_id, user_id)`, `open_id`, `role`, `status`, `ngac_node_id`
- [x] 1.4 Add backfill query: insert `tenant_users` from existing `workspaces.owner_id`
- [x] 1.5 Run migration on dev database and verify

## 2. NGAC Constants

- [x] 2.1 Add `TenantMemberUAName(tenantID)` and `TenantOwnerUAName(tenantID)` to `backend/ngac/ngac_ops.go`

## 3. Proto Changes

- [x] 3.1 Add `Signup`, `Signin`, `SwitchTenant`, `GetMe`, `ListUserTenants` RPCs to `backend/proto/auth/auth.proto`
- [x] 3.2 Add `SignupRequest/Response`, `SigninRequest/Response`, `SwitchTenantRequest/Response`, `TenantInfo`, `MeResponse` messages
- [x] 3.3 Update `UserInfo` message with `email`, `union_id`, `display_name` fields
- [x] 3.4 Run `make proto` to regenerate Go code

## 4. Auth Store Layer

- [x] 4.1 Add `email`, `union_id`, `display_name` fields to `store.User` model
- [x] 4.2 Add `TenantMembership` model
- [x] 4.3 Implement `GetUserByEmail(ctx, email)` query
- [x] 4.4 Implement `InsertTenantUser(ctx, tenantID, userID, role, status, ngacNodeID)` query
- [x] 4.5 Implement `ListTenantsByUser(ctx, userID)` query
- [x] 4.6 Implement `GetTenantUser(ctx, tenantID, userID)` query
- [x] 4.7 Implement `FindTenantByDomain(ctx, domain)` query
- [x] 4.8 Update `CreateUser` to include email, union_id, display_name

## 5. JWT & Claims

- [x] 5.1 Add `TenantID` and `SessionID` to `pkg/httputil/claims.go`
- [x] 5.2 Update `auth/jwt.go` — `GenerateToken` accepts `tenantID` parameter
- [x] 5.3 Update `ValidateToken` to parse new claims (backward-compatible)

## 6. Auth Domain Layer

- [x] 6.1 Add `ErrAccessDenied`, `ErrTenantNotFound` sentinel errors to `domain/errors.go`
- [x] 6.2 Implement `Signup(ctx, email, password, displayName, tenantName)` — email domain matching, tenant creation, NGAC init
- [x] 6.3 Implement `Signin(ctx, email, password)` — authenticate, list tenants, issue tenant-scoped JWT
- [x] 6.4 Implement `SwitchTenant(ctx, userID, tenantID)` — verify membership, re-issue JWT
- [x] 6.5 Implement `GetMe(ctx, userID, tenantID)` — return user + tenant info
- [x] 6.6 Implement `initTenantNGAC(ctx, tenantID, userNodeID)` — create TenantOwner/TenantMember UAs, associations
- [x] 6.7 Implement `assignUserToTenant(ctx, tenantID, userNodeID, isOwner)` — NGAC assignment

## 7. Handlers

- [x] 7.1 Add REST routes: `POST /api/auth/signup`, `POST /api/auth/signin`, `POST /api/auth/switch-tenant`, `GET /api/me`
- [x] 7.2 Add gRPC handlers for new RPCs
- [x] 7.3 Keep legacy `POST /api/auth/register` and `POST /api/auth/login` as backward-compatible wrappers

## 8. Verification

- [x] 8.1 `go build ./cmd/` for auth service passes
- [x] 8.2 Manual test: signup with new email → tenant created → JWT contains tenant_id
- [x] 8.3 Manual test: signin → tenant list returned → switch-tenant → new JWT
- [x] 8.4 Verify legacy register/login still works (backward compatibility)
- [x] 8.5 Verify downstream services (messaging, workspace) accept new JWT format
