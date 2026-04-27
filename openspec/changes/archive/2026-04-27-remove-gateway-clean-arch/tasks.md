# Tasks — Remove Gateway, REST per Service

## Phase 0: Shared Infrastructure
> Create shared packages used by all services

- [x] **T0.1** Create `backend/pkg/httputil/jwt.go` — Echo JWT middleware (extract from gateway `authMiddleware`)
- [x] **T0.2** Create `backend/pkg/httputil/errors.go` — `MapDomainError` function (domain error → Echo HTTP error)
- [x] **T0.3** Create `backend/pkg/httputil/claims.go` — `Claims` struct + `GetClaims(echo.Context)` helper
- [x] **T0.4** Add `echo` dependency: `go get github.com/labstack/echo/v4`
- [x] **T0.5** Verify: `go build ./pkg/httputil/` passes

---

## Phase 1: Auth Service (P1 — public routes, simplest)
> Auth has store + gRPC handler already. Add domain + REST.

- [x] **T1.1** Create `auth/internal/domain/errors.go` — `ErrInvalidCredentials`, `ErrUserExists`
- [x] **T1.2** Create `auth/internal/domain/service.go` — extract Register/Login/ListUsers logic from gRPC handler
- [x] **T1.3** Refactor `auth/internal/grpc/server.go` to delegate to domain service
- [x] **T1.4** Create `auth/internal/rest/handler.go` — REST handlers for `/api/auth/*` and `/api/users`
- [x] **T1.5** Update `auth/cmd/main.go` — start REST server (:8080) alongside gRPC (:50052)
- [x] **T1.6** Add Traefik labels to `docker-compose.yml` for auth service
- [x] **T1.7** Verify: register + login via REST directly ✅ smoke test passed

---

## Phase 2: Messaging Service (P1 — most traffic)
> Messaging already has store + domain. Add REST.

- [x] **T2.1** Create `messaging/internal/domain/errors.go` — sentinel errors
- [x] **T2.2** Create `messaging/internal/rest/handler.go` — REST handlers for channels, messages, DMs, threads, notifications
- [x] **T2.3** Update `messaging/cmd/main.go` — add REST server
- [x] **T2.4** Add Traefik labels for messaging paths
- [x] **T2.5** Verify: create channel + send message via REST directly ✅ smoke test passed

---

## Phase 3: Drive Service (P2 — needs domain layer)
> Drive has store but NO domain. Handler is 619 lines with nesting depth 6.

- [x] **T3.1** Create `drive/internal/domain/errors.go` — sentinel errors
- [ ] **T3.2** Create `drive/internal/domain/service.go` — extract business logic from `grpc/server.go`:
  - `CreateFolder`, `ListFolder`, `GetItem`, `MoveItem`, `CopyItem`, `RenameItem`
  - `TrashItem`, `RestoreItem`, `DeleteItem`
  - `CreateFile`, `ConfirmFile`, `GetDownloadURL`
  - `ensureRoot` → `EnsureRootFolder` (refactor nesting depth 6 → max 3)
- [ ] **T3.3** Create `drive/internal/domain/sharing.go` — extract sharing logic from `grpc/sharing.go`:
  - `CreateShare`, `RevokeShare`, `ListShares`, `SharedWithMe`
- [ ] **T3.4** Refactor `drive/internal/grpc/server.go` to thin handler (delegate to domain)
- [ ] **T3.5** Refactor `drive/internal/grpc/sharing.go` to thin handler
- [x] **T3.6** Create `drive/internal/rest/handler.go` — REST handlers (19 endpoints, delegates to gRPC server transitionally)
- [x] **T3.7** Update `drive/cmd/main.go` — add REST server
- [x] **T3.8** Add Traefik labels for drive paths
- [x] **T3.9** Verify: create folder + upload file via REST directly ✅ smoke test passed

---

## Phase 4: Workspace Service (P2 — needs store + domain)
> Workspace has NO store, NO domain. Everything is in gRPC handler (385 lines).

- [ ] **T4.1** Create `workspace/internal/store/models.go` — Workspace, Member DB models
- [ ] **T4.2** Create `workspace/internal/store/store.go` — extract all SQL from handler:
  - `InsertWorkspace`, `GetWorkspace`, `ListByUser`, `InsertMember`, `DeleteMember`, `ListMembers`
- [x] **T4.3** Create `workspace/internal/domain/errors.go` — sentinel errors
- [ ] **T4.4** Create `workspace/internal/domain/service.go` — extract logic from handler
- [ ] **T4.5** Refactor `workspace/internal/grpc/server.go` to thin handler (delegate to domain)
- [x] **T4.6** Create `workspace/internal/rest/handler.go` — REST handlers (10 endpoints, delegates to gRPC server transitionally)
- [x] **T4.7** Update `workspace/cmd/main.go` — add REST server
- [x] **T4.8** Add Traefik labels for workspace paths
- [x] **T4.9** Verify: create workspace + invite member via REST directly ✅ smoke test passed

---

## Phase 5: Asset Service (P3)
> Asset has store + partial domain. Needs REST.

- [x] **T5.1** Create `asset/internal/domain/errors.go` — sentinel errors
- [ ] **T5.2** Complete `asset/internal/domain/service.go` — consolidate logic from 3 gRPC handler files
- [ ] **T5.3** Refactor gRPC handlers to thin (delegate to domain)
- [x] **T5.4** Create `asset/internal/rest/handler.go` — REST handlers (16 endpoints, delegates to gRPC servers transitionally)
- [x] **T5.5** Update `asset/cmd/main.go` — add REST server
- [x] **T5.6** Add Traefik labels for asset paths
- [x] **T5.7** Verify: create asset type + asset via REST directly ✅ smoke test passed

---

## Phase 6: Document Service (P3 — legacy)
> Document is being replaced by Drive but still used.

- [x] **T6.1** Create `document/internal/rest/handler.go` — REST handlers (8 endpoints: 4 active proxying to Drive, 4 deprecated 410 Gone)
- [x] **T6.2** Update `document/cmd/main.go` — add REST server with Drive client
- [x] **T6.3** Add Traefik labels for document paths
- [x] **T6.4** Verify: document endpoints work via REST directly ✅ 410 Gone + Drive proxy confirmed

---

## Phase 7: Remove Gateway + Update Infrastructure
> Only after ALL services have working REST endpoints.

- [x] **T7.1** Parallel test: run all services with REST, verify all endpoints match gateway behavior
- [x] **T7.2** Remove gateway service from `docker-compose.yml`
- [x] **T7.3** Delete `backend/services/gateway/` directory
- [x] **T7.4** Move CORS middleware definition from gateway labels to frontend labels
- [x] **T7.5** Update frontend if any URLs need adjustment (none needed — all use relative `/api/` paths)
- [x] **T7.6** Full smoke test: register → login → workspace → channel → message → drive → asset ✅ ALL PASS

---

## Phase 8: Update Documentation + Rules
> Update architectural docs to reflect new reality.

- [x] **T8.1** Update `AGENTS.md` — architecture diagram now shows all 6 services with Document
- [x] **T8.2** Update `.agent/instructions.md` — already reflects REST handler rules (no change needed)
- [x] **T8.3** Update `README.md` — empty file, no content to update
