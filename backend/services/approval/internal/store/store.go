// Package store provides PostgreSQL data access for the approval service.
// Each public method executes a single query — no business logic here.
// All queries run on the tenant's schema via TenantConn.
package store

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"ngac-platform/services/approval/internal/domain"
)

// Store provides data access methods for approval workflow tables.
type Store struct {
	db *pgxpool.Pool
}

// NewStore creates a store backed by the given connection pool.
func NewStore(db *pgxpool.Pool) *Store {
	return &Store{db: db}
}

// InsertRequest persists a new approval request.
func (s *Store) InsertRequest(ctx context.Context, r *domain.Request) error {
	c, err := s.conn(ctx)
	if err != nil {
		return err
	}
	defer c.Release()

	var formData interface{}
	if r.FormDataJSON != "" {
		formData = r.FormDataJSON
	}

	_, err = c.Exec(ctx, `
		INSERT INTO approval_requests (id, entity_type, entity_id, template_id, template_name, template_snapshot, form_data_json, current_step, status, scope_oa_id, department_id, created_by, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13)`,
		r.ID, r.EntityType, r.EntityID, r.TemplateID, r.TemplateName, r.TemplateSnapshot, formData, r.CurrentStep, r.Status, r.ScopeOAID, r.DepartmentID, r.CreatedBy, r.CreatedAt,
	)
	if err != nil {
		return fmt.Errorf("insert request: %w", err)
	}
	return nil
}

// InsertRequestWithAssignments stores a request and its first assignments in
// one transaction: a request never exists with nobody to decide it.
func (s *Store) InsertRequestWithAssignments(ctx context.Context, r *domain.Request, assignments []*domain.AssignmentRecord) error {
	c, err := s.conn(ctx)
	if err != nil {
		return err
	}
	defer c.Release()

	tx, err := c.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback(ctx)

	var formData interface{}
	if r.FormDataJSON != "" {
		formData = r.FormDataJSON
	}
	if _, err := tx.Exec(ctx, `
		INSERT INTO approval_requests (id, entity_type, entity_id, template_id, template_name, template_snapshot, form_data_json, current_step, status, scope_oa_id, department_id, created_by, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13)`,
		r.ID, r.EntityType, r.EntityID, r.TemplateID, r.TemplateName, r.TemplateSnapshot, formData, r.CurrentStep, r.Status, r.ScopeOAID, r.DepartmentID, r.CreatedBy, r.CreatedAt,
	); err != nil {
		return fmt.Errorf("insert request: %w", err)
	}
	for _, a := range assignments {
		if _, err := tx.Exec(ctx, `
			INSERT INTO approval_assignments (id, request_id, step_order, user_node_id, grant_source, status)
			VALUES ($1, $2, $3, $4, $5, $6)`,
			a.ID, a.RequestID, a.StepOrder, a.UserNodeID, a.GrantSource, a.Status,
		); err != nil {
			return fmt.Errorf("insert assignment: %w", err)
		}
	}
	return tx.Commit(ctx)
}

// GetRequest retrieves an approval request by ID.
func (s *Store) GetRequest(ctx context.Context, id string) (*domain.Request, error) {
	c, err := s.conn(ctx)
	if err != nil {
		return nil, err
	}
	defer c.Release()

	r := &domain.Request{}
	var formDataJSON *string
	err = c.QueryRow(ctx, `
		SELECT id, entity_type, entity_id, template_id, template_name, template_snapshot, form_data_json, current_step, status,
		       scope_oa_id, department_id, created_by, created_at, completed_at
		FROM approval_requests WHERE id = $1`, id,
	).Scan(&r.ID, &r.EntityType, &r.EntityID, &r.TemplateID, &r.TemplateName, &r.TemplateSnapshot, &formDataJSON, &r.CurrentStep, &r.Status,
		&r.ScopeOAID, &r.DepartmentID, &r.CreatedBy, &r.CreatedAt, &r.CompletedAt)
	if formDataJSON != nil {
		r.FormDataJSON = *formDataJSON
	}
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, domain.ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get request: %w", err)
	}
	return r, nil
}

// AdvanceStep increments the current_step on an approval request.
func (s *Store) AdvanceStep(ctx context.Context, requestID string, fromStep, nextStep int) (bool, error) {
	c, err := s.conn(ctx)
	if err != nil {
		return false, err
	}
	defer c.Release()

	// Compare-and-swap on current_step. Two approvals that satisfy the same
	// quorum concurrently both observe the count as met and both try to
	// advance; without the guard both succeed and each goes on to create a
	// full set of assignments for the next step. Only the caller that moves
	// the step away from fromStep may proceed.
	tag, err := c.Exec(ctx, `
		UPDATE approval_requests
		SET current_step = $3
		WHERE id = $1 AND current_step = $2 AND status = 'pending'`,
		requestID, fromStep, nextStep,
	)
	if err != nil {
		return false, fmt.Errorf("advance step: %w", err)
	}
	return tag.RowsAffected() == 1, nil
}

// LockRequest locks the request's row (FOR UPDATE) and returns its status and
// current step. Inside InTx the lock is held to the end of the transaction, which
// is what serialises decisions on one request.
func (s *Store) LockRequest(ctx context.Context, requestID string) (string, int, error) {
	c, err := s.conn(ctx)
	if err != nil {
		return "", 0, err
	}
	defer c.Release()

	var status string
	var step int
	err = c.QueryRow(ctx,
		`SELECT status, current_step FROM approval_requests WHERE id = $1 FOR UPDATE`, requestID).Scan(&status, &step)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", 0, domain.ErrNotFound
	}
	if err != nil {
		return "", 0, fmt.Errorf("lock request: %w", err)
	}
	return status, step, nil
}

// CompleteRequest marks a request as completed with the given terminal status.
func (s *Store) CompleteRequest(ctx context.Context, requestID, status string) (bool, error) {
	c, err := s.conn(ctx)
	if err != nil {
		return false, err
	}
	defer c.Release()

	// Only a still-pending request may reach a terminal state, so a late
	// approval cannot overwrite a rejection (or vice versa) and the completion
	// audit entry is written exactly once.
	tag, err := c.Exec(ctx, `
		UPDATE approval_requests
		SET status = $2, completed_at = NOW()
		WHERE id = $1 AND status = 'pending'`,
		requestID, status,
	)
	if err != nil {
		return false, fmt.Errorf("complete request: %w", err)
	}
	return tag.RowsAffected() == 1, nil
}

// --- reconciliation queries ---

// --- scan helpers ---
