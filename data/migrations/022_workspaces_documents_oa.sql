-- 022_workspaces_documents_oa.sql
-- A workspace remembers its Documents OA.
--
-- The drive roots itself on that OA. It used to find it by scanning the PC's
-- children for a name containing "Documents" or "Docs" and falling back to the
-- first OA it met, which depends on the order the graph returns children in and
-- can pick a folder somebody named "Docs". A stored reference does not guess.
--
-- Backfill matches the OA the platform names itself, "<workspace id>_Documents",
-- which is keyed by the workspace's own ID. Idempotent: rows already set, and
-- workspaces with no such OA, are left alone.

ALTER TABLE workspaces
    ADD COLUMN IF NOT EXISTS documents_oa_id TEXT REFERENCES ngac_nodes(id) ON DELETE SET NULL;

UPDATE workspaces w
   SET documents_oa_id = n.id
  FROM ngac_nodes n
 WHERE w.documents_oa_id IS NULL
   AND n.node_type = 'OA'
   AND n.name = w.id || '_Documents';
