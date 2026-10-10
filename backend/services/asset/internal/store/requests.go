package store

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

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
