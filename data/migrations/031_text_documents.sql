-- 031_text_documents.sql: documents written in the app (as opposed to files
-- uploaded to the drive).
--
-- A text document is not a graph node. Like a drive file it lives under a
-- folder and is authorized on that folder's OA (the workspace's Documents OA
-- when it has no folder), so permission on the folder is permission on what is
-- written in it. `version` is the optimistic-concurrency token: every write
-- states the version it was based on and is refused when the row has moved on.

CREATE TABLE IF NOT EXISTS text_documents (
    id             TEXT PRIMARY KEY DEFAULT gen_random_uuid()::TEXT,
    workspace_id   TEXT NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    -- The drive folder it sits in; NULL is the top of the workspace's Documents.
    folder_id      TEXT REFERENCES drive_items(id) ON DELETE CASCADE,
    title          TEXT NOT NULL CHECK (char_length(title) BETWEEN 1 AND 200),
    -- HTML written by the editor. Untrusted: readers sanitize it on render.
    content        TEXT NOT NULL DEFAULT '' CHECK (octet_length(content) <= 1048576),
    version        INTEGER NOT NULL DEFAULT 1 CHECK (version >= 1),
    status         TEXT NOT NULL DEFAULT 'draft' CHECK (status IN ('draft', 'active', 'archived')),
    owner_id       TEXT NOT NULL REFERENCES users(id),
    last_editor_id TEXT REFERENCES users(id),
    created_at     TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at     TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_text_documents_workspace ON text_documents (workspace_id, updated_at DESC);
CREATE INDEX IF NOT EXISTS idx_text_documents_folder ON text_documents (folder_id) WHERE folder_id IS NOT NULL;
CREATE INDEX IF NOT EXISTS idx_text_documents_owner ON text_documents (workspace_id, owner_id);
