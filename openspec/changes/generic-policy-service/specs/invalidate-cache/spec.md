# Spec: InvalidateCache RPC

## Stories

### S11: Inbound cache invalidation via gRPC

As a consumer service engineer,
I want to call policy-service to invalidate cache for specific nodes,
so that when my service modifies user/object relationships, cached permissions are refreshed.

**Acceptance Criteria:**
- [ ] `InvalidateCache` RPC accepts a list of node IDs + reason string
- [ ] L1 (Redis) keys for those nodes are deleted
- [ ] L2 (Materialized) rows for those nodes are deleted
- [ ] Graph version is incremented
- [ ] Reason is logged for audit trail
- [ ] Empty node_ids list → no-op

**Proto mapping:** NEW — `PolicyWriteService.InvalidateCache`

```protobuf
rpc InvalidateCache(InvalidateCacheRequest) returns (InvalidateCacheResponse);

message InvalidateCacheRequest {
    repeated string node_ids = 1;   // nodes whose cache should be invalidated
    string reason = 2;             // audit: "user removed from workspace"
}

message InvalidateCacheResponse {
    int32 l1_keys_deleted = 1;     // Redis keys deleted
    int32 l2_rows_deleted = 2;     // Materialized rows deleted
    int64 new_version = 3;         // graph version after increment
}
```

---

## Flow

```
1. Consumer detects domain change (e.g., user removed from workspace)
2. Consumer calls policy.InvalidateCache(node_ids=["U_thanhttn"], reason="left workspace")
3. Policy service:
   a. version.Increment("global")
   b. For each node_id:
      - materialized.InvalidateByUser(node_id)
      - materialized.InvalidateByObject(node_id)
      - cache.InvalidateForNodes(node_id)
   c. Log: "external invalidation: reason=left workspace, nodes=[U_thanhttn]"
4. Consumer receives response with counts
5. Next CheckAccess for those nodes → L3 recalculation
```

## Edge Cases

| Case | Behavior |
|---|---|
| Node ID doesn't exist in graph | Still invalidates cache (node may have been deleted) |
| Empty node_ids | No-op, return zeros |
| Redis unavailable | L2 + version still processed, L1 count = 0 |
| Concurrent invalidation | Version increment is atomic (SQL) |
