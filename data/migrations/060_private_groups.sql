-- 060_private_groups.sql
--
-- Makes groups private. Until now every workspace member could read, write and
-- invite in every group, because `<ws>_Members` held {read, write, create_channel}
-- on `<ws>_Channels` and a grant on that OA is inherited by every group's content
-- OA, drive and calendar beneath it (Owners held every operation there too, and
-- custom roles whatever the permissions screen offered). After this migration:
--
--   Members  -> Channels : {create_channel}
--   Owners   -> Channels : {manage, create_channel}      (moderate, not read)
--   any other UA on Channels : its operations that are in {create_channel}, or removed
--
-- A group is then readable only through its own `Ch_<id>_Members` UA. The one
-- workspace-wide channel (#general, channel_type 'workspace', at most one per
-- workspace) is readable by every member through a DIRECT edge from each member's
-- user node to its Members UA plus a channel_members cache row, so the roster,
-- push recipients and member_count see them. Every other 'workspace'-typed
-- channel (a user's group made before this change) becomes 'private'.
--
-- On the first run #general is the oldest channel of type 'workspace' named "general"
-- that the workspace's owner created (the same rule messaging's WorkspaceWideChannel
-- uses before the index exists). Once channels_one_workspace_wide is valid the rule is
-- simply "the workspace's one 'workspace'-typed channel", so a re-run never retypes a
-- #general that has been renamed. A workspace with none gets none (dev has a handful);
-- nothing is invented.
--
-- Statements, in order (steps 1-6 in one transaction, 7 outside it):
--   1. guard: every workspace has PC/Members/Owners/Channels (059 has run), and the
--      number of rows this run would remove or retype is within the thresholds below
--   2. backups: _reconcile_060_associations, _reconcile_060_channels
--   3. repair: a non-DM group without its Members -> Content association gets
--      {read, write, invite} (it was unreadable by anyone, its creator included)
--   4. channels typed 'workspace' other than #general become 'private'
--   5. #general: a direct U -> Members UA edge and a channel_members row for every
--      user that reaches `<ws>_Members`
--   6. the associations on every `<id>_Channels` OA (workspaces long deleted included), as
--      listed above
--   7. CREATE UNIQUE INDEX CONCURRENTLY channels_one_workspace_wide (outside the
--      transaction)
--
-- Re-runnable: every step is a fixpoint (second run: 0 rows). deploy/migrate.sh runs
-- a file with CONCURRENTLY without --single-transaction and writes the ledger row
-- afterwards, so a crash between the two re-runs the whole file; so does plain psql
-- in `make db-migrate`.
--
-- Release train: code that writes the new shape and tolerates the old ships first
-- ("release A"); this file ships after it. Rolling images back to release A is safe.
-- Rolling back past A needs the reverse SQL below first, or the old code, which
-- types every group 'workspace', is rejected by the unique index.
--
-- Reverse (then recreate policy and policy-read, and `policy invalidate-cache
-- --all-workspaces`). The #general direct edges are harmless under the old model
-- and stay:
--
--   BEGIN;
--   DROP INDEX IF EXISTS channels_one_workspace_wide;
--   UPDATE ngac_associations a SET operations = b.operations
--     FROM _reconcile_060_associations b WHERE b.id = a.id;
--   INSERT INTO ngac_associations SELECT * FROM _reconcile_060_associations b
--     WHERE NOT EXISTS (SELECT 1 FROM ngac_associations a WHERE a.id = b.id);
--   UPDATE channels c SET channel_type = b.channel_type
--     FROM _reconcile_060_channels b WHERE b.id = c.id;
--   COMMIT;
--
-- NOTE: writes ngac_* directly and so bypasses EPP invalidation. After applying:
-- recreate policy and policy-read (deploy.sh does), run
-- `policy invalidate-cache --all-workspaces` (`make policy-invalidate` in dev;
-- deploy.sh runs it too), and restart messaging so open WebSocket subscriptions,
-- which were authorised under the old graph, are dropped.
--
-- PRE-FLIGHT (read-only; run in the prod migrate container before approving the
-- deploy and paste the counts into the PR). Each query is independent.
--
--   -- a) groups with no Members -> Content association (step 3 repairs them)
--   SELECT count(*) FROM channels c
--    WHERE c.channel_type <> 'dm' AND c.ngac_oa_id IS NOT NULL AND c.ngac_ua_id IS NOT NULL
--      AND NOT EXISTS (SELECT 1 FROM ngac_associations a WHERE a.ua_id = c.ngac_ua_id AND a.oa_id = c.ngac_oa_id);
--
--   -- b) member-cache rows with no graph edge (the cache is not the truth; it is not changed here)
--   SELECT count(*) FROM channel_members m JOIN channels c ON c.id = m.channel_id
--    WHERE c.ngac_ua_id IS NOT NULL
--      AND NOT EXISTS (SELECT 1 FROM ngac_assignments s WHERE s.child_id = m.ngac_node_id AND s.parent_id = c.ngac_ua_id);
--
--   -- c) the channel chosen as #general per workspace, with its name and age
--   SELECT DISTINCT ON (c.workspace_id) c.workspace_id, c.id, c.name, c.created_at
--     FROM channels c JOIN workspaces w ON w.id = c.workspace_id
--    WHERE c.channel_type = 'workspace' AND c.name = 'general' AND c.created_by = w.owner_id
--    ORDER BY c.workspace_id, c.created_at, c.id;
--
--   -- d) workspaces that will have no #general (nothing is created for them)
--   SELECT w.id, w.name FROM workspaces w WHERE NOT EXISTS (
--     SELECT 1 FROM channels c WHERE c.workspace_id = w.id AND c.channel_type = 'workspace'
--       AND c.name = 'general' AND c.created_by = w.owner_id);
--
--   -- e) 'workspace'-typed channels that become private
--   SELECT c.workspace_id, c.id, c.name FROM channels c
--    WHERE c.channel_type = 'workspace' AND c.id IS DISTINCT FROM (
--      SELECT c2.id FROM channels c2 JOIN workspaces w ON w.id = c2.workspace_id
--       WHERE c2.workspace_id = c.workspace_id AND c2.channel_type = 'workspace'
--         AND c2.name = 'general' AND c2.created_by = w.owner_id
--       ORDER BY c2.created_at, c2.id LIMIT 1);
--
--   -- f) people who posted in the last 30 days in a group they are not a member of
--   --    (they lose access to it; the owner re-adds them)
--   SELECT c.workspace_id, c.id AS channel_id, c.name, u.ngac_node AS poster_node, count(*) AS posts
--     FROM messages m JOIN channels c ON c.id = m.channel_id AND c.channel_type <> 'dm'
--     JOIN users u ON u.id = m.sender_id
--    WHERE m.created_at > now() - interval '30 days' AND c.ngac_ua_id IS NOT NULL
--      AND NOT EXISTS (SELECT 1 FROM ngac_assignments s WHERE s.child_id = u.ngac_node AND s.parent_id = c.ngac_ua_id)
--    GROUP BY 1, 2, 3, 4 ORDER BY 1, 2;
--
--   -- g) shares of channel-drive files whose grantee is NOT the group's own UA. Their
--   --    Share OAs hang under PC_Global, so they survive this migration and keep the
--   --    file readable by everyone they were shared with (a workspace-wide or public
--   --    share, a role, or another person). One row per share, per workspace, for a decision.
--   SELECT i.workspace_id, c.id AS channel_id, c.name AS channel, i.name AS item, s.share_type, s.target_label, s.created_at
--     FROM drive_shares s
--     JOIN drive_items i ON i.id = s.drive_item_id AND i.drive_context = 'channel'
--     JOIN channels c ON c.id = i.drive_context_id
--    WHERE s.target_ngac_id IS DISTINCT FROM c.ngac_ua_id
--    ORDER BY i.workspace_id, c.id, s.created_at;

