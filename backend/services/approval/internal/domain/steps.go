package domain

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/google/uuid"

	"ngac-platform/pkg/grpcauth"
)

// checkStepCompletion checks if the current step has enough approvals
// to advance to the next step or complete the request.
func (s *Service) checkStepCompletion(ctx context.Context, req *Request) error {
	// Recover template from snapshot
	var tmpl Template
	if err := json.Unmarshal([]byte(req.TemplateSnapshot), &tmpl); err != nil {
		return fmt.Errorf("unmarshal snapshot: %w", err)
	}

	currentStep := stepAt(tmpl.Steps, req.CurrentStep)
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

	return s.moveOn(ctx, req, tmpl.Steps, req.CurrentStep, grpcauth.CallerFrom(ctx).TenantID)
}

// soleApproverSkipReason is the reason recorded on the audit trail when a step
// is skipped because the requester is the only one who could decide it.
const soleApproverSkipReason = "người tạo là người duyệt duy nhất"

// stepAt returns the step with this order, or nil past the end of the chain.
func stepAt(steps []*Step, order int) *Step {
	for _, st := range steps {
		if st.StepOrder == order {
			return st
		}
	}
	return nil
}

// moveOn takes a request whose step `from` is finished (approved or skipped) to
// what follows: the next step, started with startStep, or, after the last
// step, the template's normal end, "approved". The advance is a compare-and-
// swap: concurrent approvals can both satisfy the same quorum, and only the
// winner goes on, otherwise every approval past the quorum would add another
// full set of approvers. Only the caller that actually moved the request out of
// "pending" logs the completion.
func (s *Service) moveOn(ctx context.Context, req *Request, steps []*Step, from int, tenantID string) error {
	next := from + 1
	if stepAt(steps, next) == nil {
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
	advanced, err := s.store.AdvanceStep(ctx, req.ID, from, next)
	if err != nil {
		return fmt.Errorf("advance step: %w", err)
	}
	if !advanced {
		return nil
	}
	return s.startStep(ctx, req, steps, next, tenantID)
}

// startStep gives a step that has just become the request's current step its
// approvers. When the requester is the only one who could decide it (see
// requesterIsSoleApprover), nobody can: the step is skipped instead, recorded
// on the trail with no actor (the system) and the reason, and the request moves
// on, which may complete it. Eligibility is read from the graph now, when the
// step starts. A step that resolves to nobody is ErrInvalidInput, as before.
func (s *Service) startStep(ctx context.Context, req *Request, steps []*Step, order int, tenantID string) error {
	step := stepAt(steps, order)
	if step == nil {
		return nil
	}
	rows, err := s.stepAssignments(ctx, req, step, tenantID)
	if err != nil {
		return err
	}
	sole, err := s.requesterIsSoleApprover(ctx, rows[0], req.CreatedBy)
	if err != nil {
		return err
	}
	if sole {
		return s.skipStep(ctx, req, steps, order, tenantID)
	}
	if err := s.store.InsertAssignments(ctx, rows); err != nil {
		return fmt.Errorf("insert assignments: %w", err)
	}
	return s.auditAssigned(ctx, req.ID, order, rows)
}

// skipStep records that the step was skipped and moves the request on. It
// records no decision: the skip is not an approval by anyone, and the
// requester's own action never counts towards any quorum.
func (s *Service) skipStep(ctx context.Context, req *Request, steps []*Step, order int, tenantID string) error {
	slog.Info("approval step skipped: the requester is its only approver",
		"request_id", req.ID, "step", order)
	if err := s.logAudit(ctx, req.ID, "step_skipped", "", order, map[string]string{
		"reason": soleApproverSkipReason,
	}); err != nil {
		return err
	}
	return s.moveOn(ctx, req, steps, order, tenantID)
}

// stepAssignments resolves a step of the request to its assignment rows.
func (s *Service) stepAssignments(ctx context.Context, req *Request, step *Step, tenantID string) ([]*AssignmentRecord, error) {
	deptUA, err := s.placeholderUA(ctx, tenantID, step, req.DepartmentID)
	if err != nil {
		return nil, err
	}
	return buildAssignments(req.ID, step, deptUA)
}

// requesterIsSoleApprover reports whether the requester is the only person who
// could decide a step's row: a person row naming them, or a role or department
// whose only member (directly or below) is the requester. A group with any
// other member is not sole; neither is an empty group the requester is not in
// (it waits for someone to join, as before). Membership comes from the graph
// tables; a failure to read it is an error, never a skip.
func (s *Service) requesterIsSoleApprover(ctx context.Context, row *AssignmentRecord, requester string) (bool, error) {
	if !isGroupRow(row) {
		return row.UserNodeID == requester, nil
	}
	members, err := s.store.GroupMembers(ctx, row.UserNodeID)
	if err != nil {
		return false, fmt.Errorf("%w: read the members of the approving group: %v", ErrUnavailable, err)
	}
	in := false
	for _, m := range members {
		if m != requester {
			return false, nil
		}
		in = true
	}
	return in, nil
}

// placeholderUA returns the UA the placeholder of a step stands for: the UA of
// the request's department, checked to be a department of the tenant.
// A step without the placeholder, or with it on a step that is not a
// department's, needs nothing ("" and no error; buildAssignments refuses the
// latter). The department is never one a client named: CreateApprovalRequest
// reads it from the graph.
func (s *Service) placeholderUA(ctx context.Context, tenantID string, step *Step, deptID string) (string, error) {
	if !isPlaceholder(step.ApproverValue) || step.ApproverType != ApproverDepartment {
		return "", nil
	}
	if deptID == "" || deptID == noDepartment {
		return "", fmt.Errorf("step %d: the requester is in no single department: %w", step.StepOrder, ErrInvalidInput)
	}
	ua, err := s.store.CanonicalApprover(ctx, tenantID, ApproverDepartment, deptID)
	if err != nil {
		return "", fmt.Errorf("step %d: requester's department: %w", step.StepOrder, err)
	}
	return ua, nil
}

// buildAssignments turns a step into its assignment rows. deptUA is what the
// placeholder resolved to (see placeholderUA) and is used for nothing else.
//
// A named person gets a "direct" row. A role or a department gets ONE group
// row whose user_node_id is the role's or department's UA and whose grant
// source is role:<ua> / department:<ua>; whoever belongs to that UA when they
// act gets a row of their own then (see actingRow). A step that names nobody is
// ErrInvalidInput, so it can be refused before anything is stored.
func buildAssignments(requestID string, step *Step, deptUA string) ([]*AssignmentRecord, error) {
	value := step.ApproverValue
	if isPlaceholder(value) {
		if step.ApproverType != ApproverDepartment {
			return nil, fmt.Errorf("step %d: %s may only name a department step: %w", step.StepOrder, creatorDeptPlaceholder, ErrInvalidInput)
		}
		value = deptUA
	}
	if value == "" || value == noDepartment {
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
