package store

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5"
)

// DriveUserSettings represents a row in drive_user_settings.
type DriveUserSettings struct {
	UserID            string
	WorkspaceID       string
	View              string
	Sort              string
	FoldersFirst      bool
	CellularPreviews  bool
	NotifyShared      bool
	UpdatedAt         time.Time
}

// GetDriveUserSettings returns the caller's settings for a workspace, or nil when no row exists.
func (s *Store) GetDriveUserSettings(ctx context.Context, userID, workspaceID string) (*DriveUserSettings, error) {
	st := &DriveUserSettings{}
	err := s.db.QueryRow(ctx,
		`SELECT user_id, workspace_id, view, sort, folders_first, cellular_previews, notify_shared, updated_at
		   FROM drive_user_settings WHERE user_id = $1 AND workspace_id = $2`, userID, workspaceID).
		Scan(&st.UserID, &st.WorkspaceID, &st.View, &st.Sort, &st.FoldersFirst, &st.CellularPreviews, &st.NotifyShared, &st.UpdatedAt)
	if err == pgx.ErrNoRows {
		return nil, nil
	}
	return st, err
}

// UpsertDriveUserSettings inserts or updates the caller's settings.
func (s *Store) UpsertDriveUserSettings(ctx context.Context, st *DriveUserSettings) error {
	_, err := s.db.Exec(ctx,
		`INSERT INTO drive_user_settings (user_id, workspace_id, view, sort, folders_first, cellular_previews, notify_shared, updated_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, NOW())
		 ON CONFLICT (user_id, workspace_id) DO UPDATE SET
		   view = EXCLUDED.view, sort = EXCLUDED.sort, folders_first = EXCLUDED.folders_first,
		   cellular_previews = EXCLUDED.cellular_previews, notify_shared = EXCLUDED.notify_shared,
		   updated_at = NOW()`,
		st.UserID, st.WorkspaceID, st.View, st.Sort, st.FoldersFirst, st.CellularPreviews, st.NotifyShared)
	return err
}

// IsMemberOfWorkspace reports whether the user is an active member of the workspace.
func (s *Store) IsMemberOfWorkspace(ctx context.Context, workspaceID, userID string) (bool, error) {
	var exists bool
	err := s.db.QueryRow(ctx,
		`SELECT EXISTS (SELECT 1 FROM tenant_users WHERE tenant_id = $1 AND user_id = $2 AND status = 'active')`,
		workspaceID, userID).Scan(&exists)
	return exists, err
}
