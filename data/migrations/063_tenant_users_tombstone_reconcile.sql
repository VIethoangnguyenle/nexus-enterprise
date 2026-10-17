-- 063_tenant_users_tombstone_reconcile.sql
--
-- Removing someone from a workspace used to DELETE their `tenant_users` row, so a
-- Google Workspace sign-in for the company's domain simply joined them again, and
-- nothing else could tell "removed" from "never invited". The code that ships before
-- this file writes a tombstone instead (`status = 'removed'`), checks membership
-- against the graph when a session is refreshed, when a WebSocket connects and on
-- every approval request, and reads `tenant_users` as "active members" only. This
-- file makes the table agree with the graph for what is already there:
--
--   a) (062 widens the CHECK constraint so `status` may be 'removed'; this file needs it)
--   b) rows with no `ngac_node_id` get the node of their user (`users.ngac_node`)
--   c) a user who is under `<ws>_Members` in the graph but has no row gets one
--      (`active`; `owner` when the user is under `<ws>_Owners`, else `member`)
--   d) an `active` row whose user is not under `<ws>_Members` is tombstoned
--      (`removed`, department cleared): the graph is the truth, the row a copy
--
-- A workspace whose `<ws>_Members` has no user under it at all is SKIPPED by c) and
-- d) and reported: migration 059 repairs the topology of a legacy workspace but does
-- not put anyone in it, so "nobody is a member" there means the graph was never
-- populated, not that every listed person left. Tombstoning would remove them all.
--
-- Statements, in order (one transaction):
--   1. guard: every workspace has its `<id>_Members` UA under its policy class
--      (059 has run), every TenantMember_/TenantOwner_ UA sits under its Members/Owners UA,
--      the number of rows each step would change is within the thresholds below, and no
--      workspace would lose more than half of its active rows
--   2. refuse to run if 062 has not (the constraint must allow 'removed')
--   3. backup of every row that changes: _reconcile_063_tenant_users (state before;
--      rows inserted by c) carry inserted = true). Kept once; a re-run adds nothing
--   4. b), c), d)
--
-- Re-runnable: every predicate is a fixpoint (second run: 0 rows, no backup rows).
-- Only `tenant_users` changes; the graph is not written, so no policy restart or
-- cache invalidation is needed.
--
-- Release train: 062 (the constraint) ships before, or with, the code that writes the
-- tombstone ("release A"); this file ships after that code. 062 alone changes nothing for
-- the code before it, which never writes 'removed'. Running this file before 062 aborts.
--
-- Reverse:
--
--   BEGIN;
--   DELETE FROM tenant_users t USING _reconcile_063_tenant_users b
--    WHERE b.inserted AND b.tenant_id = t.tenant_id AND b.user_id = t.user_id;
--   UPDATE tenant_users t SET status = b.status, department_id = b.department_id, ngac_node_id = b.ngac_node_id
--     FROM _reconcile_063_tenant_users b
--    WHERE NOT b.inserted AND b.tenant_id = t.tenant_id AND b.user_id = t.user_id
--      AND (b.department_id IS NULL OR EXISTS (SELECT 1 FROM departments d WHERE d.id = b.department_id));
--   COMMIT;
--   -- The widened CHECK stays (it is a superset); rows tombstoned by the application
--   -- after this file ran are not in the backup and stay 'removed'.
--
-- PRE-FLIGHT (read-only; run in the prod migrate container before approving the
-- deploy and paste the counts into the PR, then set the thresholds below from them):
--
--   -- every number below is printed as a NOTICE by a dry run of this file: run it with the
--   -- final COMMIT replaced by ROLLBACK
--   --   sed 's/^COMMIT;/ROLLBACK;/' 063_tenant_users_tombstone_reconcile.sql | psql -v ON_ERROR_STOP=1
--
--   -- people the tombstone would remove, by workspace (decide before applying)
--   --   run the dry run, then:  SELECT * FROM _m063_tombstone;  (inside the same transaction)
--

BEGIN;

-- Thresholds. A run that would change more rows than this aborts, so a database that
-- differs from what the pre-flight showed is looked at by a person instead of being
-- rewritten. Set them from the production pre-flight counts.
CREATE TEMP TABLE _m063_limits (backfill INT, inserted INT, tombstoned INT) ON COMMIT DROP;
INSERT INTO _m063_limits VALUES (100, 50, 50);

