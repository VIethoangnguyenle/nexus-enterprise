-- 066_dm_workspace.sql
--
-- A direct message used to belong to no workspace (`channels.workspace_id` NULL), so the
-- DM two people started in one workspace was listed, and reused, in every other workspace
-- they shared, and in a workspace where the other person is not a member nobody could name
-- them. The code that ships with this file ties a DM to exactly one workspace: it lists,
-- opens, searches and announces a DM only in `channels.workspace_id`, and starts one only
-- in a workspace both people are members of. This file gives the DMs that already exist
-- their workspace:
--
--   the workspaces in which BOTH people of the DM are members (a `tenant_users` row whose
--   status is 'active': invited, disabled and removed people are not members; matched on the node the DM records the person by:
--   `tenant_users.ngac_node_id` or `users.ngac_node`):
--     exactly one      -> that one
--     several          -> an 'organization' before a 'personal' one, then the oldest
--                         (`workspaces.created_at`, then id)
--     none             -> left NULL and reported. Such a DM is in no workspace, so it
--                         appears nowhere (the code treats NULL as "belongs to none").
--
-- A DM with fewer than two recorded people cannot be placed and is treated as "none".
-- Only DMs with a NULL workspace are looked at; one that has a workspace is never moved.
-- The graph is not written: a DM's nodes stay under PC_Global, so no policy restart or
-- cache invalidation is needed. A NOT NULL / CHECK on DMs is left for a later release
-- (the code before this file still writes DMs with a NULL workspace until it is rolled out).
--
-- Statements, in order (one transaction):
--   1. thresholds, and the candidate table for each DM
--   2. guard: the number of DMs that change and that stay unplaced is within the limits
--   3. backup of every row that changes: _reconcile_066_dms (kept once; a re-run adds nothing)
--   4. the update, and a NOTICE with the counts
--
-- Re-runnable: only NULL-workspace DMs are touched, so a second run changes 0 rows and
-- adds no backup rows (the unplaced ones are counted and reported again). `make db-migrate`
-- re-applies every file; this one is harmless to run again.
--
-- Reverse:
--
--   BEGIN;
--   UPDATE channels c SET workspace_id = b.old_workspace_id
--     FROM _reconcile_066_dms b
--    WHERE b.channel_id = c.id AND c.channel_type = 'dm'
--      AND c.workspace_id IS NOT DISTINCT FROM b.new_workspace_id;
--   COMMIT;
--   -- DMs created after this file ran are not in the backup and keep their workspace.
--
-- PRE-FLIGHT (read-only): run this file with the final COMMIT replaced by ROLLBACK; every
-- number is printed as a NOTICE, and the choices are readable inside that transaction:
--   sed 's/^COMMIT;/ROLLBACK;/' 066_dm_workspace.sql | psql -v ON_ERROR_STOP=1
-- Production before this file: 4 DMs, 3 with one shared workspace, 1 with two.

BEGIN;

-- Thresholds. A run that would change more DMs, or leave more unplaced, than this aborts,
-- so a database that differs from the pre-flight is looked at by a person instead of being
-- rewritten. Set from the production pre-flight with a little headroom.
CREATE TEMP TABLE _m066_limits (changed INT, unplaced INT) ON COMMIT DROP;
INSERT INTO _m066_limits VALUES (10, 2);

-- The DMs to place, and the people each one records.
CREATE TEMP TABLE _m066_people ON COMMIT DROP AS
SELECT DISTINCT m.channel_id, m.ngac_node_id AS node
FROM channels c
JOIN channel_members m ON m.channel_id = c.id
WHERE c.channel_type = 'dm' AND c.workspace_id IS NULL;
CREATE INDEX ON _m066_people (channel_id);

