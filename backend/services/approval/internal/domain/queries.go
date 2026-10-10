package domain

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
)

const defaultPageLimit = 20

// GetPending returns all pending assignments for a user (Tab 1 — no paging).
func (s *Service) GetPending(ctx context.Context, userNodeID string) ([]*RequestWithAssignment, error) {
	if userNodeID == "" {
		return nil, ErrInvalidInput
	}
	items, err := s.store.ListPending(ctx, userNodeID)
	if err != nil {
		return nil, fmt.Errorf("list pending: %w", err)
	}
	return items, nil
}

// GetHistory returns actioned assignments with cursor paging (Tab 2).
func (s *Service) GetHistory(ctx context.Context, userNodeID, cursor string, limit int) ([]*RequestWithAssignment, string, error) {
	if userNodeID == "" {
		return nil, "", ErrInvalidInput
	}
	if limit <= 0 {
		limit = defaultPageLimit
	}
	items, nextCursor, err := s.store.ListHistory(ctx, userNodeID, cursor, limit)
	if err != nil {
		return nil, "", fmt.Errorf("list history: %w", err)
	}
	return items, nextCursor, nil
}

// GetMyRequests returns requests created by the user with cursor paging (Tab 3).
func (s *Service) GetMyRequests(ctx context.Context, userNodeID, cursor string, limit int) ([]*Request, string, error) {
	if userNodeID == "" {
		return nil, "", ErrInvalidInput
	}
	if limit <= 0 {
		limit = defaultPageLimit
	}
	items, nextCursor, err := s.store.ListMyRequests(ctx, userNodeID, cursor, limit)
	if err != nil {
		return nil, "", fmt.Errorf("list my requests: %w", err)
	}
	return items, nextCursor, nil
}

// GetDepartmentRequests returns requests visible via scope-based access (Tab 4).
// Uses ResolveAccessibleScopes to determine which OA scopes the user can see.
func (s *Service) GetDepartmentRequests(ctx context.Context, userNodeID, cursor string, limit int) ([]*Request, string, error) {
	if userNodeID == "" {
		return nil, "", ErrInvalidInput
	}
	if limit <= 0 {
		limit = defaultPageLimit
	}

	// Resolve scopes via NGAC
	scopes, err := s.policy.ResolveAccessibleScopes(ctx, userNodeID, "read")
	if err != nil {
		return nil, "", fmt.Errorf("resolve scopes: %w", err)
	}
	if len(scopes) == 0 {
		return nil, "", nil // no visible scopes
	}

	items, nextCursor, err := s.store.ListByScopes(ctx, scopes, cursor, limit)
	if err != nil {
		return nil, "", fmt.Errorf("list by scopes: %w", err)
	}
	return items, nextCursor, nil
}

// GetAuditLog returns the audit trail of a request to a caller who can see
// that request through the list endpoints: its requester, anyone assigned to
// any of its steps (current or past), or a caller whose department-requests
// scope includes it. Anyone else is denied; the trail names every actor and
// carries the form-level detail of each action.
func (s *Service) GetAuditLog(ctx context.Context, userNodeID, requestID string) ([]*AuditEntry, error) {
	// Request ids are UUIDs; anything else is a malformed request (400), not a
	// database error (500).
	if _, err := uuid.Parse(requestID); err != nil {
		return nil, ErrInvalidInput
	}
	if userNodeID == "" {
		return nil, ErrAccessDenied
	}
	req, err := s.store.GetRequest(ctx, requestID)
	if errors.Is(err, ErrNotFound) {
		// A request the caller may not see and one that does not exist get the
		// same answer, so the trail's existence is not an oracle.
		return nil, ErrAccessDenied
	}
	if err != nil {
		return nil, fmt.Errorf("get request: %w", err)
	}
	visible, err := s.canSeeRequest(ctx, userNodeID, req)
	if err != nil {
		return nil, err
	}
	if !visible {
		return nil, ErrAccessDenied
	}
	entries, err := s.store.ListAuditEntries(ctx, requestID)
	if err != nil {
		return nil, fmt.Errorf("list audit: %w", err)
	}
	return entries, nil
}

// canSeeRequest applies the same three paths the list endpoints expose:
// my-requests (created_by), pending/history (an assignment) and
// department-requests (scope_oa_id within the caller's readable scopes). A
// failure to resolve scopes is an error, never a grant.
func (s *Service) canSeeRequest(ctx context.Context, userNodeID string, req *Request) (bool, error) {
	if req.CreatedBy == userNodeID {
		return true, nil
	}
	assigned, err := s.store.HasAssignment(ctx, req.ID, userNodeID)
	if err != nil {
		return false, fmt.Errorf("check assignment: %w", err)
	}
	if assigned {
		return true, nil
	}
	if req.ScopeOAID == "" {
		return false, nil
	}
	scopes, err := s.policy.ResolveAccessibleScopes(ctx, userNodeID, "read")
	if err != nil {
		return false, fmt.Errorf("resolve scopes: %w", err)
	}
	for _, sc := range scopes {
		if sc == req.ScopeOAID {
			return true, nil
		}
	}
	return false, nil
}
