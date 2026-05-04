# Design: Nuclear DB Clean + Re-seed

## Phase 1: TRUNCATE

Single SQL script that wipes all data in dependency order:

```sql
-- Disable FK checks via CASCADE
TRUNCATE messages, thread_participants, channel_members, channels,
         drive_items, drive_shares, drive_quotas,
         documents, notifications,
         assets, asset_transitions, asset_requests, asset_types,
         tenant_users, workspaces, users,
         ngac_associations, ngac_assignments, ngac_nodes
CASCADE;
```

## Phase 2: Clean MinIO

```bash
# Remove all workspace buckets
mc alias set local http://localhost:9000 minioadmin minioadmin
mc rb --force local/ws-* 2>/dev/null || true
```

## Phase 3: Re-seed via API

API-driven seed ensures NGAC graph is built by the same code path as production.

### Step 1: Register user
```bash
POST /api/auth/register
{"username": "hoangnlv", "password": "Aqswde123@@"}
# Returns: token, user.id, user.ngac_node_id
```

### Step 2: Create workspace
```bash
POST /api/workspaces
{"name": "NGAC Platform"}
# Returns: workspace with PC, all OA/UA nodes, drive root
```

### Step 3: Create channel
```bash
POST /api/workspaces/:ws_id/channels
{"name": "general", "channel_type": "workspace"}
# Returns: channel with content OA, members UA, drive folder
```

### Step 4: Seed messages
```bash
POST /api/channels/:ch_id/messages
{"content": "Welcome to NGAC Platform! 🎉"}

POST /api/channels/:ch_id/messages
{"content": "This workspace uses Next-Generation Access Control for fine-grained permissions."}
```

## Phase 4: Verify NGAC Integrity

SQL query to verify no orphan nodes exist:

```sql
-- Every OA/UA should have at least 1 parent assignment
SELECT n.id, n.name, n.node_type
FROM ngac_nodes n
LEFT JOIN ngac_assignments a ON n.id = a.child_id
WHERE a.child_id IS NULL
  AND n.node_type NOT IN ('PC');
-- Expected: 0 rows (only PC nodes have no parent)
```

## Phase 5: Restart services

```bash
make stop && make run
```

Services need restart to clear in-memory NGAC caches.

## Expected Final State

| Entity | Count | Details |
|--------|-------|---------|
| Users | 1 | hoangnlv |
| Workspaces | 1 | NGAC Platform |
| Channels | 1 | general |
| Messages | 2 | Welcome messages |
| NGAC Nodes | ~18 | PC + 6 OA + 3 UA + 1 U + channel nodes |
| Drive Items | ~3 | Root folder + channel drive folder |
