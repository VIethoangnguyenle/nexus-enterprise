-- 013_channel_members_invite.sql
--
-- Channel membership changes are authorized with OpInvite against the channel's
-- own Content OA. Channels created before that rule granted their Members UA
-- only {read, write}, so on those channels nobody but a workspace owner can add
-- a participant.
--
-- ngac.ChannelMemberOps() now includes invite for newly created channels; this
-- backfills the ones that already exist.
--
-- The grant is per channel: it is an association from Ch_{id}_Members to
-- Ch_{id}_Content and confers nothing on any other channel or on the workspace.
--
-- DMs are covered by the same association, and deliberately so — the guard that
-- stops a DM becoming a group chat lives in messaging.Service.AddMember, not in
-- the graph, because the graph has no notion of "this conversation has exactly
-- two sides".
--
-- NOTE: writes ngac_associations directly and so bypasses EPP invalidation.
-- Restart the services (or invalidate their graphs) after applying.

BEGIN;

UPDATE ngac_associations a
SET operations = array_append(a.operations, 'invite')
FROM ngac_nodes ua, ngac_nodes oa
WHERE ua.id = a.ua_id
  AND oa.id = a.oa_id
  AND ua.node_type = 'UA'
  AND oa.node_type = 'OA'
  AND ua.name LIKE 'Ch\_%\_Members'
  AND oa.name LIKE 'Ch\_%\_Content'
  AND NOT ('invite' = ANY(a.operations));

COMMIT;
