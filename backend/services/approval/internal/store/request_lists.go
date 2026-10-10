// Package store provides PostgreSQL data access for the approval service.
// Each public method executes a single query — no business logic here.
// All queries run on the tenant's schema via TenantConn.
package store

import (
	"context"
	"fmt"
	"strings"
	"time"

	"ngac-platform/services/approval/internal/domain"
)

// A page cursor is the keyset (timestamp, id) of the last row of the page,
// written "<RFC 3339 micro>|<id>". Lists are ordered by the same pair, descending,
// and the next page is the rows strictly before it, so rows that share a
// timestamp are neither skipped nor repeated where a page ends between them.
const cursorTimeLayout = "2006-01-02T15:04:05.999999Z07:00"

func encodeCursor(ts time.Time, id string) string {
	return ts.UTC().Format(cursorTimeLayout) + "|" + id
}

// decodeCursor parses a cursor this store issued; anything else is a bad request.
func decodeCursor(cursor string) (time.Time, string, error) {
	tsText, id, ok := strings.Cut(cursor, "|")
	if !ok || id == "" {
		return time.Time{}, "", fmt.Errorf("cursor is not one this service issued: %w", domain.ErrInvalidInput)
	}
	ts, err := time.Parse(cursorTimeLayout, tsText)
	if err != nil {
		return time.Time{}, "", fmt.Errorf("cursor is not one this service issued: %w", domain.ErrInvalidInput)
	}
	return ts, id, nil
}

// ListPending returns the user's pending assignments on their requests' current
// steps: rows of their own, and the group rows of the roles and departments in
// groupNodeIDs. A step the user has already acted on drops out even though its
// group row stays pending for the other members.
func (s *Store) ListPending(ctx context.Context, userNodeID string, groupNodeIDs []string) ([]*domain.RequestWithAssignment, error) {
	c, err := s.conn(ctx)
	if err != nil {
		return nil, err
	}
	defer c.Release()

	rows, err := c.Query(ctx, `
		SELECT ar.id, ar.entity_type, ar.entity_id, ar.status, ar.template_id, COALESCE(ar.template_name,''), ar.template_snapshot, COALESCE(ar.form_data_json::text,''),
		       ar.current_step, ar.scope_oa_id, ar.department_id, ar.created_by, ar.created_at, ar.completed_at,
		       aa.id, aa.step_order, aa.user_node_id, aa.grant_source, aa.status
		FROM approval_assignments aa
		JOIN approval_requests ar ON aa.request_id = ar.id
		WHERE aa.status = 'pending'
		  AND ar.status = 'pending'
		  AND ar.current_step = aa.step_order
		  AND (aa.user_node_id = $1
		       OR (aa.user_node_id = ANY($2)
		           AND aa.grant_source IN ('role:' || aa.user_node_id, 'department:' || aa.user_node_id)))
		  AND NOT EXISTS (
		        SELECT 1 FROM approval_assignments mine
		        WHERE mine.request_id = aa.request_id AND mine.step_order = aa.step_order
		          AND mine.user_node_id = $1 AND mine.id <> aa.id)
		ORDER BY ar.created_at ASC`, userNodeID, groupNodeIDs,
	)
	if err != nil {
		return nil, fmt.Errorf("list pending: %w", err)
	}
	defer rows.Close()

	return scanRequestsWithAssignment(rows)
}

