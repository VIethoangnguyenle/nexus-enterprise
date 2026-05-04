# Spec: Integration Guide (README.md)

## Stories

### S13: Integration documentation for consumers

As a platform engineer integrating policy-service into a new backend system,
I want a clear, step-by-step integration guide with code examples in Go and Java,
so that I can set up NGAC authorization without reading source code.

**Acceptance Criteria:**
- [ ] `README.md` in policy service root with:
  - [ ] NGAC concepts overview (5 node types, assignments, associations, prohibitions)
  - [ ] Architecture diagram (CQRS read/write split, cache 3 tầng)
  - [ ] Deployment guide (Docker, env vars, prerequisites)
  - [ ] Full gRPC API reference (all 3 services: Legacy, Read, Write)
  - [ ] Step-by-step integration flow with code examples (Go + Java)
  - [ ] Cache invalidation guide (when consumer MUST call InvalidateCache)
  - [ ] Kafka events reference (topics, payloads)
  - [ ] FAQ / Troubleshooting

---

## Integration Guide Content

### 1. NGAC Concepts (dành cho engineer mới)

```
  NGAC (Next Generation Access Control) — attribute-based access control graph
  
  5 node types:
  ┌─────────────────────────────────────────────────┐
  │ U  (User)            — represents a user        │
  │ UA (User Attribute)  — user group/role           │
  │ O  (Object)          — a resource                │
  │ OA (Object Attribute)— resource group/container  │
  │ PC (Policy Class)    — top-level policy scope    │
  └─────────────────────────────────────────────────┘
  
  2 edge types:
  ┌─────────────────────────────────────────────────┐
  │ Assignment    — containment (U→UA, UA→UA→PC,    │
  │                 O→OA, OA→OA→PC)                 │
  │ Association   — permission (UA --[ops]--> OA)    │
  └─────────────────────────────────────────────────┘
  
  1 override:
  ┌─────────────────────────────────────────────────┐
  │ Prohibition   — explicit DENY overriding ALLOW  │
  └─────────────────────────────────────────────────┘
  
  Access decision: User → UA → Association → OA ← Object
                   (cùng Policy Class)
```

### 2. Architecture

```
  ┌────────────────────────────────────────────────────┐
  │  Your Service (Consumer)                           │
  │  Go / Java / Python / ...                          │
  └──────────────┬─────────────────────────────────────┘
                 │ gRPC
  ┌──────────────▼─────────────────────────────────────┐
  │  Policy Service                                    │
  │                                                    │
  │  PolicyWriteService (singleton)                    │
  │  ├── CreateNode, DeleteNode                        │
  │  ├── CreateAssignment, RemoveAssignment             │
  │  ├── CreateAssociation, RemoveAssociation           │
  │  ├── CreateProhibition, RemoveProhibition           │
  │  ├── RegisterOperations                            │
  │  ├── InvalidateCache                               │
  │  └── InitSchema, LoadGraph                         │
  │                                                    │
  │  PolicyReadService (horizontal replicas)            │
  │  ├── CheckAccess, BatchCheckAccess                  │
  │  ├── GetNode, FindNodeByName, GetNodesByType        │
  │  ├── GetAncestors, GetDescendants, GetChildren...   │
  │  ├── ResolveAccessibleScopes                       │
  │  ├── ListOperations                                │
  │  └── ListProhibitions                              │
  │                                                    │
  │  Cache: L1 Redis → L2 Materialized → L3 Graph BFS  │
  │  Events: Kafka (ngac.graph.mutated)                 │
  └────────────────────────────────────────────────────┘
```

### 3. Deployment

```yaml
# docker-compose.yml (minimal)
services:
  policy-write:
    image: ngac-policy:latest
    environment:
      DATABASE_URL: postgres://user:pass@postgres:5432/ngac?sslmode=disable
      REDIS_URL: redis://redis:6379/0
      KAFKA_BROKERS: kafka:9092        # optional
      GRPC_PORT: 50051
      METRICS_PORT: 9090
      STRICT_OPERATIONS: "false"       # set to "true" to validate operations
    ports:
      - "50051:50051"
      - "9090:9090"
    depends_on:
      - postgres
      - redis

  policy-read:
    image: ngac-policy:latest
    environment:
      DATABASE_URL: postgres://user:pass@postgres:5432/ngac?sslmode=disable
      REDIS_URL: redis://redis:6379/0
      GRPC_PORT: 50052
    ports:
      - "50052:50052"
    deploy:
      replicas: 3                      # scale horizontally
    depends_on:
      - postgres
      - redis
```

