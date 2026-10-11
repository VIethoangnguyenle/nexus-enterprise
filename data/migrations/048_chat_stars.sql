-- A person's own stars, as in Google Chat: they mark a whole conversation or a
-- single message to find it again from the navigator. A star is private to the
-- person who set it and never widens what they may read: whether they can still
-- read the conversation is decided by the graph each time the stars are listed.
CREATE TABLE IF NOT EXISTS starred_conversations (
    user_id    TEXT        NOT NULL REFERENCES users(id)    ON DELETE CASCADE,
    channel_id TEXT        NOT NULL REFERENCES channels(id) ON DELETE CASCADE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (user_id, channel_id)
);

CREATE TABLE IF NOT EXISTS starred_messages (
    user_id    TEXT        NOT NULL REFERENCES users(id)    ON DELETE CASCADE,
    message_id TEXT        NOT NULL REFERENCES messages(id) ON DELETE CASCADE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (user_id, message_id)
);

CREATE INDEX IF NOT EXISTS idx_starred_messages_user_created
    ON starred_messages (user_id, created_at DESC);

-- Deleting a channel or a message cascades into these rows: index the far side
-- too, or each deleted message scans every star.
CREATE INDEX IF NOT EXISTS idx_starred_messages_message ON starred_messages (message_id);
CREATE INDEX IF NOT EXISTS idx_starred_conversations_channel ON starred_conversations (channel_id);
