## Architecture Overview

```
Phase 1 (Algorithm):                  Phase 2 (CQRS):
                                      
  Same architecture,                  Services ──▶ PolicyReadService (N replicas)
  optimized internals                           ──▶ PolicyWriteService (singleton)
                                                         │
  CheckAccess: DFS×4 → BFS×2                   ┌────────▼────────┐
  FindNodeByName: O(N) → O(1)                  │   3-Layer Cache  │
  Early termination: No → Yes                   │  L1: Redis (µs)  │
                                                │  L2: Matrl. (ms) │
                                                │  L3: SQL CTE(ms) │
                                                └─────────────────┘
```

---

## Phase 1: Algorithm Optimization

### 1.1 Single-Pass BFS replacing 4×DFS

**Current flow** (access.go:8-134):
```
CheckAccess(user, object, op)
  ├── DFS 1: collectAncestorsOfType(user, "UA")     → userUAs
  ├── DFS 2: collectAncestorsOfType(object, "OA")   → objectOAs
  ├── DFS 3: for each UA → collectPCsReachable()    → userPCs  (REDUNDANT)
  ├── DFS 4: for each OA → collectPCsReachable()    → objectPCs (REDUNDANT)
  └── Scan: for each UA → check associations
```

**New flow**:
```
CheckAccess(user, object, op)
  ├── BFS 1: bfsCollectAttributesAndPCs(user, "UA")  → userUAs + userPCs
  ├── BFS 2: bfsCollectAttributesAndPCs(object, "OA") → objectOAs + objectPCs
  └── Match: findMatchingAssociation() with early return
```

**Key changes to `access.go`**:

```go
// NEW: Single-pass iterative BFS — replaces collectAncestorsOfType + collectPCsReachable
func (g *Graph) bfsCollectAttributesAndPCs(startID, attrType string) (attrs, pcs map[string]bool) {
    attrs = map[string]bool{startID: true}
    pcs = make(map[string]bool)
    visited := map[string]bool{startID: true}
    queue := []string{startID}

    for len(queue) > 0 {
        current := queue[0]
        queue = queue[1:]

        for pid := range g.childToParents[current] {
            if visited[pid] { continue }
            visited[pid] = true
            node := g.Nodes[pid]
            if node == nil { continue }

            switch node.NodeType {
            case attrType:
                attrs[pid] = true
                queue = append(queue, pid)
            case NodeTypePolicyClass:
                pcs[pid] = true
                // PC is leaf — don't continue BFS
            default:
                queue = append(queue, pid)
            }
        }
    }
    return
}
```

### 1.2 Name+Type Index for O(1) FindNodeByName

**Current** (graph.go:252-261): O(N) linear scan over all nodes.

**New**: Add index maintained on AddNode/RemoveNode:

```go
type Graph struct {
    // ... existing fields ...
    nameTypeIndex map[string]*NGACNode  // key: "name:type" → node
}

func nameTypeKey(name, nodeType string) string {
    return name + ":" + nodeType
}

func (g *Graph) AddNode(node *NGACNode) {
    g.mu.Lock()
    defer g.mu.Unlock()
    g.Nodes[node.ID] = node
    g.nameTypeIndex[nameTypeKey(node.Name, node.NodeType)] = node  // NEW
}

func (g *Graph) FindNodeByName(name, nodeType string) *NGACNode {
    g.mu.RLock()
    defer g.mu.RUnlock()
    return g.nameTypeIndex[nameTypeKey(name, nodeType)]  // O(1)
}
```

### 1.3 Early Termination in Association Matching

**Current**: Collect ALL UAs and OAs first, then scan associations.

**New**: Check associations at each BFS level — return ALLOW as soon as found:

```go
func (g *Graph) findMatchingAssociation(userUAs, objectOAs, userPCs, objectPCs map[string]bool, op string) bool {
    for uaID := range userUAs {
        for _, assoc := range g.uaToAssociations[uaID] {
            if !objectOAs[assoc.OAID] { continue }
            if !containsOp(assoc.Operations, op) { continue }
            if hasCommonPC(userPCs, objectPCs) {
                return true  // EARLY EXIT
            }
        }
    }
    return false
}
```

### 1.4 Remove PC_Global hardcoded check

**Current** (access.go:88-95): String comparison `n.Name == "PC_Global"`.

**New**: PC intersection check naturally handles PC_Global — if user reaches PC_Global through any path, it's in userPCs. If object also reaches PC_Global, it's in objectPCs. Intersection finds it automatically. Remove special-case code.