### 4. gRPC API Reference

#### PolicyWriteService (singleton — 1 instance)

| RPC | Request | Response | Description |
|---|---|---|---|
| `CreateNode` | `{name, node_type, properties}` | `NGACNode` | Tạo node. node_type: U, UA, O, OA, PC |
| `DeleteNode` | `{node_id}` | `Empty` | Xóa node + cascade |
| `CreateAssignment` | `{child_id, parent_id}` | `Assignment` | Tạo containment edge (U→UA, OA→PC...) |
| `RemoveAssignment` | `{child_id, parent_id}` | `Empty` | Xóa containment edge |
| `CreateAssociation` | `{ua_id, oa_id, operations[]}` | `Association` | Tạo permission edge |
| `RemoveAssociation` | `{ua_id, oa_id}` | `Empty` | Xóa permission edge |
| `CreateProhibition` | `{name, subject_id, operations[], target_oa_ids[], intersection}` | `Prohibition` | Tạo deny override |
| `RemoveProhibition` | `{name}` | `Empty` | Xóa deny override |
| `RegisterOperations` | `{operations[]}` | `{registered[], already_exists[]}` | Đăng ký operations |
| `InvalidateCache` | `{node_ids[], reason}` | `{l1_deleted, l2_deleted, new_version}` | Xóa cache cho nodes |
| `InitSchema` | `Empty` | `Empty` | Tạo DB schema |
| `LoadGraph` | `Empty` | `Empty` | Reload graph từ DB |

#### PolicyReadService (scalable — N replicas)

| RPC | Request | Response | Description |
|---|---|---|---|
| `CheckAccess` | `{user_node_id, object_node_id, operation}` | `AccessDecision` | Kiểm tra quyền. Returns ALLOW/DENY + explanation |
| `BatchCheckAccess` | `{user_node_id, object_ids[], operations[]}` | `BatchAccessResult` | Kiểm tra nhiều objects cùng lúc |
| `ResolveAccessibleScopes` | `{user_node_id, operation}` | `{scope_oa_ids[]}` | Tìm tất cả OA mà user có quyền |
| `GetNode` | `{node_id}` | `NGACNode` | Lấy thông tin node |
| `FindNodeByName` | `{name, node_type}` | `NGACNode` | Tìm node theo name + type |
| `GetNodesByType` | `{node_type}` | `NodeList` | Lấy tất cả nodes theo type |
| `IsAssigned` | `{child_id, parent_id}` | `BoolResponse` | Kiểm tra assignment tồn tại |
| `GetAncestors` | `{node_id}` | `NodeList` | Lấy tất cả ancestors (đi lên) |
| `GetDescendants` | `{node_id}` | `NodeList` | Lấy tất cả descendants (đi xuống) |
| `GetChildren` | `{node_id}` | `NodeList` | Lấy children trực tiếp |
| `GetParents` | `{node_id}` | `NodeList` | Lấy parents trực tiếp |
| `ListOperations` | `Empty` | `OperationList` | Liệt kê operations đã registered |
| `ListProhibitions` | `{subject_id?}` | `ProhibitionList` | Liệt kê prohibitions |

### 5. Integration Flow (Step-by-step)

#### Phase 1: Setup graph structure

