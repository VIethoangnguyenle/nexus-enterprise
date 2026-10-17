-- 059_ngac_workspace_id_keyed_repair.sql
--
-- Gives every workspace the ID-keyed NGAC topology the code looks nodes up by:
--
--     PC_<id>   <id>_Owners   <id>_Members   <id>_Channels
--
-- Why: membership is "the user is under <id>_Members", and the reconcilers that
-- follow (private groups, channel membership, removed members) key on those
-- names. A workspace whose PC and attribute nodes still carry an older name, or
-- are missing, would be skipped by a check and wiped by a reconcile. Dev has
-- two (test-ws-*): their policy class is named pc-test-ws-* and has no child
-- node at all. Migration 023 renamed what it could match by name pattern; this
-- one matches through workspaces.ngac_pc_id (a foreign key), never by name alone.
--
-- Per workspace, matched through workspaces.ngac_pc_id:
--   * the PC is renamed PC_<id> and given workspace_id / tenant_id / scope
--     properties where it lacks them (existing values win);
--   * Members, Owners (UA) and Channels (OA): a node already named <id>_<Role>
--     is left alone (two nodes with that name abort the migration); otherwise the single direct child of the PC (Owners: or of
--     Members) whose name ends in the role is renamed; with none, the node is
--     created. More than one candidate aborts the migration: pick by hand, and so does a UA
--     candidate associated with an OA other than the workspace's Channels,
--     Documents or Mgmt (it is some other group). Role_, Dept_, Ch_, Cal* and
--     Tenant* nodes are never candidates.
--   * assignments as CreateWorkspace makes them: Members->PC, Owners->PC,
--     Channels->PC, Owners->Members. Existing edges are kept.
--   * associations Owners->Channels (owner ops) and Members->Channels (member
--     channel ops), only for a node this run created; a node that was only
--     renamed keeps the associations it already had.
--   * TenantMember_<id> is placed under Members and TenantOwner_<id> under
--     Owners where they exist (that is how provisioning has always arranged
--     them).
-- Mgmt, Documents and the rest of a legacy workspace are not touched.
--   * a partial unique index keeps a "..._Members/_Owners/_Channels" name to one node.
--
-- Re-runnable: every step is guarded by what it would change, so a second run
-- changes nothing. Ends with a check that aborts the transaction if any
-- workspace still lacks one of the four nodes.
--
-- Backup and reverse: _reconcile_059_nodes holds each repaired or created node
-- (with the name and properties it had), _reconcile_059_edges each edge added.
--
--   -- undo renames and property patches
--   UPDATE ngac_nodes n SET name = b.old_name, properties = b.old_properties
--     FROM _reconcile_059_nodes b WHERE b.node_id = n.id AND b.action = 'repaired';
--   -- undo creations and added edges
--   DELETE FROM ngac_assignments a USING _reconcile_059_edges e
--     WHERE e.kind = 'assignment' AND a.child_id = e.child_or_ua AND a.parent_id = e.parent_or_oa;
--   DELETE FROM ngac_associations a USING _reconcile_059_edges e
--     WHERE e.kind = 'association' AND a.ua_id = e.child_or_ua AND a.oa_id = e.parent_or_oa;
--   DELETE FROM ngac_nodes n USING _reconcile_059_nodes b WHERE b.node_id = n.id AND b.action = 'created';
--   -- then recreate policy and policy-read and run: policy invalidate-cache --all-workspaces
--
-- Dry run (read-only; prints the mapping this migration would apply) is in
-- docs/deployment.md, "Migrations that rewrite the authorization graph".
--
-- NOTE: writes ngac_* directly and so bypasses EPP invalidation. The policy
-- services keep the graph in memory and index nodes by name: recreate policy and
-- policy-read after applying (deploy.sh does), then drop the cached decisions
-- with `policy invalidate-cache --all-workspaces` (`make policy-invalidate` in
-- dev; deploy.sh runs it too).
--
-- Old code tolerates this file: it only makes names ID-keyed, as 023 did, and
-- adds nodes the old code never looked for. It ships alone, before any code that
-- depends on the names.

