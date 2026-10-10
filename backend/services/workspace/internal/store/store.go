package store

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
)

// Store handles all database operations for the workspace service.
type Store struct {
	db *pgxpool.Pool
}

// New creates a workspace Store.
func New(db *pgxpool.Pool) *Store {
	return &Store{db: db}
}

// Insert persists a new workspace row.
func (s *Store) Insert(ctx context.Context, ws *Workspace) error {
	_, err := s.db.Exec(ctx,
		"INSERT INTO workspaces (id, name, description, owner_id, ngac_pc_id, documents_oa_id) VALUES ($1, $2, $3, $4, $5, NULLIF($6, ''))",
		ws.ID, ws.Name, ws.Desc, ws.OwnerID, ws.NGACPcID, ws.DocumentsOAID,
	)
	if err != nil {
		return fmt.Errorf("insert workspace: %w", err)
	}
	return nil
}

// GetByID returns a single workspace by its ID.
func (s *Store) GetByID(ctx context.Context, id string) (*Workspace, error) {
	var ws Workspace
	err := s.db.QueryRow(ctx,
		"SELECT id, name, COALESCE(description, ''), ngac_pc_id FROM workspaces WHERE id = $1", id,
	).Scan(&ws.ID, &ws.Name, &ws.Desc, &ws.NGACPcID)
	if err != nil {
		return nil, fmt.Errorf("get workspace %s: %w", id, err)
	}
	return &ws, nil
}

// UpdateDetails sets the name and/or description given (nil leaves a field
// alone) in one statement, so concurrent edits of different fields both land,
// and returns both as they are afterwards.
func (s *Store) UpdateDetails(ctx context.Context, id string, name, description *string) (string, string, error) {
	var n, d string
	err := s.db.QueryRow(ctx,
		`UPDATE workspaces SET name = COALESCE($2, name), description = COALESCE($3, description)
		  WHERE id = $1 RETURNING name, COALESCE(description, '')`, id, name, description).Scan(&n, &d)
	if err != nil {
		return "", "", fmt.Errorf("update workspace %s: %w", id, err)
	}
	return n, d, nil
}

// ListAll returns all workspaces ordered by creation time descending.
func (s *Store) ListAll(ctx context.Context) ([]*Workspace, error) {
	rows, err := s.db.Query(ctx,
		"SELECT id, name, ngac_pc_id FROM workspaces ORDER BY created_at DESC",
	)
	if err != nil {
		return nil, fmt.Errorf("list workspaces: %w", err)
	}
	defer rows.Close()

	var result []*Workspace
	for rows.Next() {
		var ws Workspace
		if err := rows.Scan(&ws.ID, &ws.Name, &ws.NGACPcID); err != nil {
			return nil, fmt.Errorf("scan workspace: %w", err)
		}
		result = append(result, &ws)
	}
	return result, nil
}

// WithOwnerLock runs fn inside a transaction holding a per-workspace advisory
// lock. Checks of who the owners are and the writes that follow take it, so two
// owner changes at once run one after the other and the second sees the first.
// The lock is released when fn returns, whether or not it succeeded.
func (s *Store) WithOwnerLock(ctx context.Context, wsID string, fn func(ctx context.Context) error) error {
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin owner lock: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if _, err := tx.Exec(ctx, "SELECT pg_advisory_xact_lock(hashtextextended($1, 0))", "workspace-owners:"+wsID); err != nil {
		return fmt.Errorf("take owner lock: %w", err)
	}
	if err := fn(ctx); err != nil {
		return err
	}
	return tx.Commit(ctx)
}
