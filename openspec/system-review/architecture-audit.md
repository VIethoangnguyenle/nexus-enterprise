# Architecture Audit — NGAC Platform

> Audited: 2026-05-01 | Scope: All 8 backend services

---

## 1. Clean Architecture Compliance

### Per-Service Scorecard

| Service | cmd/ | rest/ | grpc/ | domain/ | store/ | models.go | errors.go | Overall |
|---------|------|-------|-------|---------|--------|-----------|-----------|---------|
| **policy** | ✅ | ✅ | ✅ (3 files) | ngac/ (custom) | ngac/store | ✅ | N/A | 🟢 **A** |
| **messaging** | ✅ | ✅ (3 files) | ✅ (4 files) | ✅ | ✅ | ✅ | ✅ | 🟢 **A** |
| **auth** | ✅ | ✅ | ✅ | ✅ | ✅ | N/A | ✅ | 🟢 **A** |
| **approval** | ✅ | ✅ | ✅ | ✅ (5 files) | ✅ | ✅ | ✅ | 🟢 **A** |
| **asset** | ✅ | ✅ | ✅ (3 files) | ✅ (4 files) | ✅ | N/A | ✅ | 🟢 **A** |
| **drive** | ✅ | ✅ | ✅ (2 files) | ❌ missing | ✅ | ✅ | ✅ | 🟡 **B** |
| **workspace** | ✅ | ✅ | ✅ | ⚠️ errors only | ❌ missing | N/A | ✅ | 🔴 **D** |
| **document** | ✅ | ✅ | ✅ | ❌ missing | ❌ missing | N/A | N/A | 🔴 **F** |

### Key Violations

#### 🔴 Workspace Service — Monolithic gRPC Handler (388 lines)

**All business logic lives in `grpc/server.go`**:
- Direct `s.db.Query/Exec` calls (should be in store/)
- Proto types used as DB models (should have internal models)
- Policy calls mixed with business logic (should be in domain/)
- Error handling uses `status.Errorf` throughout (correct for handler, but logic should be in domain)
- 17 methods, all doing SQL + Policy + business logic in one place

```
ACTUAL:   cmd/ → grpc/ (SQL + Policy + Logic)
REQUIRED: cmd/ → grpc/ → domain/ → store/
```

#### 🔴 Document Service — Skeleton Only

- `grpc/server.go`: 149 lines (proxy to MinIO)
- No domain layer
- No store layer
- Acts as a thin proxy, not a service

#### 🟡 Drive Service — Missing Domain Layer

`grpc/server.go` is **645 lines** — largest handler in the system. It:
- Uses `store.Store` correctly ✅
- Has `checkAccess()` helper ✅
- But contains business logic (ensureRoot, quota checks, NGAC graph traversal) directly in handler methods ❌
- `itemToProto()` conversion in handler file ❌ (should be in domain)
- `contains()` and `nilStr()` utility functions in handler ❌

---

## 2. NGAC Permission Flow

### Correct Implementations

| Service | Pattern | Assessment |
|---------|---------|-----------|
| **Drive** | `checkAccess(ctx, userNodeID, objectNodeID, op)` before every mutation | ✅ Consistent |
| **Drive** | ListFolder filters items by NGAC read access | ✅ Correct |
| **Messaging** | Channel access via NGAC node checks | ✅ Correct |
| **Approval** | Template/request operations check workspace membership | ✅ Correct |
| **Asset** | Asset type and request operations use policy checks | ✅ Correct |

### Issues

| Service | Issue | Severity |
|---------|-------|----------|
| **Workspace** | `ListWorkspaces` queries ALL workspaces then filters in Go loop | ⚠️ Performance — should filter at DB or policy level |
| **Workspace** | Error handling uses `_ , err :=` ignoring first return | ⚠️ Swallowed errors: `ws, _ := s.GetWorkspace(...)` (8 occurrences) |
| **Workspace** | `TransferOwnership` uses `ws.Name` for NGAC lookup but `CreateWorkspace` uses `wsID` | 🔴 **BUG**: Owner lookup uses wrong key — will fail for renamed workspaces |
| **Drive** | `ListFolder` does N+1 policy checks (one per item) | ⚠️ Performance — should batch |

---

## 3. Service Boundary Analysis

### Data Flow

