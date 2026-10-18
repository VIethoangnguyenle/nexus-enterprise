package store

import (
	"context"

	"github.com/jackc/pgx/v5"

	"ngac-platform/services/approval/internal/domain"
)

// EnsureSettingsTable creates the approval_user_settings table in the tenant
// schema if it does not already exist. Safe to call on every request.
func (s *Store) EnsureSettingsTable(ctx context.Context) error {
	c, err := s.conn(ctx)
	if err != nil {
		return err
	}
	defer c.Release()

	_, err = c.Exec(ctx, `CREATE TABLE IF NOT EXISTS approval_user_settings (
		user_id       TEXT PRIMARY KEY,
		default_tab   TEXT NOT NULL DEFAULT 'pending',
		sort          TEXT NOT NULL DEFAULT 'newest',
		notify_pending BOOLEAN NOT NULL DEFAULT TRUE,
		notify_result  BOOLEAN NOT NULL DEFAULT TRUE,
		updated_at    TIMESTAMPTZ NOT NULL DEFAULT NOW()
	)`)
	return err
}

// GetApprovalUserSettings returns the caller's approval settings, or nil when no row exists.
func (s *Store) GetApprovalUserSettings(ctx context.Context, userID string) (*domain.SettingsRow, error) {
	c, err := s.conn(ctx)
	if err != nil {
		return nil, err
	}
	defer c.Release()

	st := &domain.SettingsRow{}
	err = c.QueryRow(ctx,
		`SELECT user_id, default_tab, sort, notify_pending, notify_result, updated_at
		 FROM approval_user_settings WHERE user_id = $1`, userID).
		Scan(&st.UserID, &st.DefaultTab, &st.Sort, &st.NotifyPending, &st.NotifyResult, &st.UpdatedAt)
	if err == pgx.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return st, nil
}

// UpsertApprovalUserSettings inserts or updates the caller's approval settings.
func (s *Store) UpsertApprovalUserSettings(ctx context.Context, st *domain.SettingsRow) error {
	c, err := s.conn(ctx)
	if err != nil {
		return err
	}
	defer c.Release()

	_, err = c.Exec(ctx,
		`INSERT INTO approval_user_settings (user_id, default_tab, sort, notify_pending, notify_result, updated_at)
		 VALUES ($1, $2, $3, $4, $5, NOW())
		 ON CONFLICT (user_id) DO UPDATE SET
		   default_tab = EXCLUDED.default_tab, sort = EXCLUDED.sort,
		   notify_pending = EXCLUDED.notify_pending, notify_result = EXCLUDED.notify_result,
		   updated_at = NOW()`,
		st.UserID, st.DefaultTab, st.Sort, st.NotifyPending, st.NotifyResult)
	return err
}
