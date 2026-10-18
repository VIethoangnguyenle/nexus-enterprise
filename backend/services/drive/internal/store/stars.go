package store

import (
	"context"
	"fmt"
	"time"
)

// itemColumns is the column list scanItems reads, qualified by the alias di.
const itemColumns = `di.id, di.workspace_id, di.drive_context, COALESCE(di.drive_context_id,''),
	di.parent_id, di.item_type, di.name, di.mime_type, di.size_bytes, di.object_key,
	di.storage_doc_id, di.ngac_node_id, COALESCE(di.scope_oa_id,''), di.owner_id,
	di.status, di.trashed_at, di.created_at, di.updated_at`

// StarItem stars an item for a person. Starring twice is not an error.
func (s *Store) StarItem(ctx context.Context, userID, itemID string) error {
	_, err := s.db.Exec(ctx,
		`INSERT INTO drive_stars (user_id, item_id) VALUES ($1, $2) ON CONFLICT DO NOTHING`, userID, itemID)
	return err
}

// UnstarItem removes a person's own star. Removing a star that is not there is
// not an error.
func (s *Store) UnstarItem(ctx context.Context, userID, itemID string) error {
	_, err := s.db.Exec(ctx, `DELETE FROM drive_stars WHERE user_id = $1 AND item_id = $2`, userID, itemID)
	return err
}

// ListStarredItems returns the active items of workspaceID that userID has
// starred, newest star first. Nothing here decides who may see them: the
// caller re-checks read on every item.
func (s *Store) ListStarredItems(ctx context.Context, userID, workspaceID string) ([]*DriveItem, error) {
	rows, err := s.db.Query(ctx,
		`SELECT `+itemColumns+`
		 FROM drive_stars ds
		 JOIN drive_items di ON di.id = ds.item_id
		 WHERE ds.user_id = $1 AND di.workspace_id = $2 AND di.status = 'active' AND NOT di.is_root
		 ORDER BY ds.created_at DESC
		 LIMIT 500`, userID, workspaceID)
	if err != nil {
		return nil, fmt.Errorf("list starred: %w", err)
	}
	defer rows.Close()
	return scanItems(rows)
}

// QuickAccessCandidate is a file offered for Truy cập nhanh, with the reason it
// is offered and when that happened.
type QuickAccessCandidate struct {
	Item *DriveItem
	// Reason is "shared", "starred" or "yours".
	Reason string
	// SharedBy is the NGAC node of the person who shared it (Reason "shared").
	SharedBy string
	At       time.Time
}

