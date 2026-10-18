-- 068_user_settings.sql
--
-- Each person's options, stored once, by the service that enforces them (docs/specs/user-settings):
--   workspace  user_settings            keys with no other home; today the phone's bottom bar (`mobile.bar`)
--   messaging  push_preferences         + pause_until (Tạm dừng thông báo)
--              chat_user_settings       Enter to send, link previews, Chat's opening view and sort, double-tap
--                                       reaction, default notification level for new groups
--   calendar   calendar_user_settings   + default view, week start, visible calendars, default duration,
--                                       notification switches
--   drive      drive_user_settings      view, sort, folders first, previews on mobile data, "shared with you" switch
--   asset      asset_user_settings      opening tab, sort, notification switches
--   auth       users.language           account-scope language
-- Approval's settings live in each tenant's own schema and are created by the approval service with the rest of
-- that schema (store.EnsureSchema), never in public.
--
-- Every column has a default that is today's behaviour, so a person who never opens a settings page sees no change.
-- Re-runnable: every statement is IF NOT EXISTS / ADD COLUMN IF NOT EXISTS, so `make db-migrate`, which re-applies
-- every file, is harmless.
--
-- Reverse (nothing reads these before the release that ships this file):
--   DROP TABLE IF EXISTS user_settings, chat_user_settings, drive_user_settings, asset_user_settings;
--   ALTER TABLE push_preferences DROP COLUMN IF EXISTS pause_until;
--   ALTER TABLE calendar_user_settings DROP COLUMN IF EXISTS default_view, DROP COLUMN IF EXISTS week_start,
--     DROP COLUMN IF EXISTS visible_calendars, DROP COLUMN IF EXISTS default_duration,
--     DROP COLUMN IF EXISTS notify_invites, DROP COLUMN IF EXISTS notify_changes, DROP COLUMN IF EXISTS notify_reminders;
--   ALTER TABLE users DROP COLUMN IF EXISTS language;

BEGIN;

-- Workspace service: options with no other owner. workspace_id NULL = account scope.
CREATE TABLE IF NOT EXISTS user_settings (
    user_id      TEXT        NOT NULL,
    workspace_id TEXT        REFERENCES workspaces(id) ON DELETE CASCADE,
    key          TEXT        NOT NULL,
    value        JSONB       NOT NULL,
    updated_at   TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE UNIQUE INDEX IF NOT EXISTS user_settings_account_key
    ON user_settings (user_id, key) WHERE workspace_id IS NULL;
CREATE UNIQUE INDEX IF NOT EXISTS user_settings_workspace_key
    ON user_settings (user_id, workspace_id, key) WHERE workspace_id IS NOT NULL;

-- Messaging: pausing every push until a moment (NULL = not paused).
ALTER TABLE push_preferences ADD COLUMN IF NOT EXISTS pause_until TIMESTAMPTZ;

-- Messaging: how Chat behaves for a person, per workspace.
CREATE TABLE IF NOT EXISTS chat_user_settings (
    user_id         TEXT        NOT NULL,
    workspace_id    TEXT        NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    enter_to_send   BOOLEAN     NOT NULL DEFAULT FALSE,
    link_preview    BOOLEAN     NOT NULL DEFAULT TRUE,
    open_at         TEXT        NOT NULL DEFAULT 'all'      CHECK (open_at IN ('all', 'sections')),
    sort            TEXT        NOT NULL DEFAULT 'activity' CHECK (sort IN ('activity', 'name')),
    double_tap      TEXT        NOT NULL DEFAULT '👍',       -- '' = off
    default_level   TEXT,                                    -- NULL = the system default for new groups
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (user_id, workspace_id)
);

-- Calendar: the rest of Cài đặt Lịch on its existing table.
ALTER TABLE calendar_user_settings
    ADD COLUMN IF NOT EXISTS default_view      TEXT     NOT NULL DEFAULT 'agenda',
    ADD COLUMN IF NOT EXISTS week_start        SMALLINT NOT NULL DEFAULT 1,      -- 1 = Monday, 0 = Sunday
    ADD COLUMN IF NOT EXISTS visible_calendars JSONB,                           -- NULL = every calendar
    ADD COLUMN IF NOT EXISTS default_duration  INTEGER  NOT NULL DEFAULT 30,     -- minutes
    ADD COLUMN IF NOT EXISTS notify_invites    BOOLEAN  NOT NULL DEFAULT TRUE,
    ADD COLUMN IF NOT EXISTS notify_changes    BOOLEAN  NOT NULL DEFAULT TRUE,
    ADD COLUMN IF NOT EXISTS notify_reminders  BOOLEAN  NOT NULL DEFAULT TRUE;

-- Drive: how Tài liệu behaves for a person, per workspace.
CREATE TABLE IF NOT EXISTS drive_user_settings (
    user_id           TEXT        NOT NULL,
    workspace_id      TEXT        NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    view              TEXT        NOT NULL DEFAULT 'auto' CHECK (view IN ('auto', 'grid', 'list')),
    sort              TEXT        NOT NULL DEFAULT 'modified' CHECK (sort IN ('modified', 'name', 'size')),
    folders_first     BOOLEAN     NOT NULL DEFAULT TRUE,
    cellular_previews BOOLEAN     NOT NULL DEFAULT TRUE,
    notify_shared     BOOLEAN     NOT NULL DEFAULT TRUE,
    updated_at        TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (user_id, workspace_id)
);

-- Asset: how Tài sản behaves for a person, per workspace.
CREATE TABLE IF NOT EXISTS asset_user_settings (
    user_id            TEXT        NOT NULL,
    workspace_id       TEXT        NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    default_tab        TEXT        NOT NULL DEFAULT 'overview',
    sort               TEXT        NOT NULL DEFAULT 'name',
    notify_requests    BOOLEAN     NOT NULL DEFAULT TRUE,
    notify_maintenance BOOLEAN     NOT NULL DEFAULT TRUE,
    updated_at         TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (user_id, workspace_id)
);

-- Auth: the language a person reads the app in (account scope). NULL = the app default.
ALTER TABLE users ADD COLUMN IF NOT EXISTS language TEXT;

COMMIT;
