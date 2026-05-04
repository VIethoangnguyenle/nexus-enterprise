# Tasks: Generic Policy Service

## Phase A: Proto changes
- [x] A1: Add Prohibition messages to policy.proto
- [x] A2: Add new RPCs to policy_write.proto
- [x] A3: Add new RPCs to policy_read.proto

## Phase B: Domain layer — Remove coupling
- [x] B1: Remove Op* constants from models.go, add Prohibition model
- [x] B2: Delete constraints.go
- [x] B3: Remove constraint references from server.go + read_server.go
- [x] B4: Remove constraint wiring from cmd/main.go + cmd/policy-read/main.go

## Phase C: Domain layer — New features
- [x] C1: Create operations.go (dynamic operations store)
- [x] C2: Create prohibition.go (prohibition store + in-memory matching)
- [ ] C3: Modify access.go — add prohibition check after BFS ALLOW (TODO: ReadServer integration)
- [x] C4: Add prohibition + operations schema to store.go InitSchema

## Phase D: gRPC handlers
- [x] D1: Add RegisterOperations + InvalidateCache to write_server.go
- [x] D2: Add CreateProhibition + RemoveProhibition to write_server.go
- [x] D3: Add ListOperations + ListProhibitions to read_server.go

## Phase E: Infrastructure
- [ ] E1: Add prohibition events to producer.go (using existing PublishGraphMutated)
- [x] E2: Wire new components in cmd/main.go + cmd/policy-read/main.go

## Phase F: Migrations + Docs
- [x] F1: Create self-contained migrations/
- [ ] F2: Create README.md integration guide

## Phase G: Verification
- [x] G1: Remove constraint refs — no remnants found
- [x] G2: Build verification — `go build ./...` passes
- [x] G3: `go vet ./...` passes
- [x] G4: Existing tests pass — `go test ./internal/ngac/...` OK
