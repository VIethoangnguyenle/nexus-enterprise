-- ============================================
-- Lark Chat Parity — Message Enhancements
-- ============================================

-- Content format: markdown (new default) or plain (legacy)
ALTER TABLE messages ADD COLUMN IF NOT EXISTS content_format TEXT NOT NULL DEFAULT 'markdown'
    CHECK (content_format IN ('plain', 'markdown', 'html'));

-- Mentioned user IDs extracted from content
ALTER TABLE messages ADD COLUMN IF NOT EXISTS mentions TEXT[] DEFAULT '{}';

-- Full-text search vector (auto-generated from content)
ALTER TABLE messages ADD COLUMN IF NOT EXISTS search_vector tsvector
    GENERATED ALWAYS AS (to_tsvector('english', content)) STORED;

CREATE INDEX IF NOT EXISTS idx_messages_search ON messages USING GIN(search_vector);

-- Index on mentions for efficient @mention lookup
CREATE INDEX IF NOT EXISTS idx_messages_mentions ON messages USING GIN(mentions);
