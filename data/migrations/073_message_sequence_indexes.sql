-- 073_message_sequence_indexes.sql
--
-- Indexes for the numbers 072 added (docs/specs/chat-unread-counts):
--   messages_channel_seq       a page of a channel's history, newest first, deleted or not; and the
--                              guarantee that a number is given once in a channel
--   messages_deleted_seq       the deleted main messages after a read position, subtracted from an
--                              unread count (few rows: deleting is rare)
--   messages_thread_reply_seq  a page of a thread, and its unread replies; and the guarantee that a
--                              reply number is given once in a thread
--
-- Built CONCURRENTLY so writes to `messages` are not blocked while an index builds. Idempotent (IF NOT
-- EXISTS), as the migration runner requires of a file that runs outside a transaction. A build that is
-- interrupted leaves an INVALID index of the same name, which IF NOT EXISTS would then skip: such an
-- index is dropped first, and the file ends by refusing (so the runner does not record it as applied)
-- unless all three are valid.
DO $$
DECLARE name text;
BEGIN
  FOR name IN SELECT c.relname FROM pg_index i JOIN pg_class c ON c.oid = i.indexrelid
               WHERE NOT i.indisvalid
                 AND c.relname IN ('messages_channel_seq', 'messages_deleted_seq', 'messages_thread_reply_seq')
  LOOP
    EXECUTE format('DROP INDEX %I', name);
  END LOOP;
END
$$;

CREATE UNIQUE INDEX CONCURRENTLY IF NOT EXISTS messages_channel_seq
    ON messages (channel_id, seq DESC)
    WHERE parent_message_id IS NULL;

CREATE INDEX CONCURRENTLY IF NOT EXISTS messages_deleted_seq
    ON messages (channel_id, seq)
    WHERE parent_message_id IS NULL AND deleted_at IS NOT NULL;

CREATE UNIQUE INDEX CONCURRENTLY IF NOT EXISTS messages_thread_reply_seq
    ON messages (parent_message_id, reply_seq)
    WHERE parent_message_id IS NOT NULL;

DO $$
BEGIN
  IF (SELECT count(*) FROM pg_index i JOIN pg_class c ON c.oid = i.indexrelid
       WHERE i.indisvalid
         AND c.relname IN ('messages_channel_seq', 'messages_deleted_seq', 'messages_thread_reply_seq')) <> 3 THEN
    RAISE EXCEPTION 'migration 073: a message sequence index is missing or invalid';
  END IF;
END
$$;