---

## Phase 2: CQRS + DB-Driven Evaluation

### 2.1 Proto Split

```protobuf
// policy_read.proto
service PolicyReadService {
  rpc CheckAccess(CheckAccessRequest) returns (AccessDecision);
  rpc GetNode(GetNodeRequest) returns (NGACNode);
  rpc FindNodeByName(FindNodeByNameRequest) returns (NGACNode);
  rpc GetNodesByType(GetNodesByTypeRequest) returns (NodeList);
  rpc IsAssigned(IsAssignedRequest) returns (BoolResponse);
  rpc GetAncestors(GetAncestorsRequest) returns (NodeList);
  rpc GetDescendants(GetDescendantsRequest) returns (NodeList);
  rpc GetChildren(GetChildrenRequest) returns (NodeList);
  rpc GetParents(GetParentsRequest) returns (NodeList);
}

// policy_write.proto
service PolicyWriteService {
  rpc CreateNode(CreateNodeRequest) returns (NGACNode);
  rpc DeleteNode(DeleteNodeRequest) returns (Empty);
  rpc CreateAssignment(CreateAssignmentRequest) returns (Assignment);
  rpc RemoveAssignment(RemoveAssignmentRequest) returns (Empty);
  rpc CreateAssociation(CreateAssociationRequest) returns (Association);
  rpc RemoveAssociation(RemoveAssociationRequest) returns (Empty);
  rpc InitSchema(Empty) returns (Empty);
}
```

### 2.2 SQL Recursive CTE for Graph Traversal

```sql
-- bfs_ancestors: replaces in-memory BFS
-- Returns all ancestor node IDs reachable from start_node
CREATE OR REPLACE FUNCTION ngac_ancestors(start_id TEXT)
RETURNS TABLE(node_id TEXT, node_type TEXT) AS $$
  WITH RECURSIVE ancestors AS (
    SELECT a.parent_id AS node_id
    FROM ngac_assignments a
    WHERE a.child_id = start_id
    UNION
    SELECT a.parent_id
    FROM ngac_assignments a
    JOIN ancestors anc ON a.child_id = anc.node_id
  )
  SELECT anc.node_id, n.node_type
  FROM ancestors anc
  JOIN ngac_nodes n ON anc.node_id = n.id;
$$ LANGUAGE SQL STABLE;

-- check_access: full NGAC access decision in SQL
CREATE OR REPLACE FUNCTION ngac_check_access(
  p_user_id TEXT, p_object_id TEXT, p_operation TEXT
) RETURNS BOOLEAN AS $$
  WITH
  user_attrs AS (
    SELECT node_id FROM ngac_ancestors(p_user_id)
    WHERE node_type IN ('UA', 'PC')
    UNION SELECT p_user_id AS node_id
  ),
  object_attrs AS (
    SELECT node_id FROM ngac_ancestors(p_object_id)
    WHERE node_type IN ('OA', 'PC')
    UNION SELECT p_object_id AS node_id
  ),
  user_pcs AS (
    SELECT node_id FROM user_attrs ua
    JOIN ngac_nodes n ON ua.node_id = n.id
    WHERE n.node_type = 'PC'
  ),
  object_pcs AS (
    SELECT node_id FROM object_attrs oa
    JOIN ngac_nodes n ON oa.node_id = n.id
    WHERE n.node_type = 'PC'
  )
  SELECT EXISTS (
    SELECT 1
    FROM ngac_associations assoc
    WHERE assoc.ua_id IN (SELECT node_id FROM user_attrs)
      AND assoc.oa_id IN (SELECT node_id FROM object_attrs)
      AND p_operation = ANY(assoc.operations)
      AND EXISTS (
        SELECT 1 FROM user_pcs
        WHERE node_id IN (SELECT node_id FROM object_pcs)
      )
  );
$$ LANGUAGE SQL STABLE;
```

### 2.3 Materialized Access Table

```sql
CREATE TABLE ngac_materialized_access (
  user_node_id   TEXT NOT NULL,
  object_node_id TEXT NOT NULL,
  operation      TEXT NOT NULL,
  decision       BOOLEAN NOT NULL,
  graph_version  BIGINT NOT NULL,
  computed_at    TIMESTAMPTZ DEFAULT NOW(),
  PRIMARY KEY (user_node_id, object_node_id, operation)
);

CREATE TABLE ngac_graph_version (
  scope      TEXT PRIMARY KEY,  -- 'global' or 'ws:{workspace_id}'
  version    BIGINT NOT NULL DEFAULT 0,
  updated_at TIMESTAMPTZ DEFAULT NOW()
);
```

