package store

import (
	"context"
	"fmt"
)

// FileToRepair is a shared file still authorized on its container's OA, with
// the person the repair acts as: whoever made its earliest share that still has
// an identity, or else the file's owner. Both are "" when neither has one.
type FileToRepair struct {
	Item        *DriveItem
	ActorUserID string
	ActorNode   string
}

// SharedFilesWithoutTheirOwnOA returns up to limit files that have at least one
// share and are still authorized on the OA of what contains them: a folder's,
// a drive root's or a workspace's Documents OA. Those are the files shared
// before a shared file got an OA of its own, whose share OAs hold the container
// and so grant everything in it. A file that has its own OA points at a node no
// folder row and no workspace uses, so it is never returned: once repaired, the
// query finds nothing. An empty workspaceID looks in every workspace.
func (s *Store) SharedFilesWithoutTheirOwnOA(ctx context.Context, workspaceID string, limit int) ([]*FileToRepair, error) {
	rows, err := s.db.Query(ctx,
		`SELECT `+itemColumns+`, COALESCE(cu.id, ou.id, ''), COALESCE(cu.ngac_node, ou.ngac_node, '')
		 FROM drive_items di
		 LEFT JOIN LATERAL (
			SELECT u.id, u.ngac_node FROM drive_shares ds
			  JOIN users u ON u.ngac_node = ds.created_by
			 WHERE ds.drive_item_id = di.id
			 ORDER BY ds.created_at LIMIT 1) cu ON true
		 LEFT JOIN users ou ON ou.id = di.owner_id AND ou.ngac_node IS NOT NULL
		 WHERE di.item_type = 'file' AND ($2 = '' OR di.workspace_id = $2)
		   AND EXISTS (SELECT 1 FROM drive_shares ds WHERE ds.drive_item_id = di.id)
		   AND (EXISTS (SELECT 1 FROM drive_items o WHERE o.ngac_node_id = di.ngac_node_id AND o.item_type = 'folder')
		        OR EXISTS (SELECT 1 FROM workspaces w WHERE w.documents_oa_id = di.ngac_node_id))
		 ORDER BY di.id
		 LIMIT $1`, limit, workspaceID)
	if err != nil {
		return nil, fmt.Errorf("shared files to repair: %w", err)
	}
	defer rows.Close()
	var out []*FileToRepair
	for rows.Next() {
		item := &DriveItem{}
		f := &FileToRepair{Item: item}
		if err := rows.Scan(&item.ID, &item.WorkspaceID, &item.DriveContext, &item.DriveContextID,
			&item.ParentID, &item.ItemType, &item.Name, &item.MimeType, &item.SizeBytes,
			&item.ObjectKey, &item.StorageDocID, &item.NGACNodeID, &item.ScopeOAID,
			&item.OwnerID, &item.Status, &item.TrashedAt, &item.CreatedAt, &item.UpdatedAt,
			&f.ActorUserID, &f.ActorNode); err != nil {
			return nil, fmt.Errorf("scan shared file: %w", err)
		}
		out = append(out, f)
	}
	return out, rows.Err()
}

// RootShare is a share of a drive root, with the person a revocation acts as:
// whoever made the share, when they have an identity ("" otherwise).
type RootShare struct {
	ShareID     string
	ShareOA     string
	ItemID      string
	ActorUserID string
	ActorNode   string
}

// SharesOfDriveRoots returns up to limit shares whose item is a drive root.
// Roots can no longer be shared; any such row was made before that was refused.
// An empty workspaceID looks in every workspace.
func (s *Store) SharesOfDriveRoots(ctx context.Context, workspaceID string, limit int) ([]*RootShare, error) {
	rows, err := s.db.Query(ctx,
		`SELECT ds.id, ds.ngac_share_oa, ds.drive_item_id, COALESCE(u.id, ''), COALESCE(u.ngac_node, '')
		 FROM drive_shares ds
		 JOIN drive_items di ON di.id = ds.drive_item_id
		 LEFT JOIN users u ON u.ngac_node = ds.created_by
		 WHERE di.is_root AND ($2 = '' OR di.workspace_id = $2)
		 ORDER BY ds.id
		 LIMIT $1`, limit, workspaceID)
	if err != nil {
		return nil, fmt.Errorf("shares of drive roots: %w", err)
	}
	defer rows.Close()
	var out []*RootShare
	for rows.Next() {
		r := &RootShare{}
		if err := rows.Scan(&r.ShareID, &r.ShareOA, &r.ItemID, &r.ActorUserID, &r.ActorNode); err != nil {
			return nil, fmt.Errorf("scan root share: %w", err)
		}
		out = append(out, r)
	}
	return out, rows.Err()
}
