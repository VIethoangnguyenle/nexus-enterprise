-- ============================================
-- Lark Chat Parity — Polls
-- ============================================

CREATE TABLE IF NOT EXISTS polls (
    id          TEXT PRIMARY KEY DEFAULT gen_random_uuid()::text,
    message_id  TEXT NOT NULL REFERENCES messages(id) ON DELETE CASCADE,
    channel_id  TEXT NOT NULL REFERENCES channels(id) ON DELETE CASCADE,
    question    TEXT NOT NULL,
    is_multi    BOOLEAN DEFAULT FALSE,
    is_anonymous BOOLEAN DEFAULT FALSE,
    created_by  TEXT NOT NULL REFERENCES users(id),
    ends_at     TIMESTAMPTZ,
    created_at  TIMESTAMPTZ DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_polls_message ON polls(message_id);
CREATE INDEX IF NOT EXISTS idx_polls_channel ON polls(channel_id);

CREATE TABLE IF NOT EXISTS poll_options (
    id          TEXT PRIMARY KEY DEFAULT gen_random_uuid()::text,
    poll_id     TEXT NOT NULL REFERENCES polls(id) ON DELETE CASCADE,
    text        TEXT NOT NULL,
    position    INT NOT NULL DEFAULT 0
);

CREATE INDEX IF NOT EXISTS idx_poll_options_poll ON poll_options(poll_id);

CREATE TABLE IF NOT EXISTS poll_votes (
    id          TEXT PRIMARY KEY DEFAULT gen_random_uuid()::text,
    poll_id     TEXT NOT NULL REFERENCES polls(id) ON DELETE CASCADE,
    option_id   TEXT NOT NULL REFERENCES poll_options(id) ON DELETE CASCADE,
    user_id     TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    created_at  TIMESTAMPTZ DEFAULT NOW(),
    UNIQUE(poll_id, option_id, user_id)
);

CREATE INDEX IF NOT EXISTS idx_poll_votes_poll ON poll_votes(poll_id);
CREATE INDEX IF NOT EXISTS idx_poll_votes_option ON poll_votes(option_id);