-- 1. Guard: 059 must have given every workspace its Members UA under its policy class.
DO $$
DECLARE missing TEXT;
BEGIN
    SELECT string_agg(w.id, ', ') INTO missing
    FROM workspaces w
    WHERE NOT EXISTS (SELECT 1 FROM ngac_nodes m
                      JOIN ngac_nodes pc ON pc.name = 'PC_' || w.id AND pc.node_type = 'PC'
                      JOIN ngac_assignments a ON a.child_id = m.id AND a.parent_id = pc.id
                      WHERE m.name = w.id || '_Members' AND m.node_type = 'UA');
    IF missing IS NOT NULL THEN
        RAISE EXCEPTION 'migration 063: workspace(s) lack a Members UA under their policy class (run 059 first): %', missing;
    END IF;
END $$;

-- 2. 062 must have run: a tombstone needs the constraint to allow it.
DO $$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'tenant_users_status_check'
                   AND conrelid = 'tenant_users'::regclass AND pg_get_constraintdef(oid) LIKE '%removed%') THEN
        RAISE EXCEPTION 'migration 063: tenant_users_status_check does not allow ''removed'' (run 062 first)';
    END IF;
END $$;

-- 2b. Guard: the tenant UAs must sit where this file assumes. Membership is read as "under
--     <ws>_Members", and TenantMember_/TenantOwner_ are supposed to hang beneath it; one that
--     does not would make its people look like non-members and get them tombstoned.
DO $$
DECLARE stray TEXT;
BEGIN
    SELECT string_agg(w.id || ':' || t.name, ', ') INTO stray
    FROM workspaces w
    JOIN ngac_nodes t ON t.node_type = 'UA' AND t.name IN ('TenantMember_' || w.id, 'TenantOwner_' || w.id)
    WHERE NOT EXISTS (
        SELECT 1 FROM ngac_assignments a JOIN ngac_nodes p ON p.id = a.parent_id
         WHERE a.child_id = t.id AND p.node_type = 'UA'
           AND p.name = CASE WHEN t.name LIKE 'TenantMember\_%' THEN w.id || '_Members' ELSE w.id || '_Owners' END);
    IF stray IS NOT NULL THEN
        RAISE EXCEPTION 'migration 063: tenant UA(s) not under their workspace''s Members/Owners UA (repair the graph first): %', stray;
    END IF;
END $$;

-- The users under each workspace's Members UA (the one that sits under its policy
-- class; a node that merely carries the name is not it, as policyclient.MembersUA
-- also requires). Walked downward from the Members UA, so Owners and tenant UAs count
-- and a path through a group's UA (never beneath Members) cannot make anyone a member.
CREATE TEMP TABLE _m063_member ON COMMIT DROP AS
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
SELECT DISTINCT b.workspace_id, b.node_id
FROM below b JOIN ngac_nodes n ON n.id = b.node_id AND n.node_type = 'U';
CREATE INDEX ON _m063_member (workspace_id, node_id);

-- The users under each workspace's Owners UA.
CREATE TEMP TABLE _m063_owner ON COMMIT DROP AS
WITH RECURSIVE below(workspace_id, node_id) AS (
    SELECT w.id, s.child_id
    FROM workspaces w
    JOIN ngac_nodes o ON o.name = w.id || '_Owners' AND o.node_type = 'UA'
    JOIN ngac_assignments s ON s.parent_id = o.id
    UNION
    SELECT b.workspace_id, s.child_id FROM below b JOIN ngac_assignments s ON s.parent_id = b.node_id
)
SELECT DISTINCT b.workspace_id, b.node_id
FROM below b JOIN ngac_nodes n ON n.id = b.node_id AND n.node_type = 'U';

-- Workspaces whose graph holds no user under Members: skipped, and reported.
CREATE TEMP TABLE _m063_unpopulated ON COMMIT DROP AS
SELECT w.id AS workspace_id
FROM workspaces w
WHERE NOT EXISTS (SELECT 1 FROM _m063_member m WHERE m.workspace_id = w.id);

