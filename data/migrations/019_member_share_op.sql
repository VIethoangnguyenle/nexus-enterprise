-- 019_member_share_op.sql
--
-- Sharing a drive item now requires the `share` operation on the item's OA
-- (CreateShare used to check `write`). Members share what they work on, so
-- ngac.MemberDocumentOps() and ngac.ChannelDriveOps() now include `share`.
-- This backfills the associations that already exist:
--
--   * workspace Members UA  -> workspace Documents OA
--   * channel Members UA    -> that channel's drive OA
--
-- Scope: adds only `share`. A share grants its grantee read or write
-- (ngac.ShareOps), never share, manage or approve, so the right does not
-- spread. Owners already hold share. Idempotent: associations that already
-- contain `share` are left alone.
--
-- NOTE: writes ngac_associations directly and so bypasses EPP invalidation.
-- Restart the services (or invalidate their graphs) after applying, or runtime
-- decisions keep using the in-memory graph loaded at startup.

BEGIN;

-- Workspace Members -> Documents. Same workspace on both sides: strip
-- "_Members" (8 chars) and "_Documents" (10 chars) and require the ids to match.
UPDATE ngac_associations a
SET operations = (
    SELECT array_agg(DISTINCT op ORDER BY op)
    FROM unnest(a.operations || ARRAY['share']) AS op
)
FROM ngac_nodes ua, ngac_nodes oa
WHERE ua.id = a.ua_id
  AND oa.id = a.oa_id
  AND ua.node_type = 'UA'
  AND oa.node_type = 'OA'
  AND ua.name LIKE '%\_Members'
  AND oa.name LIKE '%\_Documents'
  AND left(ua.name, length(ua.name) - 8) = left(oa.name, length(oa.name) - 10)
  AND NOT (a.operations @> ARRAY['share']);

-- Channel Members -> that channel's drive. The drive OA is named after the
-- channel's display name but records the channel id in its properties, and the
-- Members UA is named "Ch_<channel id>_Members": match on that id.
UPDATE ngac_associations a
SET operations = (
    SELECT array_agg(DISTINCT op ORDER BY op)
    FROM unnest(a.operations || ARRAY['share']) AS op
)
FROM ngac_nodes ua, ngac_nodes oa
WHERE ua.id = a.ua_id
  AND oa.id = a.oa_id
  AND ua.node_type = 'UA'
  AND oa.node_type = 'OA'
  AND oa.properties->>'type' = 'channel_drive'
  AND ua.name = 'Ch_' || (oa.properties->>'channel_id') || '_Members'
  AND NOT (a.operations @> ARRAY['share']);

COMMIT;
