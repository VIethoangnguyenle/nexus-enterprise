-- 072_message_sequences.sql
--
-- Unread counts are kept by numbers instead of by counting rows (docs/specs/chat-unread-counts):
--   messages.seq             1, 2, 3… for the main-stream messages of a channel; NULL on a reply
--   channels.last_seq        the last number given in the channel
--   messages.reply_seq       1, 2, 3… for the replies of one thread; NULL on a main message
--   messages.last_reply_seq  on a root: the last reply number given in its thread
--   read_receipts.last_read_seq          the channel's main stream is read up to this number
-- A channel's unread main messages are then last_seq − last_read_seq, less the deleted ones after
-- the read position. A thread's unread replies are still counted from its read time: one thread is
-- small, and reply numbers serve to page through it.
--
-- Two triggers keep the numbers right whatever code writes the rows, the release before this one
-- included (it keeps serving while this runs, and runs again after a rollback):
--   messages_number       BEFORE INSERT: takes the channel's (or the thread's) next number by updating
--                         the channel row (or the root row), in the transaction that stores the message;
--                         concurrent senders in one channel take turns, and a rolled-back insert rolls
--                         its number back, so there are no gaps. A message whose id is already stored
--                         is skipped before it takes a number (what ON CONFLICT DO NOTHING did). A
--                         sender who had read up to the message before theirs has read their own.
--   read_receipts_number  BEFORE INSERT OR UPDATE: a write that moves the read time but not the read
--                         number (code that knows only times) gets the number of the last main message
--                         at or before that time.
--
-- Backfill numbers what is not numbered yet, after what is (by created_at, then id), and sets the
-- read positions from the read times. Re-runnable: IF NOT EXISTS, CREATE OR REPLACE, and every
-- backfill touches only rows still unset, which `make db-migrate` (it re-applies every file) needs.
-- The ALTERs hold messages, channels and read_receipts exclusively until COMMIT (reads wait too):
-- lock_timeout makes the file give up, changing nothing, rather than queue every chat query behind a
-- long-running one; the next deploy tries again. The indexes are built CONCURRENTLY in 073.
--
-- Reverse (additive; the release before this one never reads the columns; 073's indexes go with them):
--   DROP TRIGGER IF EXISTS messages_number ON messages;
--   DROP FUNCTION IF EXISTS messages_number();
--   DROP TRIGGER IF EXISTS read_receipts_number ON read_receipts;
--   DROP FUNCTION IF EXISTS read_receipts_number();
--   ALTER TABLE messages DROP CONSTRAINT IF EXISTS messages_numbered;
--   ALTER TABLE messages DROP COLUMN IF EXISTS seq, DROP COLUMN IF EXISTS reply_seq, DROP COLUMN IF EXISTS last_reply_seq;
--   ALTER TABLE channels DROP COLUMN IF EXISTS last_seq;
--   ALTER TABLE read_receipts DROP COLUMN IF EXISTS last_read_seq;

BEGIN;

SET LOCAL lock_timeout = '5s';

ALTER TABLE channels       ADD COLUMN IF NOT EXISTS last_seq BIGINT NOT NULL DEFAULT 0;
ALTER TABLE messages       ADD COLUMN IF NOT EXISTS seq BIGINT;
ALTER TABLE messages       ADD COLUMN IF NOT EXISTS reply_seq BIGINT;
ALTER TABLE messages       ADD COLUMN IF NOT EXISTS last_reply_seq BIGINT NOT NULL DEFAULT 0;
ALTER TABLE read_receipts  ADD COLUMN IF NOT EXISTS last_read_seq BIGINT NOT NULL DEFAULT 0;

CREATE OR REPLACE FUNCTION messages_number() RETURNS trigger AS $$
BEGIN
  IF EXISTS (SELECT 1 FROM messages WHERE id = NEW.id) THEN
    RETURN NULL;
  END IF;
  IF NEW.parent_message_id IS NULL THEN
    UPDATE channels SET last_seq = last_seq + 1 WHERE id = NEW.channel_id
      RETURNING last_seq INTO NEW.seq;
    NEW.reply_seq := NULL;
    UPDATE read_receipts
       SET last_read_seq = NEW.seq, last_read_at = GREATEST(last_read_at, NEW.created_at)
     WHERE user_id = NEW.sender_id AND channel_id = NEW.channel_id AND last_read_seq = NEW.seq - 1;
  ELSE
    UPDATE messages SET last_reply_seq = last_reply_seq + 1 WHERE id = NEW.parent_message_id
      RETURNING last_reply_seq INTO NEW.reply_seq;
    NEW.seq := NULL;
  END IF;
  RETURN NEW;
END
$$ LANGUAGE plpgsql;

DROP TRIGGER IF EXISTS messages_number ON messages;
CREATE TRIGGER messages_number BEFORE INSERT ON messages
  FOR EACH ROW EXECUTE FUNCTION messages_number();

CREATE OR REPLACE FUNCTION read_receipts_number() RETURNS trigger AS $$
BEGIN
  IF (TG_OP = 'INSERT' AND NEW.last_read_seq = 0)
     OR (TG_OP = 'UPDATE' AND NEW.last_read_seq = OLD.last_read_seq
         AND NEW.last_read_at IS DISTINCT FROM OLD.last_read_at) THEN
    NEW.last_read_seq := COALESCE((
      SELECT m.seq FROM messages m
       WHERE m.channel_id = NEW.channel_id AND m.parent_message_id IS NULL
         AND m.created_at <= NEW.last_read_at
       ORDER BY m.created_at DESC, m.id DESC LIMIT 1), 0);
  END IF;
  RETURN NEW;
END
$$ LANGUAGE plpgsql;

DROP TRIGGER IF EXISTS read_receipts_number ON read_receipts;
CREATE TRIGGER read_receipts_number BEFORE INSERT OR UPDATE ON read_receipts
  FOR EACH ROW EXECUTE FUNCTION read_receipts_number();

-- Main messages not numbered yet, after the channel's numbered ones.
WITH base AS (SELECT channel_id, max(seq) AS top FROM messages WHERE seq IS NOT NULL GROUP BY channel_id),
     fresh AS (SELECT id, channel_id,
                      row_number() OVER (PARTITION BY channel_id ORDER BY created_at, id) AS rn
                 FROM messages
                WHERE parent_message_id IS NULL AND seq IS NULL)
UPDATE messages m
   SET seq = COALESCE(b.top, 0) + f.rn
  FROM fresh f LEFT JOIN base b ON b.channel_id = f.channel_id
 WHERE m.id = f.id;

UPDATE channels c SET last_seq = s.top
  FROM (SELECT channel_id, max(seq) AS top FROM messages WHERE seq IS NOT NULL GROUP BY channel_id) s
 WHERE c.id = s.channel_id AND c.last_seq < s.top;

-- Replies not numbered yet, after their thread's numbered ones.
WITH base AS (SELECT parent_message_id AS root, max(reply_seq) AS top
                FROM messages WHERE reply_seq IS NOT NULL GROUP BY parent_message_id),
     fresh AS (SELECT id, parent_message_id AS root,
                      row_number() OVER (PARTITION BY parent_message_id ORDER BY created_at, id) AS rn
                 FROM messages
                WHERE parent_message_id IS NOT NULL AND reply_seq IS NULL)
UPDATE messages m
   SET reply_seq = COALESCE(b.top, 0) + f.rn
  FROM fresh f LEFT JOIN base b ON b.root = f.root
 WHERE m.id = f.id;

UPDATE messages r SET last_reply_seq = s.top
  FROM (SELECT parent_message_id AS id, max(reply_seq) AS top
          FROM messages WHERE reply_seq IS NOT NULL GROUP BY parent_message_id) s
 WHERE r.id = s.id AND r.last_reply_seq < s.top;

-- Read positions from read times: the last main message at or before the time.
UPDATE read_receipts rr
   SET last_read_seq = COALESCE((
         SELECT m.seq FROM messages m
          WHERE m.channel_id = rr.channel_id AND m.parent_message_id IS NULL
            AND m.created_at <= rr.last_read_at
          ORDER BY m.created_at DESC, m.id DESC LIMIT 1), 0)
 WHERE rr.last_read_seq = 0;

-- A main message has a main number and no reply number; a reply the other way round.
DO $$
BEGIN
  IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'messages_numbered') THEN
    ALTER TABLE messages ADD CONSTRAINT messages_numbered
      CHECK ((parent_message_id IS NULL AND seq IS NOT NULL AND reply_seq IS NULL)
          OR (parent_message_id IS NOT NULL AND seq IS NULL AND reply_seq IS NOT NULL));
  END IF;
END
$$;

COMMIT;
