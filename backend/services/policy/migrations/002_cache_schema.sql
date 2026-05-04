-- 002_cache_schema.sql
-- Cache and version tracking tables for 3-layer access caching.
-- L2: ngac_materialized_access — persisted decision cache
-- Version: ngac_graph_version — tracks graph mutation version for cache freshness

CREATE TABLE IF NOT EXISTS ngac_materialized_access (
    user_node_id   TEXT NOT NULL,
    object_node_id TEXT NOT NULL,
    operation      TEXT NOT NULL,
    decision       BOOLEAN NOT NULL,
    graph_version  BIGINT NOT NULL DEFAULT 0,
    computed_at    TIMESTAMPTZ DEFAULT NOW(),
    PRIMARY KEY (user_node_id, object_node_id, operation)
);

CREATE TABLE IF NOT EXISTS ngac_graph_version (
    scope   TEXT PRIMARY KEY DEFAULT 'global',
    version BIGINT NOT NULL DEFAULT 0
);

-- Seed initial version if not exists
INSERT INTO ngac_graph_version (scope, version) VALUES ('global', 0)
ON CONFLICT (scope) DO NOTHING;

CREATE INDEX IF NOT EXISTS idx_materialized_user ON ngac_materialized_access(user_node_id);
CREATE INDEX IF NOT EXISTS idx_materialized_object ON ngac_materialized_access(object_node_id);
