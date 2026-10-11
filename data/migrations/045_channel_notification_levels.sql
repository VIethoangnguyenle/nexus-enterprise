-- How much of a conversation counts for one person, as in Google Chat:
--   all  — every message and every reply counts toward the badge and notifies;
--   main — the main messages and the replies in threads they follow (default);
--   off  — nothing counts and nothing notifies.
-- A person with no row has the default. The row is the person's own; whether
-- they may read the conversation is still decided by the graph at every use.
CREATE TABLE IF NOT EXISTS channel_notification_levels (
    user_id    TEXT        NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    channel_id TEXT        NOT NULL REFERENCES channels(id) ON DELETE CASCADE,
    level      TEXT        NOT NULL CHECK (level IN ('all', 'main', 'off')),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (user_id, channel_id)
);
