-- Migration 036: structured, workspace-scoped notifications.
--
-- A notification used to carry pre-built English text and no workspace or actor,
-- so one person's list mixed every workspace and the screen could not say who did
-- what to which thing. It now records the facts and the screen words them:
--
--   workspace_id    the workspace (tenant) the notification belongs to. Every
--                   list, count, mark and live push is scoped to it.
--   actor_user_id   who did it, when a person did; NULL for none or a removed account.
--   actor_name      the actor's name in that workspace at the time, kept so a
--                   person who later left is still named rather than shown as an id.
--   target_type/id  what it is about (an approval request, an asset, ...); these
--                   are the former entity_type/entity_id.
--   target_name     the subject's name at the time. Empty means "unknown", never an id.
--   params          small per-type facts as JSON, e.g. {"reason": "..."}.
--
-- title and body are no longer written (the screen builds its own sentence); they
-- keep a default so old rows stay valid.
--
-- Rows from before this migration have no workspace. They cannot be placed in a
-- workspace and carry only English text, so no list, count or push ever matches
-- them (workspace_id IS NULL). They are kept, not deleted.
--
-- Idempotent: `make db-migrate` replays every file.

DO $$
BEGIN
    IF EXISTS (SELECT 1 FROM information_schema.columns
               WHERE table_schema = current_schema() AND table_name = 'notifications' AND column_name = 'entity_type') THEN
        ALTER TABLE notifications RENAME COLUMN entity_type TO target_type;
    END IF;
    IF EXISTS (SELECT 1 FROM information_schema.columns
               WHERE table_schema = current_schema() AND table_name = 'notifications' AND column_name = 'entity_id') THEN
        ALTER TABLE notifications RENAME COLUMN entity_id TO target_id;
    END IF;
END $$;

ALTER TABLE notifications ADD COLUMN IF NOT EXISTS workspace_id  TEXT REFERENCES workspaces(id) ON DELETE CASCADE;
ALTER TABLE notifications ADD COLUMN IF NOT EXISTS actor_user_id TEXT REFERENCES users(id) ON DELETE SET NULL;
ALTER TABLE notifications ADD COLUMN IF NOT EXISTS actor_name    TEXT NOT NULL DEFAULT '';
ALTER TABLE notifications ADD COLUMN IF NOT EXISTS target_name   TEXT NOT NULL DEFAULT '';
ALTER TABLE notifications ADD COLUMN IF NOT EXISTS params        JSONB NOT NULL DEFAULT '{}';
ALTER TABLE notifications ALTER COLUMN title SET DEFAULT '';

CREATE INDEX IF NOT EXISTS idx_notifications_user_ws
    ON notifications(user_id, workspace_id, created_at DESC, id DESC);
CREATE INDEX IF NOT EXISTS idx_notifications_user_ws_unread
    ON notifications(user_id, workspace_id) WHERE read = FALSE;
