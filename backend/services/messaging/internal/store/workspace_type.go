package store

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
)

// ErrWorkspaceNotFound: no workspace has the ID asked for.
var ErrWorkspaceNotFound = errors.New("workspace not found")

// WorkspaceType returns the workspace's type ("personal" or "organization"), or
// ErrWorkspaceNotFound.
func (s *Store) WorkspaceType(ctx context.Context, workspaceID string) (string, error) {
	var kind string
	err := s.db.QueryRow(ctx, `SELECT type FROM workspaces WHERE id = $1`, workspaceID).Scan(&kind)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", ErrWorkspaceNotFound
	}
	if err != nil {
		return "", fmt.Errorf("workspace type: %w", err)
	}
	return kind, nil
}