```go
// Go consumer example
conn, _ := grpc.Dial("policy-write:50051", grpc.WithInsecure())
write := pb.NewPolicyWriteServiceClient(conn)

// 1. Register your domain operations
write.RegisterOperations(ctx, &pb.RegisterOperationsRequest{
    Operations: []string{"read", "write", "approve", "reject"},
})

// 2. Create Policy Class (top-level scope)
pc, _ := write.CreateNode(ctx, &pb.CreateNodeRequest{
    Name: "PC_HRSystem", NodeType: "PC",
})

// 3. Create User Attributes (roles)
uaManager, _ := write.CreateNode(ctx, &pb.CreateNodeRequest{
    Name: "UA_Manager", NodeType: "UA",
})
uaStaff, _ := write.CreateNode(ctx, &pb.CreateNodeRequest{
    Name: "UA_Staff", NodeType: "UA",
})

// 4. Create Object Attributes (resource groups)
oaDocs, _ := write.CreateNode(ctx, &pb.CreateNodeRequest{
    Name: "OA_Documents", NodeType: "OA",
})

// 5. Build containment hierarchy (assignments)
write.CreateAssignment(ctx, &pb.CreateAssignmentRequest{
    ChildId: uaManager.Id, ParentId: pc.Id,   // UA_Manager → PC
})
write.CreateAssignment(ctx, &pb.CreateAssignmentRequest{
    ChildId: uaStaff.Id, ParentId: pc.Id,     // UA_Staff → PC
})
write.CreateAssignment(ctx, &pb.CreateAssignmentRequest{
    ChildId: oaDocs.Id, ParentId: pc.Id,       // OA_Documents → PC
})

// 6. Create permissions (associations)
write.CreateAssociation(ctx, &pb.CreateAssociationRequest{
    UaId: uaManager.Id, OaId: oaDocs.Id,
    Operations: []string{"read", "write", "approve"},
})
write.CreateAssociation(ctx, &pb.CreateAssociationRequest{
    UaId: uaStaff.Id, OaId: oaDocs.Id,
    Operations: []string{"read", "write"},
})
```

```java
// Java consumer example (Spring Boot + grpc-java)
var writeStub = PolicyWriteServiceGrpc.newBlockingStub(channel);

// 1. Register operations
writeStub.registerOperations(RegisterOperationsRequest.newBuilder()
    .addAllOperations(List.of("read", "write", "approve", "reject"))
    .build());

// 2. Create nodes
var pc = writeStub.createNode(CreateNodeRequest.newBuilder()
    .setName("PC_HRSystem").setNodeType("PC").build());

var uaManager = writeStub.createNode(CreateNodeRequest.newBuilder()
    .setName("UA_Manager").setNodeType("UA").build());

// 3. Assignments
writeStub.createAssignment(CreateAssignmentRequest.newBuilder()
    .setChildId(uaManager.getId()).setParentId(pc.getId()).build());

// 4. Associations
writeStub.createAssociation(CreateAssociationRequest.newBuilder()
    .setUaId(uaManager.getId()).setOaId(oaDocs.getId())
    .addAllOperations(List.of("read", "write", "approve"))
    .build());
```

#### Phase 2: Add users

```go
// Create user node
user, _ := write.CreateNode(ctx, &pb.CreateNodeRequest{
    Name: "thanhttn", NodeType: "U",
    Properties: map[string]string{"email": "thanh@company.com"},
})

// Assign user to role
write.CreateAssignment(ctx, &pb.CreateAssignmentRequest{
    ChildId: user.Id, ParentId: uaManager.Id,  // thanhttn → UA_Manager
})
```

#### Phase 3: Check access (in your API middleware)

```go
// Connect to read service (load-balanced replicas)
readConn, _ := grpc.Dial("policy-read:50052", grpc.WithInsecure())
read := pb.NewPolicyReadServiceClient(readConn)

// Check before every mutation
decision, _ := read.CheckAccess(ctx, &pb.CheckAccessRequest{
    UserNodeId:   "U_thanhttn",
    ObjectNodeId: "OA_Documents",
    Operation:    "approve",
})

if decision.Decision == "ALLOW" {
    // proceed with approval logic
} else {
    // return 403 Forbidden
    // decision.Explanation.Reason has details
}
```

```java
// Java middleware example
var decision = readStub.checkAccess(CheckAccessRequest.newBuilder()
    .setUserNodeId("U_thanhttn")
    .setObjectNodeId("OA_Documents")
    .setOperation("approve")
    .build());

if (!"ALLOW".equals(decision.getDecision())) {
    throw new AccessDeniedException(decision.getExplanation().getReason());
}
```

#### Phase 4: Deny specific access (prohibitions)

