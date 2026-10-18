package store

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
)

// UserNodeOf returns the NGAC node of a user, or "" when the user is unknown or
// has none. Drive keeps a folder's owner as the node and a file's as the user
// id, so asking for one person's items needs both.
func (s *Store) UserNodeOf(ctx context.Context, userID string) (string, error) {
	var node string
	err := s.db.QueryRow(ctx, `SELECT COALESCE(ngac_node, '') FROM users WHERE id = $1`, userID).Scan(&node)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", nil
	}
	if err != nil {
		return "", fmt.Errorf("user node: %w", err)
	}
	return node, nil
}
