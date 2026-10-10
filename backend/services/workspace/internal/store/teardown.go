package store

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
)

// The queries a workspace teardown needs. They are used only to undo a
// workspace that has just been created and must not stay (see
// domain.Service.DeleteWorkspace); nothing here is reachable from a REST route.

// WorkspaceCreator returns the user who created the workspace, and whether a
// row exists at all.
func (s *Store) WorkspaceCreator(ctx context.Context, id string) (string, bool, error) {
	var owner string
	err := s.db.QueryRow(ctx, "SELECT owner_id FROM workspaces WHERE id = $1", id).Scan(&owner)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", false, nil
	}
	if err != nil {
		return "", false, fmt.Errorf("workspace creator %s: %w", id, err)
	}
	return owner, true, nil
}

// OtherMembers counts the people listed in the workspace besides userID.
func (s *Store) OtherMembers(ctx context.Context, id, userID string) (int, error) {
	var n int
	err := s.db.QueryRow(ctx,
		"SELECT count(*) FROM tenant_users WHERE tenant_id = $1 AND user_id <> $2", id, userID).Scan(&n)
	if err != nil {
		return 0, fmt.Errorf("count other members of %s: %w", id, err)
	}
	return n, nil
}

// PurgeWorkspace removes the workspace row and everything that hangs off it:
// the tenant's approval schema and its registry row, the drive (files, root
// folder, quota), text documents, documents, assets, channels (with their
// messages and members), departments, invitations and membership rows. It runs
// in one transaction and removes nothing that is not there, so a second call, or
// a call on a workspace that was only half created, succeeds.
func (s *Store) PurgeWorkspace(ctx context.Context, id string) error {
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin purge: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var schema string
	err = tx.QueryRow(ctx, "SELECT schema_name FROM tenant_schemas WHERE tenant_id = $1", id).Scan(&schema)
	switch {
	case errors.Is(err, pgx.ErrNoRows):
	case err != nil:
		return fmt.Errorf("look up approval schema: %w", err)
	default:
		if _, err := tx.Exec(ctx, "DROP SCHEMA IF EXISTS "+pgx.Identifier{schema}.Sanitize()+" CASCADE"); err != nil {
			return fmt.Errorf("drop approval schema: %w", err)
		}
	}

	// Children before parents. Departments, invitations and the schema registry
	// row go with the workspace row (ON DELETE CASCADE).
	for _, q := range []string{
		"DELETE FROM text_documents WHERE workspace_id = $1",
		"DELETE FROM drive_items WHERE workspace_id = $1",
		"DELETE FROM drive_quotas WHERE workspace_id = $1",
		"DELETE FROM asset_requests WHERE workspace_id = $1",
		"DELETE FROM assets WHERE workspace_id = $1",
		"DELETE FROM asset_types WHERE workspace_id = $1",
		"DELETE FROM documents WHERE workspace_id = $1",
		"DELETE FROM channels WHERE workspace_id = $1",
		"DELETE FROM tenant_users WHERE tenant_id = $1",
		"DELETE FROM workspaces WHERE id = $1",
	} {
		if _, err := tx.Exec(ctx, q, id); err != nil {
			return fmt.Errorf("purge workspace %s: %w", id, err)
		}
	}
	return tx.Commit(ctx)
}
