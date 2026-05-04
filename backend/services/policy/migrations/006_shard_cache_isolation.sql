-- 006_shard_cache_isolation.sql
-- Add workspace_id to materialized cache for per-tenant shard isolation.
-- This prevents cross-tenant decision collisions when using sharded graphs.

-- Step 1: Add workspace_id column (empty string = legacy global entries)
ALTER TABLE ngac_materialized_access ADD COLUMN IF NOT EXISTS workspace_id TEXT NOT NULL DEFAULT '';

-- Step 2: Recreate unique constraint with workspace_id
-- Drop old PK and create new one including workspace_id
ALTER TABLE ngac_materialized_access DROP CONSTRAINT IF EXISTS ngac_materialized_access_pkey;
ALTER TABLE ngac_materialized_access
    ADD CONSTRAINT ngac_materialized_access_pkey
    PRIMARY KEY (workspace_id, user_node_id, object_node_id, operation);

-- Step 3: Add index for per-workspace queries
CREATE INDEX IF NOT EXISTS idx_materialized_workspace ON ngac_materialized_access(workspace_id);

-- Step 4: Seed per-workspace version rows for existing tenants
INSERT INTO ngac_graph_version (scope, version)
SELECT CONCAT('ws:', properties->>'workspace_id'), 0
FROM ngac_nodes
WHERE node_type = 'PC'
  AND properties->>'workspace_id' IS NOT NULL
  AND properties->>'workspace_id' != ''
ON CONFLICT (scope) DO NOTHING;