-- Every (DM, workspace) in which all the DM's people are members, and at least two are.
CREATE TEMP TABLE _m066_candidate ON COMMIT DROP AS
SELECT p.channel_id, w.id AS workspace_id, w.type, w.created_at
FROM _m066_people p
JOIN tenant_users tu ON tu.status = 'active'
JOIN users u ON u.id = tu.user_id
JOIN workspaces w ON w.id = tu.tenant_id
WHERE p.node IN (tu.ngac_node_id, u.ngac_node)
GROUP BY p.channel_id, w.id, w.type, w.created_at
HAVING count(DISTINCT p.node) >= 2
   AND count(DISTINCT p.node) = (SELECT count(*) FROM _m066_people q WHERE q.channel_id = p.channel_id);

-- The choice: an organization before a personal workspace, then the oldest.
CREATE TEMP TABLE _m066_choice ON COMMIT DROP AS
SELECT DISTINCT ON (channel_id) channel_id, workspace_id
FROM _m066_candidate
ORDER BY channel_id, (type = 'organization') DESC, created_at, workspace_id;

DO $$
DECLARE
    lim        _m066_limits%ROWTYPE;
    n_null     INTEGER;
    n_change   INTEGER;
    n_one      INTEGER;
    n_several  INTEGER;
    n_unplaced INTEGER;
    unplaced   TEXT;
BEGIN
    SELECT * INTO lim FROM _m066_limits;
    SELECT count(*) INTO n_null FROM channels WHERE channel_type = 'dm' AND workspace_id IS NULL;
    SELECT count(*) INTO n_change FROM _m066_choice;
    SELECT count(*) INTO n_one FROM (SELECT channel_id FROM _m066_candidate GROUP BY channel_id HAVING count(*) = 1) x;
    SELECT count(*) INTO n_several FROM (SELECT channel_id FROM _m066_candidate GROUP BY channel_id HAVING count(*) > 1) x;
    n_unplaced := n_null - n_change;
    SELECT string_agg(c.id, ', ' ORDER BY c.id) INTO unplaced
    FROM channels c
    WHERE c.channel_type = 'dm' AND c.workspace_id IS NULL
      AND NOT EXISTS (SELECT 1 FROM _m066_choice x WHERE x.channel_id = c.id);

    RAISE NOTICE 'migration 066: % DMs without a workspace; % to place (% in exactly one shared workspace, % in several); % left unplaced',
        n_null, n_change, n_one, n_several, n_unplaced;
    IF unplaced IS NOT NULL THEN
        RAISE NOTICE 'migration 066: DM(s) left without a workspace (they appear in no workspace): %', unplaced;
    END IF;

    IF n_change > lim.changed THEN
        RAISE EXCEPTION 'migration 066: % DMs would change, more than the threshold %; compare with the pre-flight before raising it', n_change, lim.changed;
    END IF;
    IF n_unplaced > lim.unplaced THEN
        RAISE EXCEPTION 'migration 066: % DMs cannot be placed, more than the threshold %; compare with the pre-flight before raising it', n_unplaced, lim.unplaced;
    END IF;
END $$;

-- Backup of every row that changes. The state recorded first (old_workspace_id) is kept; a
-- run after the reverse only refreshes where the row was sent. A re-run adds nothing, since
-- it has nothing to change.
CREATE TABLE IF NOT EXISTS _reconcile_066_dms (
    channel_id       TEXT PRIMARY KEY,
    old_workspace_id TEXT,
    new_workspace_id TEXT NOT NULL,
    backed_up_at     TIMESTAMPTZ NOT NULL DEFAULT now()
);
INSERT INTO _reconcile_066_dms (channel_id, old_workspace_id, new_workspace_id)
SELECT c.id, c.workspace_id, x.workspace_id
FROM _m066_choice x JOIN channels c ON c.id = x.channel_id
ON CONFLICT (channel_id) DO UPDATE SET new_workspace_id = EXCLUDED.new_workspace_id;

UPDATE channels c SET workspace_id = x.workspace_id
FROM _m066_choice x
WHERE c.id = x.channel_id AND c.channel_type = 'dm' AND c.workspace_id IS NULL;

COMMIT;