### 2.4 Read Service: 3-Layer Cache Lookup

```go
func (s *ReadServer) CheckAccess(ctx context.Context, req *pb.CheckAccessRequest) (*pb.AccessDecision, error) {
    // L1: Redis
    if decision, err := s.getRedisCache(ctx, req); err == nil {
        return decision, nil
    }

    // L2: Materialized table
    if decision, err := s.getMaterialized(ctx, req); err == nil {
        s.setRedisCache(ctx, req, decision)
        return decision, nil
    }

    // L3: SQL CTE (full traversal)
    decision, err := s.computeViaCTE(ctx, req)
    if err != nil {
        return nil, err
    }
    s.setMaterialized(ctx, req, decision)
    s.setRedisCache(ctx, req, decision)
    return decision, nil
}
```

### 2.5 Write Service: Persist + Publish + Invalidate

```go
func (s *WriteServer) CreateAssignment(ctx context.Context, req *pb.CreateAssignmentRequest) (*pb.Assignment, error) {
    // 1. Persist to PostgreSQL
    a, err := s.store.CreateAssignment(ctx, req.ChildId, req.ParentId)
    if err != nil {
        return nil, err
    }

    // 2. Increment graph version
    s.store.IncrementVersion(ctx, scopeFromNode(req.ChildId))

    // 3. Invalidate affected caches (targeted, not full flush)
    s.invalidateAncestorCache(ctx, req.ChildId)
    s.invalidateDescendantCache(ctx, req.ParentId)

    // 4. Publish event for other consumers
    s.producer.PublishGraphMutated("create_assignment", []string{req.ChildId, req.ParentId})

    return protoAssignment(a), nil
}
```

### 2.6 Consumer Service Migration

Each service changes from:
```go
// OLD
policyClient policypb.PolicyServiceClient

// NEW
policyRead  policypb.PolicyReadServiceClient   // for CheckAccess, GetNode, etc.
policyWrite policypb.PolicyWriteServiceClient   // for CreateNode, CreateAssignment, etc.
```

---

## Files Changed

### Phase 1 (Algorithm)
| File | Change |
|------|--------|
| `services/policy/internal/ngac/access.go` | Rewrite: BFS, early termination, remove PC_Global hack |
| `services/policy/internal/ngac/graph.go` | Add: bfsCollectAttributesAndPCs, nameTypeIndex, iterative BFS methods |
| `services/policy/internal/ngac/models.go` | Add: nameTypeKey helper |
| `services/policy/internal/ngac/store_test.go` | Update: tests for new algorithm (same results, faster) |

### Phase 2 (CQRS)
| File | Change |
|------|--------|
| `proto/policy/policy_read.proto` | NEW: read-only RPCs |
| `proto/policy/policy_write.proto` | NEW: write-only RPCs |
| `proto/policy/policy.proto` | DEPRECATE: replaced by read+write |
| `services/policy/cmd/main.go` | Split into policy-read and policy-write entrypoints |
| `services/policy/internal/grpc/read_server.go` | NEW: 3-layer cache + CTE evaluation |
| `services/policy/internal/grpc/write_server.go` | NEW: persist + invalidate + publish |
| `services/policy/internal/ngac/cte.go` | NEW: SQL CTE functions |
| `services/policy/internal/ngac/materialized.go` | NEW: materialized access table ops |
| `services/policy/internal/ngac/version.go` | NEW: graph version tracking |
| `data/migrations/003_materialized_access.sql` | NEW: materialized + version tables |
| `services/auth/internal/grpc/server.go` | Update: split client imports |
| `services/workspace/internal/grpc/server.go` | Update: split client imports |
| `services/document/internal/grpc/server.go` | Update: split client imports |
| `services/messaging/internal/grpc/server.go` | Update: split client imports |
| `services/asset/internal/grpc/*.go` | Update: split client imports |
| `docker-compose.yml` | Add: policy-read replicas |

## Migration Safety

- Phase 1 is **pure refactor** — identical inputs produce identical outputs, just faster
- Phase 2 is **backward compatible** — old proto still works during transition
- Read replicas can be rolled out one service at a time
- Materialized cache is additive — falls back to CTE on miss
