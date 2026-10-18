package domain

import (
	"context"
	"fmt"
	"log/slog"

	"ngac-platform/ngac"
)

// SearchRequests finds requests matching a text query that the caller may see.
// It resolves the caller's groups and accessible scopes, then delegates to the
// store with the full access context so the SQL can filter in one pass.
func (s *Service) SearchRequests(ctx context.Context, userNodeID string, p SearchParams) ([]*Request, int, error) {
	if err := s.requireCallerOrganization(ctx); err != nil {
		return nil, 0, err
	}
	if len(p.Query) < 2 {
		return nil, 0, fmt.Errorf("%w: query must be at least 2 characters", ErrInvalidInput)
	}
	if p.Status != "" {
		switch p.Status {
		case "pending", "approved", "rejected", "cancelled":
			// valid
		default:
			return nil, 0, fmt.Errorf("%w: status must be one of: pending, approved, rejected, cancelled", ErrInvalidInput)
		}
	}
	if userNodeID == "" {
		return nil, 0, ErrInvalidInput
	}

	// Resolve groups (ancestors) for assignment-based access.
	groups, err := s.policy.GetAncestors(ctx, userNodeID)
	if err != nil {
		slog.Warn("approval search: cannot read ancestors, listing direct access only", "error", err)
		groups = nil
	}

	// Resolve accessible scopes for department-based access.
	scopes, err := s.policy.ResolveAccessibleScopes(ctx, userNodeID, ngac.OpRead)
	if err != nil {
		slog.Warn("approval search: cannot read scopes, listing without department access", "error", err)
		scopes = nil
	}

	p.UserNodeID = userNodeID
	p.GroupNodeIDs = groups
	p.AccessibleScopes = scopes

	items, total, err := s.store.SearchRequests(ctx, p)
	if err != nil {
		return nil, 0, fmt.Errorf("search requests: %w", err)
	}
	return items, total, nil
}