// ListHistory returns acted-upon assignments with cursor-based paging.
func (s *Store) ListHistory(ctx context.Context, userNodeID, cursor string, limit int) ([]*domain.RequestWithAssignment, string, error) {
	c, err := s.conn(ctx)
	if err != nil {
		return nil, "", err
	}
	defer c.Release()

	query := `
		SELECT ar.id, ar.entity_type, ar.entity_id, ar.status, ar.template_id, COALESCE(ar.template_name,''), ar.template_snapshot, COALESCE(ar.form_data_json::text,''),
		       ar.current_step, ar.scope_oa_id, ar.department_id, ar.created_by, ar.created_at, ar.completed_at,
		       aa.id, aa.step_order, aa.user_node_id, aa.grant_source, aa.status, aa.acted_at, aa.comment
		FROM approval_assignments aa
		JOIN approval_requests ar ON aa.request_id = ar.id
		WHERE aa.user_node_id = $1
		  AND aa.status IN ('approved', 'rejected')`
	args := []any{userNodeID}
	argIdx := 2

	if cursor != "" {
		ts, id, err := decodeCursor(cursor)
		if err != nil {
			return nil, "", err
		}
		query += fmt.Sprintf(" AND (aa.acted_at, aa.id) < ($%d, $%d)", argIdx, argIdx+1)
		args = append(args, ts, id)
		argIdx += 2
	}
	query += fmt.Sprintf(" ORDER BY aa.acted_at DESC, aa.id DESC LIMIT $%d", argIdx)
	args = append(args, limit+1)

	rows, err := c.Query(ctx, query, args...)
	if err != nil {
		return nil, "", fmt.Errorf("list history: %w", err)
	}
	defer rows.Close()

	var results []*domain.RequestWithAssignment
	for rows.Next() {
		r := &domain.Request{}
		a := &domain.AssignmentRecord{}
		if err := rows.Scan(
			&r.ID, &r.EntityType, &r.EntityID, &r.Status, &r.TemplateID, &r.TemplateName, &r.TemplateSnapshot, &r.FormDataJSON,
			&r.CurrentStep, &r.ScopeOAID, &r.DepartmentID, &r.CreatedBy, &r.CreatedAt, &r.CompletedAt,
			&a.ID, &a.StepOrder, &a.UserNodeID, &a.GrantSource, &a.Status, &a.ActedAt, &a.Comment,
		); err != nil {
			return nil, "", fmt.Errorf("scan history: %w", err)
		}
		a.RequestID = r.ID
		results = append(results, &domain.RequestWithAssignment{Request: r, Assignment: a})
	}

	var nextCursor string
	if len(results) > limit {
		last := results[limit-1]
		nextCursor = encodeCursor(*last.Assignment.ActedAt, last.Assignment.ID)
		results = results[:limit]
	}
	return results, nextCursor, nil
}

// ListMyRequests returns requests created by the user with cursor paging.
func (s *Store) ListMyRequests(ctx context.Context, userNodeID, cursor string, limit int) ([]*domain.Request, string, error) {
	c, err := s.conn(ctx)
	if err != nil {
		return nil, "", err
	}
	defer c.Release()

	query := `SELECT id, entity_type, entity_id, template_id, template_name, template_snapshot, COALESCE(form_data_json::text, ''),
		current_step, status, scope_oa_id, department_id, created_by, created_at, completed_at
		FROM approval_requests WHERE created_by = $1`
	args := []any{userNodeID}
	argIdx := 2

	if cursor != "" {
		ts, id, err := decodeCursor(cursor)
		if err != nil {
			return nil, "", err
		}
		query += fmt.Sprintf(" AND (created_at, id) < ($%d, $%d)", argIdx, argIdx+1)
		args = append(args, ts, id)
		argIdx += 2
	}
	query += fmt.Sprintf(" ORDER BY created_at DESC, id DESC LIMIT $%d", argIdx)
	args = append(args, limit+1)

	rows, err := c.Query(ctx, query, args...)
	if err != nil {
		return nil, "", fmt.Errorf("list my requests: %w", err)
	}
	defer rows.Close()

	results, nextCursor, err := scanRequests(rows, limit)
	if err != nil {
		return nil, "", err
	}
	return results, nextCursor, nil
}

// ListByScopes returns requests within the given scope OA IDs with cursor paging.
func (s *Store) ListByScopes(ctx context.Context, scopeOAIDs []string, cursor string, limit int) ([]*domain.Request, string, error) {
	c, err := s.conn(ctx)
	if err != nil {
		return nil, "", err
	}
	defer c.Release()

	query := `SELECT id, entity_type, entity_id, template_id, template_name, template_snapshot, COALESCE(form_data_json::text, ''),
		current_step, status, scope_oa_id, department_id, created_by, created_at, completed_at
		FROM approval_requests WHERE scope_oa_id = ANY($1)`
	args := []any{scopeOAIDs}
	argIdx := 2

	if cursor != "" {
		ts, id, err := decodeCursor(cursor)
		if err != nil {
			return nil, "", err
		}
		query += fmt.Sprintf(" AND (created_at, id) < ($%d, $%d)", argIdx, argIdx+1)
		args = append(args, ts, id)
		argIdx += 2
	}
	query += fmt.Sprintf(" ORDER BY created_at DESC, id DESC LIMIT $%d", argIdx)
	args = append(args, limit+1)

	rows, err := c.Query(ctx, query, args...)
	if err != nil {
		return nil, "", fmt.Errorf("list by scopes: %w", err)
	}
	defer rows.Close()

	results, nextCursor, err := scanRequests(rows, limit)
	if err != nil {
		return nil, "", err
	}
	return results, nextCursor, nil
}

