package store

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// DriveShare represents a row in the drive_shares table.
type DriveShare struct {
	ID           string
	DriveItemID  string
	ShareType    string
	TargetNGACID *string
	TargetLabel  *string
	Operations   []string
	NGACShareOA  string
	CreatedBy    string
	CreatedAt    time.Time
}

// InsertShare creates a drive share record.
func (s *Store) InsertShare(ctx context.Context, share *DriveShare) error {
	if share.ID == "" {
		share.ID = uuid.New().String()
	}
	_, err := s.db.Exec(ctx,
		`INSERT INTO drive_shares (id, drive_item_id, share_type, target_ngac_id, target_label,
			operations, ngac_share_oa, created_by)
		 VALUES ($1,$2,$3,$4,$5,$6,$7,$8)`,
		share.ID, share.DriveItemID, share.ShareType, share.TargetNGACID, share.TargetLabel,
		share.Operations, share.NGACShareOA, share.CreatedBy)
	return err
}

// GetShare retrieves a share by ID.
func (s *Store) GetShare(ctx context.Context, id string) (*DriveShare, error) {
	share := &DriveShare{}
	err := s.db.QueryRow(ctx,
		`SELECT id, drive_item_id, share_type, target_ngac_id, target_label,
			operations, ngac_share_oa, created_by, created_at
		 FROM drive_shares WHERE id = $1`, id).
		Scan(&share.ID, &share.DriveItemID, &share.ShareType, &share.TargetNGACID,
			&share.TargetLabel, &share.Operations, &share.NGACShareOA, &share.CreatedBy,
			&share.CreatedAt)
	if err == pgx.ErrNoRows {
		return nil, nil
	}
	return share, err
}

// ListSharesByItem returns all shares for a drive item.
func (s *Store) ListSharesByItem(ctx context.Context, itemID string) ([]*DriveShare, error) {
	rows, err := s.db.Query(ctx,
		`SELECT id, drive_item_id, share_type, target_ngac_id, target_label,
			operations, ngac_share_oa, created_by, created_at
		 FROM drive_shares WHERE drive_item_id = $1 ORDER BY created_at`, itemID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var shares []*DriveShare
	for rows.Next() {
		share := &DriveShare{}
		if err := rows.Scan(&share.ID, &share.DriveItemID, &share.ShareType, &share.TargetNGACID,
			&share.TargetLabel, &share.Operations, &share.NGACShareOA, &share.CreatedBy,
			&share.CreatedAt); err != nil {
			return nil, err
		}
		shares = append(shares, share)
	}
	return shares, nil
}

// ListSharesByTarget returns shares targeting a specific NGAC node.
func (s *Store) ListSharesByTarget(ctx context.Context, targetNGACIDs []string) ([]*DriveShare, error) {
	rows, err := s.db.Query(ctx,
		`SELECT ds.id, ds.drive_item_id, ds.share_type, ds.target_ngac_id, ds.target_label,
			ds.operations, ds.ngac_share_oa, ds.created_by, ds.created_at
		 FROM drive_shares ds
		 JOIN drive_items di ON ds.drive_item_id = di.id
		 WHERE (ds.target_ngac_id = ANY($1) OR ds.share_type = 'public')
		   AND di.status = 'active'
		 ORDER BY ds.created_at DESC`, targetNGACIDs)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var shares []*DriveShare
	for rows.Next() {
		share := &DriveShare{}
		if err := rows.Scan(&share.ID, &share.DriveItemID, &share.ShareType, &share.TargetNGACID,
			&share.TargetLabel, &share.Operations, &share.NGACShareOA, &share.CreatedBy,
			&share.CreatedAt); err != nil {
			return nil, err
		}
		shares = append(shares, share)
	}
	return shares, nil
}

// DeleteShare removes a share record.
func (s *Store) DeleteShare(ctx context.Context, id string) error {
	_, err := s.db.Exec(ctx, `DELETE FROM drive_shares WHERE id = $1`, id)
	return err
}
