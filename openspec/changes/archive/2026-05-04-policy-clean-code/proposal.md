# Policy Service — Clean Code Refactoring

## Summary

Rà soát toàn bộ Policy service (4422 LOC, 20 files), phát hiện **12 code smells** (3 HIGH, 5 MEDIUM, 4 LOW). Refactoring tập trung vào:

1. Fix bug tiềm ẩn trong `Store.CreateAssignment` (add-remove-add pattern)
2. Eliminate duplicated code trong prohibition invalidation
3. Tách hard dependencies (Redis concrete type → interface)
4. Giảm nested ifs, sử dụng early return + helper extraction
5. Replace fragile string-based enum checks với typed constants
6. Remove dead code và fix misleading comments

## Motivation

- **store.go CreateAssignment** chứa logic Add→Remove→Add có thể gây inconsistency khi DB fail
- **write_server.go** có 15 dòng copy-paste prohibition invalidation
- **read_server.go** giữ `*redis.Client` trực tiếp thay vì interface
- **decision_engine.go** dùng string matching thay vì typed sentinel — fragile khi sửa message
- `GetUserByNGACNodeID` query bảng `users` (auth service boundary violation)

## Scope

### In-scope
- `internal/ngac/` — store, decision_engine, cache_invalidator, models
- `internal/grpc/` — read_server, write_server  
- `cmd/main.go` — wiring cleanup

### Out-of-scope
- Proto definitions (không đổi API contract)
- Test files (chỉ verify pass, không refactor)
- CTE SQL functions
- Events producer

## Non-goals
- Thay đổi behavior hay API
- Performance optimization
- Thêm feature mới

## Risk Assessment
- **Risk**: LOW — pure refactoring, no behavior change
- **Breaking**: NONE — internal restructuring only
- **Validation**: All 51 existing tests must pass after refactoring
