-- Migration: Add materialized access cache and graph versioning
-- Part of scalable-ngac-engine change: enables DB-driven evaluation

-- Optimized composite index for recursive CTE traversal (child→parent direction)
CREATE INDEX IF NOT EXISTS idx_assignments_child_parent ON ngac_assignments(child_id, parent_id);

-- Graph version tracking: enables version-based cache invalidation instead of full flush
CREATE TABLE IF NOT EXISTS ngac_graph_version (
    scope      TEXT PRIMARY KEY,  -- 'global' or 'ws:{workspace_id}'
    version    BIGINT NOT NULL DEFAULT 0,
    updated_at TIMESTAMPTZ DEFAULT NOW()
);

INSERT INTO ngac_graph_version (scope, version) VALUES ('global', 0)
    ON CONFLICT (scope) DO NOTHING;

-- Materialized access decisions: pre-computed cache for frequent access checks
CREATE TABLE IF NOT EXISTS ngac_materialized_access (
    user_node_id   TEXT NOT NULL,
    object_node_id TEXT NOT NULL,
    operation      TEXT NOT NULL,
    decision       BOOLEAN NOT NULL,
    graph_version  BIGINT NOT NULL,
    computed_at    TIMESTAMPTZ DEFAULT NOW(),
    PRIMARY KEY (user_node_id, object_node_id, operation)
);

CREATE INDEX IF NOT EXISTS idx_materialized_user ON ngac_materialized_access(user_node_id);
CREATE INDEX IF NOT EXISTS idx_materialized_object ON ngac_materialized_access(object_node_id);

-- SQL function: get all ancestor node IDs via recursive CTE
-- Replaces in-memory BFS for DB-driven evaluation
CREATE OR REPLACE FUNCTION ngac_ancestors(start_id TEXT)
RETURNS TABLE(node_id TEXT, node_type TEXT) AS $$
    WITH RECURSIVE ancestors AS (
        SELECT a.parent_id AS node_id
        FROM ngac_assignments a
        WHERE a.child_id = start_id
        UNION
        SELECT a.parent_id
        FROM ngac_assignments a
        JOIN ancestors anc ON a.child_id = anc.node_id
    )
    SELECT anc.node_id, n.node_type
    FROM ancestors anc
    JOIN ngac_nodes n ON anc.node_id = n.id;
$$ LANGUAGE SQL STABLE;

-- SQL function: full NGAC access check via CTE
-- Returns true if the user has the requested operation on the object
CREATE OR REPLACE FUNCTION ngac_check_access(
    p_user_id TEXT, p_object_id TEXT, p_operation TEXT
) RETURNS BOOLEAN AS $$
    WITH
    user_ancestors AS (
        SELECT node_id, node_type FROM ngac_ancestors(p_user_id)
        UNION ALL
        SELECT p_user_id AS node_id, n.node_type FROM ngac_nodes n WHERE n.id = p_user_id
    ),
    object_ancestors AS (
        SELECT node_id, node_type FROM ngac_ancestors(p_object_id)
        UNION ALL
        SELECT p_object_id AS node_id, n.node_type FROM ngac_nodes n WHERE n.id = p_object_id
    ),
    user_pcs AS (
        SELECT node_id FROM user_ancestors WHERE node_type = 'PC'
    ),
    object_pcs AS (
        SELECT node_id FROM object_ancestors WHERE node_type = 'PC'
    )
    SELECT EXISTS (
        SELECT 1
        FROM ngac_associations assoc
        WHERE assoc.ua_id IN (SELECT node_id FROM user_ancestors WHERE node_type IN ('UA', 'U'))
          AND assoc.oa_id IN (SELECT node_id FROM object_ancestors WHERE node_type IN ('OA', 'O'))
          AND p_operation = ANY(assoc.operations)
          AND EXISTS (
              SELECT 1 FROM user_pcs WHERE node_id IN (SELECT node_id FROM object_pcs)
          )
    );
$$ LANGUAGE SQL STABLE;
