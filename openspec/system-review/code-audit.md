# Code Quality Audit — NGAC Platform

> Audited: 2026-05-01 | Scope: Backend (Go) + Frontend (TypeScript/React)

---

## 1. Backend Code Quality

### Test Coverage

| Service | Test Files | Test LOC | Coverage Areas |
|---------|-----------|----------|----------------|
| **Approval** | 5 | ~2,000 | domain (execution, matcher, queries), store (integration, perf), events |
| **Policy** | 3 | ~1,200 | gRPC scope, ngac store, bench |
| **Auth** | 2 | ~800 | gRPC server, store |
| **Asset** | 1 | ~400 | store |
| **Drive** | 1 | ~300 | gRPC server |
| **Document** | 1 | ~200 | gRPC server |
| **Messaging** | 1 | ~400 | gRPC server |
| **Workspace** | 1 | ~300 | gRPC server |
| **Total** | **17 files** | **~5,682 lines** | — |

**Assessment**: Approval and Policy services are well-tested. Other services have minimal coverage (gRPC handler tests only). No domain or store tests for messaging, drive, workspace.

### Error Handling Audit

#### Swallowed Errors (Critical)

```go
// Workspace gRPC server — 8 instances of swallowed errors:
ws, _ := s.GetWorkspace(ctx, ...)     // Lines 199, 214, 230, 250, 276, 308, 318, 337, 351
desc, _ := s.policyRead.GetDescendants(ctx, ...)
children, _ := s.policyRead.GetChildren(ctx, ...)
```

These are **direct architecture violations**: if `GetWorkspace` fails, subsequent operations run on nil data.

#### Correct Error Handling (Reference)

```go
// Drive gRPC server — proper error handling:
item, err := s.store.GetItem(ctx, req.ItemId)
if err != nil || item == nil {
    return nil, status.Errorf(codes.NotFound, "item not found")
}
```

### Error Wrapping

| Service | `fmt.Errorf("...: %w", err)` | `status.Errorf(codes.X, "...: %v", err)` | `return err` (bare) |
|---------|------------------------------|------------------------------------------|---------------------|
| Policy | ✅ Uses domain errors | Some | — |
| Messaging | ✅ domain errors + mapError | gRPC handler | — |
| Approval | ✅ domain errors | gRPC handler | — |
| Auth | ✅ domain errors | gRPC handler | — |
| Asset | ✅ domain errors | gRPC handler | — |
| Drive | — (no domain layer) | All in gRPC handler | — |
| Workspace | — (no domain layer) | All in gRPC handler | ❌ bare return in 1 place |
| Document | — (no domain layer) | All in gRPC handler | — |

### Naming Conventions

| Check | Status | Notes |
|-------|--------|-------|
| Package names lowercase | ✅ | All packages follow Go convention |
| Exported types documented | ⚠️ | Varies — approval/asset good, workspace/drive sparse |
| Error variables `ErrXxx` | ✅ | Where domain/errors.go exists |
| Constructor `New*` | ✅ | All services use NewXxxServer |
| Interface naming `-er` suffix | ⚠️ | Some use `Store` not `Storer` (acceptable in Go) |
| Receiver names | ✅ | Consistent `s` for server/service/store |

### SQL Safety

| Check | Status | Notes |
|-------|--------|-------|
| Parameterized queries | ✅ | All `$1, $2` style — no string interpolation |
| No SELECT * | ✅ | All queries list columns explicitly |
| No query in loop | ⚠️ | Workspace `ListWorkspaces` does N policy calls in loop |
| | ⚠️ | Drive `ListFolder` does N access checks in loop |
| LIMIT on list queries | ⚠️ | Missing in some list endpoints |

### Forbidden Pattern Check

| Pattern | Occurrences | Locations |
|---------|------------|-----------|
| ❌ `init()` functions | **0** | ✅ Clean |
| ❌ `log.Fatal` in lib | **0** | ✅ Clean (only in main.go) |
| ❌ `_ = someFunc()` | **8+** | 🔴 Workspace gRPC handler |
| ❌ ORM usage | **0** | ✅ Clean — raw pgx throughout |
| ❌ `SELECT *` | **0** | ✅ Clean |
| ❌ Global state | **0** | ✅ Clean — all constructor injection |
| ❌ Hardcoded NGAC strings | **0** | ✅ Clean — `ngac` package used everywhere |
| ❌ Proto types in store | **1** | 🟡 Workspace uses `pb.Workspace` in query scan |

---

## 2. Frontend Code Quality

### File Size Distribution

| Size Range | Count | Files |
|-----------|-------|-------|
| 500+ lines | 3 | channels.$channelId (503), approval (482), websocket.store (451) |
| 300-499 lines | 5 | useMessaging (434), DriveContextPanel (420), ChannelInfoPanel (388), drive (364), ChatList (330) |
| 200-299 lines | 5 | DriveSidebar (281), login (258), contacts (254), ... |
| 100-199 lines | ~15 | Various components |
| <100 lines | ~100+ | Most components, hooks, stores |