BEGIN;

-- Thresholds. A run that would touch more rows than this aborts, so a graph that
-- differs from what the pre-flight showed is looked at by a person instead of
-- being rewritten. Set them from the production pre-flight counts.
--   other_ua_assocs : associations of UAs other than Members/Owners on a Channels
--                     OA that are reduced or removed (custom roles, departments)
--   retyped         : channels retyped 'workspace' -> 'private'
--   edge_inserts    : direct edges written into #general (all members, once)
CREATE TEMP TABLE _m060_limits (other_ua_assocs INT, retyped INT, edge_inserts INT) ON COMMIT DROP;
INSERT INTO _m060_limits VALUES (200, 2000, 100000);

-- 1. Guard: 059 must have given every workspace its ID-keyed nodes.
DO $$
DECLARE missing TEXT;
BEGIN
    SELECT string_agg(w.id, ', ') INTO missing
    FROM workspaces w
    WHERE NOT EXISTS (SELECT 1 FROM ngac_nodes n WHERE n.name = 'PC_' || w.id AND n.node_type = 'PC')
       OR NOT EXISTS (SELECT 1 FROM ngac_nodes n WHERE n.name = w.id || '_Members' AND n.node_type = 'UA')
       OR NOT EXISTS (SELECT 1 FROM ngac_nodes n WHERE n.name = w.id || '_Owners' AND n.node_type = 'UA')
       OR NOT EXISTS (SELECT 1 FROM ngac_nodes n WHERE n.name = w.id || '_Channels' AND n.node_type = 'OA');
    IF missing IS NOT NULL THEN
        RAISE EXCEPTION 'migration 060: workspace(s) lack PC/Members/Owners/Channels nodes (run 059 first): %', missing;
    END IF;
