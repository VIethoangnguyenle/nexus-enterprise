-- When an edit added a person to a message's mentions. The mention list on the
-- message says who is mentioned; this says, for the people an edit added, from
-- when — so the mention is unread for them even though the message itself is
-- older than their read position. A person mentioned when the message was sent
-- has no row here: their mention dates from the message. Removing the mention
-- by a later edit deletes the row; deleting the message or the user cascades.
CREATE TABLE IF NOT EXISTS message_mention_times (
    message_id   TEXT        NOT NULL REFERENCES messages(id) ON DELETE CASCADE,
    user_id      TEXT        NOT NULL REFERENCES users(id)    ON DELETE CASCADE,
    mentioned_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (message_id, user_id)
);

-- The primary key serves the message side; a deleted user cascades by user.
CREATE INDEX IF NOT EXISTS idx_message_mention_times_user ON message_mention_times (user_id);