// FindPendingByGrantSource finds all pending assignments whose grant_source
// matches the given pattern (e.g., "role:KeToan_Chief"). Used by the
// reconciliation consumer to find assignments affected by role changes.
func (s *Store) FindPendingByGrantSource(ctx context.Context, grantSourcePattern string) ([]*domain.AssignmentRecord, error) {
	c, err := s.conn(ctx)
	if err != nil {
		return nil, err
	}
	defer c.Release()

	rows, err := c.Query(ctx, `
		SELECT id, request_id, step_order, user_node_id, grant_source, status
		FROM approval_assignments
		WHERE status = 'pending' AND grant_source = $1`, grantSourcePattern,
	)
	if err != nil {
		return nil, fmt.Errorf("find pending by grant source: %w", err)
	}
	defer rows.Close()

	var results []*domain.AssignmentRecord
	for rows.Next() {
		a := &domain.AssignmentRecord{}
		if err := rows.Scan(&a.ID, &a.RequestID, &a.StepOrder, &a.UserNodeID, &a.GrantSource, &a.Status); err != nil {
			return nil, fmt.Errorf("scan assignment: %w", err)
		}
		results = append(results, a)
	}
	return results, nil
}

// FindPendingByUserAndSource finds a user's pending assignments matching
// a specific request ID or grant source pattern. Used to check duplicates
// during reconciliation and to find revocable assignments.
func (s *Store) FindPendingByUserAndSource(ctx context.Context, userNodeID, grantSourcePattern string) ([]*domain.AssignmentRecord, error) {
	c, err := s.conn(ctx)
	if err != nil {
		return nil, err
	}
	defer c.Release()

	rows, err := c.Query(ctx, `
		SELECT id, request_id, step_order, user_node_id, grant_source, status
		FROM approval_assignments
		WHERE user_node_id = $1 AND status = 'pending' AND grant_source = $2`,
		userNodeID, grantSourcePattern,
	)
	if err != nil {
		return nil, fmt.Errorf("find pending by user+source: %w", err)
	}
	defer rows.Close()

	var results []*domain.AssignmentRecord
	for rows.Next() {
		a := &domain.AssignmentRecord{}
		if err := rows.Scan(&a.ID, &a.RequestID, &a.StepOrder, &a.UserNodeID, &a.GrantSource, &a.Status); err != nil {
			return nil, fmt.Errorf("scan assignment: %w", err)
		}
		results = append(results, a)
	}
	return results, nil
}

// scanRequestsWithAssignment scans rows from pending/history queries.
func scanRequestsWithAssignment(rows interface {
	Next() bool
	Scan(dest ...any) error
}) ([]*domain.RequestWithAssignment, error) {
	var results []*domain.RequestWithAssignment
	for rows.Next() {
		r := &domain.Request{}
		a := &domain.AssignmentRecord{}
		if err := rows.Scan(
			&r.ID, &r.EntityType, &r.EntityID, &r.Status, &r.TemplateID, &r.TemplateName, &r.TemplateSnapshot, &r.FormDataJSON,
			&r.CurrentStep, &r.ScopeOAID, &r.DepartmentID, &r.CreatedBy, &r.CreatedAt, &r.CompletedAt,
			&a.ID, &a.StepOrder, &a.UserNodeID, &a.GrantSource, &a.Status,
		); err != nil {
			return nil, fmt.Errorf("scan request+assignment: %w", err)
		}
		a.RequestID = r.ID
		results = append(results, &domain.RequestWithAssignment{Request: r, Assignment: a})
	}
	return results, nil
}

// scanRequests scans rows into Request slice with cursor extraction.
func scanRequests(rows interface {
	Next() bool
	Scan(dest ...any) error
}, limit int) ([]*domain.Request, string, error) {
	var results []*domain.Request
	for rows.Next() {
		r := &domain.Request{}
		if err := rows.Scan(&r.ID, &r.EntityType, &r.EntityID, &r.TemplateID, &r.TemplateName, &r.TemplateSnapshot, &r.FormDataJSON,
			&r.CurrentStep, &r.Status, &r.ScopeOAID, &r.DepartmentID, &r.CreatedBy, &r.CreatedAt, &r.CompletedAt,
		); err != nil {
			return nil, "", fmt.Errorf("scan request: %w", err)
		}
		results = append(results, r)
	}

	var nextCursor string
	if len(results) > limit {
		last := results[limit-1]
		nextCursor = encodeCursor(last.CreatedAt, last.ID)
		results = results[:limit]
	}
	return results, nextCursor, nil
}
