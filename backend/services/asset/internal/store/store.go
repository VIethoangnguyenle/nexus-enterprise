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
	"github.com/jackc/pgx/v5/pgconn"
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
	// AvailableCount is the assets of the type that can be handed out now.
	AvailableCount int32
	CreatedAt      time.Time
	UpdatedAt      time.Time
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

// TransitionRecord records a lifecycle state change.
type TransitionRecord struct {
	ID        string
	AssetID   string
	FromState string
	ToState   string
	Action    string
	ActorID   string
	ActorName string
	// SubjectUserID is the person the step concerned: the new holder on a
	// hand-over, the previous holder on a return. Empty for steps about no one.
	SubjectUserID string
	SubjectName   string
	Comment       string
	CreatedAt     time.Time
	// ExpectHolder, when set, is who the caller read as the holder; the step is
	// refused if somebody else holds the asset by the time it is locked.
	ExpectHolder string
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
	// Urgency is "low", "normal", "high" or "urgent".
	Urgency           string
	AssignedAssetName string
	CreatedAt         time.Time
	UpdatedAt         time.Time
}

// ============================================
// Asset Type Queries
// ============================================

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

// clearsHolder reports whether an asset that moves into this state is no
// longer held by anyone. Maintenance and the like leave the holder in place —
// it is still their laptop — while stock, retired and disposed assets have none.
func clearsHolder(toState string) bool {
	switch toState {
	case "available", "requested", "retired", "disposed":
		return true
	}
	return false
}

// ApplyTransition changes an asset's state and records the transition in its
// history, in one database transaction. If either write fails neither takes
// effect: a state change with no history row would be an unaudited change, and
// the history is how the UI says who did what and when.
//
// assignedTo, when non-nil, also sets the asset's holder (and is the step's
// subject). A move into a state that holds no one clears the holder, and the
// step then names the person it was taken from.
func (s *Store) ApplyTransition(ctx context.Context, tr *TransitionRecord, assignedTo *string) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin transition: %w", err)
	}
	defer tx.Rollback(ctx) // no-op after Commit

	var holder *string
	var state string
	var deleted bool
	if err := tx.QueryRow(ctx, `SELECT state, deleted, assigned_to FROM assets WHERE id = $1 FOR UPDATE`, tr.AssetID).Scan(&state, &deleted, &holder); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrNotFound
		}
		return fmt.Errorf("locking asset: %w", err)
	}
	// The step was chosen from what the caller read; under the lock, it must
	// still be true.
	if deleted {
		return ErrNotFound
	}
	if state != tr.FromState {
		return ErrStateChanged
	}
	if tr.ExpectHolder != "" && (holder == nil || *holder != tr.ExpectHolder) {
		return ErrStateChanged
	}
	subject := tr.SubjectUserID
	switch {
	case assignedTo != nil:
		holder = assignedTo
		if subject == "" {
			subject = *assignedTo
		}
	case clearsHolder(tr.ToState):
		if holder != nil && subject == "" {
			subject = *holder
		}
		holder = nil
	}
	tr.SubjectUserID = subject

	if _, err := tx.Exec(ctx,
		`UPDATE assets SET state = $1, assigned_to = $2, updated_at = NOW() WHERE id = $3`,
		tr.ToState, holder, tr.AssetID); err != nil {
		return fmt.Errorf("updating asset state: %w", err)
	}
	if err := insertTransition(ctx, tx, tr); err != nil {
		return err
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit transition: %w", err)
	}
	return nil
}

type execer interface {
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
}

func insertTransition(ctx context.Context, q execer, tr *TransitionRecord) error {
	tr.ID = uuid.New().String()
	tr.CreatedAt = time.Now()
	if _, err := q.Exec(ctx,
		`INSERT INTO asset_transitions (id, asset_id, from_state, to_state, action, actor_id, subject_user_id, comment, created_at)
		 VALUES ($1, $2, $3, $4, $5, $6, NULLIF($7, ''), $8, $9)`,
		tr.ID, tr.AssetID, tr.FromState, tr.ToState, tr.Action, tr.ActorID, tr.SubjectUserID, tr.Comment, tr.CreatedAt,
	); err != nil {
		return fmt.Errorf("inserting transition: %w", err)
	}
	return nil
}

// InsertTransition records a lifecycle step without changing the asset.
func (s *Store) InsertTransition(ctx context.Context, tr *TransitionRecord) error {
	return insertTransition(ctx, s.pool, tr)
}

