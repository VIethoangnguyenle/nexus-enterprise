-- 023_ngac_id_keyed_names.sql
--
-- Renames platform-built NGAC nodes whose names were derived from something a
-- tenant chooses (a role, department, folder, file or channel name, a username)
-- to names keyed by the entity's own ID, as backend/ngac now builds them.
--
-- Why: the graph resolves a node by exact name and keeps one node per name and
-- type. Two tenants who each have a department "Sales" shared the name
-- "Dept_Sales", and a lookup by that name landed on whichever was written last.
--
--   role UA                    <name>                -> Role_<node id>        (display_name = old name)
--   department UA              Dept_<name>           -> Dept_<departments.id>
--   workspace folder OA        <name>                -> Folder_<node id>      (display_name = old name)
--   drive folder OA            Folder_<name>         -> Folder_<drive_items.id>
--   channel / workspace drive  Ch_<name>_Drive       -> Ch_<channel id>_Drive
--   share OA                   Share_<item>_<8 hex>  -> Share_<drive_shares.id>
--   user U node                <username>            -> U_<users.id>          (display_name = username)
--   asset type OA              <ws>_Type_<name>      -> <ws>_Type_<asset_types.id>
--
-- Every rename is matched through the foreign key that ties the node to its
-- entity (departments.ngac_ua_id, drive_items.ngac_node_id, ...) and checked
-- against the entity's own workspace, so a node is never renamed on a name match
-- alone. The old name is kept: as the node's display_name property where the UI
-- shows it, and in ngac_node_renames, which is also the rollback:
--
--     UPDATE ngac_nodes n SET name = r.old_name
--       FROM ngac_node_renames r WHERE r.node_id = n.id AND n.name = r.new_name;
--
-- That restores NAMES only. It does not undo the properties this file adds
-- (display_name and the like) or the drive_items names it changes (channel
-- drive items now show the channel's name); restore those from the pre-migration
-- dump if they matter.
--
-- Idempotent: a renamed node no longer matches its rule, and a rename is
-- recorded once per (node, new name).
--
-- NOTE: writes ngac_nodes directly and so bypasses EPP invalidation. The policy
-- service indexes nodes by name in memory; restart the services after applying
-- (policy first), or lookups keep resolving the old names.

BEGIN;

CREATE TABLE IF NOT EXISTS ngac_node_renames (
    node_id     TEXT        NOT NULL REFERENCES ngac_nodes(id) ON DELETE CASCADE,
    old_name    TEXT        NOT NULL,
    new_name    TEXT        NOT NULL,
    props_patch JSONB       NOT NULL DEFAULT '{}',
    reason      TEXT        NOT NULL,
    recorded_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (node_id, new_name)
);

-- ---------------------------------------------------------------------------
-- 1. Plan: one row per node to rename, from the FK that identifies its entity.
-- ---------------------------------------------------------------------------

-- Departments: Dept_<name> -> Dept_<departments.id>
INSERT INTO ngac_node_renames (node_id, old_name, new_name, props_patch, reason)
SELECT n.id, n.name, 'Dept_' || d.id,
       jsonb_build_object('display_name', d.name, 'dept_name', d.name, 'workspace_id', d.workspace_id),
       'department'
FROM departments d
JOIN ngac_nodes n ON n.id = d.ngac_ua_id
WHERE n.node_type = 'UA'
  AND n.name <> 'Dept_' || d.id
  AND (n.properties->>'workspace_id' IS NULL OR n.properties->>'workspace_id' = d.workspace_id)
ON CONFLICT DO NOTHING;

-- Roles: UAs directly under a workspace PC that the platform did not name itself.
INSERT INTO ngac_node_renames (node_id, old_name, new_name, props_patch, reason)
SELECT n.id, n.name, 'Role_' || n.id,
       jsonb_build_object('type', 'role', 'display_name', n.name, 'workspace_id', w.id),
       'role'
FROM ngac_nodes n
JOIN ngac_assignments a ON a.child_id = n.id
JOIN workspaces w ON w.ngac_pc_id = a.parent_id
WHERE n.node_type = 'UA'
  AND COALESCE(n.properties->>'type', '') NOT IN ('personal_ua', 'role')
  AND n.name !~* '^(user_|pc_|tenantmember_|tenantowner_|dept_|ch_|driveroot_|folder_|share_|asset_|role_)'
  AND n.name !~* '(_owners|_members|_mgmt|_documents|_draftdocs|_approveddocs|_channels|_assets|_content|_drive)$'
  AND lower(n.name) NOT IN ('pc_global', 'publicusers')
ON CONFLICT DO NOTHING;

-- Drive folders: Folder_<name> -> Folder_<drive_items.id>. Files share their
-- folder's node, so the folder row is the one folder item that points at it.
INSERT INTO ngac_node_renames (node_id, old_name, new_name, props_patch, reason)
SELECT n.id, n.name, 'Folder_' || di.id,
       jsonb_build_object('display_name', di.name),
       'drive folder'
FROM drive_items di
JOIN ngac_nodes n ON n.id = di.ngac_node_id
WHERE di.item_type = 'folder'
  AND n.node_type = 'OA'
  AND n.properties->>'type' = 'drive_folder'
  AND n.name LIKE 'Folder\_%'
  AND n.name <> 'Folder_' || di.id
  AND (n.properties->>'workspace_id' IS NULL OR n.properties->>'workspace_id' = di.workspace_id)
  AND (SELECT count(*) FROM drive_items d2 WHERE d2.ngac_node_id = n.id AND d2.item_type = 'folder') = 1
ON CONFLICT DO NOTHING;

-- Channel and workspace drives: Ch_<name>_Drive -> Ch_<channel id>_Drive. The
-- drive row names the context (a channel of the same workspace, or the
-- workspace itself for its root drive).
INSERT INTO ngac_node_renames (node_id, old_name, new_name, props_patch, reason)
SELECT n.id, n.name, 'Ch_' || di.drive_context_id || '_Drive',
       jsonb_build_object('display_name',
           COALESCE((SELECT c.name FROM channels c WHERE c.id = di.drive_context_id),
                    (SELECT w.name FROM workspaces w WHERE w.id = di.drive_context_id), di.name),
           'channel_id', di.drive_context_id),
       'channel drive'
FROM drive_items di
JOIN ngac_nodes n ON n.id = di.ngac_node_id
WHERE di.drive_context = 'channel'
  AND di.item_type = 'folder'
  AND di.parent_id IS NULL
  AND n.node_type = 'OA'
  AND n.name LIKE 'Ch\_%\_Drive'
  AND n.name <> 'Ch_' || di.drive_context_id || '_Drive'
  AND (EXISTS (SELECT 1 FROM channels c WHERE c.id = di.drive_context_id AND c.workspace_id = di.workspace_id)
       OR di.drive_context_id = di.workspace_id)
  AND (SELECT count(*) FROM drive_items d2
        WHERE d2.ngac_node_id = n.id AND d2.drive_context = 'channel' AND d2.parent_id IS NULL) = 1
ON CONFLICT DO NOTHING;

-- Share OAs: Share_<item>_<8 hex> -> Share_<drive_shares.id>
INSERT INTO ngac_node_renames (node_id, old_name, new_name, props_patch, reason)
SELECT n.id, n.name, 'Share_' || s.id,
       jsonb_build_object('display_name', di.name, 'workspace_id', di.workspace_id),
       'share'
FROM drive_shares s
JOIN drive_items di ON di.id = s.drive_item_id
JOIN ngac_nodes n ON n.id = s.ngac_share_oa
WHERE n.node_type = 'OA'
  AND n.name LIKE 'Share\_%'
  AND n.name <> 'Share_' || s.id
ON CONFLICT DO NOTHING;

-- User nodes: <username> -> U_<users.id>, when exactly one user owns the node.
INSERT INTO ngac_node_renames (node_id, old_name, new_name, props_patch, reason)
SELECT n.id, n.name, 'U_' || u.id,
       jsonb_build_object('display_name', u.username, 'user_id', u.id),
       'user'
FROM users u
JOIN ngac_nodes n ON n.id = u.ngac_node
WHERE n.node_type = 'U'
  AND n.name <> 'U_' || u.id
  AND (SELECT count(*) FROM users u2 WHERE u2.ngac_node = n.id) = 1
ON CONFLICT DO NOTHING;

-- Asset type OAs: <ws>_Type_<sanitized name> -> <ws>_Type_<asset_types.id>
INSERT INTO ngac_node_renames (node_id, old_name, new_name, props_patch, reason)
SELECT n.id, n.name, at.workspace_id || '_Type_' || at.id,
       jsonb_build_object('display_name', at.name, 'asset_type_id', at.id, 'workspace_id', at.workspace_id),
       'asset type'
FROM asset_types at
JOIN ngac_nodes n ON n.id = at.ngac_oa_id
WHERE n.node_type = 'OA'
  AND left(n.name, length(at.workspace_id || '_Type_')) = at.workspace_id || '_Type_'
  AND n.name <> at.workspace_id || '_Type_' || at.id
ON CONFLICT DO NOTHING;

-- Workspace folders: OAs below a workspace PC that nothing else owns (no drive
-- item, channel, asset type or Documents reference) and the platform did not
-- name itself. Only OAs reachable from exactly one workspace.
INSERT INTO ngac_node_renames (node_id, old_name, new_name, props_patch, reason)
WITH RECURSIVE tree(id, ws_id) AS (
    SELECT a.child_id, w.id
    FROM workspaces w
    JOIN ngac_assignments a ON a.parent_id = w.ngac_pc_id
    UNION
    SELECT a.child_id, t.ws_id
    FROM ngac_assignments a
    JOIN tree t ON a.parent_id = t.id
), single AS (
    SELECT id, min(ws_id) AS ws_id FROM tree GROUP BY id HAVING count(DISTINCT ws_id) = 1
)
SELECT n.id, n.name, 'Folder_' || n.id,
       jsonb_build_object('display_name', n.name, 'workspace_id', s.ws_id),
       'workspace folder'
FROM ngac_nodes n
JOIN single s ON s.id = n.id
WHERE n.node_type = 'OA'
  AND n.name !~ '^(Ch_|Folder_|Share_|DriveRoot_|Asset_)'
  AND n.name !~ '_(Mgmt|Documents|DraftDocs|ApprovedDocs|Channels|Assets|Content|Drive)$'
  AND n.name !~ '_(Category|Type)_'
  AND NOT EXISTS (SELECT 1 FROM drive_items di WHERE di.ngac_node_id = n.id OR di.scope_oa_id = n.id)
  AND NOT EXISTS (SELECT 1 FROM channels c WHERE c.ngac_oa_id = n.id)
  AND NOT EXISTS (SELECT 1 FROM asset_types at WHERE at.ngac_oa_id = n.id)
  AND NOT EXISTS (SELECT 1 FROM workspaces w WHERE w.documents_oa_id = n.id)
ON CONFLICT DO NOTHING;

-- ---------------------------------------------------------------------------
-- 2. Apply: rename every planned node that still carries its old name, and fold
--    the patch into its properties without overwriting a display_name it has.
-- ---------------------------------------------------------------------------

UPDATE ngac_nodes n
SET name = r.new_name,
    properties = r.props_patch || COALESCE(n.properties, '{}'::jsonb)
                 || CASE WHEN COALESCE(n.properties->>'display_name', '') = ''
                         THEN jsonb_build_object('display_name', r.props_patch->>'display_name')
                         ELSE '{}'::jsonb END
FROM ngac_node_renames r
WHERE r.node_id = n.id
  AND n.name = r.old_name
  AND n.name <> r.new_name;

-- Channel drive items show the channel's name, not the node name they were
-- created with.
UPDATE drive_items di
SET name = COALESCE((SELECT c.name FROM channels c WHERE c.id = di.drive_context_id),
                    (SELECT w.name FROM workspaces w WHERE w.id = di.drive_context_id), di.name)
WHERE di.drive_context = 'channel'
  AND di.item_type = 'folder'
  AND di.parent_id IS NULL
  AND di.name ~ '^Ch_.*_Drive$';

-- The graph changed underneath the decision caches: bump the version and drop
-- the materialised rows so a restarted service cannot serve an older answer.
UPDATE ngac_graph_version SET version = version + 1, updated_at = NOW();
TRUNCATE ngac_materialized_access;

COMMIT;
