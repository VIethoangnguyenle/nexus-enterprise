-- 065_tenant_users_ngac_node_index.sql
--
-- Lookups from a graph node back to its membership row (`tenant_users.ngac_node_id`)
-- had no index and scanned the table. Index-only change; no data is rewritten and the
-- statement is safe to re-run.
--
-- Reverse:
--   DROP INDEX IF EXISTS idx_tenant_users_ngac_node;

CREATE INDEX IF NOT EXISTS idx_tenant_users_ngac_node ON tenant_users(ngac_node_id);
