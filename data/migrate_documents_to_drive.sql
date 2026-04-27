-- migrate_documents_to_drive.sql
-- Migrates existing documents table records into drive_items table.
-- Run this ONCE after deploying Drive Service.
-- Prerequisite: workspace root drive folders must already exist (created by CreateWorkspace).

-- Step 1: Ensure every workspace has a root drive folder.
-- This creates root drive_item folders for workspaces that were created
-- before Drive Service was deployed.
INSERT INTO drive_items (id, workspace_id, drive_context, drive_context_id, parent_id, item_type, name, ngac_node_id, owner_id, status, created_at)
SELECT
    'root-ws-' || w.id,           -- deterministic id
    w.id,
    'workspace',
    w.id,
    NULL,                         -- root has no parent
    'folder',
    w.name || '_Root',
    COALESCE(w.ngac_pc_id, 'unknown'),  -- use workspace PC as the NGAC node
    w.owner_id,
    'active',
    w.created_at
FROM workspaces w
WHERE NOT EXISTS (
    SELECT 1 FROM drive_items di
    WHERE di.workspace_id = w.id
      AND di.drive_context = 'workspace'
      AND di.parent_id IS NULL
      AND di.item_type = 'folder'
      AND di.status = 'active'
)
ON CONFLICT (id) DO NOTHING;

-- Step 2: Migrate each document as a file in its workspace's root drive folder.
-- Maps documents → drive_items:
--   - title → name
--   - filename → stored as object_key prefix
--   - id → storage_doc_id (reference back to document storage)
--   - ngac_node → ngac_node_id
INSERT INTO drive_items (id, workspace_id, drive_context, drive_context_id, parent_id, item_type, name, mime_type, size_bytes, object_key, storage_doc_id, ngac_node_id, owner_id, status, created_at)
SELECT
    'migrated-' || d.id,                    -- deterministic id for idempotency
    d.workspace_id,
    'workspace',
    d.workspace_id,
    'root-ws-' || d.workspace_id,           -- parent = workspace root folder
    'file',
    COALESCE(d.title, d.filename),
    d.mime_type,
    0,                                       -- size unknown from old schema
    'drive/' || d.id || '/' || d.filename,  -- reconstruct object key
    d.id,                                    -- link back to document storage
    COALESCE(d.ngac_node, 'unknown'),
    COALESCE(d.owner_id, 'unknown'),
    CASE d.status
        WHEN 'draft' THEN 'active'
        WHEN 'approved' THEN 'active'
        WHEN 'published' THEN 'active'
        ELSE 'active'
    END,
    d.created_at
FROM documents d
WHERE d.workspace_id IS NOT NULL
  AND NOT EXISTS (
    SELECT 1 FROM drive_items di WHERE di.id = 'migrated-' || d.id
)
ON CONFLICT (id) DO NOTHING;

-- Step 3: Update quota counters for each workspace.
INSERT INTO drive_quotas (workspace_id, max_bytes, used_bytes, max_files, used_files, updated_at)
SELECT
    di.workspace_id,
    -1,                                    -- unlimited by default
    COALESCE(SUM(di.size_bytes), 0),
    -1,                                    -- unlimited by default
    COUNT(*)::INT,
    NOW()
FROM drive_items di
WHERE di.item_type = 'file'
  AND di.status = 'active'
GROUP BY di.workspace_id
ON CONFLICT (workspace_id) DO UPDATE SET
    used_bytes = EXCLUDED.used_bytes,
    used_files = EXCLUDED.used_files,
    updated_at = NOW();

-- Verification query (run manually to confirm):
-- SELECT w.name, COUNT(di.id) as drive_files
-- FROM workspaces w
-- LEFT JOIN drive_items di ON di.workspace_id = w.id AND di.item_type = 'file'
-- GROUP BY w.name;
