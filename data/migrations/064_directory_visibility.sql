-- 064_directory_visibility.sql
--
-- Gives every workspace and every department the contacts-directory nodes the
-- workspace service creates for new ones (docs/specs/contacts-visibility,
-- tenant-ngac-init). Per workspace <id>, by name:
--
--     Directory_<id>        OA  under <id>_Mgmt          whoever holds manage on it administers the directory
--     DirectoryOpen_<id>    OA  under Directory          every member reads it: <id>_Members {read}
--     DirectoryNoDept_<id>  OA  under DirectoryOpen      people with no department
--     DirectoryOwners_<id>  OA  under Directory          owners, always visible: <id>_Members {read}
--     DirectoryLimited_<id> UA  under PC_<id>            NOT under Members; carries no association
--     prohibition DirectoryLimitedRule_<id>: subject DirectoryLimited, [read], target DirectoryOpen
--
-- and per department <d> of a workspace that has them:
--
--     Dir_<d>        OA  under the parent's Dir (or DirectoryOpen)
--     DirPeer_<d>    OA  under the parent's DirPeer (or Directory)
--     DeptPeers_<d>  UA  flat, under PC_<id>; DeptPeers_<d> {read} DirPeer_<d>
--
-- plus the DeptPeers edge of every user directly under a department's UA, and the
-- department column of tenant_users where it has drifted from the graph (auth filters
-- people by that column; the graph is the truth, as in 063). Every node carries the
-- properties {"type":"directory","workspace_id":<id>}, the mark the workspace service
-- accepts a directory node by.
--
-- A workspace with no row of that shape is left as it is: with no Directory_<id> it has
-- no visibility rules and every member sees every member (the release-A behaviour).
--
-- What is NOT touched: a node that already exists is left exactly as it is, hidden or
-- not (release-A code creates these nodes for new workspaces and departments, and moves
-- a hidden department's Dir and DirPeer under Directory); a department whose three nodes
-- exist gets no edge from this file. Edges of nodes created here are created with their
-- node. Only two things are reconciled on existing nodes: the workspace's own fixed edges
-- and the DeptPeers edges (= the users directly under the department's UA; a stale one
-- is removed and backed up).
--
-- Statements, in order (one transaction):
--   1. guard: every workspace has PC_<id> and <id>_Members under it; a workspace with no
--      <id>_Mgmt under its PC is skipped (and reported: there is nothing to put
--      the directory under, and nobody administers such a workspace); departments form a
--      tree of their workspace; a node with a directory name that is not that workspace's
--      directory node, or a second one, aborts
--   1b. directory nodes that a re-run of migration 023 renamed (dev only) get their names back
--   2. a unique index on directory node names (also holds back a writer that races this file)
--   3. workspaces: the five nodes, their fixed edges, the two grants of Members, the rule
--   4. departments, parents first: Dir, DirPeer, DeptPeers and the association
--   5. DeptPeers edges (added for direct users, stale ones removed)
--   6. tenant_users.department_id follows the graph
--   7. verification, then the thresholds below: a run that creates or changes more than
--      the pre-flight showed aborts and changes nothing
--
-- Re-runnable: every step is a fixpoint (second run: 0 rows, no backup rows).
--
-- Dev note: `make db-migrate` re-applies every migration, 023 included, and 023 renames the
-- directory's nodes (it takes them for folders and roles). Step 1b gives them their names back in
-- the same run, so a database that is migrated again ends in the same state; the thresholds count
-- those renames separately. A server applies each file once (deploy/migrate.sh), so 023 never
-- runs again there and step 1b finds nothing.
--
-- Release train: the code that creates these nodes for new workspaces and departments, and
-- keeps them when a department or a person moves, ships first ("release A"; a workspace
-- without Directory_<id> is skipped by it); this file ships alone after it ("release B").
-- Rolling images back to release A is safe: it reads both shapes.
--
-- Backups (each row kept once; a re-run adds nothing):
--   _reconcile_064_nodes          every node this file created
--   _reconcile_064_edges          every assignment/association created or removed (action)
--   _reconcile_064_prohibitions   every prohibition created
--   _reconcile_064_tenant_users   every department_id changed (old and new)
--   _reconcile_064_renames        every directory node given back the name 023 took from it
--
-- Reverse (then recreate policy and policy-read AND workspace, which caches directory node ids
-- for the life of its process, and `policy invalidate-cache --all-workspaces`; drop the
-- _reconcile_064_* tables if you will apply this file again):
--
--   BEGIN;
--   UPDATE ngac_nodes n SET name = b.clobbered_name
--     FROM _reconcile_064_renames b WHERE n.id = b.node_id AND n.name = b.restored_name;
--   UPDATE tenant_users t SET department_id = b.old_department_id
--     FROM _reconcile_064_tenant_users b WHERE t.tenant_id = b.tenant_id AND t.user_id = b.user_id;
--   DELETE FROM ngac_prohibitions p USING _reconcile_064_prohibitions b WHERE p.name = b.name;
--   DELETE FROM ngac_associations a USING _reconcile_064_edges e
--     WHERE e.action = 'created' AND e.kind = 'association' AND a.ua_id = e.child_or_ua AND a.oa_id = e.parent_or_oa;
--   DELETE FROM ngac_assignments a USING _reconcile_064_edges e
--     WHERE e.action = 'created' AND e.kind = 'assignment' AND a.child_id = e.child_or_ua AND a.parent_id = e.parent_or_oa;
--   -- only edges whose two nodes still exist (a partial restore must not abort on a foreign key)
--   INSERT INTO ngac_assignments (id, child_id, parent_id)
--     SELECT gen_random_uuid()::text, e.child_or_ua, e.parent_or_oa FROM _reconcile_064_edges e
--      WHERE e.action = 'removed' AND e.kind = 'assignment'
--        AND EXISTS (SELECT 1 FROM ngac_nodes WHERE id = e.child_or_ua)
--        AND EXISTS (SELECT 1 FROM ngac_nodes WHERE id = e.parent_or_oa)
--     ON CONFLICT (child_id, parent_id) DO NOTHING;
--   DELETE FROM ngac_nodes n USING _reconcile_064_nodes b WHERE b.node_id = n.id;
--   DROP INDEX IF EXISTS ngac_nodes_directory_name_uniq;
--   COMMIT;
--
-- Locks: step 2 builds a unique index, which holds a SHARE lock on ngac_nodes until the
-- transaction ends, so graph writes (every workspace/department/role/folder change) wait for the
-- run, seconds on a graph this size. Run it, and its dry run (which builds the index too before
-- it rolls back), off-peak.
--
-- Thresholds: the values below are placeholders sized for a development database. They are set
-- from the production dry-run when release B is prepared.
--
-- NOTE: writes ngac_* and ngac_prohibitions directly and so bypasses EPP invalidation. The
-- policy services keep the graph and the prohibitions in memory: recreate policy and
-- policy-read after applying (deploy.sh does), then drop the cached decisions with
-- `policy invalidate-cache --all-workspaces` (`make dev-restart s=policy` and `s=policy-read`,
-- then `make policy-invalidate` in dev; deploy.sh runs it too).
--
-- Dry run (read-only: the whole file with COMMIT turned into ROLLBACK prints, in NOTICE
-- lines, what it would create and change, and still enforces its guards):
--
--   sed 's/^COMMIT;$/ROLLBACK;/' data/migrations/064_directory_visibility.sql \
--     | docker compose --profile tools run --rm -T --entrypoint psql migrate -X -v ON_ERROR_STOP=1
--
-- PRE-FLIGHT counts (read-only, same predicates; paste them in the PR and set the
-- thresholds below from them):
--
--   -- workspaces to give a directory, and those skipped for want of Mgmt
--   --   SELECT count(*) FILTER (WHERE NOT EXISTS (SELECT 1 FROM ngac_nodes WHERE name = 'Directory_' || w.id)) AS to_build,
--   --          count(*) FILTER (WHERE NOT EXISTS (SELECT 1 FROM ngac_nodes m JOIN ngac_assignments a ON a.child_id = m.id
--   --            JOIN ngac_nodes pc ON pc.id = a.parent_id AND pc.name = 'PC_' || w.id
--   --            WHERE m.name = w.id || '_Mgmt' AND m.node_type = 'OA')) AS without_mgmt FROM workspaces w;
--   -- departments without nodes, people to put under a peers UA, rows whose department drifted
--   --   SELECT count(*) FROM departments d WHERE NOT EXISTS (SELECT 1 FROM ngac_nodes WHERE name = 'Dir_' || d.id);
--   --   SELECT count(*) FROM departments d JOIN ngac_assignments a ON a.parent_id = d.ngac_ua_id
--   --     JOIN ngac_nodes u ON u.id = a.child_id AND u.node_type = 'U';
--   --   SELECT count(*) FROM tenant_users tu JOIN users u ON u.id = tu.user_id
--   --    WHERE tu.status <> 'removed' AND tu.department_id IS DISTINCT FROM
--   --      (SELECT min(d.id) FROM ngac_assignments a JOIN departments d ON d.ngac_ua_id = a.parent_id AND d.workspace_id = tu.tenant_id
--   --        WHERE a.child_id = COALESCE(tu.ngac_node_id, u.ngac_node))
--   --      AND COALESCE(tu.ngac_node_id, u.ngac_node) IS NOT NULL;

BEGIN;

-- Thresholds. A run that would create or change more than this aborts, so a graph that
-- differs from what the pre-flight showed is looked at by a person instead of being
-- rewritten. Set them from the production pre-flight counts.
--   nodes      : directory nodes created (5 per workspace, 3 per department)
--   edges      : assignments and associations created
--   removed    : stale DeptPeers assignments removed
--   rules      : prohibitions created (1 per workspace)
--   rows       : tenant_users.department_id values changed
--   skipped    : workspaces left alone for want of a Mgmt OA
--   restored   : directory nodes given back the name migration 023 took from them (0 on a server)
CREATE TEMP TABLE _m064_limits (nodes INT, edges INT, removed INT, rules INT, rows INT, skipped INT, restored INT) ON COMMIT DROP;
INSERT INTO _m064_limits VALUES (2000, 3000, 100, 400, 200, 10, 2000);

-- What this run did so far (the backup tables keep rows across runs; these count one run).
CREATE TEMP TABLE _m064_run (nodes INT, edges INT, removed INT, rules INT, rows INT, skipped INT, restored INT) ON COMMIT DROP;
INSERT INTO _m064_run VALUES (0, 0, 0, 0, 0, 0, 0);
CREATE TEMP TABLE _m064_made (node_id TEXT PRIMARY KEY) ON COMMIT DROP;

-- The workspaces this run handles, with the ids it needs, and the directory nodes it finds.
CREATE TEMP TABLE _m064_ws (
    workspace_id TEXT PRIMARY KEY, pc_id TEXT NOT NULL, members_id TEXT NOT NULL, mgmt_id TEXT NOT NULL,
    directory_id TEXT, open_id TEXT, nodept_id TEXT, owners_id TEXT, limited_id TEXT
) ON COMMIT DROP;

-- 1. Guard.
DO $$
DECLARE missing TEXT;
BEGIN
    SELECT string_agg(w.id, ', ') INTO missing
    FROM workspaces w
    WHERE NOT EXISTS (SELECT 1 FROM ngac_nodes n WHERE n.name = 'PC_' || w.id AND n.node_type = 'PC')
       OR NOT EXISTS (SELECT 1 FROM ngac_nodes m JOIN ngac_nodes pc ON pc.name = 'PC_' || w.id AND pc.node_type = 'PC'
                      JOIN ngac_assignments a ON a.child_id = m.id AND a.parent_id = pc.id
                      WHERE m.name = w.id || '_Members' AND m.node_type = 'UA');
    IF missing IS NOT NULL THEN
        RAISE EXCEPTION 'migration 064: workspace(s) lack PC_<id> or <id>_Members under it (run 059 first): %', missing;
    END IF;
END $$;

INSERT INTO _m064_ws (workspace_id, pc_id, members_id, mgmt_id)
SELECT w.id, pc.id, m.id, g.id
FROM workspaces w
JOIN ngac_nodes pc ON pc.name = 'PC_' || w.id AND pc.node_type = 'PC'
JOIN ngac_nodes m ON m.name = w.id || '_Members' AND m.node_type = 'UA'
JOIN ngac_nodes g ON g.name = w.id || '_Mgmt' AND g.node_type = 'OA'
WHERE EXISTS (SELECT 1 FROM ngac_assignments a WHERE a.child_id = m.id AND a.parent_id = pc.id)
  AND EXISTS (SELECT 1 FROM ngac_assignments a WHERE a.child_id = g.id AND a.parent_id = pc.id);

DO $$
DECLARE
    skipped TEXT;
    n_skipped INTEGER;
    lim _m064_limits%ROWTYPE;
    n_unreached INTEGER;
BEGIN
    SELECT * INTO lim FROM _m064_limits;
    SELECT string_agg(w.id, ', '), count(*) INTO skipped, n_skipped
    FROM workspaces w WHERE NOT EXISTS (SELECT 1 FROM _m064_ws x WHERE x.workspace_id = w.id);
    UPDATE _m064_run SET skipped = n_skipped;
    IF n_skipped > 0 THEN
        RAISE NOTICE 'migration 064: % workspace(s) have no % under their policy class and are left alone: %', n_skipped, '<id>_Mgmt', skipped;
    END IF;
    IF n_skipped > lim.skipped THEN
        RAISE EXCEPTION 'migration 064: % workspaces skipped for want of Mgmt (limit %); check the pre-flight and raise the limit if expected', n_skipped, lim.skipped;
    END IF;

    -- Departments must form trees inside their workspace: parents are placed first, and a cycle or a
    -- parent in another workspace would leave a department with nowhere to hang.
    WITH RECURSIVE t(id, path) AS (
        SELECT d.id, ARRAY[d.id] FROM departments d WHERE d.parent_id IS NULL
        UNION ALL
        SELECT c.id, t.path || c.id FROM departments c JOIN t ON c.parent_id = t.id
        JOIN departments p ON p.id = c.parent_id AND p.workspace_id = c.workspace_id
        WHERE NOT c.id = ANY (t.path)
    )
    SELECT count(*) INTO n_unreached FROM departments d WHERE NOT EXISTS (SELECT 1 FROM t WHERE t.id = d.id);
    IF n_unreached > 0 THEN
        RAISE EXCEPTION 'migration 064: % department(s) are not reachable from a root of their own workspace (cycle, or a parent in another workspace); resolve by hand', n_unreached;
    END IF;

    -- A department's UA must exist: the peers edges are read from it.
    SELECT string_agg(d.id, ', ') INTO skipped FROM departments d
    WHERE EXISTS (SELECT 1 FROM _m064_ws x WHERE x.workspace_id = d.workspace_id)
      AND NOT EXISTS (SELECT 1 FROM ngac_nodes n WHERE n.id = d.ngac_ua_id AND n.node_type = 'UA');
    IF skipped IS NOT NULL THEN
        RAISE EXCEPTION 'migration 064: department(s) have no UA node: %', skipped;
    END IF;
END $$;

-- 1b. Names that migration 023 took back. A server applies each migration once, but `make db-migrate`
--     re-applies the whole chain to a dev database, and 023 renames any OA below a workspace's policy
--     class, and any UA directly under it, that carries none of the names it knows: which is how the
--     directory's nodes look to it (Folder_<id> / Role_<id>, the old name kept in ngac_node_renames).
--     A directory node (marked) that 023 renamed gets its name back; one whose name is taken by
--     another node cannot, and the migration stops. On a server this finds nothing.
CREATE TABLE IF NOT EXISTS _reconcile_064_renames (
    node_id        TEXT        PRIMARY KEY,
    clobbered_name TEXT        NOT NULL,
    restored_name  TEXT        NOT NULL,
    recorded_at    TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

DO $$
DECLARE
    blocked TEXT;
    n_restored INTEGER := 0;
BEGIN
    IF to_regclass('public.ngac_node_renames') IS NULL THEN
        RETURN;
    END IF;
    SELECT string_agg(n.id || ' (' || r.old_name || ')', ', ') INTO blocked
    FROM ngac_nodes n
    JOIN ngac_node_renames r ON r.node_id = n.id AND r.new_name = n.name
    WHERE n.properties->>'type' = 'directory' AND n.name <> r.old_name
      AND EXISTS (SELECT 1 FROM ngac_nodes x WHERE x.name = r.old_name AND x.id <> n.id);
    IF blocked IS NOT NULL THEN
        RAISE EXCEPTION 'migration 064: directory node(s) renamed by migration 023 cannot give back its name, another node holds it: %; resolve by hand', blocked;
    END IF;

    INSERT INTO _reconcile_064_renames (node_id, clobbered_name, restored_name)
    SELECT n.id, n.name, r.old_name
    FROM ngac_nodes n JOIN ngac_node_renames r ON r.node_id = n.id AND r.new_name = n.name
    WHERE n.properties->>'type' = 'directory' AND n.name <> r.old_name
    ON CONFLICT (node_id) DO UPDATE SET clobbered_name = EXCLUDED.clobbered_name, restored_name = EXCLUDED.restored_name;

    UPDATE ngac_nodes n SET name = r.old_name
    FROM ngac_node_renames r
    WHERE r.node_id = n.id AND r.new_name = n.name AND n.name <> r.old_name
      AND n.properties->>'type' = 'directory';
    GET DIAGNOSTICS n_restored = ROW_COUNT;
    UPDATE _m064_run SET restored = n_restored;
    IF n_restored > 0 THEN
        RAISE NOTICE 'migration 064: % directory node(s) renamed by a re-run of migration 023 got their names back', n_restored;
    END IF;
END $$;

-- 2. One node per directory name. Partial so that it concerns only the directory's nodes.
CREATE UNIQUE INDEX IF NOT EXISTS ngac_nodes_directory_name_uniq
    ON ngac_nodes (name) WHERE properties->>'type' = 'directory';

CREATE TABLE IF NOT EXISTS _reconcile_064_nodes (
    node_id      TEXT        PRIMARY KEY,
    workspace_id TEXT        NOT NULL,
    node_type    TEXT        NOT NULL,
    name         TEXT        NOT NULL,
    recorded_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE TABLE IF NOT EXISTS _reconcile_064_edges (
    action       TEXT        NOT NULL CHECK (action IN ('created', 'removed')),
    kind         TEXT        NOT NULL CHECK (kind IN ('assignment', 'association')),
    workspace_id TEXT        NOT NULL,
    child_or_ua  TEXT        NOT NULL,
    parent_or_oa TEXT        NOT NULL,
    operations   TEXT[],
    recorded_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (action, kind, child_or_ua, parent_or_oa)
);
CREATE TABLE IF NOT EXISTS _reconcile_064_prohibitions (
    name         TEXT        PRIMARY KEY,
    workspace_id TEXT        NOT NULL,
    subject_id   TEXT        NOT NULL,
    target_oa_ids TEXT[]     NOT NULL,
    recorded_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE TABLE IF NOT EXISTS _reconcile_064_tenant_users (
    tenant_id         TEXT        NOT NULL,
    user_id           TEXT        NOT NULL,
    old_department_id TEXT,
    new_department_id TEXT,
    recorded_at       TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (tenant_id, user_id)
);

-- Session-local helpers (dropped with the connection).

-- A directory node by name: the one that exists (it must be this workspace's directory node
-- of that type, or the migration stops), or a new one.
CREATE OR REPLACE FUNCTION pg_temp.m064_node(ws TEXT, nm TEXT, nt TEXT) RETURNS TEXT
LANGUAGE plpgsql AS $$
DECLARE
    n_found INTEGER; found_id TEXT; found_type TEXT; found_props JSONB; new_id TEXT;
BEGIN
    SELECT count(*), min(id), min(node_type), (array_agg(properties))[1]
      INTO n_found, found_id, found_type, found_props FROM ngac_nodes WHERE name = nm;
    IF n_found > 1 THEN
        RAISE EXCEPTION 'migration 064: % nodes are named %; resolve by hand', n_found, nm;
    END IF;
    IF n_found = 1 THEN
        IF found_type <> nt OR COALESCE(found_props->>'type', '') <> 'directory' OR COALESCE(found_props->>'workspace_id', '') <> ws THEN
            RAISE EXCEPTION 'migration 064: node % (%) is named like a directory node of workspace % but is not one; resolve by hand', found_id, nm, ws;
        END IF;
        RETURN found_id;
    END IF;
    new_id := gen_random_uuid()::text;
    INSERT INTO ngac_nodes (id, name, node_type, properties)
    VALUES (new_id, nm, nt, jsonb_build_object('type', 'directory', 'workspace_id', ws));
    INSERT INTO _reconcile_064_nodes (node_id, workspace_id, node_type, name) VALUES (new_id, ws, nt, nm);
    INSERT INTO _m064_made (node_id) VALUES (new_id);
    UPDATE _m064_run SET nodes = nodes + 1;
    RETURN new_id;
END $$;

CREATE OR REPLACE FUNCTION pg_temp.m064_made(id TEXT) RETURNS BOOLEAN
LANGUAGE sql AS $$ SELECT EXISTS (SELECT 1 FROM _m064_made WHERE node_id = id) $$;

CREATE OR REPLACE FUNCTION pg_temp.m064_assign(ws TEXT, child TEXT, parent TEXT) RETURNS VOID
LANGUAGE plpgsql AS $$
BEGIN
    INSERT INTO ngac_assignments (id, child_id, parent_id) VALUES (gen_random_uuid()::text, child, parent)
    ON CONFLICT (child_id, parent_id) DO NOTHING;
    IF FOUND THEN
        INSERT INTO _reconcile_064_edges (action, kind, workspace_id, child_or_ua, parent_or_oa)
        VALUES ('created', 'assignment', ws, child, parent) ON CONFLICT DO NOTHING;
        UPDATE _m064_run SET edges = edges + 1;
    END IF;
END $$;

CREATE OR REPLACE FUNCTION pg_temp.m064_associate(ws TEXT, ua TEXT, oa TEXT, ops TEXT[]) RETURNS VOID
LANGUAGE plpgsql AS $$
BEGIN
    INSERT INTO ngac_associations (id, ua_id, oa_id, operations) VALUES (gen_random_uuid()::text, ua, oa, ops)
    ON CONFLICT (ua_id, oa_id) DO NOTHING;
    IF FOUND THEN
        INSERT INTO _reconcile_064_edges (action, kind, workspace_id, child_or_ua, parent_or_oa, operations)
        VALUES ('created', 'association', ws, ua, oa, ops) ON CONFLICT DO NOTHING;
        UPDATE _m064_run SET edges = edges + 1;
    END IF;
END $$;

-- 3. Workspaces. Names are the ngac package's helpers (ngac_ops.go) with the id filled in.
DO $$
DECLARE
    w   RECORD;
    d   _m064_ws%ROWTYPE;
    rule RECORD;
    rule_name TEXT;
BEGIN
    FOR w IN SELECT * FROM _m064_ws ORDER BY workspace_id LOOP
        d := w;
        d.directory_id := pg_temp.m064_node(w.workspace_id, format('Directory_%s',        w.workspace_id), 'OA');
        d.open_id      := pg_temp.m064_node(w.workspace_id, format('DirectoryOpen_%s',    w.workspace_id), 'OA');
        d.nodept_id    := pg_temp.m064_node(w.workspace_id, format('DirectoryNoDept_%s',  w.workspace_id), 'OA');
        d.owners_id    := pg_temp.m064_node(w.workspace_id, format('DirectoryOwners_%s',  w.workspace_id), 'OA');
        d.limited_id   := pg_temp.m064_node(w.workspace_id, format('DirectoryLimited_%s', w.workspace_id), 'UA');

        -- The workspace's fixed shape is reconciled whether or not its nodes are new.
        PERFORM pg_temp.m064_assign(w.workspace_id, d.directory_id, w.mgmt_id);
        PERFORM pg_temp.m064_assign(w.workspace_id, d.open_id,      d.directory_id);
        PERFORM pg_temp.m064_assign(w.workspace_id, d.nodept_id,    d.open_id);
        -- Beside Open, never under it: the limit must not reach the owners.
        PERFORM pg_temp.m064_assign(w.workspace_id, d.owners_id,    d.directory_id);
        -- Under the class, never under Members: it grants nothing and makes nobody a member.
        PERFORM pg_temp.m064_assign(w.workspace_id, d.limited_id,   w.pc_id);
        PERFORM pg_temp.m064_associate(w.workspace_id, w.members_id, d.open_id,   ARRAY['read']);
        PERFORM pg_temp.m064_associate(w.workspace_id, w.members_id, d.owners_id, ARRAY['read']);

        -- The one rule: people under DirectoryLimited cannot read DirectoryOpen.
        rule_name := format('DirectoryLimitedRule_%s', w.workspace_id);
        SELECT * INTO rule FROM ngac_prohibitions WHERE name = rule_name;
        IF NOT FOUND THEN
            INSERT INTO ngac_prohibitions (name, subject_id, operations, target_oa_ids, intersection)
            VALUES (rule_name, d.limited_id, ARRAY['read'], ARRAY[d.open_id], FALSE);
            INSERT INTO _reconcile_064_prohibitions (name, workspace_id, subject_id, target_oa_ids)
            VALUES (rule_name, w.workspace_id, d.limited_id, ARRAY[d.open_id])
            ON CONFLICT (name) DO UPDATE SET subject_id = EXCLUDED.subject_id, target_oa_ids = EXCLUDED.target_oa_ids;
            UPDATE _m064_run SET rules = rules + 1;
        ELSIF rule.subject_id <> d.limited_id OR rule.target_oa_ids <> ARRAY[d.open_id]
              OR rule.operations <> ARRAY['read'] OR COALESCE(rule.intersection, FALSE) THEN
            RAISE EXCEPTION 'migration 064: prohibition % exists but is not the directory rule of workspace %; resolve by hand', rule_name, w.workspace_id;
        END IF;

        UPDATE _m064_ws SET directory_id = d.directory_id, open_id = d.open_id, nodept_id = d.nodept_id,
                            owners_id = d.owners_id, limited_id = d.limited_id
         WHERE workspace_id = w.workspace_id;
    END LOOP;
END $$;

-- 4. Departments, parents first. A department whose nodes exist is not touched; the
--    edges of the nodes created here are created with them.
DO $$
DECLARE
    dep      RECORD;
    ws       _m064_ws%ROWTYPE;
    dir_id   TEXT; peer_id TEXT; peers_id TEXT;
    pdir     TEXT; ppeer   TEXT;
BEGIN
    FOR dep IN
        WITH RECURSIVE t(id, workspace_id, parent_id, ua_id, depth, path) AS (
            SELECT d.id, d.workspace_id, d.parent_id, d.ngac_ua_id, 0, ARRAY[d.id] FROM departments d WHERE d.parent_id IS NULL
            UNION ALL
            SELECT c.id, c.workspace_id, c.parent_id, c.ngac_ua_id, t.depth + 1, t.path || c.id
            FROM departments c JOIN t ON c.parent_id = t.id
            WHERE NOT c.id = ANY (t.path) AND c.workspace_id = t.workspace_id
        )
        SELECT * FROM t ORDER BY depth, workspace_id, id
    LOOP
        SELECT * INTO ws FROM _m064_ws WHERE workspace_id = dep.workspace_id;
        IF NOT FOUND THEN CONTINUE; END IF;   -- a workspace left alone

        dir_id   := pg_temp.m064_node(dep.workspace_id, format('Dir_%s',       dep.id), 'OA');
        peer_id  := pg_temp.m064_node(dep.workspace_id, format('DirPeer_%s',   dep.id), 'OA');
        peers_id := pg_temp.m064_node(dep.workspace_id, format('DeptPeers_%s', dep.id), 'UA');

        pdir := ws.open_id; ppeer := ws.directory_id;
        IF dep.parent_id IS NOT NULL THEN
            SELECT id INTO pdir  FROM ngac_nodes WHERE name = format('Dir_%s',     dep.parent_id) AND node_type = 'OA';
            SELECT id INTO ppeer FROM ngac_nodes WHERE name = format('DirPeer_%s', dep.parent_id) AND node_type = 'OA';
            IF pdir IS NULL OR ppeer IS NULL THEN
                RAISE EXCEPTION 'migration 064: department % has no directory nodes for its parent %', dep.id, dep.parent_id;
            END IF;
        END IF;

        IF pg_temp.m064_made(dir_id)   THEN PERFORM pg_temp.m064_assign(dep.workspace_id, dir_id,   pdir); END IF;
        IF pg_temp.m064_made(peer_id)  THEN PERFORM pg_temp.m064_assign(dep.workspace_id, peer_id,  ppeer); END IF;
        IF pg_temp.m064_made(peers_id) THEN PERFORM pg_temp.m064_assign(dep.workspace_id, peers_id, ws.pc_id); END IF;
        IF pg_temp.m064_made(peer_id) OR pg_temp.m064_made(peers_id) THEN
            PERFORM pg_temp.m064_associate(dep.workspace_id, peers_id, peer_id, ARRAY['read']);
        END IF;
    END LOOP;
END $$;

-- 5. DeptPeers edges: a department's peers UA holds the users directly under the department's UA.
DO $$
DECLARE r RECORD;
BEGIN
    FOR r IN
        SELECT d.workspace_id, a.child_id AS user_node, p.id AS peers_id
        FROM departments d
        JOIN _m064_ws x ON x.workspace_id = d.workspace_id
        JOIN ngac_nodes p ON p.name = format('DeptPeers_%s', d.id) AND p.node_type = 'UA'
        JOIN ngac_assignments a ON a.parent_id = d.ngac_ua_id
        JOIN ngac_nodes u ON u.id = a.child_id AND u.node_type = 'U'
        ORDER BY d.workspace_id, d.id, a.child_id
    LOOP
        PERFORM pg_temp.m064_assign(r.workspace_id, r.user_node, r.peers_id);
    END LOOP;

    -- A user under a peers UA who is no longer directly under the department's UA: the edge is
    -- stale (a person who left the department keeps a view of its peers), backed up and removed.
    FOR r IN
        SELECT d.workspace_id, a.id AS assignment_id, a.child_id, a.parent_id
        FROM departments d
        JOIN _m064_ws x ON x.workspace_id = d.workspace_id
        JOIN ngac_nodes p ON p.name = format('DeptPeers_%s', d.id) AND p.node_type = 'UA'
        JOIN ngac_assignments a ON a.parent_id = p.id
        JOIN ngac_nodes u ON u.id = a.child_id AND u.node_type = 'U'
        WHERE NOT EXISTS (SELECT 1 FROM ngac_assignments s WHERE s.child_id = a.child_id AND s.parent_id = d.ngac_ua_id)
        ORDER BY d.workspace_id, d.id, a.child_id
    LOOP
        INSERT INTO _reconcile_064_edges (action, kind, workspace_id, child_or_ua, parent_or_oa)
        VALUES ('removed', 'assignment', r.workspace_id, r.child_id, r.parent_id) ON CONFLICT DO NOTHING;
        DELETE FROM ngac_assignments WHERE id = r.assignment_id;
        UPDATE _m064_run SET removed = removed + 1;
    END LOOP;
END $$;

-- 6. tenant_users.department_id follows the graph: the department whose UA the person sits
--    directly under, none when there is none. A person under two departments is ambiguous:
--    the graph itself needs a decision, so the migration stops.
CREATE TEMP TABLE _m064_rows ON COMMIT DROP AS
SELECT tu.tenant_id, tu.user_id, tu.department_id AS old_department_id, g.dept_id AS new_department_id, g.n AS n_departments
FROM tenant_users tu
JOIN users u ON u.id = tu.user_id
JOIN _m064_ws x ON x.workspace_id = tu.tenant_id
CROSS JOIN LATERAL (
    SELECT min(d.id) AS dept_id, count(*) AS n
    FROM ngac_assignments a
    JOIN departments d ON d.ngac_ua_id = a.parent_id AND d.workspace_id = tu.tenant_id
    WHERE a.child_id = COALESCE(tu.ngac_node_id, u.ngac_node)
) g
WHERE tu.status <> 'removed'
  AND COALESCE(tu.ngac_node_id, u.ngac_node) IS NOT NULL
  AND tu.department_id IS DISTINCT FROM g.dept_id;

DO $$
DECLARE ambiguous TEXT;
BEGIN
    SELECT string_agg(tenant_id || '/' || user_id, ', ') INTO ambiguous FROM _m064_rows WHERE n_departments > 1;
    IF ambiguous IS NOT NULL THEN
        RAISE EXCEPTION 'migration 064: person(s) sit directly under more than one department (tenant/user): %; move them to one by hand', ambiguous;
    END IF;
END $$;

INSERT INTO _reconcile_064_tenant_users (tenant_id, user_id, old_department_id, new_department_id)
SELECT tenant_id, user_id, old_department_id, new_department_id FROM _m064_rows
ON CONFLICT (tenant_id, user_id) DO NOTHING;

UPDATE tenant_users tu SET department_id = r.new_department_id
FROM _m064_rows r WHERE tu.tenant_id = r.tenant_id AND tu.user_id = r.user_id;
UPDATE _m064_run SET rows = (SELECT count(*) FROM _m064_rows);

-- 7. Verification and thresholds. Either failing leaves the database as it was.
DO $$
DECLARE
    lim  _m064_limits%ROWTYPE;
    run  _m064_run%ROWTYPE;
    bad  TEXT;
BEGIN
    SELECT * INTO lim FROM _m064_limits;
    SELECT * INTO run FROM _m064_run;

    SELECT string_agg(x.workspace_id, ', ') INTO bad FROM _m064_ws x
    WHERE x.directory_id IS NULL OR x.open_id IS NULL OR x.nodept_id IS NULL OR x.owners_id IS NULL OR x.limited_id IS NULL
       OR NOT EXISTS (SELECT 1 FROM ngac_prohibitions p WHERE p.name = 'DirectoryLimitedRule_' || x.workspace_id)
       OR NOT EXISTS (SELECT 1 FROM ngac_associations a WHERE a.ua_id = x.members_id AND a.oa_id = x.open_id)
       OR NOT EXISTS (SELECT 1 FROM ngac_associations a WHERE a.ua_id = x.members_id AND a.oa_id = x.owners_id);
    IF bad IS NOT NULL THEN
        RAISE EXCEPTION 'migration 064: workspace(s) still lack their directory: %', bad;
    END IF;

    SELECT string_agg(d.id, ', ') INTO bad FROM departments d
    WHERE EXISTS (SELECT 1 FROM _m064_ws x WHERE x.workspace_id = d.workspace_id)
      AND (SELECT count(*) FROM ngac_nodes n WHERE n.name IN ('Dir_' || d.id, 'DirPeer_' || d.id, 'DeptPeers_' || d.id)) <> 3;
    IF bad IS NOT NULL THEN
        RAISE EXCEPTION 'migration 064: department(s) still lack their directory nodes: %', bad;
    END IF;

    RAISE NOTICE 'migration 064: created % nodes, % edges, % prohibitions; removed % stale peers edges; changed % department_id rows; skipped % workspaces; restored % names',
        run.nodes, run.edges, run.rules, run.removed, run.rows, run.skipped, run.restored;
    IF run.nodes > lim.nodes THEN
        RAISE EXCEPTION 'migration 064: % nodes would be created (limit %); check the pre-flight and raise the limit if expected', run.nodes, lim.nodes;
    END IF;
    IF run.edges > lim.edges THEN
        RAISE EXCEPTION 'migration 064: % edges would be created (limit %); check the pre-flight and raise the limit if expected', run.edges, lim.edges;
    END IF;
    IF run.removed > lim.removed THEN
        RAISE EXCEPTION 'migration 064: % stale edges would be removed (limit %); check the pre-flight and raise the limit if expected', run.removed, lim.removed;
    END IF;
    IF run.rules > lim.rules THEN
        RAISE EXCEPTION 'migration 064: % prohibitions would be created (limit %); check the pre-flight and raise the limit if expected', run.rules, lim.rules;
    END IF;
    IF run.rows > lim.rows THEN
        RAISE EXCEPTION 'migration 064: % department_id values would change (limit %); check the pre-flight and raise the limit if expected', run.rows, lim.rows;
    END IF;
    IF run.restored > lim.restored THEN
        RAISE EXCEPTION 'migration 064: % renamed directory nodes would be restored (limit %); check the pre-flight and raise the limit if expected', run.restored, lim.restored;
    END IF;
END $$;

COMMIT;