BEGIN;

CREATE TABLE IF NOT EXISTS _reconcile_059_nodes (
    workspace_id   TEXT        NOT NULL,
    node_id        TEXT        NOT NULL,
    action         TEXT        NOT NULL CHECK (action IN ('repaired', 'created')),
    node_type      TEXT        NOT NULL,
    old_name       TEXT,
    new_name       TEXT        NOT NULL,
    old_properties JSONB,
    recorded_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (node_id, action)
);

CREATE TABLE IF NOT EXISTS _reconcile_059_edges (
    workspace_id TEXT        NOT NULL,
    kind         TEXT        NOT NULL CHECK (kind IN ('assignment', 'association')),
    child_or_ua  TEXT        NOT NULL,
    parent_or_oa TEXT        NOT NULL,
    operations   TEXT[],
    recorded_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (kind, child_or_ua, parent_or_oa)
);

-- Session-local helpers (dropped with the connection).
CREATE OR REPLACE FUNCTION pg_temp.m059_assign(ws TEXT, child TEXT, parent TEXT) RETURNS VOID
LANGUAGE plpgsql AS $$
BEGIN
    INSERT INTO ngac_assignments (id, child_id, parent_id)
    VALUES (gen_random_uuid()::text, child, parent)
    ON CONFLICT (child_id, parent_id) DO NOTHING;
    IF FOUND THEN
        INSERT INTO _reconcile_059_edges (workspace_id, kind, child_or_ua, parent_or_oa)
        VALUES (ws, 'assignment', child, parent) ON CONFLICT DO NOTHING;
    END IF;
END $$;

CREATE OR REPLACE FUNCTION pg_temp.m059_associate(ws TEXT, ua TEXT, oa TEXT, ops TEXT[]) RETURNS VOID
LANGUAGE plpgsql AS $$
BEGIN
    INSERT INTO ngac_associations (id, ua_id, oa_id, operations)
    VALUES (gen_random_uuid()::text, ua, oa, ops)
    ON CONFLICT (ua_id, oa_id) DO NOTHING;
    IF FOUND THEN
        INSERT INTO _reconcile_059_edges (workspace_id, kind, child_or_ua, parent_or_oa, operations)
        VALUES (ws, 'association', ua, oa, ops) ON CONFLICT DO NOTHING;
    END IF;
END $$;

DO $$
DECLARE
    w          RECORD;
    k          RECORD;
    pc_id      TEXT;
    pc_props   JSONB;
    target     TEXT;
    -- ids[1]=Members, ids[2]=Owners, ids[3]=Channels; made[] marks the ones created here.
    ids        TEXT[];
    made       BOOLEAN[];
    n_found    INTEGER;
    cand_id    TEXT;
    tenant_id  TEXT;
