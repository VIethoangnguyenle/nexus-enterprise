-- Files attached to a message, Google-Chat style: several files and text in
-- one message, in the order they were attached.
--
-- The file itself lives in the channel's drive (drive_items); a row here keeps
-- what the conversation shows of it — name, type, size and, for images, the
-- pixel size measured when it was uploaded — so a message renders without a
-- drive lookup per file. Older messages that linked a single drive file through
-- linked_entity_type = 'drive_file' are read as one attachment; they are not
-- copied here.

CREATE TABLE IF NOT EXISTS message_attachments (
    message_id    TEXT    NOT NULL REFERENCES messages(id) ON DELETE CASCADE,
    position      INT     NOT NULL,
    drive_file_id TEXT    NOT NULL,
    file_name     TEXT    NOT NULL,
    mime_type     TEXT    NOT NULL DEFAULT '',
    size_bytes    BIGINT  NOT NULL DEFAULT 0,
    width         INT     NOT NULL DEFAULT 0,
    height        INT     NOT NULL DEFAULT 0,
    PRIMARY KEY (message_id, position)
);

CREATE INDEX IF NOT EXISTS idx_message_attachments_file ON message_attachments (drive_file_id);