// QuickAccessCandidates returns up to limit active files of workspaceID that
// were shared with one of shareTargets (the person's node and groups) by
// someone else, starred by userID, or put there by userID. Each file comes
// once, with its strongest reason (shared, then starred, then yours), and the
// list is ordered by when that reason happened, newest first. Read is not
// checked here; the caller filters on the policy's answer.
func (s *Store) QuickAccessCandidates(ctx context.Context, userID, callerNode, workspaceID string, shareTargets []string, limit int) ([]*QuickAccessCandidate, error) {
	rows, err := s.db.Query(ctx,
		`WITH reasons AS (
			(SELECT sh.drive_item_id AS id, 'shared' AS reason, sh.created_by AS shared_by, sh.created_at AS at, 1 AS pri
			   FROM drive_shares sh JOIN drive_items di ON di.id = sh.drive_item_id
			  WHERE sh.target_ngac_id = ANY($4) AND sh.created_by <> $2 AND di.workspace_id = $3
			    AND di.status = 'active' AND di.item_type = 'file'
			  ORDER BY sh.created_at DESC LIMIT $5)
			UNION ALL
			(SELECT ds.item_id, 'starred', '', ds.created_at, 2
			   FROM drive_stars ds JOIN drive_items di ON di.id = ds.item_id
			  WHERE ds.user_id = $1 AND di.workspace_id = $3 AND di.status = 'active' AND di.item_type = 'file'
			  ORDER BY ds.created_at DESC LIMIT $5)
			UNION ALL
			(SELECT di.id, 'yours', '', di.updated_at, 3
			   FROM drive_items di
			  WHERE di.owner_id = $1 AND di.workspace_id = $3 AND di.status = 'active' AND di.item_type = 'file'
			  ORDER BY di.updated_at DESC LIMIT $5)
		), best AS (
			SELECT DISTINCT ON (id) id, reason, shared_by, at FROM reasons ORDER BY id, pri, at DESC
		)
		SELECT `+itemColumns+`, b.reason, COALESCE(b.shared_by, ''), b.at
		  FROM best b JOIN drive_items di ON di.id = b.id
		 WHERE di.workspace_id = $3 AND di.status = 'active' AND di.item_type = 'file'
		 ORDER BY b.at DESC, di.id
		 LIMIT $5`, userID, callerNode, workspaceID, shareTargets, limit)
	if err != nil {
		return nil, fmt.Errorf("quick access: %w", err)
	}
	defer rows.Close()
	var out []*QuickAccessCandidate
	for rows.Next() {
		item := &DriveItem{}
		c := &QuickAccessCandidate{Item: item}
		if err := rows.Scan(&item.ID, &item.WorkspaceID, &item.DriveContext, &item.DriveContextID,
			&item.ParentID, &item.ItemType, &item.Name, &item.MimeType, &item.SizeBytes,
			&item.ObjectKey, &item.StorageDocID, &item.NGACNodeID, &item.ScopeOAID,
			&item.OwnerID, &item.Status, &item.TrashedAt, &item.CreatedAt, &item.UpdatedAt,
			&c.Reason, &c.SharedBy, &c.At); err != nil {
			return nil, fmt.Errorf("scan quick access: %w", err)
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

// RecentFiles returns up to limit active files of workspaceID, last edited
// first. Read is not checked here.
func (s *Store) RecentFiles(ctx context.Context, workspaceID string, limit int) ([]*DriveItem, error) {
	rows, err := s.db.Query(ctx,
		`SELECT `+itemColumns+`
		 FROM drive_items di
		 WHERE di.workspace_id = $1 AND di.status = 'active' AND di.item_type = 'file'
		 ORDER BY di.updated_at DESC, di.id
		 LIMIT $2`, workspaceID, limit)
	if err != nil {
		return nil, fmt.Errorf("recent files: %w", err)
	}
	defer rows.Close()
	return scanItems(rows)
}

// StalePendingUpload is an upload left pending, with the NGAC node of the
// person who started it ("" when they have none).
type StalePendingUpload struct {
	Item      *DriveItem
	OwnerNode string
}

// StalePendingUploads returns up to limit uploads still pending more than
// olderThan after they were started, oldest first.
func (s *Store) StalePendingUploads(ctx context.Context, olderThan time.Duration, limit int) ([]*StalePendingUpload, error) {
	rows, err := s.db.Query(ctx,
		`SELECT `+itemColumns+`, COALESCE(u.ngac_node, '')
		 FROM drive_items di
		 LEFT JOIN users u ON u.id = di.owner_id
		 WHERE di.status = 'pending' AND di.created_at < NOW() - make_interval(secs => $1)
		 ORDER BY di.created_at
		 LIMIT $2`, olderThan.Seconds(), limit)
	if err != nil {
		return nil, fmt.Errorf("stale uploads: %w", err)
	}
	defer rows.Close()
	var out []*StalePendingUpload
	for rows.Next() {
		item := &DriveItem{}
		u := &StalePendingUpload{Item: item}
		if err := rows.Scan(&item.ID, &item.WorkspaceID, &item.DriveContext, &item.DriveContextID,
			&item.ParentID, &item.ItemType, &item.Name, &item.MimeType, &item.SizeBytes,
			&item.ObjectKey, &item.StorageDocID, &item.NGACNodeID, &item.ScopeOAID,
			&item.OwnerID, &item.Status, &item.TrashedAt, &item.CreatedAt, &item.UpdatedAt,
			&u.OwnerNode); err != nil {
			return nil, fmt.Errorf("scan stale upload: %w", err)
		}
		out = append(out, u)
	}
	return out, rows.Err()
}

// DeletePendingItem removes an item only while it is still pending, so an
// upload confirmed in the meantime is never taken away. It reports whether a
// row was removed. A pending upload was never charged to the quota.
func (s *Store) DeletePendingItem(ctx context.Context, id string) (bool, error) {
	tag, err := s.db.Exec(ctx, `DELETE FROM drive_items WHERE id = $1 AND status = 'pending'`, id)
	if err != nil {
		return false, err
	}
	return tag.RowsAffected() == 1, nil
}
