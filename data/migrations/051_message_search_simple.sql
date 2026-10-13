-- Message search that reads Vietnamese and ignores markup.
--
-- The full-text column was built with the 'english' configuration, which stems
-- and drops words by English rules, so a Vietnamese message was indexed
-- wrongly; and it indexed the stored body, so a formatted message matched the
-- names of its own tags. It now uses the 'simple' configuration (every word
-- kept as written, lower-cased) over the readable text with any tag removed.
--
-- Rewrites the whole `messages` table: a generated column cannot change its
-- expression in place, so it is dropped, added back (every row recomputed) and
-- re-indexed. The table is locked for the length of that rewrite.
--
-- Runs once: `make db-migrate` re-applies every migration in dev, so a column
-- that already has the 'simple' expression is left alone.
--
-- Rollback (the previous English configuration):
--   DROP INDEX IF EXISTS idx_messages_search;
--   ALTER TABLE messages DROP COLUMN search_vector;
--   ALTER TABLE messages ADD COLUMN search_vector tsvector
--       GENERATED ALWAYS AS (to_tsvector('english', COALESCE(content_text, content))) STORED;
--   CREATE INDEX idx_messages_search ON messages USING gin (search_vector);
DO $$
BEGIN
    IF EXISTS (SELECT 1
               FROM pg_attribute a
               JOIN pg_attrdef d ON d.adrelid = a.attrelid AND d.adnum = a.attnum
               WHERE a.attrelid = to_regclass('messages') AND a.attname = 'search_vector'
                 AND pg_get_expr(d.adbin, d.adrelid) LIKE '%simple%') THEN
        RETURN;
    END IF;

    DROP INDEX IF EXISTS idx_messages_search;
    ALTER TABLE messages DROP COLUMN IF EXISTS search_vector;
    ALTER TABLE messages ADD COLUMN search_vector tsvector
        GENERATED ALWAYS AS (
            to_tsvector('simple', regexp_replace(COALESCE(content_text, content), '<[^>]+>', ' ', 'g'))
        ) STORED;
    CREATE INDEX idx_messages_search ON messages USING gin (search_vector);
END $$;
