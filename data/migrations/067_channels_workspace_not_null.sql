-- 067_channels_workspace_not_null.sql
--
-- Every channel belongs to exactly one workspace. Since the release that ships 066 the code
-- writes `channels.workspace_id` for every channel, direct messages included, and 066 gave
-- the DMs that predate it their workspace. This file makes the schema say so: a channel
-- without a workspace can no longer be stored, so no code path has to ask what such a row
-- would mean. (A DM with no workspace used to be treated as "belongs to none" and appear
-- nowhere.)
--
-- The foreign key `channels.workspace_id REFERENCES workspaces(id)` (no ON DELETE action,
-- i.e. restrict) is unchanged: a workspace is torn down by deleting its channels first
-- (workspace service, teardown), so a channel can never outlive its workspace.
--
-- Statements, in order (one transaction):
--   1. guard: aborts, changing nothing, if any channel still has a NULL workspace_id,
--      and says how many there are per channel_type
--   2. ALTER TABLE channels ALTER COLUMN workspace_id SET NOT NULL
--   3. a NOTICE
--
-- Re-runnable: SET NOT NULL on a column that is already NOT NULL is a no-op, so
-- `make db-migrate`, which re-applies every file, is harmless.
--
-- Reverse:
--   ALTER TABLE channels ALTER COLUMN workspace_id DROP NOT NULL;
--
-- PRE-FLIGHT (read-only):
--   SELECT channel_type, count(*) FROM channels WHERE workspace_id IS NULL GROUP BY 1;
-- Production before this file: 21 channels (4 private, 4 dm, 13 workspace), none NULL.

BEGIN;

DO $$
DECLARE
    n_null  INTEGER;
    by_type TEXT;
BEGIN
    SELECT count(*) INTO n_null FROM channels WHERE workspace_id IS NULL;
    IF n_null > 0 THEN
        SELECT string_agg(channel_type || '=' || n, ', ' ORDER BY channel_type) INTO by_type
        FROM (SELECT channel_type, count(*) AS n FROM channels WHERE workspace_id IS NULL GROUP BY channel_type) x;
        RAISE EXCEPTION 'migration 067: % channel(s) have no workspace (%); give each one a workspace (see 066) or remove it, then re-run', n_null, by_type;
    END IF;
END $$;

ALTER TABLE channels ALTER COLUMN workspace_id SET NOT NULL;

DO $$
BEGIN
    RAISE NOTICE 'migration 067: channels.workspace_id is NOT NULL (% channels)', (SELECT count(*) FROM channels);
END $$;

COMMIT;
