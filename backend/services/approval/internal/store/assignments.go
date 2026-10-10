// Package store provides PostgreSQL data access for the approval service.
// Each public method executes a single query — no business logic here.
// All queries run on the tenant's schema via TenantConn.
package store

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"ngac-platform/services/approval/internal/domain"
)

// InsertAssignments bulk-inserts approval assignments for a request step.
func (s *Store) InsertAssignments(ctx context.Context, assignments []*domain.AssignmentRecord) error {
	c, err := s.conn(ctx)
	if err != nil {
		return err
	}
	defer c.Release()

	for _, a := range assignments {
		_, err := c.Exec(ctx, `
			INSERT INTO approval_assignments (id, request_id, step_order, user_node_id, grant_source, status)
			VALUES ($1, $2, $3, $4, $5, $6)`,
			a.ID, a.RequestID, a.StepOrder, a.UserNodeID, a.GrantSource, a.Status,
		)
		if err != nil {
			return fmt.Errorf("insert assignment: %w", err)
		}
	}
	return nil
}

// GetAssignment finds a specific user's pending assignment for a request.
func (s *Store) GetAssignment(ctx context.Context, requestID, userNodeID string) (*domain.AssignmentRecord, error) {
	c, err := s.conn(ctx)
	if err != nil {
		return nil, err
	}
	defer c.Release()

	a := &domain.AssignmentRecord{}
	var comment *string
	err = c.QueryRow(ctx, `
		SELECT id, request_id, step_order, user_node_id, grant_source, status, acted_at, comment
		FROM approval_assignments
		WHERE request_id = $1 AND user_node_id = $2 AND status = 'pending'`, requestID, userNodeID,
	).Scan(&a.ID, &a.RequestID, &a.StepOrder, &a.UserNodeID, &a.GrantSource, &a.Status, &a.ActedAt, &comment)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, domain.ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get assignment: %w", err)
	}
	if comment != nil {
		a.Comment = *comment
	}
	return a, nil
}

// HasAssignment reports whether any of the nodes has an assignment of any
// status on any step of the request.
func (s *Store) HasAssignment(ctx context.Context, requestID string, nodeIDs []string) (bool, error) {
	c, err := s.conn(ctx)
	if err != nil {
		return false, err
	}
	defer c.Release()

	var found bool
	err = c.QueryRow(ctx, `
		SELECT EXISTS (SELECT 1 FROM approval_assignments WHERE request_id = $1 AND user_node_id = ANY($2))`,
		requestID, nodeIDs).Scan(&found)
	if err != nil {
		return false, fmt.Errorf("has assignment: %w", err)
	}
	return found, nil
}

// FindGroupAssignment returns the pending role or department row of a step
// whose group is among groupNodeIDs. A group row is the one whose grant source
// names its own user_node_id (role:<ua> / department:<ua>), which a person's
// row never does.
func (s *Store) FindGroupAssignment(ctx context.Context, requestID string, stepOrder int, groupNodeIDs []string) (*domain.AssignmentRecord, error) {
	c, err := s.conn(ctx)
	if err != nil {
		return nil, err
	}
	defer c.Release()

	a := &domain.AssignmentRecord{}
	err = c.QueryRow(ctx, `
		SELECT id, request_id, step_order, user_node_id, grant_source, status
		FROM approval_assignments
		WHERE request_id = $1 AND step_order = $2 AND status = 'pending'
		  AND user_node_id = ANY($3)
		  AND grant_source IN ('role:' || user_node_id, 'department:' || user_node_id)
		ORDER BY id LIMIT 1`, requestID, stepOrder, groupNodeIDs,
	).Scan(&a.ID, &a.RequestID, &a.StepOrder, &a.UserNodeID, &a.GrantSource, &a.Status)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, domain.ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("find group assignment: %w", err)
	}
	return a, nil
}

// InsertActedAssignment records a person's own decision on a step assigned to
// a group. The unique (request, step, person) index makes a second decision by
// the same person ErrAlreadyExists.
func (s *Store) InsertActedAssignment(ctx context.Context, a *domain.AssignmentRecord) error {
	c, err := s.conn(ctx)
	if err != nil {
		return err
	}
	defer c.Release()

	_, err = c.Exec(ctx, `
		INSERT INTO approval_assignments (id, request_id, step_order, user_node_id, grant_source, status, acted_at, comment)
		VALUES ($1, $2, $3, $4, $5, $6, COALESCE($7, NOW()), $8)`,
		a.ID, a.RequestID, a.StepOrder, a.UserNodeID, a.GrantSource, a.Status, a.ActedAt, a.Comment)
	var pg *pgconn.PgError
	if errors.As(err, &pg) && pg.Code == "23505" {
		return domain.ErrAlreadyExists
	}
	if err != nil {
		return fmt.Errorf("insert acted assignment: %w", err)
	}
	return nil
}