BEGIN
    FOR w IN SELECT id, ngac_pc_id FROM workspaces WHERE ngac_pc_id IS NOT NULL ORDER BY id LOOP
        pc_id := w.ngac_pc_id;
        ids  := ARRAY[NULL, NULL, NULL]::TEXT[];
        made := ARRAY[FALSE, FALSE, FALSE];

        -- 1. The policy class.
        SELECT properties INTO pc_props FROM ngac_nodes WHERE id = pc_id AND node_type = 'PC';
        IF NOT FOUND THEN
            RAISE EXCEPTION 'migration 059: workspace % points at ngac_pc_id % which is not a PC node', w.id, pc_id;
        END IF;
        IF COALESCE(pc_props->>'workspace_id', '') NOT IN ('', w.id) THEN
            RAISE EXCEPTION 'migration 059: workspace %: its PC % claims workspace_id %; resolve by hand', w.id, pc_id, pc_props->>'workspace_id';
        END IF;
        IF (SELECT name FROM ngac_nodes WHERE id = pc_id) <> 'PC_' || w.id
           OR COALESCE(pc_props->>'workspace_id', '') = '' THEN
            IF EXISTS (SELECT 1 FROM ngac_nodes WHERE name = 'PC_' || w.id AND node_type = 'PC' AND id <> pc_id) THEN
                RAISE EXCEPTION 'migration 059: workspace %: another PC node is already named PC_%; resolve by hand', w.id, w.id;
            END IF;
            INSERT INTO _reconcile_059_nodes (workspace_id, node_id, action, node_type, old_name, new_name, old_properties)
            SELECT w.id, id, 'repaired', node_type, name, 'PC_' || w.id, properties FROM ngac_nodes WHERE id = pc_id
            ON CONFLICT DO NOTHING;
            INSERT INTO ngac_node_renames (node_id, old_name, new_name, props_patch, reason)
            SELECT id, name, 'PC_' || w.id,
                   jsonb_build_object('workspace_id', w.id, 'tenant_id', w.id, 'scope', 'tenant'),
                   'workspace policy class'
            FROM ngac_nodes WHERE id = pc_id AND name <> 'PC_' || w.id
            ON CONFLICT DO NOTHING;
            UPDATE ngac_nodes
            SET name = 'PC_' || w.id,
                properties = jsonb_build_object('workspace_id', w.id, 'tenant_id', w.id, 'scope', 'tenant')
                             || COALESCE(properties, '{}'::jsonb)
            WHERE id = pc_id;
        END IF;

        -- 2. Members, Owners (UA) and Channels (OA).
        FOR k IN SELECT * FROM (VALUES
                    (1, 'Members',  'UA', '(^|[_ -])members$'),
                    (2, 'Owners',   'UA', '(^|[_ -])owners$'),
                    (3, 'Channels', 'OA', '(^|[_ -])channels$')
                 ) AS t(ord, suffix, ntype, legacy) ORDER BY ord LOOP
            target := w.id || '_' || k.suffix;

            -- The target name may be held by at most one node, and it must be of the right type.
            SELECT count(*), min(id) INTO n_found, cand_id FROM ngac_nodes WHERE name = target;
            IF n_found > 1 THEN
                RAISE EXCEPTION 'migration 059: workspace %: % nodes are already named %; resolve by hand', w.id, n_found, target;
            END IF;
            IF n_found = 1 THEN
                IF (SELECT node_type FROM ngac_nodes WHERE id = cand_id) <> k.ntype THEN
                    RAISE EXCEPTION 'migration 059: workspace %: node % named % is not a %; resolve by hand', w.id, cand_id, target, k.ntype;
                END IF;
                ids[k.ord] := cand_id;
                CONTINUE;
            END IF;

            -- Direct children of the PC (Owners: or of Members) that look like the role.
            SELECT count(DISTINCT n.id), min(n.id) INTO n_found, cand_id
            FROM ngac_nodes n
            JOIN ngac_assignments a ON a.child_id = n.id
            WHERE n.node_type = k.ntype
              AND n.name ~* k.legacy
              AND n.name !~ '^(Ch_|TenantMember_|TenantOwner_|Role_|Dept_|Cal_|CalEv_|CalEvGuests_)'
              AND (a.parent_id = pc_id OR (k.ord = 2 AND a.parent_id = ids[1]));
            IF n_found > 1 THEN
                RAISE EXCEPTION 'migration 059: workspace %: % nodes under its PC could be its % %; rename the right one to % by hand',
                    w.id, n_found, k.ntype, k.suffix, target;
            END IF;

            IF n_found = 1 AND k.ntype = 'UA' AND EXISTS (
                -- A UA that is associated with anything but this workspace's Channels or Documents
                -- (Owners: also Mgmt) is some other group that happens to be called "...Members":
                -- not renamed. A Members candidate holding Mgmt would hand every member admin rights.
                SELECT 1 FROM ngac_associations s JOIN ngac_nodes o ON o.id = s.oa_id
                WHERE s.ua_id = cand_id
                  AND NOT (o.name IN (w.id || '_Channels', w.id || '_Documents')
                           OR (k.ord = 2 AND o.name = w.id || '_Mgmt')
                           OR (o.name ~* CASE WHEN k.ord = 2 THEN '(^|[_ -])(channels|documents|mgmt)$'
                                              ELSE '(^|[_ -])(channels|documents)$' END
                               AND EXISTS (SELECT 1 FROM ngac_assignments pa WHERE pa.child_id = o.id AND pa.parent_id = pc_id)))
            ) THEN
                RAISE EXCEPTION 'migration 059: workspace %: candidate % (%) is associated with an OA other than this workspace''s Channels/Documents/Mgmt; resolve by hand',
                    w.id, cand_id, (SELECT name FROM ngac_nodes WHERE id = cand_id);
            END IF;

            IF n_found = 1 THEN
                INSERT INTO _reconcile_059_nodes (workspace_id, node_id, action, node_type, old_name, new_name, old_properties)
                SELECT w.id, id, 'repaired', node_type, name, target, properties FROM ngac_nodes WHERE id = cand_id
                ON CONFLICT DO NOTHING;
                INSERT INTO ngac_node_renames (node_id, old_name, new_name, props_patch, reason)
                SELECT id, name, target, '{}'::jsonb, 'workspace ' || lower(k.suffix) FROM ngac_nodes WHERE id = cand_id
                ON CONFLICT DO NOTHING;
                UPDATE ngac_nodes SET name = target WHERE id = cand_id;
                ids[k.ord] := cand_id;
            ELSE
                cand_id := gen_random_uuid()::text;
                INSERT INTO ngac_nodes (id, name, node_type, properties) VALUES (cand_id, target, k.ntype, '{}'::jsonb);
                INSERT INTO _reconcile_059_nodes (workspace_id, node_id, action, node_type, old_name, new_name)
                VALUES (w.id, cand_id, 'created', k.ntype, NULL, target);
                ids[k.ord] := cand_id;
                made[k.ord] := TRUE;
            END IF;
        END LOOP;

        -- 3. The edges CreateWorkspace makes between them.
        PERFORM pg_temp.m059_assign(w.id, ids[1], pc_id);
        PERFORM pg_temp.m059_assign(w.id, ids[2], pc_id);
        PERFORM pg_temp.m059_assign(w.id, ids[3], pc_id);
        PERFORM pg_temp.m059_assign(w.id, ids[2], ids[1]);

        -- Owner ops = ngac.AllOwnerOps(), member channel ops = ngac.MemberChannelOps().
        IF made[2] OR made[3] THEN
            PERFORM pg_temp.m059_associate(w.id, ids[2], ids[3],
                ARRAY['read', 'write', 'approve', 'upload', 'share', 'manage', 'invite', 'create_channel']);
        END IF;
        IF made[1] OR made[3] THEN
            PERFORM pg_temp.m059_associate(w.id, ids[1], ids[3], ARRAY['read', 'write', 'create_channel']);
        END IF;

        -- 4. TenantMember / TenantOwner hang under Members / Owners.
        SELECT id INTO tenant_id FROM ngac_nodes WHERE name = 'TenantMember_' || w.id AND node_type = 'UA' LIMIT 1;
        IF FOUND THEN PERFORM pg_temp.m059_assign(w.id, tenant_id, ids[1]); END IF;
        SELECT id INTO tenant_id FROM ngac_nodes WHERE name = 'TenantOwner_' || w.id AND node_type = 'UA' LIMIT 1;
        IF FOUND THEN PERFORM pg_temp.m059_assign(w.id, tenant_id, ids[2]); END IF;
    END LOOP;
END $$;

-- A workspace's Members/Owners/Channels name must belong to one node: membership is looked up by it.
-- Existing duplicates were refused above for workspaces; this covers every remaining name.
CREATE UNIQUE INDEX IF NOT EXISTS ngac_nodes_workspace_role_name_uniq
    ON ngac_nodes (name)
    WHERE name ~ '_(Members|Owners|Channels)$';

-- Every workspace must now have all four nodes, or the migration does not commit.
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
        RAISE EXCEPTION 'migration 059: workspace(s) still lack PC/Members/Owners/Channels nodes: %', missing;
    END IF;
END $$;

COMMIT;
