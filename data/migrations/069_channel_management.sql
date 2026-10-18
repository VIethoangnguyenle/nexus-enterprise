-- 069_channel_management.sql
--
-- Group management settings: three per-channel columns that control who may post, add people
-- and mention everyone (docs/specs/chat-group-management). Each is TEXT NOT NULL DEFAULT 'everyone'
-- with a CHECK that limits values to ('everyone', 'managers'). Direct messages and the
-- workspace-wide channel keep the default and the messaging service ignores them.
--
-- Columns on `channels` rather than a separate table: the settings belong to the channel and
-- are read together with it on every message, so a join is noise.
--
-- Re-runnable: every statement uses ADD COLUMN IF NOT EXISTS, so `make db-migrate`, which
-- re-applies every file, is harmless. The second run of each ADD COLUMN sees the column
-- already present and does nothing.
--
-- Reverse (nothing reads these before the release that ships this file):
--   ALTER TABLE channels DROP COLUMN IF EXISTS post_policy;
--   ALTER TABLE channels DROP COLUMN IF EXISTS invite_policy;
--   ALTER TABLE channels DROP COLUMN IF EXISTS mention_all_policy;

BEGIN;

ALTER TABLE channels
    ADD COLUMN IF NOT EXISTS post_policy        TEXT NOT NULL DEFAULT 'everyone'
        CHECK (post_policy        IN ('everyone', 'managers'));

ALTER TABLE channels
    ADD COLUMN IF NOT EXISTS invite_policy      TEXT NOT NULL DEFAULT 'everyone'
        CHECK (invite_policy      IN ('everyone', 'managers'));

ALTER TABLE channels
    ADD COLUMN IF NOT EXISTS mention_all_policy TEXT NOT NULL DEFAULT 'everyone'
        CHECK (mention_all_policy IN ('everyone', 'managers'));

COMMIT;
