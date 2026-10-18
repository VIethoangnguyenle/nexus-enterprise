package store

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
)

// ErrWorkspaceNotFound means no workspace has that ID.
var ErrWorkspaceNotFound = errors.New("workspace not found")

// ConversionRecord says who converted a workspace and when, by name. ByName is
// empty and At nil for a workspace that was never converted.
type ConversionRecord struct {
	ByName string
	At     *time.Time
}

// personName is what a person is called: their display name, else their username.
func personName(alias string) string {
	return "COALESCE(NULLIF(" + alias + ".display_name, ''), " + alias + ".username, '')"
}

// ConversionRecord returns who converted the workspace and when.
// ErrWorkspaceNotFound if there is no such workspace.
func (s *Store) ConversionRecord(ctx context.Context, wsID string) (*ConversionRecord, error) {
	var r ConversionRecord
	err := s.db.QueryRow(ctx, `
		SELECT `+personName("c")+`, w.converted_at
		  FROM workspaces w
		  LEFT JOIN users c ON c.id = w.converted_by
		 WHERE w.id = $1`, wsID,
	).Scan(&r.ByName, &r.At)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrWorkspaceNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("workspace conversion record %s: %w", wsID, err)
	}
	return &r, nil
}
