-- 015_member_document_write.sql
--
-- Workspace members held only {read} on the Documents OA, so they could open
-- the drive and see the Upload button but never complete an upload: the drive
-- gates file creation on `write` (CreateFile and ConfirmFile both check it on
-- the destination folder), and members did not have it.
--
-- ngac.MemberDocumentOps() now grants {read, write, upload} for newly created
-- workspaces; this backfills the ones that already exist. It matches what a
-- member already holds on any channel drive they belong to
-- (ngac.ChannelDriveOps), so the workspace drive was the odd one out.
--
-- Scope: this adds only write and upload. It does not grant approve, share or
-- manage, which stay with the Owners UA.
--
-- NOTE: writes ngac_associations directly and so bypasses EPP invalidation.
-- Restart the services (or invalidate their graphs) after applying, or runtime
-- decisions keep using the in-memory graph loaded at startup.

BEGIN;

UPDATE ngac_associations a
SET operations = (
    SELECT array_agg(DISTINCT op ORDER BY op)
    FROM unnest(a.operations || ARRAY['write', 'upload']) AS op
)
FROM ngac_nodes ua, ngac_nodes oa
WHERE ua.id = a.ua_id
  AND oa.id = a.oa_id
  AND ua.node_type = 'UA'
  AND oa.node_type = 'OA'
  AND ua.name LIKE '%\_Members'
  AND oa.name LIKE '%\_Documents'
  -- Same workspace on both sides: strip "_Members" (8 chars) and "_Documents"
  -- (10 chars) and require the remaining ids to match.
  AND left(ua.name, length(ua.name) - 8) = left(oa.name, length(oa.name) - 10)
  AND NOT (a.operations @> ARRAY['write', 'upload']);

COMMIT;
