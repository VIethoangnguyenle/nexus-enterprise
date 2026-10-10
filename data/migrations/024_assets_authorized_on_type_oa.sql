-- 024_assets_authorized_on_type_oa.sql
--
-- Assets are no longer graph nodes. The graph holds attributes, not objects:
-- an asset is authorized on the OA of its type (asset_types.ngac_oa_id), and a
-- grant on the Assets or a category OA reaches it from above. See
-- docs/specs/asset-authorization/spec.md.
--
--   1. Every O node, and with it its assignments and associations, is removed.
--      The only O nodes the platform ever created were per-asset ones, and the
--      PDP loads U, UA, OA and PC only, so each check on one fell through to the
--      SQL fallback.
--   2. assets.ngac_node_id (the foreign key onto that node) is dropped. The
--      asset's authorization OA is read from its type; keeping a second copy on
--      the asset would be one more thing that can disagree.
--   3. The Assets OA of every workspace hangs under the workspace's policy class
--      and nothing else, and the global PC_AssetManagement class is removed.
--      An access needs the user to reach every policy class the object reaches;
--      no workspace UA is assigned to PC_AssetManagement, so while the Assets OA
--      was under it the in-memory graph denied every asset check to everyone.
--      (The SQL fallback used to answer with an any-PC rule, which hid that.)
--
--   4. Legacy blanket grants on the Assets OA are removed. Before 9389e25
--      (2026-08-02) the asset hierarchy associated EVERY UA under the workspace
--      PC (Members, TenantMember, department UAs, channel Members UAs, roles)
--      with <ws>_Assets for all eight operations. Nothing removed them. They were
--      inert only because PC_AssetManagement made the in-memory PDP deny; once
--      that class is gone (step 3) they would give every member manage and
--      approve over every asset. An association on <ws>_Assets is deleted when
--      its UA is not <ws>_Owners and it carries the full owner operation set,
--      which is exactly what the old loop wrote. A partial grant (read, say)
--      made later through the permissions API is a deliberate grant and stays.
--      A full-set grant someone made on purpose to a non-owner UA is
--      indistinguishable from the legacy rows and is removed with them; re-grant
--      it through the permissions API if it was meant.
--   5. The decision caches are reset (see the end of the file).
--
-- Idempotent: nothing is left for a second run to match.
--
-- NOTE: writes the graph tables directly and so bypasses EPP invalidation.
-- Restart the services (policy first) after applying.
--
-- Rollback is the pre-migration dump: the dropped column and the deleted O nodes
-- are not reconstructed by this file.

BEGIN;

-- The Assets OA stays reachable from its workspace's policy class.
INSERT INTO ngac_assignments (id, child_id, parent_id)
SELECT gen_random_uuid()::text, oa.id, w.ngac_pc_id
FROM ngac_nodes oa
JOIN workspaces w ON oa.name = w.id || '_Assets'
WHERE oa.node_type = 'OA'
  AND w.ngac_pc_id IS NOT NULL
ON CONFLICT (child_id, parent_id) DO NOTHING;

-- 1 + 2. Detach assets from their O nodes, then remove every O node.
DO $$
BEGIN
    IF EXISTS (SELECT 1 FROM information_schema.columns
                WHERE table_schema = current_schema() AND table_name = 'assets' AND column_name = 'ngac_node_id') THEN
        EXECUTE 'UPDATE assets SET ngac_node_id = NULL WHERE ngac_node_id IS NOT NULL';
    END IF;
END $$;

DELETE FROM ngac_nodes WHERE node_type = 'O';

ALTER TABLE assets DROP COLUMN IF EXISTS ngac_node_id;

-- 4. Legacy blanket grants (see header). Done before the class is removed so
-- there is no moment at which they are live.
DELETE FROM ngac_associations a
USING ngac_nodes oa, ngac_nodes ua
WHERE a.oa_id = oa.id
  AND a.ua_id = ua.id
  AND oa.node_type = 'OA'
  AND oa.name LIKE '%\_Assets'
  AND ua.name <> left(oa.name, length(oa.name) - length('_Assets')) || '_Owners'
  AND a.operations @> ARRAY['read','write','upload','approve','share','manage','invite','create_channel'];

-- 3. No second policy class over the asset tree. Deleting the class removes its
-- assignments and associations with it.
DELETE FROM ngac_nodes WHERE node_type = 'PC' AND name = 'PC_AssetManagement';

-- Verification: no UA other than the workspace Owners holds the full operation
-- set on an Assets OA. Fails the migration (and rolls it back) if one does.
DO $$
DECLARE n integer;
BEGIN
    SELECT count(*) INTO n
    FROM ngac_associations a
    JOIN ngac_nodes oa ON oa.id = a.oa_id
    JOIN ngac_nodes ua ON ua.id = a.ua_id
    WHERE oa.node_type = 'OA'
      AND oa.name LIKE '%\_Assets'
      AND ua.name <> left(oa.name, length(oa.name) - length('_Assets')) || '_Owners'
      AND a.operations @> ARRAY['read','write','upload','approve','share','manage','invite','create_channel'];
    IF n > 0 THEN
        RAISE EXCEPTION 'migration 024: % blanket association(s) on an Assets OA remain', n;
    END IF;
END $$;

-- The graph changed underneath the decision caches: bump the version so a
-- restarted service cannot serve a materialised answer from before this file,
-- and drop the materialised rows themselves.
UPDATE ngac_graph_version SET version = version + 1, updated_at = NOW();
TRUNCATE ngac_materialized_access;

COMMIT;
