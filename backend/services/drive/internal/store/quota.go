package store

import (
	"context"
	"fmt"
	"time"
)

// DriveQuota represents a row in the drive_quotas table.
type DriveQuota struct {
	WorkspaceID string
	MaxBytes    int64
	UsedBytes   int64
	MaxFiles    int32
	UsedFiles   int32
	UpdatedAt   time.Time
}

// GetOrCreateQuota returns the quota for a workspace, creating the default row
// if there is none. Reading an existing row takes no lock: the insert is
// DO NOTHING, so only the first call for a workspace writes.
func (s *Store) GetOrCreateQuota(ctx context.Context, workspaceID string) (*DriveQuota, error) {
	if _, err := s.db.Exec(ctx,
		`INSERT INTO drive_quotas (workspace_id) VALUES ($1) ON CONFLICT (workspace_id) DO NOTHING`, workspaceID); err != nil {
		return nil, fmt.Errorf("create quota: %w", err)
	}
	q := &DriveQuota{}
	err := s.db.QueryRow(ctx,
		`SELECT workspace_id, max_bytes, used_bytes, max_files, used_files, updated_at
		 FROM drive_quotas WHERE workspace_id = $1`, workspaceID).
		Scan(&q.WorkspaceID, &q.MaxBytes, &q.UsedBytes, &q.MaxFiles, &q.UsedFiles, &q.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("get quota: %w", err)
	}
	return q, nil
}

// UpdateQuotaLimits sets the max_bytes and max_files for a workspace.
func (s *Store) UpdateQuotaLimits(ctx context.Context, workspaceID string, maxBytes int64, maxFiles int32) error {
	_, err := s.db.Exec(ctx,
		`UPDATE drive_quotas SET max_bytes = $1, max_files = $2, updated_at = NOW()
		 WHERE workspace_id = $3`, maxBytes, maxFiles, workspaceID)
	return err
}

// IncrementQuota adds to used_bytes and used_files atomically.
func (s *Store) IncrementQuota(ctx context.Context, workspaceID string, bytes int64, files int32) error {
	_, err := s.db.Exec(ctx,
		`UPDATE drive_quotas SET used_bytes = used_bytes + $1, used_files = used_files + $2,
			updated_at = NOW()
		 WHERE workspace_id = $3`, bytes, files, workspaceID)
	return err
}

// DecrementQuota subtracts from used_bytes and used_files atomically.
func (s *Store) DecrementQuota(ctx context.Context, workspaceID string, bytes int64, files int32) error {
	_, err := s.db.Exec(ctx,
		`UPDATE drive_quotas SET used_bytes = GREATEST(0, used_bytes - $1),
			used_files = GREATEST(0, used_files - $2), updated_at = NOW()
		 WHERE workspace_id = $3`, bytes, files, workspaceID)
	return err
}

// CheckQuota returns true if the workspace has room for the given size.
func (s *Store) CheckQuota(ctx context.Context, workspaceID string, additionalBytes int64) (bool, error) {
	q, err := s.GetOrCreateQuota(ctx, workspaceID)
	if err != nil {
		return false, err
	}
	if q.MaxBytes >= 0 && q.UsedBytes+additionalBytes > q.MaxBytes {
		return false, nil
	}
	if q.MaxFiles >= 0 && q.UsedFiles+1 > q.MaxFiles {
		return false, nil
	}
	return true, nil
}
