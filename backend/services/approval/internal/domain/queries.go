package domain

import (
	"context"
	"fmt"
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

// GetAuditLog returns the full audit trail for a request.
func (s *Service) GetAuditLog(ctx context.Context, requestID string) ([]*AuditEntry, error) {
	if requestID == "" {
		return nil, ErrInvalidInput
	}
	entries, err := s.store.ListAuditEntries(ctx, requestID)
	if err != nil {
		return nil, fmt.Errorf("list audit: %w", err)
	}
	return entries, nil
}
