-- 061_channel_membership_reconcile.sql
--
-- A group's Members UA (`Ch_<id>_Members`) sits under the workspace's policy class
-- (the intersection rule needs it there), so whoever is assigned beneath it reaches
-- that class. Until the code that ships before this file, AddMember assigned any
-- node a client sent, and the workspace services read "reaches the policy class" as
-- "is a member". So a person of another tenant, a whole UA, or someone removed from
-- the workspace could sit in a group and be taken for a member. The code now asks
-- whether the node is a user under `<ws>_Members`; this file removes what the old
-- code let in:
--
--   a) a user node assigned under a non-DM group's Members UA who is not under
--      `<ws>_Members` (the workspace-wide #general included: it is the channel of
--      type 'workspace', and its direct edges are written only for members, so a
--      leftover one belongs to somebody who was detached afterwards)
--   b) any UA assigned under a non-DM group's Members UA (a person is the only kind
--      of node a group holds)
--   c) channel_members cache rows of those edges, and cache rows of a non-DM group
--      with no matching edge in the graph (the graph is the truth; the cache is a copy)
--
-- Direct messages are not touched (they are a two-person conversation with their own
-- rule). A user who reaches `<ws>_Members` only through a role or department UA is not
-- a member either; the pre-flight below lists them before anything is deleted.
--
-- Statements, in order (one transaction):
--   1. guard: every workspace has PC/Members/Owners/Channels (059 has run), and the
--      number of rows this run would delete is within the thresholds below
--   2. backups: _reconcile_061_assignments, _reconcile_061_channel_members (every
--      row this run deletes, kept once; a re-run adds nothing)
--   3. delete the edges of a) and b)
--   4. delete the cache rows of c)
--
-- Re-runnable: every predicate is a fixpoint (second run: 0 rows, no backup rows).
--
-- Release train: the code that checks the target in AddMember and reads membership
-- as "under `<ws>_Members`" ships first ("release A"); this file ships after it.
-- Rolling images back to release A is safe. Old code run against this data still works:
-- it only ever treated the removed edges as membership it had no business granting.
--
-- Reverse (then recreate policy and policy-read, and `policy invalidate-cache
-- --all-workspaces`):
--
--   BEGIN;
--   -- only rows whose nodes and channels still exist (a partial restore must not abort on a foreign key)
--   INSERT INTO ngac_assignments SELECT b.* FROM _reconcile_061_assignments b
--    WHERE EXISTS (SELECT 1 FROM ngac_nodes WHERE id = b.child_id) AND EXISTS (SELECT 1 FROM ngac_nodes WHERE id = b.parent_id)
--   ON CONFLICT DO NOTHING;
--   INSERT INTO channel_members SELECT b.* FROM _reconcile_061_channel_members b
--    WHERE EXISTS (SELECT 1 FROM channels WHERE id = b.channel_id)
--   ON CONFLICT DO NOTHING;
--   COMMIT;
--
-- NOTE: writes ngac_* directly and so bypasses EPP invalidation. After applying:
-- recreate policy and policy-read (deploy.sh does), run
-- `policy invalidate-cache --all-workspaces` (`make policy-invalidate` in dev;
-- deploy.sh runs it too), and restart messaging so open WebSocket subscriptions,
-- which were authorised under the old graph, are dropped.
--
-- PRE-FLIGHT (read-only; run in the prod migrate container before approving the
-- deploy and paste the counts into the PR). Set the thresholds below from them.
-- Each query is independent; they use the same predicate as the statements.
--
--   -- common: who is under each workspace's Members UA
--   --   WITH RECURSIVE below(workspace_id, node_id) AS (
--   --     SELECT w.id, s.child_id FROM workspaces w
--   --       JOIN ngac_nodes m ON m.name = w.id || '_Members' AND m.node_type = 'UA'
--   --       JOIN ngac_assignments s ON s.parent_id = m.id
--   --     UNION
--   --     SELECT b.workspace_id, s.child_id FROM below b JOIN ngac_assignments s ON s.parent_id = b.node_id)
--
--   -- a) edges to delete, by kind (user outside the workspace / UA), and how many are #general's
--   --   <common> SELECT n.node_type, c.channel_type, count(*)
--   --     FROM channels c JOIN ngac_assignments s ON s.parent_id = c.ngac_ua_id
--   --     JOIN ngac_nodes n ON n.id = s.child_id
--   --    WHERE c.channel_type <> 'dm' AND (n.node_type = 'UA' OR (n.node_type = 'U' AND NOT EXISTS
--   --       (SELECT 1 FROM below b WHERE b.workspace_id = c.workspace_id AND b.node_id = s.child_id)))
--   --    GROUP BY 1, 2;
--
--   -- b) the same, per person and group, for a decision (who loses which group)
--   --   <common> SELECT c.workspace_id, c.id AS channel_id, c.name, c.channel_type, n.node_type, n.name AS node, u.display_name
--   --     FROM channels c JOIN ngac_assignments s ON s.parent_id = c.ngac_ua_id
--   --     JOIN ngac_nodes n ON n.id = s.child_id LEFT JOIN users u ON u.ngac_node = n.id
--   --    WHERE <the predicate of a)> ORDER BY 1, 2;
--
--   -- c) the people who would lose a group, and what else each sits under (a person who
--   --    sits under a role or department of the workspace but not under its Members UA
--   --    shows up here with that role or department; decide before applying)
--   --   <common> SELECT c.workspace_id, c.name AS channel, n.name AS person,
--   --          (SELECT string_agg(p.name, ', ') FROM ngac_assignments x JOIN ngac_nodes p ON p.id = x.parent_id
--   --            WHERE x.child_id = n.id AND x.parent_id <> c.ngac_ua_id) AS also_under
--   --     FROM channels c JOIN ngac_assignments s ON s.parent_id = c.ngac_ua_id
--   --     JOIN ngac_nodes n ON n.id = s.child_id AND n.node_type = 'U'
--   --    WHERE c.channel_type <> 'dm' AND NOT EXISTS
--   --      (SELECT 1 FROM below b WHERE b.workspace_id = c.workspace_id AND b.node_id = s.child_id);
--
--   -- d) cache rows with no graph edge
--   --   SELECT c.channel_type, count(*) FROM channel_members m JOIN channels c ON c.id = m.channel_id
--   --    WHERE c.channel_type <> 'dm' AND c.ngac_ua_id IS NOT NULL AND NOT EXISTS
--   --      (SELECT 1 FROM ngac_assignments s WHERE s.child_id = m.ngac_node_id AND s.parent_id = c.ngac_ua_id)
--   --    GROUP BY 1;
--