```go
// Block thanhttn from writing to sensitive docs (even though UA_Manager has write)
write.CreateProhibition(ctx, &pb.CreateProhibitionRequest{
    Name:         "deny-thanhttn-sensitive",
    SubjectId:    "U_thanhttn",
    Operations:   []string{"write"},
    TargetOaIds:  []string{"OA_SensitiveDocs"},
    Intersection: false,
})

// Now CheckAccess(thanhttn, OA_SensitiveDocs, write) → DENY
// But CheckAccess(thanhttn, OA_Documents, write) → still ALLOW
```

### 6. Cache Invalidation Guide

**Khi nào consumer PHẢI gọi `InvalidateCache`:**

| Tình huống | Action |
|---|---|
| User bị xóa khỏi hệ thống | `InvalidateCache(["U_<user_id>"], "user deleted")` |
| Object/resource bị xóa | `InvalidateCache(["OA_<resource_id>"], "resource deleted")` |
| User đổi department (ngoài NGAC graph) | `InvalidateCache(["U_<user_id>"], "department changed")` |
| Bulk permission reset | Gọi `LoadGraph` thay vì invalidate từng node |

**Khi nào KHÔNG cần gọi:**

| Tình huống | Lý do |
|---|---|
| CreateAssignment / RemoveAssignment | Policy service tự invalidate |
| CreateAssociation / RemoveAssociation | Policy service tự invalidate |
| CreateProhibition / RemoveProhibition | Policy service tự invalidate |
| Node CRUD | Policy service tự invalidate |

### 7. Kafka Events Reference

| Topic | Event | Payload | Khi nào |
|---|---|---|---|
| `ngac.access.checked` | AccessCheckedEvent | `{user_id, object_id, operation, decision, cached, timestamp}` | Mỗi CheckAccess call |
| `ngac.graph.mutated` | GraphMutatedEvent | `{mutation_type, node_ids[], child_type, parent_type, timestamp}` | Graph thay đổi |

**mutation_type values:**
- `create_assignment` / `remove_assignment`
- `create_association` / `remove_association`  
- `create_prohibition` / `remove_prohibition`
- `delete_node`

**Consumer use case:** Subscribe `ngac.graph.mutated` để react khi permissions thay đổi:

```go
// Example: khi user bị xóa role → notify UI
for event := range kafkaConsumer.Events("ngac.graph.mutated") {
    if event.MutationType == "remove_assignment" {
        // Notify affected user via WebSocket
        notifyUser(event.NodeIDs[0], "Your permissions have changed")
    }
}
```

### 8. Environment Variables

| Variable | Default | Description |
|---|---|---|
| `DATABASE_URL` | `postgres://...localhost:5433/ngac` | PostgreSQL connection |
| `REDIS_URL` | `redis://localhost:6379/0` | Redis for L1 cache (optional) |
| `KAFKA_BROKERS` | `localhost:19092` | Kafka brokers (optional) |
| `GRPC_PORT` | `50051` | gRPC listen port |
| `METRICS_PORT` | `9090` | Prometheus metrics port |
| `STRICT_OPERATIONS` | `false` | Validate operations in CreateAssociation |

### 9. Node Type Rules

| Child Type | Valid Parent Types | Example |
|---|---|---|
| `U` (User) | `UA` | U_thanhttn → UA_Manager |
| `UA` (User Attribute) | `UA`, `PC` | UA_Manager → UA_AdminGroup → PC |
| `O` (Object) | `OA` | O_doc123 → OA_Documents |
| `OA` (Object Attribute) | `OA`, `PC` | OA_Documents → OA_AllDocs → PC |

Invalid assignments (e.g., U → OA) will return `INVALID_ARGUMENT`.

### 10. FAQ

**Q: Có cần gọi InitSchema trước khi dùng?**
A: Có. Policy service gọi InitSchema tự động khi start. Nếu deploy fresh, service tự tạo tables.

**Q: CheckAccess trả DENY — làm sao debug?**
A: Xem `explanation.reason` và `explanation.path`. Dùng `GetAncestors` để trace graph.

**Q: Có thể tạo circular assignments không?**
A: Không. Assignments là DAG (Directed Acyclic Graph). Circular sẽ bị reject.

**Q: L1 cache TTL là bao lâu?**
A: 30 giây (hardcoded). Graph mutations trigger immediate invalidation.

**Q: Prohibition có cascade delete khi subject node bị xóa không?**
A: Prohibition row vẫn tồn tại nhưng không có effect (subject not found in graph).
