-- The operation registry. The policy service created this table at runtime
-- (InitSchema) and in a policy-local migration chain that nothing applied, so a
-- database built from init.sql and this chain lacked it.

CREATE TABLE IF NOT EXISTS ngac_operations (
    name        TEXT PRIMARY KEY,
    description TEXT DEFAULT '',
    created_at  TIMESTAMPTZ DEFAULT NOW()
);

-- Operations already granted by an association are registered, so turning on
-- strict operations never rejects an existing grant.
INSERT INTO ngac_operations (name)
SELECT DISTINCT unnest(operations) AS op
FROM ngac_associations
ON CONFLICT (name) DO NOTHING;
