## Phase 1: Algorithm Optimization

### 1.1 Graph Structure Enhancement
- [x] 1.1.1 Add `nameTypeIndex map[string]*NGACNode` to `Graph` struct in `graph.go`
- [x] 1.1.2 Add `nameTypeKey(name, nodeType string)` helper in `models.go`
- [x] 1.1.3 Update `NewGraph()` to initialize `nameTypeIndex`
- [x] 1.1.4 Update `AddNode()` to maintain `nameTypeIndex`
- [x] 1.1.5 Update `RemoveNode()` to clean up `nameTypeIndex`
- [x] 1.1.6 Rewrite `FindNodeByName()` to use index — O(1) instead of O(N)

### 1.2 BFS Traversal Methods
- [x] 1.2.1 Add `bfsCollectAttributesAndPCs(startID, attrType string) (attrs, pcs map[string]bool)` — single-pass iterative BFS that collects both attributes and PCs in one traversal
- [x] 1.2.2 Rewrite `GetAncestors()` to use iterative BFS queue instead of recursive DFS
- [x] 1.2.3 Rewrite `GetDescendants()` to use iterative BFS queue instead of recursive DFS
- [x] 1.2.4 Remove deprecated recursive helpers: `getAncestorsRecursive`, `getDescendantsRecursive`, `collectAncestorsOfType`, `collectPCsReachable`, `getAncestorPathsRecursive`

### 1.3 CheckAccess Rewrite
- [x] 1.3.1 Rewrite `CheckAccess()` in `access.go`: 2×BFS instead of 4×DFS, single-pass collection
- [x] 1.3.2 Add `findMatchingAssociation()` helper with early termination
- [x] 1.3.3 Remove `PC_Global` hardcoded string check — let PC intersection handle it naturally
- [x] 1.3.4 Preserve `AccessExplanation` output format for backward compatibility
- [x] 1.3.5 Keep `containsOp()` utility (still needed)

### 1.4 Tests
- [x] 1.4.1 Update `TestCheckAccess_Allow` — verify same result with new algorithm
- [x] 1.4.2 Update `TestCheckAccess_DenyNoAssociation` — verify same result
- [x] 1.4.3 Update `TestCheckAccess_DenyWrongOperation` — verify same result
- [x] 1.4.4 Update `TestCheckAccess_NodeNotFound` — verify same result
- [x] 1.4.5 Update `TestCheckAccess_MultiPC_InheritedPermissions` — verify same result
- [x] 1.4.6 Add `TestBfsCollectAttributesAndPCs` — verify single-pass collects both attrs and PCs
- [x] 1.4.7 Add `TestFindNodeByName_IndexedLookup` — verify O(1) lookup
- [x] 1.4.8 Add `TestCheckAccess_CrossWorkspaceShare` — verify PC_Global handled via intersection
- [x] 1.4.9 Build verification: `cd backend/services/policy && go build ./cmd/`
- [x] 1.4.10 Run all tests: `cd backend/services/policy && go test ./...`

---

## Phase 2: CQRS + DB-Driven Evaluation

### 2.1 Database Schema
- [x] 2.1.1 Create migration `data/migrations/003_materialized_access.sql` with `ngac_materialized_access` table
- [x] 2.1.2 Add `ngac_graph_version` table to migration
- [x] 2.1.3 Add optimized indexes for recursive CTE: composite index on `(child_id, parent_id)` for assignments
- [x] 2.1.4 Add `ngac_ancestors()` SQL function (recursive CTE)
- [x] 2.1.5 Add `ngac_check_access()` SQL function (full access decision in SQL)

### 2.2 Proto Split
- [x] 2.2.1 Create `proto/policy/policy_read.proto` with read-only RPCs
- [x] 2.2.2 Create `proto/policy/policy_write.proto` with write-only RPCs
- [x] 2.2.3 Run `cd backend && make proto` to generate Go code
- [x] 2.2.4 Keep `policy.proto` as deprecated alias (backward compat during migration)

### 2.3 Read Service Implementation
- [x] 2.3.1 Create `services/policy/internal/ngac/cte.go` — SQL CTE evaluation functions
- [x] 2.3.2 Create `services/policy/internal/ngac/materialized.go` — materialized access CRUD
- [x] 2.3.3 Create `services/policy/internal/ngac/version.go` — graph version tracking
- [x] 2.3.4 Create `services/policy/internal/grpc/read_server.go` — implement PolicyReadService with 3-layer cache
- [x] 2.3.5 Implement `CheckAccess` in read server: L1 Redis → L2 materialized → L3 CTE fallback
- [x] 2.3.6 Implement read-only graph queries: GetNode, FindNodeByName, GetAncestors, etc.

### 2.4 Write Service Implementation
- [x] 2.4.1 Create `services/policy/internal/grpc/write_server.go` — implement PolicyWriteService
- [x] 2.4.2 Implement targeted cache invalidation: invalidate only affected user/object ancestors
- [x] 2.4.3 Implement graph version increment on each mutation
- [x] 2.4.4 Implement Kafka event publishing for mutations (reuse existing `events/producer.go`)

### 2.5 Service Entrypoints
- [x] 2.5.1 Create `services/policy/cmd/policy-read/main.go` — stateless read service, connects to PG + Redis
- [x] 2.5.2 Shares `services/policy/go.mod` (same module, separate cmd entrypoint)
- [x] 2.5.3 Create `services/policy/Dockerfile.read` for policy-read container
- [x] 2.5.4 Refactor `services/policy/cmd/main.go` to serve PolicyWriteService (keep backward compat by also serving read)

### 2.6 Consumer Migration
- [x] 2.6.1 Update `services/auth/internal/grpc/server.go` — split policyClient into read + write
- [x] 2.6.2 Update `services/auth/cmd/main.go` — connect to both policy-read and policy-write
- [x] 2.6.3 Update `services/workspace/internal/grpc/server.go` — split clients
- [x] 2.6.4 Update `services/workspace/cmd/main.go` — connect to both
- [x] 2.6.5 Update `services/document/internal/grpc/server.go` — split clients
- [x] 2.6.6 Update `services/document/cmd/main.go` — connect to both
- [x] 2.6.7 Update `services/messaging/internal/grpc/server.go` — split clients
- [x] 2.6.8 Update `services/messaging/cmd/main.go` — connect to both
- [x] 2.6.9 Update `services/asset/internal/grpc/*.go` — split clients
- [x] 2.6.10 Update `services/asset/cmd/main.go` — connect to both

### 2.7 Infrastructure
- [x] 2.7.1 Update `docker-compose.yml` — add policy-read service (2 replicas)
- [x] 2.7.2 Configure gRPC client-side load balancing for policy-read
- [x] 2.7.3 Add health check for policy-read service

### 2.8 Tests & Verification
- [x] 2.8.1 Add tests for SQL CTE functions: verify same results as in-memory BFS
- [x] 2.8.2 Add tests for materialized access: insert, lookup, version check, invalidation
- [x] 2.8.3 Add tests for 3-layer cache: L1 hit, L2 hit, L3 fallback, cache population
- [x] 2.8.4 Add tests for targeted invalidation: only affected entries cleared
- [x] 2.8.5 Update all service tests to use split clients (mock both read and write)
- [x] 2.8.6 Build all services: policy, policy-read, auth, workspace, document, messaging, asset, gateway
- [x] 2.8.7 Run all tests: all services pass
- [ ] 2.8.8 End-to-end: docker-compose up, verify cross-workspace share still works with CTE evaluation
