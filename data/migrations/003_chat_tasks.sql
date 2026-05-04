-- ============================================
-- Lark Chat Parity — Chat Tasks
-- ============================================

CREATE TABLE IF NOT EXISTS chat_tasks (
    id          TEXT PRIMARY KEY DEFAULT gen_random_uuid()::text,
    message_id  TEXT NOT NULL REFERENCES messages(id) ON DELETE CASCADE,
    channel_id  TEXT NOT NULL REFERENCES channels(id) ON DELETE CASCADE,
    title       TEXT NOT NULL,
    assignee_id TEXT REFERENCES users(id) ON DELETE SET NULL,
    status      TEXT NOT NULL DEFAULT 'todo' CHECK (status IN ('todo', 'in_progress', 'done')),
    due_date    DATE,
    created_by  TEXT NOT NULL REFERENCES users(id),
    created_at  TIMESTAMPTZ DEFAULT NOW(),
    updated_at  TIMESTAMPTZ DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_tasks_channel ON chat_tasks(channel_id, status);
CREATE INDEX IF NOT EXISTS idx_tasks_assignee ON chat_tasks(assignee_id, status);
CREATE INDEX IF NOT EXISTS idx_tasks_message ON chat_tasks(message_id);
