-- ============================================
-- Lark Chat Parity — Message Reactions
-- ============================================

CREATE TABLE IF NOT EXISTS message_reactions (
    id          TEXT PRIMARY KEY DEFAULT gen_random_uuid()::text,
    message_id  TEXT NOT NULL REFERENCES messages(id) ON DELETE CASCADE,
    user_id     TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    emoji       TEXT NOT NULL,
    created_at  TIMESTAMPTZ DEFAULT NOW(),
    UNIQUE(message_id, user_id, emoji)
);

CREATE INDEX IF NOT EXISTS idx_reactions_message ON message_reactions(message_id);
CREATE INDEX IF NOT EXISTS idx_reactions_user ON message_reactions(user_id);

-- ============================================
-- Lark Chat Parity — Pinned Messages
-- ============================================

CREATE TABLE IF NOT EXISTS message_pins (
    id          TEXT PRIMARY KEY DEFAULT gen_random_uuid()::text,
    channel_id  TEXT NOT NULL REFERENCES channels(id) ON DELETE CASCADE,
    message_id  TEXT NOT NULL REFERENCES messages(id) ON DELETE CASCADE,
    pinned_by   TEXT NOT NULL REFERENCES users(id),
    created_at  TIMESTAMPTZ DEFAULT NOW(),
    UNIQUE(channel_id, message_id)
);

CREATE INDEX IF NOT EXISTS idx_pins_channel ON message_pins(channel_id, created_at DESC);

-- ============================================
-- Lark Chat Parity — Read Receipts
-- ============================================

CREATE TABLE IF NOT EXISTS read_receipts (
    user_id         TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    channel_id      TEXT NOT NULL REFERENCES channels(id) ON DELETE CASCADE,
    last_read_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    last_message_id TEXT REFERENCES messages(id) ON DELETE SET NULL,
    PRIMARY KEY (user_id, channel_id)
);