BEGIN;

-- Thresholds. A run that would delete more rows than this aborts, so a graph that
-- differs from what the pre-flight showed is looked at by a person instead of being
-- rewritten. Set them from the production pre-flight counts.
--   edges      : graph edges removed (users outside their workspace, UAs under groups)
--   cache_rows : channel_members rows removed
CREATE TEMP TABLE _m061_limits (edges INT, cache_rows INT) ON COMMIT DROP;
INSERT INTO _m061_limits VALUES (50, 200);

-- 1. Guard: 059 must have given every workspace its ID-keyed nodes, the Members UA under the PC.
DO $$
DECLARE missing TEXT;
BEGIN
    SELECT string_agg(w.id, ', ') INTO missing
    FROM workspaces w
    WHERE NOT EXISTS (SELECT 1 FROM ngac_nodes n WHERE n.name = 'PC_' || w.id AND n.node_type = 'PC')
       OR NOT EXISTS (SELECT 1 FROM ngac_nodes m JOIN ngac_nodes pc ON pc.name = 'PC_' || w.id AND pc.node_type = 'PC'
                      JOIN ngac_assignments a ON a.child_id = m.id AND a.parent_id = pc.id
                      WHERE m.name = w.id || '_Members' AND m.node_type = 'UA')
       OR NOT EXISTS (SELECT 1 FROM ngac_nodes n WHERE n.name = w.id || '_Owners' AND n.node_type = 'UA')
       OR NOT EXISTS (SELECT 1 FROM ngac_nodes n WHERE n.name = w.id || '_Channels' AND n.node_type = 'OA');
    IF missing IS NOT NULL THEN
        RAISE EXCEPTION 'migration 061: workspace(s) lack PC/Members/Owners/Channels nodes (run 059 first): %', missing;
    END IF;
END $$;

