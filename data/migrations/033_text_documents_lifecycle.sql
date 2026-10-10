-- 033_text_documents_lifecycle.sql: what happens to a text document when what
-- it hangs on goes away.
--
-- * Its folder. The folder row used to take its documents with it (CASCADE), so
--   a hard delete of a folder destroyed writing nobody was asked about.
--   SET NULL would be worse: a document of a restricted folder would reappear at
--   the top of the workspace, under the Documents OA everyone can read. So the
--   row is kept (RESTRICT): the folder's OA is gone, nothing is granted on it,
--   and the document is unreachable until someone deals with it on purpose.
-- * Its author. A user row used to block deleting the user (no ON DELETE). The
--   document outlives them with no owner; screens show an unknown person.

ALTER TABLE text_documents DROP CONSTRAINT IF EXISTS text_documents_folder_id_fkey;
ALTER TABLE text_documents
    ADD CONSTRAINT text_documents_folder_id_fkey
    FOREIGN KEY (folder_id) REFERENCES drive_items(id) ON DELETE RESTRICT;

ALTER TABLE text_documents ALTER COLUMN owner_id DROP NOT NULL;
ALTER TABLE text_documents DROP CONSTRAINT IF EXISTS text_documents_owner_id_fkey;
ALTER TABLE text_documents
    ADD CONSTRAINT text_documents_owner_id_fkey
    FOREIGN KEY (owner_id) REFERENCES users(id) ON DELETE SET NULL;

ALTER TABLE text_documents DROP CONSTRAINT IF EXISTS text_documents_last_editor_id_fkey;
ALTER TABLE text_documents
    ADD CONSTRAINT text_documents_last_editor_id_fkey
    FOREIGN KEY (last_editor_id) REFERENCES users(id) ON DELETE SET NULL;
