## Tasks

### Fix 1: NGAC fallback lookup (P0)
- [x] Add name-based fallback in `assignChannelToWorkspace` (`domain/service.go:129`)
- [x] Verify `go build ./cmd/` passes

### Fix 2: Channel type normalization (P1)
- [x] Normalize `"group"` → `"workspace"` in `CreateChannel` before DB insert
- [x] Verify `go build ./cmd/` passes

### Fix 3: Error mapping (P2)
- [x] Create `domain/errors.go` with `ErrAccessDenied` sentinel (already existed)
- [x] Update `checkAccess` to wrap `ErrAccessDenied`
- [x] Verify REST `MapDomainError` handles `ErrAccessDenied` → 403 (already handled in `pkg/httputil`)
- [x] Verify `go build ./cmd/` passes

### Verification
- [x] API test: Create channel → GET messages → 200
- [x] API test: Create channel with `channel_type: "group"` → 200
- [ ] API test: Unauthorized access → 403 (not 500) — 2nd user unavailable, manual verify
- [x] Regression: Existing "Test" channel still works
