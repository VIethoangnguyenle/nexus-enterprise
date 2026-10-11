package store

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"

	"ngac-platform/services/approval/internal/domain"
)

// WorkspaceType returns the type ("personal" or "organization") of the
// workspace a tenant is. It reads the shared workspaces table on the pool, not
// a tenant connection: a workspace that has no tenant schema (a personal one)
// still has to be answered. A workspace that does not exist is ErrNotFound.
func (s *Store) WorkspaceType(ctx context.Context, workspaceID string) (string, error) {
	var kind string
	err := s.db.QueryRow(ctx, `SELECT type FROM workspaces WHERE id = $1`, workspaceID).Scan(&kind)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", domain.ErrNotFound
	}
	if err != nil {
		return "", fmt.Errorf("workspace type: %w", err)
	}
	return kind, nil
}