// ListAssignments returns every assignment of a request, step by step.
func (s *Store) ListAssignments(ctx context.Context, requestID string) ([]*domain.AssignmentRecord, error) {
	c, err := s.conn(ctx)
	if err != nil {
		return nil, err
	}
	defer c.Release()

	rows, err := c.Query(ctx, `
		SELECT id, request_id, step_order, user_node_id, grant_source, status, acted_at, COALESCE(comment, '')
		FROM approval_assignments
		WHERE request_id = $1
		ORDER BY step_order, acted_at NULLS LAST, id`, requestID)
	if err != nil {
		return nil, fmt.Errorf("list assignments: %w", err)
	}
	defer rows.Close()

	var out []*domain.AssignmentRecord
	for rows.Next() {
		a := &domain.AssignmentRecord{}
		if err := rows.Scan(&a.ID, &a.RequestID, &a.StepOrder, &a.UserNodeID, &a.GrantSource, &a.Status, &a.ActedAt, &a.Comment); err != nil {
			return nil, fmt.Errorf("scan assignment: %w", err)
		}
		out = append(out, a)
	}
	return out, rows.Err()
}

// UpdateAssignmentStatus updates an assignment's status and sets acted_at.
func (s *Store) UpdateAssignmentStatus(ctx context.Context, id, status, comment string) error {
	c, err := s.conn(ctx)
	if err != nil {
		return err
	}
	defer c.Release()

	_, err = c.Exec(ctx, `
		UPDATE approval_assignments SET status = $2, acted_at = NOW(), comment = $3
		WHERE id = $1`, id, status, comment,
	)
	if err != nil {
		return fmt.Errorf("update assignment: %w", err)
	}
	return nil
}

// CountApprovedForStep counts approved assignments for a specific step.
func (s *Store) CountApprovedForStep(ctx context.Context, requestID string, stepOrder int) (int, error) {
	c, err := s.conn(ctx)
	if err != nil {
		return 0, err
	}
	defer c.Release()

	var count int
	err = c.QueryRow(ctx, `
		SELECT COUNT(*) FROM approval_assignments
		WHERE request_id = $1 AND step_order = $2 AND status = 'approved'`,
		requestID, stepOrder,
	).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("count approved: %w", err)
	}
	return count, nil
}

// ListPendingAssignees returns the user nodes whose assignment on a step is
// still pending.
func (s *Store) ListPendingAssignees(ctx context.Context, requestID string, stepOrder int) ([]string, error) {
	c, err := s.conn(ctx)
	if err != nil {
		return nil, err
	}
	defer c.Release()

	rows, err := c.Query(ctx, `
		SELECT user_node_id FROM approval_assignments
		WHERE request_id = $1 AND step_order = $2 AND status = 'pending'
		ORDER BY user_node_id`,
		requestID, stepOrder,
	)
	if err != nil {
		return nil, fmt.Errorf("list pending assignees: %w", err)
	}
	defer rows.Close()

	var out []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, fmt.Errorf("scan pending assignee: %w", err)
		}
		out = append(out, id)
	}
	return out, rows.Err()
}

// SkipRemainingAssignments marks all pending assignments for a step as skipped.
func (s *Store) SkipRemainingAssignments(ctx context.Context, requestID string, stepOrder int) error {
	c, err := s.conn(ctx)
	if err != nil {
		return err
	}
	defer c.Release()

	_, err = c.Exec(ctx, `
		UPDATE approval_assignments SET status = 'skipped'
		WHERE request_id = $1 AND step_order = $2 AND status = 'pending'`,
		requestID, stepOrder,
	)
	if err != nil {
		return fmt.Errorf("skip remaining: %w", err)
	}
	return nil
}

// SkipAllPendingAssignments marks ALL pending assignments across all steps as skipped.
func (s *Store) SkipAllPendingAssignments(ctx context.Context, requestID string) error {
	c, err := s.conn(ctx)
	if err != nil {
		return err
	}
	defer c.Release()

	_, err = c.Exec(ctx, `
		UPDATE approval_assignments SET status = 'skipped'
		WHERE request_id = $1 AND status = 'pending'`, requestID,
	)
	if err != nil {
		return fmt.Errorf("skip all pending: %w", err)
	}
	return nil
}
