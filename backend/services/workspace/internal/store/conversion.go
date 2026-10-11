package store

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
)

var (
	// ErrWorkspaceNotFound means no workspace has that ID.
	ErrWorkspaceNotFound = errors.New("workspace not found")
	// ErrNotPersonal means the workspace is not a personal one (it is already an
	// organization, or was made one).
	ErrNotPersonal = errors.New("workspace is not personal")
)

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

// Conversion is the record of a personal workspace becoming an organization.
type Conversion struct {
	ByName string
	At     time.Time
}

// ConvertToOrganization turns a personal workspace into an organization, once:
// the type changes only if it is still personal, in the same statement that
// records the user behind byNodeID and the time. Of two calls at once exactly one
// wins; the other gets ErrNotPersonal. ErrWorkspaceNotFound if there is no such
// workspace.
func (s *Store) ConvertToOrganization(ctx context.Context, wsID, byNodeID string) (*Conversion, error) {
	var c Conversion
	err := s.db.QueryRow(ctx, `
		WITH me AS (SELECT u.id, `+personName("u")+` AS name FROM users u WHERE u.ngac_node = $2)
		UPDATE workspaces w
		   SET type = 'organization', converted_by = (SELECT id FROM me), converted_at = NOW()
		 WHERE w.id = $1 AND w.type = 'personal'
		RETURNING w.converted_at, COALESCE((SELECT name FROM me), '')`, wsID, byNodeID,
	).Scan(&c.At, &c.ByName)
	if err == nil {
		return &c, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return nil, fmt.Errorf("convert workspace %s: %w", wsID, err)
	}
	// Nothing changed: tell a workspace that is not there from one that is not personal.
	var exists bool
	if err := s.db.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM workspaces WHERE id = $1)`, wsID).Scan(&exists); err != nil {
		return nil, fmt.Errorf("re-read workspace %s: %w", wsID, err)
	}
	if !exists {
		return nil, ErrWorkspaceNotFound
	}
	return nil, ErrNotPersonal
}
