package store

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
)

// ErrNotPending is returned when a file is confirmed that is no longer pending:
// it was already confirmed, or removed, since the caller looked.
var ErrNotPending = errors.New("file is not pending")

// ActivateFile publishes a pending upload and charges it to the workspace's
// quota in one transaction: a file is never active without being counted, and a
// second confirmation (which finds nothing pending) charges nothing. sizeBytes,
// the size the store reports for the uploaded object, replaces the declared one.
func (s *Store) ActivateFile(ctx context.Context, id, workspaceID string, sizeBytes int64) error {
	return pgx.BeginFunc(ctx, s.db, func(tx pgx.Tx) error {
		tag, err := tx.Exec(ctx,
			`UPDATE drive_items SET size_bytes = $2, status = 'active', trashed_at = NULL, updated_at = NOW()
			 WHERE id = $1 AND status = 'pending'`, id, sizeBytes)
		if err != nil {
			return fmt.Errorf("publish file: %w", err)
		}
		if tag.RowsAffected() != 1 {
			return ErrNotPending
		}
		if _, err := tx.Exec(ctx,
			`INSERT INTO drive_quotas (workspace_id, used_bytes, used_files) VALUES ($1, $2, 1)
			 ON CONFLICT (workspace_id) DO UPDATE SET
			   used_bytes = drive_quotas.used_bytes + EXCLUDED.used_bytes,
			   used_files = drive_quotas.used_files + 1, updated_at = NOW()`, workspaceID, sizeBytes); err != nil {
			return fmt.Errorf("charge quota: %w", err)
		}
		return nil
	})
}

// GetChildFiles returns all file items under a folder recursively (for permanent delete + quota).
func (s *Store) GetChildFiles(ctx context.Context, parentID string) ([]*DriveItem, error) {
	rows, err := s.db.Query(ctx,
		`WITH RECURSIVE tree AS (
			SELECT id FROM drive_items WHERE parent_id = $1
			UNION ALL
			SELECT di.id FROM drive_items di JOIN tree t ON di.parent_id = t.id
		)
		SELECT id, workspace_id, drive_context, COALESCE(drive_context_id,''), parent_id,
			item_type, name, mime_type, size_bytes, object_key, storage_doc_id, ngac_node_id,
			COALESCE(scope_oa_id,''), owner_id, status, trashed_at, created_at, updated_at
		FROM drive_items WHERE id IN (SELECT id FROM tree) AND item_type = 'file'`, parentID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var items []*DriveItem
	for rows.Next() {
		item := &DriveItem{}
		if err := rows.Scan(&item.ID, &item.WorkspaceID, &item.DriveContext, &item.DriveContextID,
			&item.ParentID, &item.ItemType, &item.Name, &item.MimeType, &item.SizeBytes,
			&item.ObjectKey, &item.StorageDocID, &item.NGACNodeID, &item.ScopeOAID,
			&item.OwnerID, &item.Status, &item.TrashedAt, &item.CreatedAt, &item.UpdatedAt); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, nil
}

// UpdateFileSize updates the size_bytes column for a drive item.
func (s *Store) UpdateFileSize(ctx context.Context, id string, sizeBytes int64) error {
	_, err := s.db.Exec(ctx, `UPDATE drive_items SET size_bytes = $1 WHERE id = $2`, sizeBytes, id)
	return err
}
