package store

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"ngac-platform/services/approval/internal/domain"
)

// execer is what the tenant-table DDL runs on: the pool, or the transaction
// that is provisioning the schema.
type execer interface {
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
}

// ensureTenantTables creates, in the given tenant schema, the tables the
// approval service owns beyond what provision_tenant_schema() makes: today
// approval_user_settings, each person's approval settings. It is part of
// provisioning (EnsureSchema, ProvisionSchema, and EnsureTenantTables at start
// for schemas made before the table existed), never of a settings request.
// Idempotent.
func ensureTenantTables(ctx context.Context, q execer, schema string) error {
	if schema == "" {
		return fmt.Errorf("ensure tenant tables: no schema")
	}
	if _, err := q.Exec(ctx, fmt.Sprintf(`CREATE TABLE IF NOT EXISTS %s.approval_user_settings (
		user_id        TEXT PRIMARY KEY,
		default_tab    TEXT NOT NULL DEFAULT 'pending',
		sort           TEXT NOT NULL DEFAULT 'newest',
		notify_pending BOOLEAN NOT NULL DEFAULT TRUE,
		notify_result  BOOLEAN NOT NULL DEFAULT TRUE,
		updated_at     TIMESTAMPTZ NOT NULL DEFAULT NOW()
	)`, pgx.Identifier{schema}.Sanitize())); err != nil {
		return fmt.Errorf("ensure approval_user_settings in %s: %w", schema, err)
	}
	return nil
}

// EnsureTenantTables brings every active tenant schema up to date with the
// tables ensureTenantTables makes. The service runs it once when it starts, so
// a schema provisioned before a table existed gets it before any request needs
// it. Running it again changes nothing.
func (s *Store) EnsureTenantTables(ctx context.Context) error {
	rows, err := s.db.Query(ctx, `
		SELECT schema_name FROM tenant_schemas
		WHERE status = 'active' AND schema_name IN (SELECT nspname FROM pg_namespace)
		ORDER BY schema_name`)
	if err != nil {
		return fmt.Errorf("list tenant schemas: %w", err)
	}
	schemas, err := pgx.CollectRows(rows, pgx.RowTo[string])
	if err != nil {
		return fmt.Errorf("list tenant schemas: %w", err)
	}
	for _, schema := range schemas {
		if err := ensureTenantTables(ctx, s.db, schema); err != nil {
			return err
		}
	}
	return nil
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
