# Spec: Prohibitions (Deny Overrides)

## Stories

### S7: Create a prohibition

As a platform engineer,
I want to create deny rules that override ALLOW decisions from associations,
so that I can block specific users/groups from accessing specific resources.

**Acceptance Criteria:**
- [ ] `CreateProhibition` RPC creates a prohibition with: name, subject (user or UA), operations, target OA nodes
- [ ] Prohibition is persisted in `ngac_prohibitions` table
- [ ] Duplicate prohibition name returns `ALREADY_EXISTS` error
- [ ] Graph version is incremented (same mechanism as assignment/association changes)
- [ ] L1 (Redis) keys invalidated for subject + all users under subject (if UA)
- [ ] L2 (Materialized) rows invalidated for subject user + target OA combinations
- [ ] Kafka event published: `ngac.graph.mutated` with `mutation_type=create_prohibition`

**Proto mapping:** NEW — `PolicyWriteService.CreateProhibition`

```protobuf
rpc CreateProhibition(CreateProhibitionRequest) returns (Prohibition);

message CreateProhibitionRequest {
    string name = 1;               // unique name: "deny-thanhttn-sensitive"
    string subject_id = 2;          // U or UA node ID being denied
    repeated string operations = 3; // ["read", "write"]
    repeated string target_oa_ids = 4; // OA nodes being denied access to
    bool intersection = 5;          // true=ALL targets must match, false=ANY target
}

message Prohibition {
    string id = 1;
    string name = 2;
    string subject_id = 3;
    repeated string operations = 4;
    repeated string target_oa_ids = 5;
    bool intersection = 6;
}
```

### S8: Remove a prohibition

As a platform engineer,
I want to remove a prohibition by name,
so that the deny override is lifted.

**Acceptance Criteria:**
- [ ] `RemoveProhibition` RPC deletes by name
- [ ] Non-existent name returns `NOT_FOUND`
- [ ] Graph version is incremented
- [ ] L1 + L2 cache invalidated for subject + target nodes
- [ ] After removal, next CheckAccess → L3 recalculation → returns ALLOW (if association path exists)
- [ ] Kafka event published: `ngac.graph.mutated` with `mutation_type=remove_prohibition`

**Proto mapping:** NEW — `PolicyWriteService.RemoveProhibition`

### S9: List prohibitions

As a platform engineer,
I want to query all active prohibitions,
so that I can audit deny rules.

**Acceptance Criteria:**
- [ ] `ListProhibitions` RPC returns all prohibitions
- [ ] Optional filter by `subject_id`
- [ ] Empty list on fresh deploy

**Proto mapping:** NEW — `PolicyReadService.ListProhibitions`

### S10: CheckAccess respects prohibitions

As a platform engineer,
I want CheckAccess to automatically check prohibitions after finding an ALLOW path,
so that deny overrides work transparently.

**Acceptance Criteria:**
- [ ] If association path ALLOW + no prohibition match → ALLOW
- [ ] If association path ALLOW + prohibition matches subject AND operation AND target → DENY
- [ ] If association path DENY → DENY (prohibition not checked)
- [ ] Prohibition on UA applies to ALL users assigned to that UA
- [ ] Explanation includes `prohibition_denied` field when prohibition blocks access
- [ ] L2 materialized stores the FINAL decision (after prohibition check), not just BFS result
- [ ] L1 Redis stores the FINAL decision (after prohibition check)

**Proto mapping:** Existing `CheckAccess` — behavior change.

```protobuf
// Add to AccessExplanation:
message AccessExplanation {
    // ... existing fields ...
    ProhibitionDenial prohibition_denied = 8; // NEW
}

message ProhibitionDenial {
    string prohibition_name = 1;
    string subject_id = 2;
}
```

---

## DB Schema

```sql
CREATE TABLE IF NOT EXISTS ngac_prohibitions (
    id              TEXT PRIMARY KEY DEFAULT gen_random_uuid()::TEXT,
    name            TEXT UNIQUE NOT NULL,
    subject_id      TEXT NOT NULL,       -- U or UA node
    operations      TEXT[] NOT NULL,     -- {"read","write"}
    target_oa_ids   TEXT[] NOT NULL,     -- {OA_1, OA_2}
    intersection    BOOLEAN DEFAULT FALSE,
    created_at      TIMESTAMPTZ DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_prohibitions_subject ON ngac_prohibitions(subject_id);
```

---

## CRITICAL: Cache 3 tầng interaction

### Vấn đề: Stale ALLOW sau khi tạo prohibition

```
  T0: CheckAccess(thanhttn, OA_docs, read) → BFS → ALLOW
      → L2: (thanhttn, OA_docs, read, decision=true, v=42)
      → L1: ngac:access:U_thanhttn:OA_docs:read → ALLOW

  T1: CreateProhibition(subject=thanhttn, ops=["read"], targets=[OA_docs])
      → NẾU KHÔNG invalidate cache → T2 sẽ trả ALLOW sai!
```

### Giải pháp: Prohibition mutation = Graph mutation

