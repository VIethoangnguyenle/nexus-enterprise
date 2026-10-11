-- Personal sections of the chat navigator, as in Google Chat.
--
-- A section belongs to one user in one workspace and only arranges that user's
-- own list: it grants nothing and nobody else sees it, so it is not an NGAC
-- node. A conversation sits in at most one of a user's sections; a channel
-- that is deleted leaves every section it was in.
CREATE TABLE IF NOT EXISTS chat_sections (
    id           TEXT        PRIMARY KEY,
    workspace_id TEXT        NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    user_id      TEXT        NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    name         TEXT        NOT NULL CHECK (char_length(name) BETWEEN 1 AND 40),
    position     INT         NOT NULL,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Names are unique per user and workspace, whatever their case.
CREATE UNIQUE INDEX IF NOT EXISTS uq_chat_sections_name
    ON chat_sections (workspace_id, user_id, lower(name));
CREATE INDEX IF NOT EXISTS idx_chat_sections_owner
    ON chat_sections (workspace_id, user_id, position);

CREATE TABLE IF NOT EXISTS chat_section_items (
    section_id TEXT NOT NULL REFERENCES chat_sections(id) ON DELETE CASCADE,
    channel_id TEXT NOT NULL REFERENCES channels(id) ON DELETE CASCADE,
    user_id    TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    -- A conversation is in at most one of a user's sections. A DM has no
    -- workspace, so this holds across the user's workspaces too.
    UNIQUE (user_id, channel_id)
);

CREATE INDEX IF NOT EXISTS idx_chat_section_items_section ON chat_section_items (section_id);