-- b) Rows without a node: the node of their user.
CREATE TEMP TABLE _m063_backfill ON COMMIT DROP AS
SELECT tu.tenant_id, tu.user_id, u.ngac_node AS node_id
FROM tenant_users tu
JOIN users u ON u.id = tu.user_id
JOIN ngac_nodes n ON n.id = u.ngac_node
WHERE tu.ngac_node_id IS NULL;

-- c) Users in the graph with no row.
CREATE TEMP TABLE _m063_insert ON COMMIT DROP AS
SELECT m.workspace_id AS tenant_id, u.id AS user_id, m.node_id,
       CASE WHEN EXISTS (SELECT 1 FROM _m063_owner o WHERE o.workspace_id = m.workspace_id AND o.node_id = m.node_id)
            THEN 'owner' ELSE 'member' END AS role
FROM _m063_member m
JOIN users u ON u.ngac_node = m.node_id
WHERE NOT EXISTS (SELECT 1 FROM tenant_users tu WHERE tu.tenant_id = m.workspace_id AND tu.user_id = u.id);

-- d) Active rows of a populated workspace whose user is not in the graph's membership.
CREATE TEMP TABLE _m063_tombstone ON COMMIT DROP AS
SELECT tu.tenant_id, tu.user_id, tu.ngac_node_id
FROM tenant_users tu
JOIN users u ON u.id = tu.user_id
WHERE tu.status = 'active'
  AND NOT EXISTS (SELECT 1 FROM _m063_unpopulated x WHERE x.workspace_id = tu.tenant_id)
  AND NOT EXISTS (SELECT 1 FROM _m063_member m
                  WHERE m.workspace_id = tu.tenant_id
                    AND m.node_id IN (COALESCE(tu.ngac_node_id, ''), COALESCE(u.ngac_node, '')));

DO $$
DECLARE
    lim   _m063_limits%ROWTYPE;
    n_bf  INTEGER;
    n_in  INTEGER;
    n_tomb INTEGER;
    skipped TEXT;
    non_active INTEGER;
    outside INTEGER;
    outside_ws TEXT;
    majority TEXT;