CreateProhibition và RemoveProhibition PHẢI chạy **cùng flow invalidation** như CreateAssignment/RemoveAssignment:

```
CreateProhibition(name, subject_id, operations, target_oa_ids):
    1. INSERT INTO ngac_prohibitions

    2. incrementAndInvalidate(ctx, affectedNodes...):
       a) version.Increment("global") → v42 → v43

       b) Identify affected users:
          - subject_id = type U → affected = [subject_id]
          - subject_id = type UA → affected = GetDescendants(subject_id)
                                   → tìm ALL U nodes dưới UA

       c) L2 invalidation:
          - FOR EACH affected_user:
              materialized.InvalidateByUser(affected_user)
          - FOR EACH target_oa:
              materialized.InvalidateByObject(target_oa)

       d) L1 invalidation:
          - cache.InvalidateForNodes(subject_id, target_oa_ids...)
          → Targeted key deletion cho từng affected user

    3. publishEvent("create_prohibition", [...])
```

### CheckAccess flow (SAU KHI có prohibition)

```
ReadServer.CheckAccess(user, object, operation):

  L1: Redis → if HIT → return cached decision
                        (đã bao gồm prohibition check từ lần compute trước)

  L2: Materialized → if HIT + version fresh → return cached decision
                      (đã bao gồm prohibition check từ lần compute trước)

  L3: Compute from scratch:
      Step 1: BFS graph traversal
              → if DENY → finalDecision = DENY (skip prohibition check)

      Step 2: if BFS = ALLOW → check prohibitions:
              → Query: SELECT * FROM ngac_prohibitions
                       WHERE subject_id IN (user, user_ancestor_UAs)
                       AND operations @> ARRAY[operation]
              → For each matching prohibition:
                 - intersection=false: ANY target_oa_id in object's OA ancestors → DENY
                 - intersection=true:  ALL target_oa_ids in object's OA ancestors → DENY
              → if match found → finalDecision = DENY + set prohibition_denied
              → else → finalDecision = ALLOW

      Step 3: populateCaches(finalDecision):
              → L2: UPSERT (user, object, op, finalDecision, currentVersion)
              → L1: SET key → finalDecision (TTL 30s)

  QUAN TRỌNG: L2 và L1 lưu decision CUỐI CÙNG (đã tính prohibition),
              KHÔNG PHẢI chỉ kết quả BFS.
```

### Ví dụ End-to-End: Prohibition + Cache

```
  T0: thanhttn thuộc UA_Staff → assoc(read) → OA_Docs
      CheckAccess → BFS ALLOW → no prohibition → ALLOW
      L2: (thanhttn, OA_Docs, read, true, v=42)
      L1: ALLOW

  T1: CreateProhibition("deny-thanhttn-docs", thanhttn, ["read"], [OA_Docs])
      → version: 42 → 43
      → L2: DELETE WHERE user_node_id='U_thanhttn'     ← xóa row cũ
      → L1: DEL ngac:access:U_thanhttn:*               ← xóa key cũ

  T2: CheckAccess(thanhttn, OA_Docs, read)
      → L1: MISS (đã bị DEL)
      → L2: MISS (đã bị DELETE)
      → L3: BFS → ALLOW → check prohibitions → MATCH → DENY
      → L2: UPSERT (thanhttn, OA_Docs, read, false, v=43)    ← lưu DENY
      → L1: SET → DENY
      ✘ Kết quả: DENY ✓ (đúng!)

  T3: RemoveProhibition("deny-thanhttn-docs")
      → version: 43 → 44
      → L2: DELETE WHERE user_node_id='U_thanhttn'     ← xóa DENY row
      → L1: DEL ngac:access:U_thanhttn:*

  T4: CheckAccess(thanhttn, OA_Docs, read)
      → L1: MISS → L2: MISS
      → L3: BFS → ALLOW → no prohibitions → ALLOW
      → L2: UPSERT (thanhttn, OA_Docs, read, true, v=44)     ← lưu ALLOW
      → L1: SET → ALLOW
      ✓ Kết quả: ALLOW ✓ (đúng!)
```

## Edge Cases

| Case | Behavior |
|---|---|
| Prohibition on UA, user assigned to UA | DENY — prohibition inherits. Cache invalidated cho ALL users dưới UA |
| Prohibition on UA, user removed from UA | ALLOW — no longer subject (RemoveAssignment đã invalidate) |
| Prohibition on user + association via UA | DENY — direct prohibition wins |
| Delete subject node → prohibition orphaned | Prohibition remains but CheckAccess skips (subject not in graph) |
| Multiple prohibitions on same subject | ALL checked, any match → DENY |
| Prohibition created → L2 has stale ALLOW | version.Increment + InvalidateByUser → stale row deleted/ignored |
| Prohibition removed → L2 has stale DENY | version.Increment + InvalidateByUser → stale row deleted, L3 recalculates |
| Prohibition on UA + L1 cache for descendant users | cache.InvalidateForNodes traverses UA descendants → DEL keys cho tất cả users |
