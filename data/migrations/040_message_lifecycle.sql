-- A message's life after it is sent: edited, deleted, quoted.
--
-- edited_at is when its sender last changed the text. A deleted message keeps
-- its row (soft delete): its body is never returned again, a root message
-- with replies stays as a "deleted" placeholder so its thread survives, and
-- deleted_by records who removed it — the sender, or someone who manages the
-- channel — which is the channel's record of the deletion. quoted_message_id is
-- the message a reply quotes; a quote of a message that is later removed for
-- good simply loses its target.
ALTER TABLE messages ADD COLUMN IF NOT EXISTS edited_at TIMESTAMPTZ;
ALTER TABLE messages ADD COLUMN IF NOT EXISTS deleted_at TIMESTAMPTZ;
ALTER TABLE messages ADD COLUMN IF NOT EXISTS deleted_by TEXT REFERENCES users(id) ON DELETE SET NULL;
ALTER TABLE messages ADD COLUMN IF NOT EXISTS quoted_message_id TEXT REFERENCES messages(id) ON DELETE SET NULL;

-- Removing a message (or a whole workspace) looks up the messages quoting it.
CREATE INDEX IF NOT EXISTS idx_messages_quoted ON messages (quoted_message_id) WHERE quoted_message_id IS NOT NULL;
