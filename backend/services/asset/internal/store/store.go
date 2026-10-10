package store

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Store provides database access for the Asset Service domain.
type Store struct {
	pool *pgxpool.Pool
}

// New creates a Store backed by the given connection pool.
func New(pool *pgxpool.Pool) *Store {
	return &Store{pool: pool}
}

// DB returns the underlying connection pool for cross-service queries.
func (s *Store) DB() *pgxpool.Pool {
	return s.pool
}

// Asset represents a single asset instance.
type Asset struct {
	ID                 string
	Name               string
	TypeID             string
	TypeName           string
	WorkspaceID        string
	State              string
	CustomFields       json.RawMessage
	AssignedTo         *string
	AssignedToUsername string
	// AssignedToName is the holder's display name, empty when the holder is not
	// a member of the asset's workspace.
	AssignedToName string
	// TypeOAID is the OA of the asset's type, which is what the asset is
	// authorized on. It comes from the type, not from a column on the asset.
	TypeOAID  string
	CreatedBy string
	Deleted   bool
	CreatedAt time.Time
	UpdatedAt time.Time
}

// ============================================
// Asset Type Queries
// ============================================

// ============================================
// Asset Instance Queries
// ============================================

// CreateAsset inserts a new asset instance.
func (s *Store) CreateAsset(ctx context.Context, a *Asset) error {
	if a.ID == "" {
		a.ID = uuid.New().String()
	}
	a.CreatedAt = time.Now()
	a.UpdatedAt = a.CreatedAt

	_, err := s.pool.Exec(ctx,
		`INSERT INTO assets (id, name, type_id, workspace_id, state, custom_fields, created_by, created_at, updated_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)`,
		a.ID, a.Name, a.TypeID, a.WorkspaceID, a.State,
		a.CustomFields, a.CreatedBy, a.CreatedAt, a.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("inserting asset: %w", err)
	}
	return nil
}

// assetSelect reads an asset with its type and, when the holder is a member of
// the asset's own workspace, their name.
const assetSelect = `SELECT a.id, a.name, a.type_id, t.name, a.workspace_id, a.state,
		        a.custom_fields, a.assigned_to, ` + "%s" + `, ` + "%s" + `,
		        COALESCE(t.ngac_oa_id, ''), a.created_by, a.deleted, a.created_at, a.updated_at`

var assetFrom = " FROM assets a JOIN asset_types t ON a.type_id = t.id" + memberJoin("a.assigned_to", "a.workspace_id", "hu", "htu")

func assetQuery(tail string) string {
	return fmt.Sprintf(assetSelect, personLogin("hu", "htu"), personName("hu", "htu")) + assetFrom + " " + tail
}

type rowScanner interface{ Scan(dest ...any) error }

func scanAsset(row rowScanner) (*Asset, error) {
	a := &Asset{}
	var assignedTo, login *string
	if err := row.Scan(
		&a.ID, &a.Name, &a.TypeID, &a.TypeName, &a.WorkspaceID, &a.State,
		&a.CustomFields, &assignedTo, &login, &a.AssignedToName,
		&a.TypeOAID, &a.CreatedBy, &a.Deleted, &a.CreatedAt, &a.UpdatedAt,
	); err != nil {
		return nil, err
	}
	a.AssignedTo = assignedTo
	if login != nil {
		a.AssignedToUsername = *login
	}
	return a, nil
}

// GetAsset retrieves a single asset by ID with its type name and holder's name.
func (s *Store) GetAsset(ctx context.Context, assetID string) (*Asset, error) {
	a, err := scanAsset(s.pool.QueryRow(ctx, assetQuery("WHERE a.id = $1"), assetID))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, fmt.Errorf("getting asset: %w", ErrNotFound)
		}
		return nil, fmt.Errorf("getting asset: %w", err)
	}
	return a, nil
}

// ListAssetsFilter holds optional filters for listing assets.
type ListAssetsFilter struct {
	WorkspaceID string
	TypeID      string
	State       string
	AssignedTo  string
	// Search matches, as plain text anywhere in it, the asset's name or its
	// holder's name.
	Search string
	Limit  int32
	Offset int32

	// VisibleTypeIDs, when non-nil, restricts the result — rows and total — to
	// assets of these types. An empty non-nil slice matches nothing. The gRPC
	// layer sets it to the types the caller may read; the store itself makes
	// no authorization decision.
	VisibleTypeIDs []string
}

