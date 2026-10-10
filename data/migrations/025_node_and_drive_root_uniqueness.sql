-- 025_node_and_drive_root_uniqueness.sql
--
-- Two uniqueness guarantees the provisioning code relies on, so that two
-- concurrent runs cannot both create "the" node or "the" root.
--
-- 1. Platform singleton node names. The graph keeps one node per (name, type)
--    in memory and resolves nodes by exact name, but the table allowed any
--    number of rows. A check-then-create by two runs could leave two nodes for
--    one name. The names below are keyed by a workspace, tenant or channel ID,
--    so each must exist at most once per type:
--        TenantMember_*  TenantOwner_*  DriveRoot_*  *_Assets  *_Drive  *_Category_*
--    (Type OAs are keyed by a fresh type ID, personal UAs have their own index
--    in 020.) A create that loses the race fails on this index, and the
--    provisioning code then looks the winner up.
--
-- 2. One drive root per drive context. A root used to be "the top-level folder
--    of the context", but top-level user folders are parent-less too, so
--    nothing could say which row was the root and nothing stopped two. A root is
--    now marked (drive_items.is_root); at most one active root exists per
--    (workspace, context, context id).
--
-- Existing data was checked for duplicates first (none in the dev and CI
-- databases). If a database does hold some, this file refuses to run rather than
-- pick one: resolve them by hand, then re-run.
--
-- Backfill of is_root: for each (workspace, context, context id) with a
-- non-empty context id, the OLDEST active parent-less folder is the root, which
-- is the order ensureRoot and CreateDriveForChannel created it in. Idempotent.

BEGIN;

DO $$
DECLARE n integer;
BEGIN
    SELECT count(*) INTO n FROM (
        SELECT name, node_type FROM ngac_nodes
        WHERE name ~ '^(TenantMember_|TenantOwner_|DriveRoot_)' OR name ~ '_(Assets|Drive)$' OR name ~ '_Category_'
        GROUP BY name, node_type HAVING count(*) > 1
    ) d;
    IF n > 0 THEN
        RAISE EXCEPTION 'migration 025: % duplicated platform node name(s); resolve them and re-run', n;
    END IF;
END $$;

CREATE UNIQUE INDEX IF NOT EXISTS ngac_nodes_platform_name_uniq
    ON ngac_nodes (name, node_type)
    WHERE name ~ '^(TenantMember_|TenantOwner_|DriveRoot_)' OR name ~ '_(Assets|Drive)$' OR name ~ '_Category_';

ALTER TABLE drive_items ADD COLUMN IF NOT EXISTS is_root BOOLEAN NOT NULL DEFAULT FALSE;

UPDATE drive_items di
SET is_root = TRUE
FROM (
    SELECT DISTINCT ON (workspace_id, drive_context, drive_context_id) id
    FROM drive_items
    WHERE parent_id IS NULL AND item_type = 'folder' AND status = 'active'
      AND COALESCE(drive_context_id, '') <> ''
    ORDER BY workspace_id, drive_context, drive_context_id, created_at, id
) r
WHERE di.id = r.id AND NOT di.is_root;

CREATE UNIQUE INDEX IF NOT EXISTS drive_items_one_root_per_context
    ON drive_items (workspace_id, drive_context, COALESCE(drive_context_id, ''))
    WHERE is_root AND parent_id IS NULL AND item_type = 'folder' AND status = 'active';

COMMIT;
