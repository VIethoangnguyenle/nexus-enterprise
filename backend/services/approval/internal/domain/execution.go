package domain

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/google/uuid"
)

// CreateRequestInput contains the fields to create a new approval request.
type CreateRequestInput struct {
	// TemplateID, when set, is the template the submitter chose; the request
	// then follows it instead of being matched by entity type and conditions.
	TemplateID   string
	EntityType   string
	EntityID     string
	EntityFields EntityFields // for template matching
	FormDataJSON string       // JSON-encoded submitted form values
	ScopeOAID    string       // department-level OA for visibility
	DepartmentID string       // department ID for auditing
	CreatedBy    string       // user_node_id of the creator
}

// ApproveInput contains fields for a single approval action.
type ApproveInput struct {
	RequestID  string
	UserNodeID string
	Comment    string
}

// RejectInput contains fields for a rejection action.
type RejectInput struct {
	RequestID  string
	UserNodeID string
	Comment    string
}

// BatchApproveInput contains fields for batch approval.
type BatchApproveInput struct {
	RequestIDs []string
	UserNodeID string
	Comment    string
}

// CreateApprovalRequest creates a new approval request by:
// 1. Choosing the template: the one the submitter picked (if it fits), or the best match
// 2. Snapshotting the template
// 3. Building the first step's assignments
// 4. Storing request and assignments as one change
// 5. Logging audit
//
// Conditions are always judged on values the server derives from the submitted
// form (see fieldsFor), never on figures the client states separately: a form
// that says 500 million cannot be routed through the "under 10 million" chain.
func (s *Service) CreateApprovalRequest(ctx context.Context, in CreateRequestInput) (*Request, error) {
	if (in.EntityType == "" && in.TemplateID == "") || in.CreatedBy == "" {
		return nil, fmt.Errorf("entity_type or template_id, created_by: %w", ErrInvalidInput)
	}
	// A request about nothing outside the workflow (a leave request, an
	// advance) has no entity of its own; it gets an identity here instead of
	// making the client invent one.
	if in.EntityID == "" {
		in.EntityID = uuid.New().String()
	}
	// Default scope/department if not provided — workspace-level context
	// will supply these once OA/department features are implemented.
	if in.ScopeOAID == "" {
		in.ScopeOAID = "default"
	}
	if in.DepartmentID == "" {
		in.DepartmentID = "default"
	}
	form := parseForm(in.FormDataJSON)

	// 1. The chosen template, or the best match for the entity
	var tmpl *Template
	if in.TemplateID != "" {
		t, err := s.store.GetTemplate(ctx, in.TemplateID)
		if err != nil {
			return nil, fmt.Errorf("get template: %w", err)
		}
		if err := s.checkChosen(ctx, t, form, in.EntityFields); err != nil {
			return nil, err
		}
		tmpl = t
		in.EntityType = t.EntityType
	} else {
		t, err := s.resolveForForm(ctx, in.EntityType, form, in.EntityFields)
		if err != nil {
			return nil, err
		}
		tmpl = t
	}

	// 2. Snapshot the template
	snapshot, err := json.Marshal(tmpl)
	if err != nil {
		return nil, fmt.Errorf("snapshot template: %w", err)
	}

	now := time.Now()
	req := &Request{
		ID:               uuid.New().String(),
		EntityType:       in.EntityType,
		EntityID:         in.EntityID,
		TemplateID:       tmpl.ID,
		TemplateName:     tmpl.Name,
		TemplateSnapshot: string(snapshot),
		FormDataJSON:     in.FormDataJSON,
		CurrentStep:      1,
		Status:           "pending",
		ScopeOAID:        in.ScopeOAID,
		DepartmentID:     in.DepartmentID,
		CreatedBy:        in.CreatedBy,
		CreatedAt:        now,
	}

	// 3-4. Someone must be able to decide the request before it exists: a
	// request whose first step resolves to nobody is refused, not left pending.
	var first []*AssignmentRecord
	if len(tmpl.Steps) > 0 {
		if first, err = buildAssignments(req.ID, tmpl.Steps[0], in.DepartmentID); err != nil {
			return nil, fmt.Errorf("assign step 1: %w", err)
		}
	}
	if err := s.store.InsertRequestWithAssignments(ctx, req, first); err != nil {
		return nil, fmt.Errorf("insert request: %w", err)
	}

	// 5. Audit
	s.logAudit(ctx, req.ID, "created", in.CreatedBy, 0, map[string]string{
		"entity_type": in.EntityType,
		"entity_id":   in.EntityID,
		"template":    tmpl.Name,
	})
	s.auditAssigned(ctx, req.ID, 1, first)

	return req, nil
}

