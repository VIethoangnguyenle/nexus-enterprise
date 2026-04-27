package store

import (
	"context"
	"encoding/json"
	"fmt"
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
	CreatedAt    time.Time
	UpdatedAt    time.Time
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
	AssignedToUsername  string
	NgacNodeID         string
	CreatedBy          string
	Deleted            bool
	CreatedAt          time.Time
	UpdatedAt          time.Time
}

// TransitionRecord records a lifecycle state change.
type TransitionRecord struct {
	ID        string
	AssetID   string
	FromState string
	ToState   string
	Action    string
	ActorID   string
	ActorName string
	Comment   string
	CreatedAt time.Time
}

// AssetRequest represents a request to obtain an asset.
type AssetRequest struct {
	ID              string
	TypeID          string
	TypeName        string
	WorkspaceID     string
	RequesterID     string
	RequesterName   string
	Status          string
	Justification   string
	Quantity        int32
	AssignedAssetID *string
	ApproverID      *string
	ApproverName    string
	ApproverComment string
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

// ============================================
// Asset Type Queries
// ============================================

// CreateType inserts a new asset type and returns it.
func (s *Store) CreateType(ctx context.Context, at *AssetType) error {
	at.ID = uuid.New().String()
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
		        (SELECT COUNT(*) FROM assets a WHERE a.type_id = t.id AND a.deleted = FALSE)
		 FROM asset_types t WHERE t.id = $1`, typeID,
	).Scan(
		&at.ID, &at.Name, &at.Description, &at.Category, &at.WorkspaceID,
		&at.FieldsSchema, &at.Lifecycle, &at.NgacOAID, &at.CreatedAt, &at.UpdatedAt,
		&at.AssetCount,
	)
	if err != nil {
		return nil, fmt.Errorf("getting asset type: %w", err)
	}
	return at, nil
}

// ListTypes returns all asset types for a workspace.
func (s *Store) ListTypes(ctx context.Context, workspaceID string) ([]*AssetType, error) {
	rows, err := s.pool.Query(ctx,
		`SELECT t.id, t.name, t.description, t.category, t.workspace_id,
		        t.fields_schema, t.lifecycle, COALESCE(t.ngac_oa_id, ''), t.created_at, t.updated_at,
		        (SELECT COUNT(*) FROM assets a WHERE a.type_id = t.id AND a.deleted = FALSE)
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
			&at.AssetCount,
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

// ============================================
// Asset Instance Queries
// ============================================

// CreateAsset inserts a new asset instance.
func (s *Store) CreateAsset(ctx context.Context, a *Asset) error {
	a.ID = uuid.New().String()
	a.CreatedAt = time.Now()
	a.UpdatedAt = a.CreatedAt

	_, err := s.pool.Exec(ctx,
		`INSERT INTO assets (id, name, type_id, workspace_id, state, custom_fields, ngac_node_id, created_by, created_at, updated_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)`,
		a.ID, a.Name, a.TypeID, a.WorkspaceID, a.State,
		a.CustomFields, a.NgacNodeID, a.CreatedBy, a.CreatedAt, a.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("inserting asset: %w", err)
	}
	return nil
}

// GetAsset retrieves a single asset by ID with its type name and assigned username.
func (s *Store) GetAsset(ctx context.Context, assetID string) (*Asset, error) {
	a := &Asset{}
	var assignedTo, assignedUsername *string
	err := s.pool.QueryRow(ctx,
		`SELECT a.id, a.name, a.type_id, t.name, a.workspace_id, a.state,
		        a.custom_fields, a.assigned_to, u.username,
		        COALESCE(a.ngac_node_id, ''), a.created_by, a.deleted, a.created_at, a.updated_at
		 FROM assets a
		 JOIN asset_types t ON a.type_id = t.id
		 LEFT JOIN users u ON a.assigned_to = u.id
		 WHERE a.id = $1`, assetID,
	).Scan(
		&a.ID, &a.Name, &a.TypeID, &a.TypeName, &a.WorkspaceID, &a.State,
		&a.CustomFields, &assignedTo, &assignedUsername,
		&a.NgacNodeID, &a.CreatedBy, &a.Deleted, &a.CreatedAt, &a.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("getting asset: %w", err)
	}
	a.AssignedTo = assignedTo
	if assignedUsername != nil {
		a.AssignedToUsername = *assignedUsername
	}
	return a, nil
}

// ListAssetsFilter holds optional filters for listing assets.
type ListAssetsFilter struct {
	WorkspaceID string
	TypeID      string
	State       string
	AssignedTo  string
	Limit       int32
	Offset      int32
}

// ListAssets returns filtered assets with total count.
func (s *Store) ListAssets(ctx context.Context, f ListAssetsFilter) ([]*Asset, int32, error) {
	baseWhere := "WHERE a.workspace_id = $1 AND a.deleted = FALSE"
	args := []any{f.WorkspaceID}
	argIdx := 2

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

	// Count query
	var total int32
	err := s.pool.QueryRow(ctx,
		fmt.Sprintf("SELECT COUNT(*) FROM assets a %s", baseWhere), args...,
	).Scan(&total)
	if err != nil {
		return nil, 0, fmt.Errorf("counting assets: %w", err)
	}

	// Pagination defaults
	limit := f.Limit
	if limit <= 0 || limit > 100 {
		limit = 25
	}

	query := fmt.Sprintf(
		`SELECT a.id, a.name, a.type_id, t.name, a.workspace_id, a.state,
		        a.custom_fields, a.assigned_to, u.username,
		        COALESCE(a.ngac_node_id, ''), a.created_by, a.deleted, a.created_at, a.updated_at
		 FROM assets a
		 JOIN asset_types t ON a.type_id = t.id
		 LEFT JOIN users u ON a.assigned_to = u.id
		 %s ORDER BY a.created_at DESC LIMIT $%d OFFSET $%d`,
		baseWhere, argIdx, argIdx+1,
	)
	args = append(args, limit, f.Offset)

	rows, err := s.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("listing assets: %w", err)
	}
	defer rows.Close()

	var assets []*Asset
	for rows.Next() {
		a := &Asset{}
		var assignedTo, assignedUsername *string
		if err := rows.Scan(
			&a.ID, &a.Name, &a.TypeID, &a.TypeName, &a.WorkspaceID, &a.State,
			&a.CustomFields, &assignedTo, &assignedUsername,
			&a.NgacNodeID, &a.CreatedBy, &a.Deleted, &a.CreatedAt, &a.UpdatedAt,
		); err != nil {
			return nil, 0, fmt.Errorf("scanning asset row: %w", err)
		}
		a.AssignedTo = assignedTo
		if assignedUsername != nil {
			a.AssignedToUsername = *assignedUsername
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

// UpdateAssetState changes the state of an asset and optionally the assigned_to field.
func (s *Store) UpdateAssetState(ctx context.Context, assetID, newState string, assignedTo *string) error {
	var err error
	if assignedTo != nil {
		_, err = s.pool.Exec(ctx,
			`UPDATE assets SET state = $1, assigned_to = $2, updated_at = NOW() WHERE id = $3`,
			newState, *assignedTo, assetID,
		)
	} else {
		_, err = s.pool.Exec(ctx,
			`UPDATE assets SET state = $1, updated_at = NOW() WHERE id = $2`,
			newState, assetID,
		)
	}
	if err != nil {
		return fmt.Errorf("updating asset state: %w", err)
	}
	return nil
}

// ClearAssignment removes the assigned_to from an asset.
func (s *Store) ClearAssignment(ctx context.Context, assetID string) error {
	_, err := s.pool.Exec(ctx,
		`UPDATE assets SET assigned_to = NULL, updated_at = NOW() WHERE id = $1`, assetID,
	)
	if err != nil {
		return fmt.Errorf("clearing asset assignment: %w", err)
	}
	return nil
}

// ============================================
// Transition Queries
// ============================================

// InsertTransition records a lifecycle state change.
func (s *Store) InsertTransition(ctx context.Context, tr *TransitionRecord) error {
	tr.ID = uuid.New().String()
	tr.CreatedAt = time.Now()

	_, err := s.pool.Exec(ctx,
		`INSERT INTO asset_transitions (id, asset_id, from_state, to_state, action, actor_id, comment, created_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`,
		tr.ID, tr.AssetID, tr.FromState, tr.ToState, tr.Action, tr.ActorID, tr.Comment, tr.CreatedAt,
	)
	if err != nil {
		return fmt.Errorf("inserting transition: %w", err)
	}
	return nil
}

// GetAssetHistory returns all transitions for an asset ordered chronologically.
func (s *Store) GetAssetHistory(ctx context.Context, assetID string) ([]*TransitionRecord, error) {
	rows, err := s.pool.Query(ctx,
		`SELECT t.id, t.asset_id, t.from_state, t.to_state, t.action,
		        t.actor_id, COALESCE(u.username, ''), t.comment, t.created_at
		 FROM asset_transitions t
		 LEFT JOIN users u ON t.actor_id = u.id
		 WHERE t.asset_id = $1 ORDER BY t.created_at`, assetID,
	)
	if err != nil {
		return nil, fmt.Errorf("getting asset history: %w", err)
	}
	defer rows.Close()

	var records []*TransitionRecord
	for rows.Next() {
		tr := &TransitionRecord{}
		if err := rows.Scan(
			&tr.ID, &tr.AssetID, &tr.FromState, &tr.ToState, &tr.Action,
			&tr.ActorID, &tr.ActorName, &tr.Comment, &tr.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("scanning transition row: %w", err)
		}
		records = append(records, tr)
	}
	return records, rows.Err()
}

// ============================================
// Asset Request Queries
// ============================================

// CreateRequest inserts a new asset request.
func (s *Store) CreateRequest(ctx context.Context, req *AssetRequest) error {
	req.ID = uuid.New().String()
	req.CreatedAt = time.Now()
	req.UpdatedAt = req.CreatedAt

	_, err := s.pool.Exec(ctx,
		`INSERT INTO asset_requests (id, type_id, workspace_id, requester_id, status, justification, quantity, created_at, updated_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)`,
		req.ID, req.TypeID, req.WorkspaceID, req.RequesterID, req.Status,
		req.Justification, req.Quantity, req.CreatedAt, req.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("inserting asset request: %w", err)
	}
	return nil
}

// GetRequest retrieves a request by ID with requester/approver names and type name.
func (s *Store) GetRequest(ctx context.Context, requestID string) (*AssetRequest, error) {
	r := &AssetRequest{}
	var approverID, assignedAssetID *string
	err := s.pool.QueryRow(ctx,
		`SELECT r.id, r.type_id, t.name, r.workspace_id, r.requester_id, u.username,
		        r.status, r.justification, r.quantity, r.assigned_asset_id,
		        r.approver_id, COALESCE(au.username, ''), r.approver_comment, r.created_at, r.updated_at
		 FROM asset_requests r
		 JOIN asset_types t ON r.type_id = t.id
		 JOIN users u ON r.requester_id = u.id
		 LEFT JOIN users au ON r.approver_id = au.id
		 WHERE r.id = $1`, requestID,
	).Scan(
		&r.ID, &r.TypeID, &r.TypeName, &r.WorkspaceID, &r.RequesterID, &r.RequesterName,
		&r.Status, &r.Justification, &r.Quantity, &assignedAssetID,
		&approverID, &r.ApproverName, &r.ApproverComment, &r.CreatedAt, &r.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("getting asset request: %w", err)
	}
	r.ApproverID = approverID
	r.AssignedAssetID = assignedAssetID
	return r, nil
}

// UpdateRequestStatus updates the status and approver info of a request.
func (s *Store) UpdateRequestStatus(ctx context.Context, requestID, status, approverID, comment string) error {
	_, err := s.pool.Exec(ctx,
		`UPDATE asset_requests SET status = $1, approver_id = $2, approver_comment = $3, updated_at = NOW()
		 WHERE id = $4`,
		status, approverID, comment, requestID,
	)
	if err != nil {
		return fmt.Errorf("updating request status: %w", err)
	}
	return nil
}

// FulfillRequest marks a request as fulfilled with an assigned asset.
func (s *Store) FulfillRequest(ctx context.Context, requestID, assetID string) error {
	_, err := s.pool.Exec(ctx,
		`UPDATE asset_requests SET status = 'fulfilled', assigned_asset_id = $1, updated_at = NOW()
		 WHERE id = $2`,
		assetID, requestID,
	)
	if err != nil {
		return fmt.Errorf("fulfilling request: %w", err)
	}
	return nil
}

// ListRequestsFilter holds filters for listing requests.
type ListRequestsFilter struct {
	WorkspaceID string
	UserID      string
	Status      string
	MineOnly    bool
	Limit       int32
	Offset      int32
}

// ListRequests returns filtered requests.
func (s *Store) ListRequests(ctx context.Context, f ListRequestsFilter) ([]*AssetRequest, int32, error) {
	baseWhere := "WHERE r.workspace_id = $1"
	args := []any{f.WorkspaceID}
	argIdx := 2

	if f.MineOnly && f.UserID != "" {
		baseWhere += fmt.Sprintf(" AND r.requester_id = $%d", argIdx)
		args = append(args, f.UserID)
		argIdx++
	}
	if f.Status != "" {
		baseWhere += fmt.Sprintf(" AND r.status = $%d", argIdx)
		args = append(args, f.Status)
		argIdx++
	}

	var total int32
	err := s.pool.QueryRow(ctx,
		fmt.Sprintf("SELECT COUNT(*) FROM asset_requests r %s", baseWhere), args...,
	).Scan(&total)
	if err != nil {
		return nil, 0, fmt.Errorf("counting requests: %w", err)
	}

	limit := f.Limit
	if limit <= 0 || limit > 100 {
		limit = 25
	}

	query := fmt.Sprintf(
		`SELECT r.id, r.type_id, t.name, r.workspace_id, r.requester_id, u.username,
		        r.status, r.justification, r.quantity, r.assigned_asset_id,
		        r.approver_id, COALESCE(au.username, ''), r.approver_comment, r.created_at, r.updated_at
		 FROM asset_requests r
		 JOIN asset_types t ON r.type_id = t.id
		 JOIN users u ON r.requester_id = u.id
		 LEFT JOIN users au ON r.approver_id = au.id
		 %s ORDER BY r.created_at DESC LIMIT $%d OFFSET $%d`,
		baseWhere, argIdx, argIdx+1,
	)
	args = append(args, limit, f.Offset)

	rows, err := s.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("listing requests: %w", err)
	}
	defer rows.Close()

	var requests []*AssetRequest
	for rows.Next() {
		r := &AssetRequest{}
		var approverID, assignedAssetID *string
		if err := rows.Scan(
			&r.ID, &r.TypeID, &r.TypeName, &r.WorkspaceID, &r.RequesterID, &r.RequesterName,
			&r.Status, &r.Justification, &r.Quantity, &assignedAssetID,
			&approverID, &r.ApproverName, &r.ApproverComment, &r.CreatedAt, &r.UpdatedAt,
		); err != nil {
			return nil, 0, fmt.Errorf("scanning request row: %w", err)
		}
		r.ApproverID = approverID
		r.AssignedAssetID = assignedAssetID
		requests = append(requests, r)
	}
	return requests, total, rows.Err()
}

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
