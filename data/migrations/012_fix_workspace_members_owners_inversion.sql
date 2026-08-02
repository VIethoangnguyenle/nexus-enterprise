-- 012_fix_workspace_members_owners_inversion.sql
--
-- Workspace bootstrap assigned the Members UA *under* the Owners UA
-- ({wsID}_Members -> {wsID}_Owners). NGAC derives privilege by walking
-- child -> parent, so that made every workspace member reach the Owners UA and
-- inherit its whole association set: manage, invite, approve, share and write
-- on the Mgmt, Documents and Channels OAs. The narrow member-scoped
-- associations created alongside it were dead code.
--
-- The assignment is meant to run the other way: Owners is assigned under
-- Members so that an owner picks up the member grants in addition to their own.
--
-- The code path is fixed in backend/services/workspace/internal/domain/service.go;
-- this repairs workspaces created before that fix.
--
-- NOTE: this writes ngac_assignments directly and therefore does not travel the
-- EPP invalidation path. Runtime decisions are served from the in-memory graph,
-- so every service must be restarted (or its graph invalidated) after this
-- migration is applied, or they will keep serving the old, wider decisions.

BEGIN;

-- 1. Add the correct Owners -> Members assignment wherever the inverted pair
--    exists and the correct one does not.
INSERT INTO ngac_assignments (id, child_id, parent_id)
SELECT gen_random_uuid()::text, owners.id, members.id
FROM ngac_assignments a
JOIN ngac_nodes members ON members.id = a.child_id
JOIN ngac_nodes owners  ON owners.id  = a.parent_id
WHERE members.node_type = 'UA'
  AND owners.node_type  = 'UA'
  AND members.name LIKE '%\_Members'
  AND owners.name  LIKE '%\_Owners'
  AND left(members.name, length(members.name) - 8) = left(owners.name, length(owners.name) - 7)
ON CONFLICT (child_id, parent_id) DO NOTHING;

-- 2. Remove the inverted Members -> Owners assignment.
DELETE FROM ngac_assignments a
USING ngac_nodes members, ngac_nodes owners
WHERE members.id = a.child_id
  AND owners.id  = a.parent_id
  AND members.node_type = 'UA'
  AND owners.node_type  = 'UA'
  AND members.name LIKE '%\_Members'
  AND owners.name  LIKE '%\_Owners'
  AND left(members.name, length(members.name) - 8) = left(owners.name, length(owners.name) - 7);

COMMIT;
