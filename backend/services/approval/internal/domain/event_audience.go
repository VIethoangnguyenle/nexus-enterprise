package domain

import (
	"context"
	"fmt"
	"log/slog"
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
// A role's or department's row names the group, not a person, so it is
// replaced by the people in it who have not already acted on the step: they are
// the ones who must act next.
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
	rows, err := s.store.ListAssignments(ctx, requestID)
	if err != nil {
		return nil, fmt.Errorf("list assignments: %w", err)
	}
	acted := map[string]bool{}
	for _, a := range rows {
		if a.StepOrder == req.CurrentStep && a.Status != "pending" {
			acted[a.UserNodeID] = true
		}
	}
	seen := map[string]bool{}
	for _, id := range assignees {
		members, err := s.policy.GetMembers(ctx, id)
		if err != nil {
			// The event still goes to the others; this group's members miss the push.
			slog.Warn("approval event: cannot expand group", "error", err)
		}
		if len(members) == 0 {
			members = []string{id} // a person, not a group
		}
		for _, m := range members {
			if !seen[m] && !acted[m] {
				seen[m] = true
				aud.AssigneeNodeIDs = append(aud.AssigneeNodeIDs, m)
			}
		}
	}
	return aud, nil
}
