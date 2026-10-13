-- A conversation list previews the newest live message of each conversation,
-- and the newest live thread reply. The two lookups take one row per channel
-- (the newest, by an ordered index probe), so the list costs one probe per
-- conversation however long their histories are. The main stream and the
-- replies are different partial indexes because a query asks for one or the
-- other.
CREATE INDEX IF NOT EXISTS idx_messages_latest_main
    ON messages (channel_id, created_at DESC, id DESC)
    WHERE parent_message_id IS NULL AND deleted_at IS NULL;

CREATE INDEX IF NOT EXISTS idx_messages_latest_reply
    ON messages (channel_id, created_at DESC, id DESC)
    WHERE parent_message_id IS NOT NULL AND deleted_at IS NULL;
