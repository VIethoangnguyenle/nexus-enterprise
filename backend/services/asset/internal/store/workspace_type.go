package store

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
)

// WorkspaceType returns the workspace's type ("personal" or "organization").
// A workspace that does not exist is ErrNotFound; any other failure is the
// database's and is returned as it is.
func (s *Store) WorkspaceType(ctx context.Context, workspaceID string) (string, error) {
	var kind string
	err := s.pool.QueryRow(ctx, `SELECT type FROM workspaces WHERE id = $1`, workspaceID).Scan(&kind)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", fmt.Errorf("workspace type: %w", ErrNotFound)
	}
	if err != nil {
		return "", fmt.Errorf("workspace type: %w", err)
	}
	return kind, nil
}
