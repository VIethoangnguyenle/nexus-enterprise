package domain

import (
	"context"
	"fmt"
)

// EventAudience is who an approval lifecycle event concerns.
type EventAudience struct {
	// Request is the request as it stands after the action.
	Request *Request
	// AssigneeNodeIDs are the approvers still pending on the current step —
	// the people who must act next. Empty once the request is terminal.
	AssigneeNodeIDs []string
}

// EventAudience resolves the audience of an event about requestID: the
// request itself (for its requester) and the approvers pending on its current
// step. Call it after the action, so a step that just advanced names the next
// step's approvers.
//
// Assignees of the "department" or role-fallback kinds are UA nodes rather than
// users; they are passed through as recorded and simply match no session.
func (s *Service) EventAudience(ctx context.Context, requestID string) (*EventAudience, error) {
	if requestID == "" {
		return nil, ErrInvalidInput
	}
	req, err := s.store.GetRequest(ctx, requestID)
	if err != nil {
		return nil, fmt.Errorf("get request: %w", err)
	}
	aud := &EventAudience{Request: req}
	if req.Status != "pending" {
		return aud, nil
	}
	assignees, err := s.store.ListPendingAssignees(ctx, requestID, req.CurrentStep)
	if err != nil {
		return nil, fmt.Errorf("list pending assignees: %w", err)
	}
	aud.AssigneeNodeIDs = assignees
	return aud, nil
}
