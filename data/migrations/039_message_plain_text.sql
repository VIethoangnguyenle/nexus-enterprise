-- The readable text of a message, beside its stored body.
--
-- A formatted (HTML) message keeps its filtered markup in `content`; search,
-- notifications and previews read `content_text`, the same text without tags,
-- so a search for "strong" does not match every bold word. Plain and markdown
-- bodies are their own text. The full-text index moves onto it.
--
-- Everything runs once, when the column is first added: `make db-migrate`
-- re-applies every migration in dev.
DO $$
BEGIN
    IF EXISTS (SELECT 1 FROM information_schema.columns
               WHERE table_schema = current_schema() AND table_name = 'messages'
                 AND column_name = 'content_text') THEN
        RETURN;
    END IF;

    ALTER TABLE messages ADD COLUMN content_text TEXT;

    UPDATE messages
    SET content_text = CASE
        WHEN content_format = 'html' THEN btrim(regexp_replace(
            replace(replace(replace(replace(regexp_replace(content, '<[^>]*>', ' ', 'g'),
                '&amp;', '&'), '&lt;', '<'), '&gt;', '>'), '&nbsp;', ' '),
            '\s+', ' ', 'g'))
        ELSE content
    END;

    DROP INDEX IF EXISTS idx_messages_search;
    ALTER TABLE messages DROP COLUMN IF EXISTS search_vector;
    ALTER TABLE messages ADD COLUMN search_vector tsvector
        GENERATED ALWAYS AS (to_tsvector('english', COALESCE(content_text, content))) STORED;
    CREATE INDEX idx_messages_search ON messages USING gin (search_vector);
END $$;
