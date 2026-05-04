-- 003_operations.sql
-- Dynamic operations registry.
-- Operations are registered at runtime via RegisterOperations RPC.
-- On migration, auto-populates from existing associations for backward compatibility.

CREATE TABLE IF NOT EXISTS ngac_operations (
    name        TEXT PRIMARY KEY,
    description TEXT DEFAULT '',
    created_at  TIMESTAMPTZ DEFAULT NOW()
);

-- Auto-populate from existing associations (safe for both fresh and existing deployments)
INSERT INTO ngac_operations (name)
SELECT DISTINCT unnest(operations) AS op
FROM ngac_associations
ON CONFLICT (name) DO NOTHING;
