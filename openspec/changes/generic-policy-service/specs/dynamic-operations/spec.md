# Spec: Dynamic Operations

## Stories

### S4: Register operations at runtime

As a platform engineer,
I want to register my domain-specific operations (e.g., "approve", "transfer") via gRPC,
so that policy-service validates associations against a known operation set.

**Acceptance Criteria:**
- [ ] `RegisterOperations` RPC accepts a list of operation names
- [ ] Operations are persisted in `ngac_operations` table
- [ ] Duplicate registration returns `already_exists` list (idempotent)
- [ ] Response includes both `registered` and `already_exists` lists

**Proto mapping:** NEW — `PolicyWriteService.RegisterOperations`

```protobuf
rpc RegisterOperations(RegisterOperationsRequest) returns (RegisterOperationsResponse);

message RegisterOperationsRequest {
    repeated string operations = 1;
}
message RegisterOperationsResponse {
    repeated string registered = 1;
    repeated string already_exists = 2;
}
```

### S5: List registered operations

As a platform engineer,
I want to query all registered operations,
so that I can verify my system's operation set.

**Acceptance Criteria:**
- [ ] `ListOperations` RPC returns all registered operations
- [ ] Empty list returned on fresh deploy (no operations registered)

**Proto mapping:** NEW — `PolicyReadService.ListOperations`

```protobuf
rpc ListOperations(Empty) returns (OperationList);

message OperationList {
    repeated string operations = 1;
}
```

### S6: Strict mode validation (optional)

As a platform engineer who wants to prevent typos,
I want strict mode that rejects unknown operations in CreateAssociation,
so that mistyped operations are caught early.

**Acceptance Criteria:**
- [ ] When `STRICT_OPERATIONS=true`: CreateAssociation with unregistered operation returns `INVALID_ARGUMENT` error
- [ ] When `STRICT_OPERATIONS=false` (default): any operation string accepted (backward compatible)
- [ ] CheckAccess always works regardless of strict mode (query-only, no validation needed)

**Proto mapping:** Existing `CreateAssociation` RPC — behavior change controlled by env var.

---

## DB Schema + Data Migration

```sql
CREATE TABLE IF NOT EXISTS ngac_operations (
    name        TEXT PRIMARY KEY,
    description TEXT DEFAULT '',
    created_at  TIMESTAMPTZ DEFAULT NOW()
);

-- CRITICAL: Auto-populate từ associations hiện có
-- Hệ thống đang chạy → ngac_associations đã có operations ["read","write","approve",...]
-- Nếu không populate → strict mode sẽ reject hết associations mới
INSERT INTO ngac_operations (name)
SELECT DISTINCT unnest(operations) AS op
FROM ngac_associations
ON CONFLICT (name) DO NOTHING;
```

### Tại sao cần auto-populate?

```
  Hệ thống hiện tại:
  ┌─────────────────────────────────────┐
  │ ngac_associations                   │
  │ (UA_Staff, OA_Docs, ["read","write"])│
  │ (UA_Admin, OA_All, ["manage"])      │
  └─────────────────────────────────────┘
  
  Sau migration:
  ┌─────────────────────────────────────┐
  │ ngac_operations                     │
  │ read    ← extracted from assoc     │
  │ write   ← extracted from assoc     │
  │ manage  ← extracted from assoc     │
  └─────────────────────────────────────┘
  
  → Strict mode bật → "read" đã registered → OK
  → Consumer RegisterOperations("approve") → thêm mới → OK
```

## Edge Cases

| Case | Behavior |
|---|---|
| Register empty list | No-op, return empty response |
| Register same op twice | Idempotent, appears in `already_exists` |
| Strict mode + unregistered op in CreateAssociation | Return `INVALID_ARGUMENT` with message |
| Strict mode OFF | All operations accepted |
| Fresh deploy (no associations) | `ngac_operations` empty — consumer registers via RPC |
| Existing system migration | `INSERT ... SELECT DISTINCT unnest(operations)` populates automatically |