END $$;

-- Has this migration already enforced one workspace-wide channel per workspace? A valid
-- channels_one_workspace_wide says so, and from then on a workspace's only
-- 'workspace'-typed channel IS its #general, whatever it is called now: the name and
-- creator heuristic below is for the first run only, over data written before the type
-- meant anything. Re-running it later would retype a #general that was renamed.
CREATE TEMP TABLE _m060_state ON COMMIT DROP AS
SELECT EXISTS (SELECT 1 FROM pg_index i JOIN pg_class c ON c.oid = i.indexrelid
               WHERE c.relname = 'channels_one_workspace_wide' AND i.indisvalid) AS enforced;

-- The channel chosen as #general per workspace (the rule above).
CREATE TEMP TABLE _m060_general ON COMMIT DROP AS
SELECT DISTINCT ON (c.workspace_id) c.id AS channel_id, c.workspace_id, c.ngac_ua_id
FROM channels c JOIN workspaces w ON w.id = c.workspace_id, _m060_state st
WHERE c.channel_type = 'workspace'
  AND (st.enforced OR (c.name = 'general' AND c.created_by = w.owner_id))
ORDER BY c.workspace_id, c.created_at, c.id;

-- The associations on every `<id>_Channels` OA, with the role of the UA in it. Every one,
-- not only those of workspaces that still have a row: the graph also holds the nodes of
-- workspaces deleted long ago (dev has hundreds), and a grant of read on their Channels
-- OA is the same leak for anyone still assigned beneath them.
CREATE TEMP TABLE _m060_chan_assocs ON COMMIT DROP AS
SELECT a.id AS assoc_id, a.ua_id, a.oa_id,
       regexp_replace(oa.name, '_Channels$', '') AS workspace_id,
       CASE WHEN ua.name = regexp_replace(oa.name, '_Channels$', '') || '_Members' THEN 'members'
            WHEN ua.name = regexp_replace(oa.name, '_Channels$', '') || '_Owners'  THEN 'owners'
            ELSE 'other' END AS role
FROM ngac_nodes oa
JOIN ngac_associations a ON a.oa_id = oa.id
JOIN ngac_nodes ua ON ua.id = a.ua_id
WHERE oa.node_type = 'OA' AND oa.name LIKE '%\_Channels';

-- Every user that reaches the Members UA of a #general's workspace (Owners and the
-- Tenant* UAs sit beneath it, so their people are reached too).
CREATE TEMP TABLE _m060_members ON COMMIT DROP AS
WITH RECURSIVE reach(channel_id, node_id) AS (
    SELECT g.channel_id, s.child_id
    FROM _m060_general g
    JOIN ngac_nodes m ON m.name = g.workspace_id || '_Members' AND m.node_type = 'UA'
    JOIN ngac_assignments s ON s.parent_id = m.id
    UNION
    SELECT r.channel_id, s.child_id FROM reach r JOIN ngac_assignments s ON s.parent_id = r.node_id
)
SELECT DISTINCT r.channel_id, r.node_id, g.ngac_ua_id
FROM reach r
JOIN ngac_nodes u ON u.id = r.node_id AND u.node_type = 'U'
JOIN _m060_general g ON g.channel_id = r.channel_id
WHERE g.ngac_ua_id IS NOT NULL;

