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
		"INSERT INTO workspaces (id, name, description, owner_id, ngac_pc_id) VALUES ($1, $2, $3, $4, $5)",
		ws.ID, ws.Name, ws.Desc, ws.OwnerID, ws.NGACPcID,
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
		"SELECT id, name, ngac_pc_id FROM workspaces WHERE id = $1", id,
	).Scan(&ws.ID, &ws.Name, &ws.NGACPcID)
	if err != nil {
		return nil, fmt.Errorf("get workspace %s: %w", id, err)
	}
	return &ws, nil
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
