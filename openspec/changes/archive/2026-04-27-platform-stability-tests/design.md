# Design: Platform Stability Tests

## Chiến lược test

```
┌──────────────────────────────────────────────────────────────┐
│                       Test Pyramid                           │
│                                                              │
│                    ┌──────────┐                              │
│                    │ test_app │  59 tests (existing)          │
│                    │  .sh     │  HTTP integration             │
│                    └────┬─────┘                               │
│               ┌─────────┴──────────┐                         │
│               │  Go Unit Tests     │  NEW: ~80 tests          │
│               │  per service       │  Store + gRPC handler    │
│               └─────────┬──────────┘                         │
│          ┌──────────────┴───────────────┐                    │
│          │   Frontend Component Tests   │  NEW: ~20 tests     │
│          │   Vitest + Testing Library   │  Modal + API layer  │
│          └──────────────────────────────┘                    │
└──────────────────────────────────────────────────────────────┘
```

## Bug Fixes

### Fix 1: Messaging NULL scan (Critical)

**File:** `backend/services/messaging/internal/grpc/server.go`

2 queries cần thêm `COALESCE`:

```diff
# ListChannels (line 123)
- SELECT id, name, channel_type, workspace_id, ngac_oa_id, ...
+ SELECT id, name, channel_type, COALESCE(workspace_id,''), COALESCE(ngac_oa_id,''), COALESCE(ngac_ua_id,''), COALESCE(created_by,''), ...

# GetChannel (line 153)
- SELECT id, name, channel_type, workspace_id, ngac_oa_id, ...
+ SELECT id, name, channel_type, COALESCE(workspace_id,''), COALESCE(ngac_oa_id,''), COALESCE(ngac_ua_id,''), COALESCE(created_by,''), ...
```

### Fix 2: Frontend wsId guard

**File:** `frontend/src/components/CreateChannelModal.tsx`

```diff
+ if (!wsId) {
+   // Disable create button, show error
+   return
+ }
```

### Fix 3: Clean orphan channels

```sql
DELETE FROM channels WHERE channel_type = 'workspace' AND workspace_id IS NULL;
```

## Go Test Architecture

### Per-service test structure

```
backend/services/{service}/
├── internal/
│   ├── grpc/
│   │   ├── server.go
│   │   └── server_test.go      ← gRPC handler tests (mock store/clients)
│   ├── store/
│   │   ├── store.go
│   │   └── store_test.go       ← DB integration tests (real pgx, test DB)
│   └── domain/
│       ├── domain.go
│       └── domain_test.go      ← Pure logic tests (no deps)
```

### Test approach per service

| Service | Store Tests | Handler Tests | Focus Areas |
|---------|------------|---------------|-------------|
| **messaging** | Channel CRUD, message pagination, thread queries | NULL column handling, access checks, DM creation | Nullable columns, COALESCE consistency |
| **asset** | Type CRUD, asset lifecycle, request flow | State transitions, concurrent assignment | `*string` nullable fields, filter combos |
| **document** | Upload/download, presigned URLs | Access control delegation | Status transitions |
| **auth** | User CRUD, token validation | Login/register, JWT generation | Password hashing, token expiry |
| **workspace** | Workspace CRUD, member management | NGAC node creation, invite flow | Policy integration |
| **policy** | Graph load, node CRUD | Access check algorithm | Graph traversal correctness |
| **gateway** | — (no store) | Route matching, auth middleware, request parsing | Context value extraction, error responses |

### Test DB strategy

- Use `testcontainers-go` for PostgreSQL — each test suite gets an isolated DB
- Alternative: use the existing Docker PostgreSQL with a test schema
- Decision: **testcontainers** for CI isolation, **existing Docker DB** for local dev speed

### Mock strategy

```go
// Interface-based mocking for gRPC clients
type PolicyChecker interface {
    CheckAccess(ctx context.Context, req *policypb.CheckAccessRequest) (*policypb.CheckAccessResponse, error)
}

// testify/mock implementation
type MockPolicyChecker struct { mock.Mock }
func (m *MockPolicyChecker) CheckAccess(ctx context.Context, req *policypb.CheckAccessRequest) (*policypb.CheckAccessResponse, error) {
    args := m.Called(ctx, req)
    return args.Get(0).(*policypb.CheckAccessResponse), args.Error(1)
}
```

## Frontend Test Architecture

### Setup: Vitest + React Testing Library

```
frontend/
├── vitest.config.ts              ← Test config
├── src/
│   ├── test/
│   │   └── setup.ts              ← Global test setup (jsdom, matchers)
│   ├── components/
│   │   └── __tests__/
│   │       ├── CreateChannelModal.test.tsx
│   │       └── Sidebar.test.tsx
│   └── api/
│       └── __tests__/
│           └── messaging.test.ts
```

### Test coverage targets

| Component | Tests | Focus |
|-----------|-------|-------|
| `CreateChannelModal` | renders, validates empty name, submits with correct payload, shows error, disabled when no workspace | Payload correctness |
| `Sidebar` | renders sections, collapse toggle, channel list, asset link | Layout states |
| `messaging.ts` API | correct URL construction, error handling | API contract |

## Dependencies mới

### Backend
- `github.com/stretchr/testify` — assertions + mocking
- `github.com/testcontainers/testcontainers-go` — isolated test DBs (optional)

### Frontend
- `vitest` — test runner
- `@testing-library/react` — component testing
- `@testing-library/jest-dom` — DOM matchers
- `jsdom` — browser environment

## Rule: Test-First Development

Từ change này trở đi:
1. Viết test case trước (expect failure)
2. Implement fix/feature
3. Test pass
4. Mark task done

Không có ngoại lệ.
