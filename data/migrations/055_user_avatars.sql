-- A profile picture, kept by the auth service in three sizes so a list of
-- 24px avatars never downloads the picture a detail view would show.
-- The token is random per upload and is what the public URL names
-- (/api/auth/avatars/<token>): it says nothing about the picture, so nobody
-- can work out the URL from a photo of the person they already have.
-- users.avatar_url points here; both are written in one transaction.
-- Only adds a table, so the release before this one keeps working.
CREATE TABLE IF NOT EXISTS user_avatars (
    user_id    TEXT PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
    token      TEXT NOT NULL UNIQUE,
    small      BYTEA NOT NULL,   -- 64×64 JPEG
    medium     BYTEA NOT NULL,   -- 192×192 JPEG
    large      BYTEA NOT NULL,   -- 1024×1024 JPEG
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