// GetAssetHistory returns all transitions for an asset ordered chronologically.
// The people in it are named only if they belong to the asset's workspace.
func (s *Store) GetAssetHistory(ctx context.Context, assetID string) ([]*TransitionRecord, error) {
	rows, err := s.pool.Query(ctx,
		`SELECT t.id, t.asset_id, t.from_state, t.to_state, t.action,
		        t.actor_id, `+personName("au", "atu")+`, COALESCE(t.subject_user_id, ''), `+personName("su", "stu")+`,
		        t.comment, t.created_at
		 FROM asset_transitions t
		 JOIN assets a ON a.id = t.asset_id`+
			memberJoin("t.actor_id", "a.workspace_id", "au", "atu")+
			memberJoin("t.subject_user_id", "a.workspace_id", "su", "stu")+`
		 WHERE t.asset_id = $1 ORDER BY t.created_at, t.id`, assetID,
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
			&tr.ActorID, &tr.ActorName, &tr.SubjectUserID, &tr.SubjectName, &tr.Comment, &tr.CreatedAt,
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
	if req.Urgency == "" {
		req.Urgency = "normal"
	}

	_, err := s.pool.Exec(ctx,
		`INSERT INTO asset_requests (id, type_id, workspace_id, requester_id, status, justification, quantity, urgency, created_at, updated_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)`,
		req.ID, req.TypeID, req.WorkspaceID, req.RequesterID, req.Status,
		req.Justification, req.Quantity, req.Urgency, req.CreatedAt, req.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("inserting asset request: %w", err)
	}
	return nil
}

// requestSelect reads a request with its type, the people on it (named only if
// they belong to the request's workspace) and the asset it was given.
var requestSelect = `SELECT r.id, r.type_id, t.name, r.workspace_id, r.requester_id, ` + personName("ru", "rtu") + `,
		        r.status, r.justification, r.quantity, r.assigned_asset_id,
		        r.approver_id, ` + personName("au", "atu") + `, r.approver_comment, r.urgency,
		        COALESCE(aa.name, ''), r.created_at, r.updated_at
		 FROM asset_requests r
		 JOIN asset_types t ON r.type_id = t.id` +
	memberJoin("r.requester_id", "r.workspace_id", "ru", "rtu") +
	memberJoin("r.approver_id", "r.workspace_id", "au", "atu") +
	` LEFT JOIN assets aa ON aa.id = r.assigned_asset_id`

func scanRequest(row rowScanner) (*AssetRequest, error) {
	r := &AssetRequest{}
	var approverID, assignedAssetID *string
	if err := row.Scan(
		&r.ID, &r.TypeID, &r.TypeName, &r.WorkspaceID, &r.RequesterID, &r.RequesterName,
		&r.Status, &r.Justification, &r.Quantity, &assignedAssetID,
		&approverID, &r.ApproverName, &r.ApproverComment, &r.Urgency,
		&r.AssignedAssetName, &r.CreatedAt, &r.UpdatedAt,
	); err != nil {
		return nil, err
	}
	r.ApproverID = approverID
	r.AssignedAssetID = assignedAssetID
	return r, nil
}

// GetRequest retrieves a request by ID with requester/approver names and type name.
func (s *Store) GetRequest(ctx context.Context, requestID string) (*AssetRequest, error) {
	r, err := scanRequest(s.pool.QueryRow(ctx, requestSelect+` WHERE r.id = $1`, requestID))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, fmt.Errorf("getting asset request: %w", ErrNotFound)
		}
		return nil, fmt.Errorf("getting asset request: %w", err)
	}
	return r, nil
}

// UpdateRequestStatus updates the status and approver info of a request.
func (s *Store) UpdateRequestStatus(ctx context.Context, requestID, status, approverID, comment string) error {
	tag, err := s.pool.Exec(ctx,
		`UPDATE asset_requests SET status = $1, approver_id = $2, approver_comment = $3, updated_at = NOW()
		 WHERE id = $4 AND status = 'pending'`,
		status, approverID, comment, requestID,
	)
	if err != nil {
		return fmt.Errorf("updating request status: %w", err)
	}
	// Decided only while still pending: a request someone fulfilled, rejected or
	// approved a moment ago is not overwritten.
	if tag.RowsAffected() == 0 {
		return ErrRequestNotOpen
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
	// Status is one status, or several separated by commas ("approved,fulfilled").
	Status   string
	MineOnly bool
	Limit    int32
	Offset   int32

	// Visibility, when non-nil, restricts the result — rows and total — to
	// requests the caller may see. The store makes no authorization decision;
	// the gRPC layer fills this in.
	Visibility *RequestVisibility
}

// RequestVisibility selects requests made by RequesterID, or of any type in
// TypeIDs. Both empty matches nothing.
type RequestVisibility struct {
	RequesterID string
	TypeIDs     []string
}

// ListRequests returns filtered requests, newest first.
func (s *Store) ListRequests(ctx context.Context, f ListRequestsFilter) ([]*AssetRequest, int32, error) {
	baseWhere := "WHERE r.workspace_id = $1"
	args := []any{f.WorkspaceID}
	argIdx := 2

	if v := f.Visibility; v != nil {
		typeIDs := v.TypeIDs
		if typeIDs == nil {
			typeIDs = []string{}
		}
		// requester_id is NOT NULL and references users, so an empty
		// RequesterID matches no row.
		baseWhere += fmt.Sprintf(" AND (r.requester_id = $%d OR r.type_id = ANY($%d))", argIdx, argIdx+1)
		args = append(args, v.RequesterID, typeIDs)
		argIdx += 2
	}

	if f.MineOnly && f.UserID != "" {
		baseWhere += fmt.Sprintf(" AND r.requester_id = $%d", argIdx)
		args = append(args, f.UserID)
		argIdx++
	}
	if statuses := splitList(f.Status); len(statuses) > 0 {
		baseWhere += fmt.Sprintf(" AND r.status = ANY($%d)", argIdx)
		args = append(args, statuses)
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

	query := fmt.Sprintf("%s %s ORDER BY r.created_at DESC, r.id LIMIT $%d OFFSET $%d", requestSelect, baseWhere, argIdx, argIdx+1)
	args = append(args, limit, f.Offset)

	rows, err := s.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("listing requests: %w", err)
	}
	defer rows.Close()

	var requests []*AssetRequest
	for rows.Next() {
		r, err := scanRequest(rows)
		if err != nil {
			return nil, 0, fmt.Errorf("scanning request row: %w", err)
		}
		requests = append(requests, r)
	}
	return requests, total, rows.Err()
}

func splitList(s string) []string {
	var out []string
	for _, p := range strings.Split(s, ",") {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	return out
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