// actingRow finds the assignment the caller acts through on the request's
// current step, and says whether it is a group's row.
//
//   - The caller's own pending row (a named approver, or one added by
//     reconciliation): if it came from a role or department, the caller must
//     still belong to it.
//   - Otherwise a role or department row of the current step whose group is
//     among the caller's ancestors right now. Membership is read at the moment
//     of acting, so a member removed since the request was made cannot act, and
//     someone in a different department never could.
//
// Anything else is ErrAccessDenied.
func (s *Service) actingRow(ctx context.Context, req *Request, userNodeID string) (*AssignmentRecord, bool, error) {
	own, err := s.store.GetAssignment(ctx, req.ID, userNodeID)
	switch {
	case err == nil:
		if own.StepOrder != req.CurrentStep {
			return nil, false, ErrStepNotActive
		}
		if own.GrantSource != "direct" {
			if err := s.verifyMember(ctx, userNodeID, groupOf(own.GrantSource)); err != nil {
				return nil, false, err
			}
		}
		return own, false, nil
	case !errors.Is(err, ErrNotFound):
		return nil, false, fmt.Errorf("get assignment: %w", err)
	}

	ancestors, err := s.policy.GetAncestors(ctx, userNodeID)
	if err != nil {
		return nil, false, fmt.Errorf("ngac ancestors: %w", err)
	}
	group, err := s.store.FindGroupAssignment(ctx, req.ID, req.CurrentStep, ancestors)
	if errors.Is(err, ErrNotFound) {
		return nil, false, fmt.Errorf("%w: not an approver of this request", ErrAccessDenied)
	}
	if err != nil {
		return nil, false, fmt.Errorf("find group assignment: %w", err)
	}
	return group, true, nil
}

// decide records the caller's decision on the row they act through. A group
// row stays pending (other members may still act); the person gets a row of
// their own, which is what the quorum counts and what stops a second action.
func (s *Service) decide(ctx context.Context, row *AssignmentRecord, group bool, userNodeID, status, comment string) error {
	if !group {
		if err := s.store.UpdateAssignmentStatus(ctx, row.ID, status, comment); err != nil {
			return fmt.Errorf("update assignment: %w", err)
		}
		return nil
	}
	now := time.Now()
	err := s.store.InsertActedAssignment(ctx, &AssignmentRecord{
		ID: uuid.New().String(), RequestID: row.RequestID, StepOrder: row.StepOrder,
		UserNodeID: userNodeID, GrantSource: row.GrantSource, Status: status, ActedAt: &now, Comment: comment,
	})
	if errors.Is(err, ErrAlreadyExists) {
		return fmt.Errorf("%w: already acted on this step", ErrAlreadyExists)
	}
	if err != nil {
		return fmt.Errorf("record decision: %w", err)
	}
	return nil
}

// Approve processes a single approval action.
func (s *Service) Approve(ctx context.Context, in ApproveInput) error {
	if in.RequestID == "" || in.UserNodeID == "" {
		return ErrInvalidInput
	}

	req, err := s.store.GetRequest(ctx, in.RequestID)
	if err != nil {
		return fmt.Errorf("get request: %w", err)
	}
	if req.Status != "pending" {
		return ErrRequestCompleted
	}

	row, group, err := s.actingRow(ctx, req, in.UserNodeID)
	if err != nil {
		return err
	}
	if err := s.decide(ctx, row, group, in.UserNodeID, "approved", in.Comment); err != nil {
		return err
	}

	// Audit
	s.logAudit(ctx, in.RequestID, "approved", in.UserNodeID, req.CurrentStep, map[string]string{
		"comment": in.Comment,
	})

	// Check if step is complete
	return s.checkStepCompletion(ctx, req)
}

