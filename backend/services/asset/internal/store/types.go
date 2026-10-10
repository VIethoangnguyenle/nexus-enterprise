package store

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// AssetType represents a user-defined asset type with custom fields schema and lifecycle.
type AssetType struct {
	ID           string
	Name         string
	Description  string
	Category     string
	WorkspaceID  string
	FieldsSchema json.RawMessage
	Lifecycle    json.RawMessage
	NgacOAID     string
	AssetCount   int32
	// AvailableCount is the assets of the type that can be handed out now.
	AvailableCount int32
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

// CreateType inserts a new asset type and returns it.
func (s *Store) CreateType(ctx context.Context, at *AssetType) error {
	// The caller sets the ID when the type's OA has to be named by it before
	// the row exists.
	if at.ID == "" {
		at.ID = uuid.New().String()
	}
	at.CreatedAt = time.Now()
	at.UpdatedAt = at.CreatedAt

	_, err := s.pool.Exec(ctx,
		`INSERT INTO asset_types (id, name, description, category, workspace_id, fields_schema, lifecycle, ngac_oa_id, created_at, updated_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)`,
		at.ID, at.Name, at.Description, at.Category, at.WorkspaceID,
		at.FieldsSchema, at.Lifecycle, at.NgacOAID, at.CreatedAt, at.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("inserting asset type: %w", err)
	}
	return nil
}

// GetType retrieves a single asset type by ID, including asset count.
func (s *Store) GetType(ctx context.Context, typeID string) (*AssetType, error) {
	at := &AssetType{}
	err := s.pool.QueryRow(ctx,
		`SELECT t.id, t.name, t.description, t.category, t.workspace_id,
		        t.fields_schema, t.lifecycle, COALESCE(t.ngac_oa_id, ''), t.created_at, t.updated_at,
		        (SELECT COUNT(*) FROM assets a WHERE a.type_id = t.id AND a.deleted = FALSE),
		        (SELECT COUNT(*) FROM assets a WHERE a.type_id = t.id AND a.deleted = FALSE AND a.state = 'available')
		 FROM asset_types t WHERE t.id = $1`, typeID,
	).Scan(
		&at.ID, &at.Name, &at.Description, &at.Category, &at.WorkspaceID,
		&at.FieldsSchema, &at.Lifecycle, &at.NgacOAID, &at.CreatedAt, &at.UpdatedAt,
		&at.AssetCount, &at.AvailableCount,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, fmt.Errorf("getting asset type: %w", ErrNotFound)
		}
		return nil, fmt.Errorf("getting asset type: %w", err)
	}
	return at, nil
}

// ListTypes returns all asset types for a workspace.
func (s *Store) ListTypes(ctx context.Context, workspaceID string) ([]*AssetType, error) {
	rows, err := s.pool.Query(ctx,
		`SELECT t.id, t.name, t.description, t.category, t.workspace_id,
		        t.fields_schema, t.lifecycle, COALESCE(t.ngac_oa_id, ''), t.created_at, t.updated_at,
		        (SELECT COUNT(*) FROM assets a WHERE a.type_id = t.id AND a.deleted = FALSE),
		        (SELECT COUNT(*) FROM assets a WHERE a.type_id = t.id AND a.deleted = FALSE AND a.state = 'available')
		 FROM asset_types t WHERE t.workspace_id = $1 ORDER BY t.category, t.name`, workspaceID,
	)
	if err != nil {
		return nil, fmt.Errorf("listing asset types: %w", err)
	}
	defer rows.Close()

	var types []*AssetType
	for rows.Next() {
		at := &AssetType{}
		if err := rows.Scan(
			&at.ID, &at.Name, &at.Description, &at.Category, &at.WorkspaceID,
			&at.FieldsSchema, &at.Lifecycle, &at.NgacOAID, &at.CreatedAt, &at.UpdatedAt,
			&at.AssetCount, &at.AvailableCount,
		); err != nil {
			return nil, fmt.Errorf("scanning asset type row: %w", err)
		}
		types = append(types, at)
	}
	return types, rows.Err()
}

// UpdateTypeSchema updates the fields_schema of an existing asset type.
func (s *Store) UpdateTypeSchema(ctx context.Context, typeID string, schema json.RawMessage) error {
	tag, err := s.pool.Exec(ctx,
		`UPDATE asset_types SET fields_schema = $1, updated_at = NOW() WHERE id = $2`,
		schema, typeID,
	)
	if err != nil {
		return fmt.Errorf("updating type schema: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("asset type not found: %s", typeID)
	}
	return nil
}
