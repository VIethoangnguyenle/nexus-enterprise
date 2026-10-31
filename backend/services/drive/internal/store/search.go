package store

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
)

// SearchItems finds active drive items in a workspace whose name holds the
// query (case-insensitive ILIKE, wildcards taken literally); a drive's root is
// not an item and is never found. Results are ordered by relevance
// (exact prefix first, then by updated_at descending). At most limit items are
// returned; pass 0 for the default (50). The caller is responsible for filtering
// the results by NGAC access — the store has no knowledge of policy.
func (s *Store) SearchItems(ctx context.Context, workspaceID, query string, limit int) ([]*DriveItem, error) {
	if workspaceID == "" || query == "" {
		return nil, nil
	}
	if limit <= 0 {
		limit = 50
	}

	pattern := "%" + escapeLike(query) + "%"

	rows, err := s.db.Query(ctx,
		`SELECT id, workspace_id, drive_context, COALESCE(drive_context_id,''), parent_id,
			item_type, name, mime_type, size_bytes, object_key, storage_doc_id, ngac_node_id,
			COALESCE(scope_oa_id,''), owner_id, status, trashed_at, created_at, updated_at
		 FROM drive_items
		 WHERE workspace_id = $1
		   AND status = 'active'
		   AND NOT is_root
		   AND name ILIKE $2
		 ORDER BY
		   CASE WHEN lower(name) LIKE lower($3) || '%' THEN 0 ELSE 1 END,
		   updated_at DESC
		 LIMIT $4`, workspaceID, pattern, escapeLike(query), limit)
	if err != nil {
		return nil, fmt.Errorf("search items: %w", err)
	}
	defer rows.Close()

	return scanItems(rows)
}

// scanItems scans rows into DriveItem slices; shared between ListChildren and
// SearchItems to keep column-list changes in one place.
func scanItems(rows pgx.Rows) ([]*DriveItem, error) {
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
	return items, rows.Err()
}

// escapeLike escapes the SQL LIKE special characters % and _ so that a search
// term is taken literally.
func escapeLike(s string) string {
	var out []byte
	for i := range len(s) {
		switch s[i] {
		case '%', '_', '\\':
			out = append(out, '\\')
		}
		out = append(out, s[i])
	}
	return string(out)
}
