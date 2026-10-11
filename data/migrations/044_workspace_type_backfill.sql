-- Every workspace's type, decided by how it came to be.
--
-- Until now only a Google company-domain claim set 'organization'; every other
-- path left the column at its default, so 'personal' said nothing. Creation now
-- states the type: the workspace made for a person on first sign-in is
-- 'personal'; one created on purpose, or owned by a company domain, is
-- 'organization'.
--
-- Existing rows: a workspace named "<someone>'s Workspace" (the name sign-in
-- gives the one it makes) that has exactly one active member and claims no
-- domain is personal; everything else is an organization.
--
-- The backfill runs once: `make db-migrate` re-applies every migration in dev,
-- and a second run must not undo a workspace an owner later turned into an
-- organization. The column comment marks it done.
DO $$
DECLARE
    marker CONSTANT TEXT := 'personal | organization; decided at creation (backfilled by 044)';
    n_personal INT;
    n_org INT;
BEGIN
    IF col_description('workspaces'::regclass,
                       (SELECT attnum FROM pg_attribute
                         WHERE attrelid = 'workspaces'::regclass AND attname = 'type')) = marker THEN
        RETURN;
    END IF;

    UPDATE workspaces w
       SET type = CASE
           WHEN w.domain IS NULL
            AND w.name LIKE '%''s Workspace'
            AND (SELECT count(*) FROM tenant_users tu
                  WHERE tu.tenant_id = w.id AND tu.status = 'active') = 1
           THEN 'personal'
           ELSE 'organization'
       END;

    EXECUTE format('COMMENT ON COLUMN workspaces.type IS %L', marker);

    SELECT count(*) FILTER (WHERE type = 'personal'), count(*) FILTER (WHERE type = 'organization')
      INTO n_personal, n_org FROM workspaces;
    RAISE NOTICE 'workspace types: % personal, % organization', n_personal, n_org;
END $$;

-- No default: every insert states the type. A writer that forgets it fails
-- instead of quietly making a personal workspace (an old workspace service
-- during a deploy, a new creation path, a test fixture).
ALTER TABLE workspaces ALTER COLUMN type DROP DEFAULT;