BEGIN
    SELECT * INTO lim FROM _m063_limits;
    SELECT count(*) INTO n_bf FROM _m063_backfill;
    SELECT count(*) INTO n_in FROM _m063_insert;
    SELECT count(*) INTO n_tomb FROM _m063_tombstone;
    RAISE NOTICE 'migration 063: % ngac_node_id to backfill, % graph members to list, % active rows to tombstone', n_bf, n_in, n_tomb;

    SELECT string_agg(x.workspace_id || ' (' || (SELECT count(*) FROM tenant_users tu WHERE tu.tenant_id = x.workspace_id AND tu.status = 'active') || ' active rows)', ', ')
      INTO skipped FROM _m063_unpopulated x
     WHERE EXISTS (SELECT 1 FROM tenant_users tu WHERE tu.tenant_id = x.workspace_id AND tu.status = 'active');
    IF skipped IS NOT NULL THEN
        RAISE NOTICE 'migration 063: skipped, the graph holds no user under Members (rows left as they are): %', skipped;
    END IF;

    -- Information only: a person listed as not active who is a member in the graph is
    -- not changed here (a disabled person is the administrator's decision).
    SELECT count(*) INTO non_active FROM tenant_users tu JOIN users u ON u.id = tu.user_id
     WHERE tu.status <> 'active' AND EXISTS (SELECT 1 FROM _m063_member m
                                              WHERE m.workspace_id = tu.tenant_id AND m.node_id IN (COALESCE(tu.ngac_node_id, ''), COALESCE(u.ngac_node, '')));
    RAISE NOTICE 'migration 063: % non-active rows whose user is a graph member (left as they are)', non_active;

    -- People who reach the workspace only through a department or role UA that is not under
    -- Members would be tombstoned: list them so a person looks before it is applied.
    SELECT count(*), string_agg(DISTINCT t.tenant_id, ', ') INTO outside, outside_ws
      FROM _m063_tombstone t
     WHERE EXISTS (SELECT 1 FROM ngac_assignments a JOIN ngac_nodes p ON p.id = a.parent_id
                    WHERE a.child_id = t.ngac_node_id AND p.node_type = 'UA'
                      AND (p.name LIKE 'Role\_%' OR p.name LIKE 'Dept\_%'
                           OR EXISTS (SELECT 1 FROM departments d WHERE d.workspace_id = t.tenant_id AND d.ngac_ua_id = p.id)));
    IF outside > 0 THEN
        RAISE NOTICE 'migration 063: % tombstone candidate(s) sit under a department/role UA that is not under Members (workspaces: %) - confirm they are meant to go', outside, outside_ws;
    END IF;

    -- One workspace losing most of its people is a sign the graph is wrong, not that they left.
    SELECT string_agg(x.tenant_id || ' (' || x.gone || ' of ' || x.total || ')', ', ') INTO majority
      FROM (SELECT t.tenant_id, count(*) AS gone,
                   (SELECT count(*) FROM tenant_users a WHERE a.tenant_id = t.tenant_id AND a.status = 'active') AS total
              FROM _m063_tombstone t GROUP BY t.tenant_id) x
     WHERE x.gone * 2 > x.total AND x.gone > 1;
    IF majority IS NOT NULL THEN
        RAISE EXCEPTION 'migration 063: more than half of a workspace''s active rows would be tombstoned (check the graph first): %', majority;
    END IF;

    IF n_bf > lim.backfill THEN
        RAISE EXCEPTION 'migration 063: % rows would get a node (limit %); check the pre-flight and raise the limit if expected', n_bf, lim.backfill;
    END IF;
    IF n_in > lim.inserted THEN
        RAISE EXCEPTION 'migration 063: % rows would be inserted (limit %); check the pre-flight and raise the limit if expected', n_in, lim.inserted;
    END IF;
    IF n_tomb > lim.tombstoned THEN
        RAISE EXCEPTION 'migration 063: % rows would be tombstoned (limit %); check the pre-flight and raise the limit if expected', n_tomb, lim.tombstoned;
    END IF;
END $$;

-- 3. Backup. The state before this run of every row it changes, kept once (the
--    primary key makes a repeat a no-op); rows it inserts are marked.
CREATE TABLE IF NOT EXISTS _reconcile_063_tenant_users (
    LIKE tenant_users INCLUDING DEFAULTS,
    inserted BOOLEAN NOT NULL DEFAULT false,
    PRIMARY KEY (tenant_id, user_id)
);

INSERT INTO _reconcile_063_tenant_users (tenant_id, user_id, role, status, open_id, ngac_node_id, joined_at, department_id)
SELECT tu.tenant_id, tu.user_id, tu.role, tu.status, tu.open_id, tu.ngac_node_id, tu.joined_at, tu.department_id
FROM tenant_users tu
WHERE (tu.tenant_id, tu.user_id) IN (SELECT tenant_id, user_id FROM _m063_backfill
                                     UNION SELECT tenant_id, user_id FROM _m063_tombstone)
ON CONFLICT DO NOTHING;

-- 4. b) The node of the user.
UPDATE tenant_users tu SET ngac_node_id = b.node_id
FROM _m063_backfill b
WHERE tu.tenant_id = b.tenant_id AND tu.user_id = b.user_id AND tu.ngac_node_id IS NULL;

-- 4. c) Graph members the table did not list.
INSERT INTO tenant_users (tenant_id, user_id, role, status, ngac_node_id)
SELECT tenant_id, user_id, role, 'active', node_id FROM _m063_insert
ON CONFLICT (tenant_id, user_id) DO NOTHING;

INSERT INTO _reconcile_063_tenant_users (tenant_id, user_id, role, status, open_id, ngac_node_id, joined_at, department_id, inserted)
SELECT tu.tenant_id, tu.user_id, tu.role, tu.status, tu.open_id, tu.ngac_node_id, tu.joined_at, tu.department_id, true
FROM tenant_users tu JOIN _m063_insert i ON i.tenant_id = tu.tenant_id AND i.user_id = tu.user_id
ON CONFLICT DO NOTHING;

-- 4. d) Rows of people the graph no longer holds.
UPDATE tenant_users tu SET status = 'removed', department_id = NULL
FROM _m063_tombstone t
WHERE tu.tenant_id = t.tenant_id AND tu.user_id = t.user_id AND tu.status = 'active';

COMMIT;
