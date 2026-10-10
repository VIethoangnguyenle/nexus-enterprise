package store

import (
	"context"

	"github.com/jackc/pgx/v5"
)

// FindRootByContext finds the root folder for a workspace or channel drive.
func (s *Store) FindRootByContext(ctx context.Context, workspaceID, driveContext, driveContextID string) (*DriveItem, error) {
	item := &DriveItem{}
	err := s.db.QueryRow(ctx,
		`SELECT id, workspace_id, drive_context, COALESCE(drive_context_id,''), parent_id,
			item_type, name, mime_type, size_bytes, object_key, storage_doc_id, ngac_node_id,
			COALESCE(scope_oa_id,''), owner_id, status, trashed_at, created_at, updated_at
		 FROM drive_items
		 WHERE workspace_id = $1 AND drive_context = $2
		   AND COALESCE(drive_context_id,'') = $3
		   AND parent_id IS NULL AND item_type = 'folder' AND is_root AND status = 'active'
		 LIMIT 1`, workspaceID, driveContext, driveContextID).
		Scan(&item.ID, &item.WorkspaceID, &item.DriveContext, &item.DriveContextID,
			&item.ParentID, &item.ItemType, &item.Name, &item.MimeType, &item.SizeBytes,
			&item.ObjectKey, &item.StorageDocID, &item.NGACNodeID, &item.ScopeOAID,
			&item.OwnerID, &item.Status, &item.TrashedAt, &item.CreatedAt, &item.UpdatedAt)
	if err == pgx.ErrNoRows {
		return nil, nil
	}
	return item, err
}

// RootWorkspaceByNode returns the workspace whose drive root hangs on the given
// OA, or "" when no root does. A drive OA that is the root of one workspace must
// never be adopted by another.
func (s *Store) RootWorkspaceByNode(ctx context.Context, nodeID string) (string, error) {
	var ws string
	err := s.db.QueryRow(ctx,
		`SELECT workspace_id FROM drive_items WHERE ngac_node_id = $1 AND is_root LIMIT 1`, nodeID).Scan(&ws)
	if err == pgx.ErrNoRows {
		return "", nil
	}
	return ws, err
}

// GetWorkspacePCID returns the NGAC PC node ID for a workspace.
func (s *Store) GetWorkspacePCID(ctx context.Context, workspaceID string) (string, error) {
	var pcID string
	err := s.db.QueryRow(ctx,
		`SELECT ngac_pc_id FROM workspaces WHERE id = $1`, workspaceID).Scan(&pcID)
	if err == pgx.ErrNoRows {
		return "", nil
	}
	return pcID, err
}

// GetWorkspaceDocumentsOAID returns the Documents OA recorded for a workspace,
// or "" when the workspace has none recorded (or does not exist). The drive roots
// itself there; it never works the OA out from node names.
func (s *Store) GetWorkspaceDocumentsOAID(ctx context.Context, workspaceID string) (string, error) {
	var oaID string
	err := s.db.QueryRow(ctx,
		`SELECT COALESCE(documents_oa_id, '') FROM workspaces WHERE id = $1`, workspaceID).Scan(&oaID)
	if err == pgx.ErrNoRows {
		return "", nil
	}
	return oaID, err
}

// GetChannelWorkspaceID returns the workspace_id for a channel.
func (s *Store) GetChannelWorkspaceID(ctx context.Context, channelID string) (string, error) {
	var wsID string
	err := s.db.QueryRow(ctx,
		`SELECT COALESCE(workspace_id, '') FROM channels WHERE id = $1`, channelID).Scan(&wsID)
	if err == pgx.ErrNoRows {
		return "", nil
	}
	return wsID, err
}