-- Every node below each workspace's Members UA (the one that sits under its PC; a node that
-- only carries the name is not it, as policyclient.MembersUA also requires): its Owners and tenant UAs and the
-- users under them. Walked downward from the Members UA, so a path through a group's
-- UA (which is never beneath Members) cannot make anyone a member.
CREATE TEMP TABLE _m061_below ON COMMIT DROP AS
WITH RECURSIVE below(workspace_id, node_id) AS (
    SELECT w.id, s.child_id
    FROM workspaces w
    JOIN ngac_nodes pc ON pc.name = 'PC_' || w.id AND pc.node_type = 'PC'
    JOIN ngac_nodes m ON m.name = w.id || '_Members' AND m.node_type = 'UA'
    JOIN ngac_assignments mp ON mp.child_id = m.id AND mp.parent_id = pc.id
    JOIN ngac_assignments s ON s.parent_id = m.id
    UNION
    SELECT b.workspace_id, s.child_id FROM below b JOIN ngac_assignments s ON s.parent_id = b.node_id
)
SELECT workspace_id, node_id FROM below;
CREATE INDEX ON _m061_below (workspace_id, node_id);

-- The edges to delete: under a non-DM group's Members UA, a UA, or a user who is not
-- under the workspace's Members UA.
CREATE TEMP TABLE _m061_edges ON COMMIT DROP AS
SELECT s.id AS assignment_id, s.child_id, s.parent_id, c.id AS channel_id, c.channel_type
FROM channels c
JOIN ngac_assignments s ON s.parent_id = c.ngac_ua_id
JOIN ngac_nodes n ON n.id = s.child_id
WHERE c.channel_type <> 'dm' AND c.ngac_ua_id IS NOT NULL
  AND (n.node_type = 'UA'
       OR (n.node_type = 'U' AND NOT EXISTS (
            SELECT 1 FROM _m061_below b WHERE b.workspace_id = c.workspace_id AND b.node_id = s.child_id)));

-- The cache rows to delete: those of a deleted edge, and those with no edge at all.
CREATE TEMP TABLE _m061_cache ON COMMIT DROP AS
SELECT m.channel_id, m.ngac_node_id
FROM channel_members m
JOIN channels c ON c.id = m.channel_id
WHERE c.channel_type <> 'dm' AND c.ngac_ua_id IS NOT NULL
  AND (EXISTS (SELECT 1 FROM _m061_edges e WHERE e.channel_id = m.channel_id AND e.child_id = m.ngac_node_id)
       OR NOT EXISTS (SELECT 1 FROM ngac_assignments s WHERE s.child_id = m.ngac_node_id AND s.parent_id = c.ngac_ua_id));

DO $$
DECLARE
    lim      _m061_limits%ROWTYPE;
    n_edges  INTEGER;
    n_cache  INTEGER;
BEGIN
    SELECT * INTO lim FROM _m061_limits;
    SELECT count(*) INTO n_edges FROM _m061_edges;
    SELECT count(*) INTO n_cache FROM _m061_cache;
    RAISE NOTICE 'migration 061: % group edges and % cache rows to remove', n_edges, n_cache;
    IF n_edges > lim.edges THEN
        RAISE EXCEPTION 'migration 061: % group edges would be removed (limit %); check the pre-flight and raise the limit if expected', n_edges, lim.edges;
    END IF;
    IF n_cache > lim.cache_rows THEN
        RAISE EXCEPTION 'migration 061: % channel_members rows would be removed (limit %); check the pre-flight and raise the limit if expected', n_cache, lim.cache_rows;
    END IF;
END $$;

-- 2. Backups. Every row deleted below is kept here once (the primary keys copied by
--    LIKE make a repeat a no-op), so the reverse above restores exactly what was removed.
CREATE TABLE IF NOT EXISTS _reconcile_061_assignments (LIKE ngac_assignments INCLUDING ALL);
CREATE TABLE IF NOT EXISTS _reconcile_061_channel_members (LIKE channel_members INCLUDING ALL);

INSERT INTO _reconcile_061_assignments
SELECT a.* FROM ngac_assignments a JOIN _m061_edges e ON e.assignment_id = a.id
ON CONFLICT DO NOTHING;

INSERT INTO _reconcile_061_channel_members
SELECT m.* FROM channel_members m
JOIN _m061_cache x ON x.channel_id = m.channel_id AND x.ngac_node_id = m.ngac_node_id
ON CONFLICT DO NOTHING;

-- 3. Edges: whoever the old AddMember let in who is not a person of this workspace.
DELETE FROM ngac_assignments a USING _m061_edges e WHERE a.id = e.assignment_id;

-- 4. The cache copies of them.
DELETE FROM channel_members m USING _m061_cache x
WHERE m.channel_id = x.channel_id AND m.ngac_node_id = x.ngac_node_id;

COMMIT;