DO $$
DECLARE
    lim        _m060_limits%ROWTYPE;
    n_other    INTEGER;
    n_retyped  INTEGER;
    n_edges    INTEGER;
BEGIN
    SELECT * INTO lim FROM _m060_limits;

    SELECT count(*) INTO n_other FROM _m060_chan_assocs x
    JOIN ngac_associations a ON a.id = x.assoc_id
    WHERE x.role = 'other' AND a.operations <> ARRAY['create_channel'];
    IF n_other > lim.other_ua_assocs THEN
        RAISE EXCEPTION 'migration 060: % custom-UA associations on Channels OAs would be reduced or removed (limit %); check the pre-flight and raise the limit if expected', n_other, lim.other_ua_assocs;
    END IF;

    SELECT count(*) INTO n_retyped FROM channels c
    WHERE c.channel_type = 'workspace' AND c.id NOT IN (SELECT channel_id FROM _m060_general);
    IF n_retyped > lim.retyped THEN
        RAISE EXCEPTION 'migration 060: % channels would be retyped to private (limit %); check the pre-flight and raise the limit if expected', n_retyped, lim.retyped;
    END IF;

    SELECT count(*) INTO n_edges FROM _m060_members m
    WHERE NOT EXISTS (SELECT 1 FROM ngac_assignments s WHERE s.child_id = m.node_id AND s.parent_id = m.ngac_ua_id);
    IF n_edges > lim.edge_inserts THEN
        RAISE EXCEPTION 'migration 060: % direct edges would be written into #general channels (limit %)', n_edges, lim.edge_inserts;
    END IF;
END $$;

-- 2. Backups, taken once. The associations are copied only by the run that creates the
--    table (that run's transaction also makes every change, so the copy is the state
--    before this migration touched anything); a re-run leaves it alone and so changes
--    nothing. Channel types are keyed by id and only ever recorded while still 'workspace'.
DO $$
BEGIN
    IF to_regclass('_reconcile_060_associations') IS NULL THEN
        CREATE TABLE _reconcile_060_associations (LIKE ngac_associations INCLUDING ALL);
        INSERT INTO _reconcile_060_associations
        SELECT a.* FROM ngac_associations a JOIN _m060_chan_assocs x ON x.assoc_id = a.id;
    END IF;
END $$;