```
Frontend (Vite + TanStack Router)
     │
     │ HTTP REST
     ▼
Traefik (path routing)
     │
     ├─ /api/auth/*    → Auth Service    :8180
     ├─ /api/ws/*      → Workspace Svc   :8181
     ├─ /api/doc/*     → Document Svc    :8182
     ├─ /api/msg/*     → Messaging Svc   :8183
     ├─ /api/asset/*   → Asset Svc       :8184
     ├─ /api/drive/*   → Drive Svc       :8185
     └─ /api/approval/*→ Approval Svc    :8186
```

### Inter-Service Dependencies (gRPC)

```
                    ┌────────────────┐
                    │  Policy Svc    │  ← EVERYONE depends on this
                    │  (PDP)         │
                    └───────▲────────┘
                            │ gRPC
     ┌──────────┬──────────┬┼──────────┬──────────┐
     │          │          ││          │          │
     │          │          ││          │          │
   Auth    Workspace    Drive     Messaging    Asset
                │          │                     │
                │          │                     │
                │     ┌────▼────┐                │
                │     │Document │                │
                │     │(MinIO   │                │
                └────►│ proxy)  │                │
                      └─────────┘                │
                                                 │
                                          ┌──────▼──────┐
                                          │  Approval   │
                                          │  (via events)│
                                          └─────────────┘
```

### Integration Point Issues

| Integration | Mechanism | Issue |
|-------------|-----------|-------|
| Workspace → Drive | gRPC call in CreateWorkspace | ✅ Correct (creates root drive) |
| Drive → Document | gRPC for presigned URLs | ✅ Correct |
| Asset → Approval | Redpanda events | ✅ Correct (event-driven) |
| Frontend → WS | WebSocket (messaging:8183) | ⚠️ Store is 15KB monolith |
| Workspace → Policy | gRPC (read + write) | ⚠️ N+1 calls in ListWorkspaces |

---

## 4. REST Handler Size Analysis

| Service | handler.go Lines | Methods (est.) | Avg Lines/Method | Assessment |
|---------|-----------------|----------------|-------------------|-----------|
| Drive | 488 | ~18 | ~27 | ⚠️ Some exceed 20-line rule |
| Approval | 452 | ~15 | ~30 | ⚠️ Exceeds 20-line rule |
| Auth | 392 | ~12 | ~33 | ⚠️ Login/register are complex |
| Asset | 381 | ~14 | ~27 | ⚠️ Borderline |
| Messaging | 358 + 2 extra files | ~20 | ~18 | ✅ Split correctly |
| Workspace | 237 | ~8 | ~30 | ⚠️ Should delegate to domain |
| Document | 143 | ~5 | ~29 | OK (simple proxy) |

### gRPC Handler Analysis

| Service | server.go Lines | Assessment |
|---------|----------------|-----------|
| **Drive** | **645** | 🔴 Monolithic — contains domain logic |
| **Workspace** | **387** | 🔴 Monolithic — contains SQL + domain logic |
| **Policy** | 324 + 2 files | ✅ Split into read/write servers |
| **Auth** | 201 | ✅ Reasonable |
| **Approval** | 175 | ✅ Clean |
| **Messaging** | 165 + hub + notifications | ✅ Well split |
| **Document** | 149 | ✅ Simple proxy |

---

## 5. Summary

### Critical Architecture Issues

1. **Workspace service has no store/domain layers** — all SQL and logic in gRPC handler (387 lines)
2. **Drive gRPC handler is 645 lines** — domain logic mixed into handler
3. **Workspace TransferOwnership bug** — uses `ws.Name` instead of `ws.Id` for NGAC lookup
4. **Document service is skeleton** — no domain model, just proxies MinIO

### Performance Concerns

5. **ListWorkspaces** queries all workspaces and filters in Go loop
6. **Drive ListFolder** does N+1 policy checks per item
7. **Workspace swallows errors** — 8 instances of `_, err :=` ignoring results

### Compliant Services

- **Messaging**: Best-structured service. 3 REST files, 4 gRPC files, clean domain/store split
- **Approval**: Most tested service. Multiple test files, benchmark tests, proper domain models
- **Auth**: Solid clean architecture with OTP module
- **Asset**: Clean domain layer with lifecycle, schema, matcher modules
- **Policy**: Appropriate custom structure for NGAC engine (ngac/ replaces domain/)