// Reject processes a rejection — immediately terminates the request.
func (s *Service) Reject(ctx context.Context, in RejectInput) error {
	if in.RequestID == "" || in.UserNodeID == "" {
		return ErrInvalidInput
	}

	req, err := s.store.GetRequest(ctx, in.RequestID)
	if err != nil {
		return fmt.Errorf("get request: %w", err)
	}
	if req.Status != "pending" {
		return ErrRequestCompleted
	}

	row, group, err := s.actingRow(ctx, req, in.UserNodeID)
	if err != nil {
		return err
	}
	if err := s.decide(ctx, row, group, in.UserNodeID, "rejected", in.Comment); err != nil {
		return err
	}

	// Skip ALL remaining pending assignments across all steps
	if err := s.store.SkipAllPendingAssignments(ctx, in.RequestID); err != nil {
		return fmt.Errorf("skip remaining: %w", err)
	}

	// Mark request as rejected (terminal)
	rejected, err := s.store.CompleteRequest(ctx, in.RequestID, "rejected")
	if err != nil {
		return fmt.Errorf("complete request: %w", err)
	}
	if !rejected {
		return ErrRequestCompleted
	}

	// Audit
	s.logAudit(ctx, in.RequestID, "rejected", in.UserNodeID, req.CurrentStep, map[string]string{
		"comment": in.Comment,
	})
	s.logAudit(ctx, in.RequestID, "completed", in.UserNodeID, 0, map[string]string{
		"final_status": "rejected",
	})

	return nil
}

// BatchApprove approves several requests in one call.
//
// It is a loop over Approve, not a bulk UPDATE, and deliberately so. Approve
// carries the guards that make an approval legitimate — the request is still
// pending, the assignment belongs to the step that is actually running, and the
// approver's role has not been revoked since the assignment was created. A
// separate bulk statement would be a second decision path that starts out
// equivalent and drifts, and while it drifted this endpoint would be the way
// around the checks the single-approve path enforces.
//
// Requests the caller may not approve are skipped rather than failing the whole
// batch: the caller gets back exactly the set that succeeded.
func (s *Service) BatchApprove(ctx context.Context, in BatchApproveInput) ([]string, error) {
	if len(in.RequestIDs) == 0 || in.UserNodeID == "" {
		return nil, ErrInvalidInput
	}

	var approved []string
	for _, reqID := range in.RequestIDs {
		err := s.Approve(ctx, ApproveInput{
			RequestID:  reqID,
			UserNodeID: in.UserNodeID,
			Comment:    in.Comment,
		})
		if err != nil {
			slog.Info("batch approve skipped a request",
				"request_id", reqID, "user_node_id", in.UserNodeID, "reason", err)
			continue
		}
		approved = append(approved, reqID)
	}

	return approved, nil
}

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

	s.logAudit(ctx, req.ID, "step_advanced", "", req.CurrentStep, map[string]string{
		"from_step": fmt.Sprintf("%d", req.CurrentStep),
	})

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
			s.logAudit(ctx, req.ID, "completed", "", 0, map[string]string{
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
	s.auditAssigned(ctx, requestID, step.StepOrder, assignments)
	return nil
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

func (s *Service) auditAssigned(ctx context.Context, requestID string, stepOrder int, rows []*AssignmentRecord) {
	for _, a := range rows {
		s.logAudit(ctx, requestID, "assigned", a.UserNodeID, stepOrder, map[string]string{
			"grant_source": a.GrantSource,
		})
	}
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

// logAudit is a fire-and-forget audit logger. Errors are swallowed since
// audit failures should not break the approval flow.
func (s *Service) logAudit(ctx context.Context, requestID, action, actorNodeID string, stepOrder int, detail map[string]string) {
	detailJSON, _ := json.Marshal(detail)
	s.store.InsertAuditEntry(ctx, &AuditEntry{
		ID:          uuid.New().String(),
		RequestID:   requestID,
		Action:      action,
		ActorNodeID: actorNodeID,
		StepOrder:   stepOrder,
		DetailJSON:  string(detailJSON),
		CreatedAt:   time.Now(),
	})
}
