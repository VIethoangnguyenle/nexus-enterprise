// Package store — files attached to messages.
package store

import (
	"context"
	"fmt"
)

// Attachment is one file of a message, as the conversation shows it.
type Attachment struct {
	DriveFileID string
	FileName    string
	MimeType    string
	SizeBytes   int64
	Width       int32
	Height      int32
}

// DriveFile is a confirmed file of a channel's drive.
type DriveFile struct {
	ID        string
	Name      string
	MimeType  string
	SizeBytes int64
}

// ChannelDriveFiles returns, of the given ids, the confirmed files that sit
// under the channel's own drive root. Whatever a file's row claims about its
// context, only its chain of parents counts: a file moved or created outside
// the channel's drive is not returned, nor is a folder, a pending upload or an
// unknown id.
func (s *Store) ChannelDriveFiles(ctx context.Context, channelID string, fileIDs []string) (map[string]DriveFile, error) {
	out := make(map[string]DriveFile, len(fileIDs))
	if len(fileIDs) == 0 {
		return out, nil
	}
	rows, err := s.db.Query(ctx,
		`WITH RECURSIVE chain AS (
		   SELECT f.id AS file_id, f.parent_id, 0 AS depth
		   FROM drive_items f
		   WHERE f.id = ANY($2) AND f.item_type = 'file' AND f.status = 'active'
		   UNION ALL
		   SELECT c.file_id, p.parent_id, c.depth + 1
		   FROM chain c JOIN drive_items p ON p.id = c.parent_id
		   WHERE c.depth < 64 AND p.status = 'active'
		 )
		 SELECT f.id, f.name, COALESCE(f.mime_type, ''), COALESCE(f.size_bytes, 0)
		 FROM drive_items f
		 WHERE f.id = ANY($2) AND f.item_type = 'file' AND f.status = 'active'
		   AND EXISTS (
		     SELECT 1 FROM chain c JOIN drive_items root ON root.id = c.parent_id
		     WHERE c.file_id = f.id AND root.parent_id IS NULL AND root.is_root
		       AND root.item_type = 'folder' AND root.status = 'active'
		       AND root.drive_context = 'channel' AND root.drive_context_id = $1)`,
		channelID, fileIDs)
	if err != nil {
		return nil, fmt.Errorf("channel drive files: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var f DriveFile
		if err := rows.Scan(&f.ID, &f.Name, &f.MimeType, &f.SizeBytes); err != nil {
			return nil, err
		}
		out[f.ID] = f
	}
	return out, rows.Err()
}

// insertAttachments writes a message's attachments, in order.
func insertAttachments(ctx context.Context, x execer, msg *Message) error {
	for i, a := range msg.Attachments {
		if _, err := x.Exec(ctx,
			`INSERT INTO message_attachments (message_id, position, drive_file_id, file_name, mime_type, size_bytes, width, height)
			 VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`,
			msg.ID, i, a.DriveFileID, a.FileName, a.MimeType, a.SizeBytes, a.Width, a.Height); err != nil {
			return fmt.Errorf("insert attachment: %w", err)
		}
	}
	return nil
}

// ListAttachmentsForMessages returns the attachments of a page of messages in
// one query, each message's in the order attached. A message from before
// attachments that linked one drive file reads as that one attachment.
func (s *Store) ListAttachmentsForMessages(ctx context.Context, messageIDs []string) (map[string][]Attachment, error) {
	out := map[string][]Attachment{}
	if len(messageIDs) == 0 {
		return out, nil
	}
	rows, err := s.db.Query(ctx,
		`SELECT message_id, drive_file_id, file_name, mime_type, size_bytes, width, height, position
		 FROM message_attachments WHERE message_id = ANY($1)
		 UNION ALL
		 SELECT m.id, d.id, d.name, COALESCE(d.mime_type, ''), COALESCE(d.size_bytes, 0), 0, 0, 0
		 FROM messages m JOIN drive_items d ON d.id = m.linked_entity_id
		 WHERE m.id = ANY($1) AND m.linked_entity_type = 'drive_file'
		   -- Only a live file of the message's own channel: the link was never
		   -- checked when it was stored.
		   AND d.status = 'active' AND d.item_type = 'file'
		   AND d.drive_context = 'channel' AND d.drive_context_id = m.channel_id
		   AND NOT EXISTS (SELECT 1 FROM message_attachments a WHERE a.message_id = m.id)
		 ORDER BY 1, 8`, messageIDs)
	if err != nil {
		return nil, fmt.Errorf("list attachments: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var id string
		var a Attachment
		var pos int
		if err := rows.Scan(&id, &a.DriveFileID, &a.FileName, &a.MimeType, &a.SizeBytes, &a.Width, &a.Height, &pos); err != nil {
			return nil, err
		}
		out[id] = append(out[id], a)
	}
	return out, rows.Err()
}
