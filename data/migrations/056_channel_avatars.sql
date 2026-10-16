-- A group's picture, kept by the messaging service in three sizes so a list of
-- 24px avatars never downloads the picture a detail view would show.
-- The token is random per upload and is what the public URL names
-- (/api/channels/avatars/<token>): it says nothing about the picture or the
-- group, so nobody can work out the URL of a group they are not in.
-- channels.avatar_url points here; both are written in one transaction.
-- Only adds a table and a defaulted column, so the release before this one
-- keeps working.
CREATE TABLE IF NOT EXISTS channel_avatars (
    channel_id TEXT PRIMARY KEY REFERENCES channels(id) ON DELETE CASCADE,
    token      TEXT NOT NULL UNIQUE,
    small      BYTEA NOT NULL,   -- 64×64 JPEG
    medium     BYTEA NOT NULL,   -- 192×192 JPEG
    large      BYTEA NOT NULL,   -- 1024×1024 JPEG
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

ALTER TABLE channels ADD COLUMN IF NOT EXISTS avatar_url TEXT NOT NULL DEFAULT '';
