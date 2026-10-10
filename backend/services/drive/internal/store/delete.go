package store

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

// DeleteItemReleasingQuota permanently removes an item and, in the same
// transaction, gives back the quota the files under it held. The files are read
// inside the transaction with their rows locked, so a file added or confirmed
// while the delete runs is either in the list (and released) or blocked until
// the delete is over; the list is never a stale copy. It returns every file the
// delete removed (the caller removes their stored objects) and nothing when the
// item was already gone. A pending upload was never charged and releases
// nothing. It returns ErrFolderHasDocuments when the database refuses because
// text documents still hang on the folder or one beneath it.
func (s *Store) DeleteItemReleasingQuota(ctx context.Context, id string) ([]*DriveItem, error) {
	var files []*DriveItem
	err := pgx.BeginFunc(ctx, s.db, func(tx pgx.Tx) error {
		files = nil
		rows, err := tx.Query(ctx,
			`SELECT id, workspace_id, drive_context, COALESCE(drive_context_id,''), parent_id,
				item_type, name, mime_type, size_bytes, object_key, storage_doc_id, ngac_node_id,
				COALESCE(scope_oa_id,''), owner_id, status, trashed_at, created_at, updated_at
			 FROM drive_items WHERE id IN (
				WITH RECURSIVE tree AS (
					SELECT id FROM drive_items WHERE id = $1
					UNION ALL
					SELECT di.id FROM drive_items di JOIN tree t ON di.parent_id = t.id
				) SELECT id FROM tree)
			 ORDER BY id FOR UPDATE`, id)
		if err != nil {
			return fmt.Errorf("lock subtree: %w", err)
		}
		for rows.Next() {
			item := &DriveItem{}
			if err := rows.Scan(&item.ID, &item.WorkspaceID, &item.DriveContext, &item.DriveContextID,
				&item.ParentID, &item.ItemType, &item.Name, &item.MimeType, &item.SizeBytes,
				&item.ObjectKey, &item.StorageDocID, &item.NGACNodeID, &item.ScopeOAID,
				&item.OwnerID, &item.Status, &item.TrashedAt, &item.CreatedAt, &item.UpdatedAt); err != nil {
				rows.Close()
				return fmt.Errorf("scan subtree: %w", err)
			}
			if item.ItemType == "file" {
				files = append(files, item)
			}
		}
		rows.Close()
		if err := rows.Err(); err != nil {
			return fmt.Errorf("read subtree: %w", err)
		}

		tag, err := tx.Exec(ctx, `DELETE FROM drive_items WHERE id = $1`, id)
		if err != nil {
			return err
		}
		if tag.RowsAffected() == 0 {
			files = nil
			return nil
		}
		for _, f := range files {
			if f.SizeBytes == nil || f.Status == "pending" {
				continue
			}
			if _, err := tx.Exec(ctx,
				`UPDATE drive_quotas SET used_bytes = GREATEST(0, used_bytes - $1),
					used_files = GREATEST(0, used_files - 1), updated_at = NOW()
				 WHERE workspace_id = $2`, *f.SizeBytes, f.WorkspaceID); err != nil {
				return fmt.Errorf("release quota: %w", err)
			}
		}
		return nil
	})
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "23503" && pgErr.ConstraintName == "text_documents_folder_id_fkey" {
		return nil, ErrFolderHasDocuments
	}
	if err != nil {
		return nil, err
	}
	return files, nil
}

// ErrFolderHasDocuments is returned when a delete would remove a folder (or one
// beneath it) that still holds text documents. text_documents.folder_id is
// ON DELETE RESTRICT: the writing in it is never destroyed as a side effect.
var ErrFolderHasDocuments = errors.New("folder holds text documents")

// CountTextDocumentsUnder counts the text documents in a folder and in every
// folder beneath it, in any state.
func (s *Store) CountTextDocumentsUnder(ctx context.Context, folderID string) (int, error) {
	var n int
	err := s.db.QueryRow(ctx,
		`WITH RECURSIVE tree AS (
			SELECT id FROM drive_items WHERE id = $1
			UNION ALL
			SELECT di.id FROM drive_items di JOIN tree t ON di.parent_id = t.id
		)
		SELECT count(*) FROM text_documents WHERE folder_id IN (SELECT id FROM tree)`, folderID).Scan(&n)
	return n, err
}

// DeleteItem permanently removes a drive item. It returns ErrFolderHasDocuments
// when the database refuses because text documents still hang on the folder or
// one beneath it (a document saved after the caller last looked).
func (s *Store) DeleteItem(ctx context.Context, id string) error {
	_, err := s.db.Exec(ctx, `DELETE FROM drive_items WHERE id = $1`, id)
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "23503" && pgErr.ConstraintName == "text_documents_folder_id_fkey" {
		return ErrFolderHasDocuments
	}
	return err
}