CREATE TABLE IF NOT EXISTS _reconcile_060_channels (
    id           TEXT PRIMARY KEY,
    channel_type TEXT NOT NULL,
    recorded_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
INSERT INTO _reconcile_060_channels (id, channel_type)
SELECT c.id, c.channel_type FROM channels c
WHERE c.channel_type = 'workspace' AND c.id NOT IN (SELECT channel_id FROM _m060_general)
ON CONFLICT (id) DO NOTHING;

-- 3. A group whose Members UA holds nothing on its Content OA is unreadable by its own members.
INSERT INTO ngac_associations (id, ua_id, oa_id, operations)
SELECT gen_random_uuid()::text, c.ngac_ua_id, c.ngac_oa_id, ARRAY['read', 'write', 'invite']
FROM channels c
WHERE c.channel_type <> 'dm' AND c.ngac_oa_id IS NOT NULL AND c.ngac_ua_id IS NOT NULL
ON CONFLICT (ua_id, oa_id) DO NOTHING;

-- 4. Every 'workspace'-typed channel but #general is a private group.
UPDATE channels SET channel_type = 'private'
WHERE channel_type = 'workspace' AND id NOT IN (SELECT channel_id FROM _m060_general);

-- 5. #general: every user reaching `<ws>_Members` is a direct member.
INSERT INTO ngac_assignments (id, child_id, parent_id)
SELECT gen_random_uuid()::text, node_id, ngac_ua_id FROM _m060_members
ON CONFLICT (child_id, parent_id) DO NOTHING;

INSERT INTO channel_members (channel_id, ngac_node_id)
SELECT channel_id, node_id FROM _m060_members
ON CONFLICT DO NOTHING;

-- 6. The associations on every workspace's Channels OA.
--    Members and Owners: set to the new operations, creating the association if it is missing.
INSERT INTO ngac_associations (id, ua_id, oa_id, operations)
SELECT gen_random_uuid()::text, ua.id, oa.id,
       CASE r.role WHEN 'members' THEN ARRAY['create_channel'] ELSE ARRAY['manage', 'create_channel'] END
FROM workspaces w
JOIN ngac_nodes oa ON oa.name = w.id || '_Channels' AND oa.node_type = 'OA'
CROSS JOIN (VALUES ('members', '_Members'), ('owners', '_Owners')) AS r(role, suffix)
JOIN ngac_nodes ua ON ua.name = w.id || r.suffix AND ua.node_type = 'UA'
ON CONFLICT (ua_id, oa_id) DO UPDATE SET operations = EXCLUDED.operations
WHERE ngac_associations.operations IS DISTINCT FROM EXCLUDED.operations;

--    The same for the Members and Owners associations that exist (including those of
--    workspaces no longer in the table), without creating any.
UPDATE ngac_associations a
SET operations = CASE x.role WHEN 'members' THEN ARRAY['create_channel'] ELSE ARRAY['manage', 'create_channel'] END
FROM _m060_chan_assocs x
WHERE x.assoc_id = a.id AND x.role IN ('members', 'owners')
  AND a.operations IS DISTINCT FROM CASE x.role WHEN 'members' THEN ARRAY['create_channel'] ELSE ARRAY['manage', 'create_channel'] END;

--    Any other UA keeps only create_channel, if it had it, and loses the association otherwise.
UPDATE ngac_associations a SET operations = ARRAY['create_channel']
FROM _m060_chan_assocs x
WHERE x.assoc_id = a.id AND x.role = 'other'
  AND 'create_channel' = ANY (a.operations) AND a.operations <> ARRAY['create_channel'];

DELETE FROM ngac_associations a
USING _m060_chan_assocs x
WHERE x.assoc_id = a.id AND x.role = 'other' AND NOT ('create_channel' = ANY (a.operations));

COMMIT;

-- 7. One workspace-wide channel per workspace, enforced. Outside the transaction:
--    CONCURRENTLY does not block writers and cannot run inside one. A build that
--    failed earlier leaves an INVALID index of this name, which IF NOT EXISTS would
--    skip, so it is dropped first. The retype is repeated just before, to narrow the
--    window in which an older writer could have added a 'workspace'-typed group
--    after the transaction above.
DO $$
BEGIN
    IF EXISTS (SELECT 1 FROM pg_index i JOIN pg_class c ON c.oid = i.indexrelid
               WHERE c.relname = 'channels_one_workspace_wide' AND NOT i.indisvalid) THEN
        DROP INDEX channels_one_workspace_wide;
    END IF;
END $$;

INSERT INTO _reconcile_060_channels (id, channel_type)
SELECT c.id, c.channel_type FROM channels c
WHERE c.channel_type = 'workspace'
  AND NOT EXISTS (SELECT 1 FROM pg_class WHERE relname = 'channels_one_workspace_wide')
  AND c.id IS DISTINCT FROM (
    SELECT c2.id FROM channels c2 JOIN workspaces w ON w.id = c2.workspace_id
    WHERE c2.workspace_id = c.workspace_id AND c2.channel_type = 'workspace'
      AND c2.name = 'general' AND c2.created_by = w.owner_id
    ORDER BY c2.created_at, c2.id LIMIT 1)
ON CONFLICT (id) DO NOTHING;

UPDATE channels c SET channel_type = 'private'
WHERE c.channel_type = 'workspace'
  AND NOT EXISTS (SELECT 1 FROM pg_class WHERE relname = 'channels_one_workspace_wide')
  AND c.id IS DISTINCT FROM (
    SELECT c2.id FROM channels c2 JOIN workspaces w ON w.id = c2.workspace_id
    WHERE c2.workspace_id = c.workspace_id AND c2.channel_type = 'workspace'
      AND c2.name = 'general' AND c2.created_by = w.owner_id
    ORDER BY c2.created_at, c2.id LIMIT 1);

CREATE UNIQUE INDEX CONCURRENTLY IF NOT EXISTS channels_one_workspace_wide
    ON channels (workspace_id) WHERE channel_type = 'workspace';
