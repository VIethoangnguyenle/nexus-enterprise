package domain

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
)

// checkStepCompletion checks if the current step has enough approvals
// to advance to the next step or complete the request.
func (s *Service) checkStepCompletion(ctx context.Context, req *Request) error {
	// Recover template from snapshot
	var tmpl Template
	if err := json.Unmarshal([]byte(req.TemplateSnapshot), &tmpl); err != nil {
		return fmt.Errorf("unmarshal snapshot: %w", err)
	}

	// Find current step in template
	var currentStep *Step
	for _, st := range tmpl.Steps {
		if st.StepOrder == req.CurrentStep {
			currentStep = st
			break
		}
	}
	if currentStep == nil {
		return nil
	}

	approvedCount, err := s.store.CountApprovedForStep(ctx, req.ID, req.CurrentStep)
	if err != nil {
		return fmt.Errorf("count approved: %w", err)
	}

	if approvedCount < currentStep.RequiredCount {
		return nil // not enough yet
	}

	// Step complete — skip remaining assignments for this step
	if err := s.store.SkipRemainingAssignments(ctx, req.ID, req.CurrentStep); err != nil {
		return fmt.Errorf("skip remaining: %w", err)
	}

	if err := s.logAudit(ctx, req.ID, "step_advanced", "", req.CurrentStep, map[string]string{
		"from_step": fmt.Sprintf("%d", req.CurrentStep),
	}); err != nil {
		return err
	}

	// Check if there's a next step
	nextStep := req.CurrentStep + 1
	var nextStepDef *Step
	for _, st := range tmpl.Steps {
		if st.StepOrder == nextStep {
			nextStepDef = st
			break
		}
	}

	if nextStepDef == nil {
		// No more steps — request fully approved. Only the caller that actually
		// moved the request out of "pending" logs the completion.
		completed, err := s.store.CompleteRequest(ctx, req.ID, "approved")
		if err != nil {
			return fmt.Errorf("complete request: %w", err)
		}
		if completed {
			return s.logAudit(ctx, req.ID, "completed", "", 0, map[string]string{
				"final_status": "approved",
			})
		}
		return nil
	}

	// Advance to the next step. Concurrent approvals can both satisfy the same
	// quorum, so the advance is a compare-and-swap and only the winner goes on
	// to create the next step's assignments — otherwise every approval past the
	// quorum would add another full set of approvers.
	advanced, err := s.store.AdvanceStep(ctx, req.ID, req.CurrentStep, nextStep)
	if err != nil {
		return fmt.Errorf("advance step: %w", err)
	}
	if !advanced {
		return nil
	}

	// Resolve approvers for next step
	return s.assignStep(ctx, req.ID, nextStepDef, req.DepartmentID)
}

// assignStep builds a step's assignments and stores them.
func (s *Service) assignStep(ctx context.Context, requestID string, step *Step, deptID string) error {
	assignments, err := buildAssignments(requestID, step, deptID)
	if err != nil {
		return err
	}
	if err := s.store.InsertAssignments(ctx, assignments); err != nil {
		return fmt.Errorf("insert assignments: %w", err)
	}
	return s.auditAssigned(ctx, requestID, step.StepOrder, assignments)
}

// buildAssignments turns a step into its assignment rows.
//
// A named person gets a "direct" row. A role or a department gets ONE group
// row whose user_node_id is the role's or department's UA and whose grant
// source is role:<ua> / department:<ua>; whoever belongs to that UA when they
// act gets a row of their own then (see actingRow). A step that names nobody is
// ErrInvalidInput, so it can be refused before anything is stored.
func buildAssignments(requestID string, step *Step, deptID string) ([]*AssignmentRecord, error) {
	value := ResolvePlaceholder(step.ApproverValue, deptID)
	if value == "" || value == "default" {
		return nil, fmt.Errorf("no approvers resolved for step %d: %w", step.StepOrder, ErrInvalidInput)
	}
	grant := ""
	switch step.ApproverType {
	case ApproverSpecificUser:
		grant = "direct"
	case ApproverRole:
		grant = "role:" + value
	case ApproverDepartment:
		grant = "department:" + value
	default:
		return nil, fmt.Errorf("unknown approver_type %q: %w", step.ApproverType, ErrInvalidInput)
	}
	return []*AssignmentRecord{{
		ID: uuid.New().String(), RequestID: requestID, StepOrder: step.StepOrder,
		UserNodeID: value, GrantSource: grant, Status: "pending",
	}}, nil
}

func (s *Service) auditAssigned(ctx context.Context, requestID string, stepOrder int, rows []*AssignmentRecord) error {
	for _, a := range rows {
		if err := s.logAudit(ctx, requestID, "assigned", a.UserNodeID, stepOrder, map[string]string{
			"grant_source": a.GrantSource,
		}); err != nil {
			return err
		}
	}
	return nil
}

// groupOf is the UA a role:/department: grant source names ("" for anything else).
func groupOf(grantSource string) string {
	if _, ua, ok := strings.Cut(grantSource, ":"); ok {
		return ua
	}
	return ""
}

// isGroupRow reports whether an assignment is a role's or department's own row
// (its user_node_id is the group named by its grant source) rather than a person's.
func isGroupRow(a *AssignmentRecord) bool {
	return a.UserNodeID != "" && groupOf(a.GrantSource) == a.UserNodeID
}

// verifyMember checks, at the moment of acting, that the person still belongs to
// the role or department an assignment came from. This is what stops a member
// removed since the request was made, and what keeps the check on the group's
// own UA rather than on some scope attribute.
func (s *Service) verifyMember(ctx context.Context, userNodeID, groupNodeID string) error {
	if groupNodeID == "" {
		return fmt.Errorf("%w: assignment names no role or department", ErrAccessDenied)
	}
	ancestors, err := s.policy.GetAncestors(ctx, userNodeID)
	if err != nil {
		return fmt.Errorf("ngac ancestors: %w", err)
	}
	for _, a := range ancestors {
		if a == groupNodeID {
			return nil
		}
	}
	return fmt.Errorf("%w: no longer a member of the approving role or department", ErrAccessDenied)
}

// logAudit appends to the audit trail and reports a failure to append. It runs
// inside the same change as the thing it records (see InTx), so a failure rolls
// that back too: the trail has no gaps.
func (s *Service) logAudit(ctx context.Context, requestID, action, actorNodeID string, stepOrder int, detail map[string]string) error {
	detailJSON, err := json.Marshal(detail)
	if err != nil {
		return fmt.Errorf("encode audit detail: %w", err)
	}
	if err := s.store.InsertAuditEntry(ctx, &AuditEntry{
		ID:          uuid.New().String(),
		RequestID:   requestID,
		Action:      action,
		ActorNodeID: actorNodeID,
		StepOrder:   stepOrder,
		DetailJSON:  string(detailJSON),
		CreatedAt:   time.Now(),
	}); err != nil {
		return fmt.Errorf("audit %s: %w", action, err)
	}
	return nil
}
