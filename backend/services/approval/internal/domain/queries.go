package domain

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"

	"github.com/google/uuid"

	"ngac-platform/ngac"
)

const defaultPageLimit = 20

// GetPending returns all pending assignments for a user (Tab 1 — no paging).
func (s *Service) GetPending(ctx context.Context, userNodeID string) ([]*RequestWithAssignment, error) {
	if userNodeID == "" {
		return nil, ErrInvalidInput
	}
	// The roles and departments the user belongs to, so requests waiting on them
	// show up too. If membership cannot be read, what waits on the person
	// directly still shows: a shorter list, never a wider one.
	groups, err := s.policy.GetAncestors(ctx, userNodeID)
	if err != nil {
		slog.Warn("approval pending: cannot read memberships, listing direct assignments only", "error", err)
		groups = nil
	}
	items, err := s.store.ListPending(ctx, userNodeID, groups)
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
	scopes, err := s.policy.ResolveAccessibleScopes(ctx, userNodeID, ngac.OpRead)
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

// GetRequestDetail opens one request for a caller who can see it by any of the
// three paths GetAuditLog accepts. Anyone else, and a request that does not
// exist, get ErrAccessDenied, so the answer does not reveal which ids are real.
func (s *Service) GetRequestDetail(ctx context.Context, userNodeID, requestID string) (*RequestDetail, error) {
	if _, err := uuid.Parse(requestID); err != nil {
		return nil, ErrInvalidInput
	}
	if userNodeID == "" {
		return nil, ErrAccessDenied
	}
	req, err := s.store.GetRequest(ctx, requestID)
	if errors.Is(err, ErrNotFound) {
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
	assignments, err := s.store.ListAssignments(ctx, requestID)
	if err != nil {
		return nil, fmt.Errorf("list assignments: %w", err)
	}

	d := &RequestDetail{Request: req, Assignments: assignments}
	d.CanAct, err = s.canAct(ctx, req, assignments, userNodeID)
	if err != nil {
		return nil, err
	}
	if req.TemplateSnapshot != "" {
		var snap Template
		if err := json.Unmarshal([]byte(req.TemplateSnapshot), &snap); err != nil {
			// The chain can still be read from the assignments; losing the
			// frozen steps must not lock an approver out of acting.
			slog.Warn("approval request has an unreadable template snapshot", "request_id", requestID, "error", err)
		} else {
			d.Steps, d.FormFields = snap.Steps, snap.FormFields
		}
	}
	// The snapshot is already unpacked into Steps and FormFields.
	shown := *req
	shown.TemplateSnapshot = ""
	d.Request = &shown
	return d, nil
}

// canAct reports whether it is the caller's turn on the request: it is pending,
// and on its current step the caller holds a pending row of their own, or
// belongs to a role or department that does and has not yet acted themselves.
func (s *Service) canAct(ctx context.Context, req *Request, rows []*AssignmentRecord, userNodeID string) (bool, error) {
	if req.Status != "pending" {
		return false, nil
	}
	acted := false
	var groups []*AssignmentRecord
	for _, a := range rows {
		if a.StepOrder != req.CurrentStep {
			continue
		}
		if a.UserNodeID == userNodeID {
			if a.Status == "pending" {
				return true, nil
			}
			acted = true
		} else if a.Status == "pending" && isGroupRow(a) {
			groups = append(groups, a)
		}
	}
	if acted || len(groups) == 0 {
		return false, nil
	}
	ancestors, err := s.policy.GetAncestors(ctx, userNodeID)
	if err != nil {
		return false, fmt.Errorf("ngac ancestors: %w", err)
	}
	for _, g := range groups {
		for _, a := range ancestors {
			if a == g.UserNodeID {
				return true, nil
			}
		}
	}
	return false, nil
}

// canSeeRequest applies the same three paths the list endpoints expose:
// my-requests (created_by), pending/history (an assignment) and
// department-requests (scope_oa_id within the caller's readable scopes). A
// failure to resolve scopes is an error, never a grant.
func (s *Service) canSeeRequest(ctx context.Context, userNodeID string, req *Request) (bool, error) {
	if req.CreatedBy == userNodeID {
		return true, nil
	}
	assigned, err := s.store.HasAssignment(ctx, req.ID, []string{userNodeID})
	if err != nil {
		return false, fmt.Errorf("check assignment: %w", err)
	}
	if assigned {
		return true, nil
	}
	// Assigned through a role or department the caller belongs to.
	ancestors, err := s.policy.GetAncestors(ctx, userNodeID)
	if err != nil {
		return false, fmt.Errorf("ngac ancestors: %w", err)
	}
	if len(ancestors) > 0 {
		viaGroup, err := s.store.HasAssignment(ctx, req.ID, ancestors)
		if err != nil {
			return false, fmt.Errorf("check group assignment: %w", err)
		}
		if viaGroup {
			return true, nil
		}
	}
	if req.ScopeOAID == "" {
		return false, nil
	}
	scopes, err := s.policy.ResolveAccessibleScopes(ctx, userNodeID, ngac.OpRead)
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
