-- 005_check_access_function.sql
-- CTE-based NGAC access check function.
-- Implements BFS traversal + ALL-PC intersection per NIST NGAC spec.
-- Used as fallback when in-memory graph shard is not loaded.

-- Helper: find all ancestors of a node via recursive CTE.
CREATE OR REPLACE FUNCTION ngac_ancestors(p_node_id TEXT)
RETURNS TABLE(node_id TEXT, node_type TEXT) AS $$
    WITH RECURSIVE ancestors AS (
        -- Base: the node itself
        SELECT n.id AS node_id, n.node_type
        FROM ngac_nodes n
        WHERE n.id = p_node_id

        UNION

        -- Recurse: follow assignment edges upward (child → parent)
        SELECT n.id AS node_id, n.node_type
        FROM ngac_assignments a
        JOIN ngac_nodes n ON n.id = a.parent_id
        JOIN ancestors anc ON anc.node_id = a.child_id
    )
    SELECT ancestors.node_id, ancestors.node_type FROM ancestors;
$$ LANGUAGE SQL STABLE;

-- Main access check function.
-- Returns TRUE if user has the requested operation on the target object.
--
-- Algorithm (mirrors in-memory BFS in access.go):
-- 1. Collect all UAs and PCs reachable from user (upward BFS)
-- 2. Collect all OAs and PCs reachable from object (upward BFS)
-- 3. ALL-PC intersection: every PC the object reaches, the user must also reach
-- 4. Find at least one association linking a user-UA to an object-OA with the operation
CREATE OR REPLACE FUNCTION ngac_check_access(
    p_user_id TEXT,
    p_object_id TEXT,
    p_operation TEXT
) RETURNS BOOLEAN AS $$
DECLARE
    v_object_pc_count INT;
    v_common_pc_count INT;
BEGIN
    -- Step 1: Collect user's reachable UAs
    CREATE TEMP TABLE IF NOT EXISTS _user_uas (node_id TEXT PRIMARY KEY) ON COMMIT DROP;
    TRUNCATE _user_uas;
    INSERT INTO _user_uas (node_id)
    SELECT a.node_id FROM ngac_ancestors(p_user_id) a
    WHERE a.node_type = 'UA'
    ON CONFLICT DO NOTHING;

    -- Step 2: Collect user's reachable PCs
    CREATE TEMP TABLE IF NOT EXISTS _user_pcs (node_id TEXT PRIMARY KEY) ON COMMIT DROP;
    TRUNCATE _user_pcs;
    INSERT INTO _user_pcs (node_id)
    SELECT a.node_id FROM ngac_ancestors(p_user_id) a
    WHERE a.node_type = 'PC'
    ON CONFLICT DO NOTHING;

    -- Step 3: Collect object's reachable OAs
    CREATE TEMP TABLE IF NOT EXISTS _object_oas (node_id TEXT PRIMARY KEY) ON COMMIT DROP;
    TRUNCATE _object_oas;
    INSERT INTO _object_oas (node_id)
    SELECT a.node_id FROM ngac_ancestors(p_object_id) a
    WHERE a.node_type = 'OA'
    ON CONFLICT DO NOTHING;
    -- Include the object itself if it's an OA
    INSERT INTO _object_oas (node_id)
    SELECT p_object_id FROM ngac_nodes WHERE id = p_object_id AND node_type = 'OA'
    ON CONFLICT DO NOTHING;

    -- Step 4: Collect object's reachable PCs
    CREATE TEMP TABLE IF NOT EXISTS _object_pcs (node_id TEXT PRIMARY KEY) ON COMMIT DROP;
    TRUNCATE _object_pcs;
    INSERT INTO _object_pcs (node_id)
    SELECT a.node_id FROM ngac_ancestors(p_object_id) a
    WHERE a.node_type = 'PC'
    ON CONFLICT DO NOTHING;

    -- Step 5: ALL-PC intersection check
    -- Every PC the object reaches, the user must also reach
    SELECT COUNT(*) INTO v_object_pc_count FROM _object_pcs;
    SELECT COUNT(*) INTO v_common_pc_count
    FROM _object_pcs op
    WHERE EXISTS (SELECT 1 FROM _user_pcs up WHERE up.node_id = op.node_id);

    IF v_object_pc_count = 0 OR v_common_pc_count < v_object_pc_count THEN
        RETURN FALSE;
    END IF;

    -- Step 6: Find matching association (UA → OA with operation)
    RETURN EXISTS (
        SELECT 1
        FROM ngac_associations assoc
        WHERE assoc.ua_id IN (SELECT node_id FROM _user_uas)
          AND assoc.oa_id IN (SELECT node_id FROM _object_oas)
          AND p_operation = ANY(assoc.operations)
    );
END;
$$ LANGUAGE plpgsql STABLE;
