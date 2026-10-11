-- In-line threads: when a thread last moved, who follows it, and how far each
-- follower has read it.
--
-- A thread is a root message plus the messages whose parent_message_id points
-- at it. messages.last_reply_at orders the threads of a channel by activity and
-- is written in the same transaction as reply_count.
--
-- thread_follows holds one row per (user, thread) once the user has a state
-- for it: following, or explicitly not. A user with no row follows a thread
-- they started or were mentioned in, and nothing else. last_read_at is the
-- follower's read position: replies after it, by someone else, are unread.

-- The backfills run once, when the column is first added: `make db-migrate`
-- re-applies every migration in dev, and a second run must not reset anyone's
-- read position.
DO $$
BEGIN
    IF EXISTS (SELECT 1 FROM information_schema.columns
               WHERE table_schema = current_schema() AND table_name = 'messages'
                 AND column_name = 'last_reply_at') THEN
        RETURN;
    END IF;

    ALTER TABLE messages ADD COLUMN last_reply_at TIMESTAMPTZ;

    UPDATE messages root
    SET last_reply_at = r.last_at
    FROM (
        SELECT parent_message_id, MAX(created_at) AS last_at
        FROM messages
        WHERE parent_message_id IS NOT NULL
        GROUP BY parent_message_id
    ) r
    WHERE root.id = r.parent_message_id;

    CREATE TABLE IF NOT EXISTS thread_follows (
        user_id         TEXT        NOT NULL REFERENCES users(id) ON DELETE CASCADE,
        root_message_id TEXT        NOT NULL REFERENCES messages(id) ON DELETE CASCADE,
        following       BOOLEAN     NOT NULL DEFAULT TRUE,
        last_read_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
        updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
        PRIMARY KEY (user_id, root_message_id)
    );

    -- Existing threads: their starter and everyone who replied follow them, with
    -- the history already read so the upgrade raises no wave of old unreads.
    INSERT INTO thread_follows (user_id, root_message_id, following, last_read_at)
    SELECT m.sender_id, m.id, TRUE, NOW()
    FROM messages m
    WHERE m.parent_message_id IS NULL AND m.reply_count > 0
    ON CONFLICT DO NOTHING;

    INSERT INTO thread_follows (user_id, root_message_id, following, last_read_at)
    SELECT tp.user_id, tp.message_id, TRUE, NOW()
    FROM thread_participants tp
    ON CONFLICT DO NOTHING;
END $$;

-- Unread replies of one thread, and the newest repliers, are read by thread and time.
CREATE INDEX IF NOT EXISTS idx_messages_thread_time
    ON messages (parent_message_id, created_at)
    WHERE parent_message_id IS NOT NULL;

CREATE INDEX IF NOT EXISTS idx_thread_follows_user_following
    ON thread_follows (user_id, following);

-- idx_messages_thread (parent_message_id) is a prefix of the index above.
DROP INDEX IF EXISTS idx_messages_thread;

-- Unread counts look up the threads a user follows without a follow row: the
-- ones they started and the ones that mention them. Only roots with replies
-- are threads.
CREATE INDEX IF NOT EXISTS idx_messages_thread_starter
    ON messages (sender_id)
    WHERE parent_message_id IS NULL AND reply_count > 0;

CREATE INDEX IF NOT EXISTS idx_messages_thread_mentions
    ON messages USING gin (mentions)
    WHERE parent_message_id IS NULL AND reply_count > 0;
