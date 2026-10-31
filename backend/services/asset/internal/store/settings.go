package store

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5"
)

// AssetUserSettings represents a row in asset_user_settings.
type AssetUserSettings struct {
	UserID            string
	WorkspaceID       string
	DefaultTab        string
	Sort              string
	NotifyRequests    bool
	NotifyMaintenance bool
	UpdatedAt         time.Time
}

// GetAssetUserSettings returns the caller's asset settings, or nil when no row exists.
func (s *Store) GetAssetUserSettings(ctx context.Context, userID, workspaceID string) (*AssetUserSettings, error) {
	st := &AssetUserSettings{}
	err := s.pool.QueryRow(ctx,
		`SELECT user_id, workspace_id, default_tab, sort, notify_requests, notify_maintenance, updated_at
		   FROM asset_user_settings WHERE user_id = $1 AND workspace_id = $2`, userID, workspaceID).
		Scan(&st.UserID, &st.WorkspaceID, &st.DefaultTab, &st.Sort, &st.NotifyRequests, &st.NotifyMaintenance, &st.UpdatedAt)
	if err == pgx.ErrNoRows {
		return nil, nil
	}
	return st, err
}

// UpsertAssetUserSettings inserts or updates the caller's asset settings.
func (s *Store) UpsertAssetUserSettings(ctx context.Context, st *AssetUserSettings) error {
	_, err := s.pool.Exec(ctx,
		`INSERT INTO asset_user_settings (user_id, workspace_id, default_tab, sort, notify_requests, notify_maintenance, updated_at)
		 VALUES ($1, $2, $3, $4, $5, $6, NOW())
		 ON CONFLICT (user_id, workspace_id) DO UPDATE SET
		   default_tab = EXCLUDED.default_tab, sort = EXCLUDED.sort,
		   notify_requests = EXCLUDED.notify_requests, notify_maintenance = EXCLUDED.notify_maintenance,
		   updated_at = NOW()`,
		st.UserID, st.WorkspaceID, st.DefaultTab, st.Sort, st.NotifyRequests, st.NotifyMaintenance)
	return err
}

// IsWorkspaceMember reports whether the user is an active member of the workspace.
func (s *Store) IsWorkspaceMember(ctx context.Context, workspaceID, userID string) (bool, error) {
	var exists bool
	err := s.pool.QueryRow(ctx,
		`SELECT EXISTS (SELECT 1 FROM tenant_users WHERE tenant_id = $1 AND user_id = $2 AND status = 'active')`,
		workspaceID, userID).Scan(&exists)
	return exists, err
}
