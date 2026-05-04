-- 001_core_schema.sql
-- Core NGAC graph tables: nodes, assignments, associations
-- These are the minimum tables needed for any NGAC deployment.

CREATE TABLE IF NOT EXISTS ngac_nodes (
    id         TEXT PRIMARY KEY,
    name       TEXT NOT NULL,
    node_type  TEXT NOT NULL CHECK (node_type IN ('U','UA','O','OA','PC')),
    properties JSONB DEFAULT '{}',
    created_at TIMESTAMPTZ DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS ngac_assignments (
    id        TEXT PRIMARY KEY,
    child_id  TEXT NOT NULL REFERENCES ngac_nodes(id) ON DELETE CASCADE,
    parent_id TEXT NOT NULL REFERENCES ngac_nodes(id) ON DELETE CASCADE,
    UNIQUE(child_id, parent_id)
);

CREATE TABLE IF NOT EXISTS ngac_associations (
    id         TEXT PRIMARY KEY,
    ua_id      TEXT NOT NULL REFERENCES ngac_nodes(id) ON DELETE CASCADE,
    oa_id      TEXT NOT NULL REFERENCES ngac_nodes(id) ON DELETE CASCADE,
    operations TEXT[] NOT NULL,
    UNIQUE(ua_id, oa_id)
);

CREATE INDEX IF NOT EXISTS idx_assignments_child ON ngac_assignments(child_id);
CREATE INDEX IF NOT EXISTS idx_assignments_parent ON ngac_assignments(parent_id);
CREATE INDEX IF NOT EXISTS idx_associations_ua ON ngac_associations(ua_id);
CREATE INDEX IF NOT EXISTS idx_associations_oa ON ngac_associations(oa_id);
CREATE INDEX IF NOT EXISTS idx_nodes_type ON ngac_nodes(node_type);
CREATE INDEX IF NOT EXISTS idx_nodes_name ON ngac_nodes(name);