// ListAssets returns filtered assets with total count, most recently changed first.
func (s *Store) ListAssets(ctx context.Context, f ListAssetsFilter) ([]*Asset, int32, error) {
	baseWhere := "WHERE a.workspace_id = $1 AND a.deleted = FALSE"
	args := []any{f.WorkspaceID}
	argIdx := 2

	if f.VisibleTypeIDs != nil {
		baseWhere += fmt.Sprintf(" AND a.type_id = ANY($%d)", argIdx)
		args = append(args, f.VisibleTypeIDs)
		argIdx++
	}

	if f.TypeID != "" {
		baseWhere += fmt.Sprintf(" AND a.type_id = $%d", argIdx)
		args = append(args, f.TypeID)
		argIdx++
	}
	if f.State != "" {
		baseWhere += fmt.Sprintf(" AND a.state = $%d", argIdx)
		args = append(args, f.State)
		argIdx++
	}
	if f.AssignedTo != "" {
		baseWhere += fmt.Sprintf(" AND a.assigned_to = $%d", argIdx)
		args = append(args, f.AssignedTo)
		argIdx++
	}
	if strings.TrimSpace(f.Search) != "" {
		baseWhere += fmt.Sprintf(" AND (a.name ILIKE $%d ESCAPE '\\' OR %s ILIKE $%d ESCAPE '\\')", argIdx, personName("hu", "htu"), argIdx)
		args = append(args, containsPattern(f.Search))
		argIdx++
	}

	var total int32
	err := s.pool.QueryRow(ctx, "SELECT COUNT(*)"+assetFrom+" "+baseWhere, args...).Scan(&total)
	if err != nil {
		return nil, 0, fmt.Errorf("counting assets: %w", err)
	}

	limit := f.Limit
	if limit <= 0 || limit > 100 {
		limit = 25
	}

	query := assetQuery(fmt.Sprintf("%s ORDER BY a.updated_at DESC, a.id LIMIT $%d OFFSET $%d", baseWhere, argIdx, argIdx+1))
	args = append(args, limit, f.Offset)

	rows, err := s.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("listing assets: %w", err)
	}
	defer rows.Close()

	var assets []*Asset
	for rows.Next() {
		a, err := scanAsset(rows)
		if err != nil {
			return nil, 0, fmt.Errorf("scanning asset row: %w", err)
		}
		assets = append(assets, a)
	}
	return assets, total, rows.Err()
}

// UpdateAsset updates mutable fields of an asset.
func (s *Store) UpdateAsset(ctx context.Context, assetID, name string, customFields json.RawMessage) error {
	tag, err := s.pool.Exec(ctx,
		`UPDATE assets SET name = COALESCE(NULLIF($1, ''), name), custom_fields = $2, updated_at = NOW()
		 WHERE id = $3 AND deleted = FALSE`,
		name, customFields, assetID,
	)
	if err != nil {
		return fmt.Errorf("updating asset: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("asset not found or deleted: %s", assetID)
	}
	return nil
}

// SoftDeleteAsset marks an asset as deleted.
func (s *Store) SoftDeleteAsset(ctx context.Context, assetID string) error {
	tag, err := s.pool.Exec(ctx,
		`UPDATE assets SET deleted = TRUE, updated_at = NOW() WHERE id = $1 AND deleted = FALSE`,
		assetID,
	)
	if err != nil {
		return fmt.Errorf("deleting asset: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("asset not found or already deleted: %s", assetID)
	}
	return nil
}

// ============================================
// Transition Queries
// ============================================

// ============================================
// Asset Request Queries
// ============================================

// HasExistingAssets checks if any non-deleted assets exist for a given type.
func (s *Store) HasExistingAssets(ctx context.Context, typeID string) (bool, error) {
	var count int
	err := s.pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM assets WHERE type_id = $1 AND deleted = FALSE`, typeID,
	).Scan(&count)
	if err != nil {
		return false, fmt.Errorf("checking existing assets: %w", err)
	}
	return count > 0, nil
}

// IsAssetAssigned checks if an asset is currently assigned to someone.
func (s *Store) IsAssetAssigned(ctx context.Context, assetID string) (bool, error) {
	var assigned bool
	err := s.pool.QueryRow(ctx,
		`SELECT assigned_to IS NOT NULL FROM assets WHERE id = $1 AND deleted = FALSE`, assetID,
	).Scan(&assigned)
	if err != nil {
		if err == pgx.ErrNoRows {
			return false, fmt.Errorf("asset not found: %s", assetID)
		}
		return false, fmt.Errorf("checking asset assignment: %w", err)
	}
	return assigned, nil
}