### Component Composition

| Check | Status | Notes |
|-------|--------|-------|
| Primitives use M3 tokens | ✅ | All 10 primitives consistent |
| Composites compose primitives | ⚠️ | 7/10 compose, 3 use raw elements (Tabs, PeekPanel, DataTable) |
| Patterns compose composites | ⚠️ | Some bypass (raw buttons, inline styling) |
| Routes compose patterns | ⚠️ | 4 routes have excessive inline logic |

### Hook Organization

| Hook | Lines | Concerns | Assessment |
|------|-------|----------|-----------|
| `useMessaging` | 434 | channels, messages, members, polls, tasks, typing, reactions | 🔴 God hook — should split |
| `useApproval` | 195 | templates, requests | ✅ Reasonable |
| `useAssets` | 172 | types, instances, requests | ✅ Reasonable |
| `useDrive` | 163 | folders, files, breadcrumbs | ✅ Reasonable |
| `useContacts` | 58 | contacts list | ✅ Minimal |
| `useAuth` | 29 | login, register | ✅ Minimal |
| `useWorkspaces` | 27 | list, create | ✅ Minimal |
| `usePermissions` | 66 | NGAC access checks | ✅ Reasonable |
| `useResizable` | 91 | panel resizing | ✅ Utility |
| `useDocuments` | 61 | document links | ✅ Minimal |
| `useNotifications` | 35 | notifications | ✅ Minimal |

### TypeScript Quality

| Check | Status | Notes |
|-------|--------|-------|
| `any` usage | ✅ Minimal | Proto generated types use some `any` |
| Interface definitions | ✅ | All component props typed |
| Enum usage | ⚠️ | Some string literals instead of enums (status values) |
| Null safety | ⚠️ | Optional chaining used but no strict null checks in config |
| Import paths | ✅ | Relative, consistent |

### Performance Patterns

| Check | Status | Issue |
|-------|--------|-------|
| Memoization | ⚠️ | `useMemo`/`useCallback` used inconsistently |
| Re-render prevention | ⚠️ | Large route components re-render entirely on any state change |
| Bundle size | ✅ | Code splitting via TanStack Router lazy routes |
| Image optimization | ⚠️ | No lazy loading for images in drive |
| WebSocket cleanup | ✅ | Cleanup on unmount |

---

## 3. Cross-Cutting Quality Issues

### Documentation

| Area | Status | Notes |
|------|--------|-------|
| Backend Go doc comments | ⚠️ | Inconsistent — approval/asset well-documented, workspace/drive sparse |
| Frontend JSDoc | ⚠️ | Primitives have comments, patterns mostly don't |
| API documentation | ❌ | No OpenAPI/Swagger docs |
| Proto documentation | ⚠️ | Some proto files have comments, some don't |
| README | ❌ | No project README |

### Security

| Check | Status | Notes |
|-------|--------|-------|
| JWT auth on all endpoints | ✅ | Middleware applied in every service |
| Input validation | ⚠️ | Basic checks in handlers, no schema validation |
| CORS | ✅ | Handled by Traefik |
| SQL injection | ✅ | Parameterized queries everywhere |
| XSS | ⚠️ | React handles by default, but no CSP headers |
| Rate limiting | ❌ | No rate limiting on any endpoint |
| Secrets management | ✅ | Environment variables, `envOr()` pattern |

### Dependency Health

| Backend | Frontend |
|---------|----------|
| Go 1.22+ | Vite 5 |
| pgx v5 | React 19 |
| Echo v4 | TanStack Router 1.x |
| gRPC 1.x | TanStack Query 5 |
| MinIO Client v7 | Zustand 5 |

---

## 4. Quality Score Summary

| Category | Backend | Frontend | Notes |
|----------|---------|----------|-------|
| Architecture compliance | 🟡 **65%** | 🟡 **60%** | Workspace/Drive violate clean arch; route files too large |
| Test coverage | 🟡 **50%** | 🔴 **0%** | No frontend tests. Backend tests concentrated in 3 services |
| Error handling | 🟡 **70%** | 🟡 **60%** | Workspace swallows errors; frontend lacks error boundaries |
| Code organization | 🟢 **80%** | 🟡 **55%** | Backend mostly clean; frontend patterns/ is unorganized |
| Security | 🟢 **80%** | 🟡 **65%** | No rate limiting, no CSP |
| Documentation | 🟡 **40%** | 🔴 **30%** | No README, no API docs |
| Performance | 🟡 **60%** | 🟡 **50%** | N+1 queries; no pagination; no memoization |
| Design system compliance | N/A | 🟡 **65%** | Primitives ✅, composites mixed, patterns violate |

### Overall System Quality: **60% — Functional but inconsistent**
