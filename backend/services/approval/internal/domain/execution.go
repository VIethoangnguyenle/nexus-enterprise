package domain

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
)

// CreateRequestInput contains the fields to create a new approval request.
type CreateRequestInput struct {
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
// 1. Matching a template
// 2. Snapshotting the template
// 3. Resolving approvers for step 1
// 4. Inserting assignments
// 5. Logging audit
func (s *Service) CreateApprovalRequest(ctx context.Context, in CreateRequestInput) (*Request, error) {
	if in.EntityType == "" || in.EntityID == "" || in.CreatedBy == "" {
		return nil, fmt.Errorf("entity_type, entity_id, created_by: %w", ErrInvalidInput)
	}
	// Default scope/department if not provided — workspace-level context
	// will supply these once OA/department features are implemented.
	if in.ScopeOAID == "" {
		in.ScopeOAID = "default"
	}
	if in.DepartmentID == "" {
		in.DepartmentID = "default"
	}

	// 1. Match template
	tmpl, err := s.ResolveTemplate(ctx, in.EntityType, in.EntityFields)
	if err != nil {
		return nil, err
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

	if err := s.store.InsertRequest(ctx, req); err != nil {
		return nil, fmt.Errorf("insert request: %w", err)
	}

	// 3. Resolve approvers for step 1 and insert assignments
	if len(tmpl.Steps) > 0 {
		if err := s.assignStep(ctx, req.ID, tmpl.Steps[0], in.DepartmentID); err != nil {
			return nil, fmt.Errorf("assign step 1: %w", err)
		}
	}

	// 4. Audit
	s.logAudit(ctx, req.ID, "created", in.CreatedBy, 0, map[string]string{
		"entity_type": in.EntityType,
		"entity_id":   in.EntityID,
		"template":    tmpl.Name,
	})

	return req, nil
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

	assignment, err := s.store.GetAssignment(ctx, in.RequestID, in.UserNodeID)
	if err != nil {
		return fmt.Errorf("get assignment: %w", err)
	}
	if assignment.StepOrder != req.CurrentStep {
		return ErrStepNotActive
	}

	// NGAC double-check: only for role-based grants where role could be revoked
	// after assignment. Direct assignments (specific_user) are self-authorizing.
	if assignment.GrantSource != "direct" {
		if err := s.verifyApproveAccess(ctx, in.UserNodeID, req.ScopeOAID); err != nil {
			return err
		}
	}

	// Update assignment
	if err := s.store.UpdateAssignmentStatus(ctx, assignment.ID, "approved", in.Comment); err != nil {
		return fmt.Errorf("update assignment: %w", err)
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

	assignment, err := s.store.GetAssignment(ctx, in.RequestID, in.UserNodeID)
	if err != nil {
		return fmt.Errorf("get assignment: %w", err)
	}
	if assignment.StepOrder != req.CurrentStep {
		return ErrStepNotActive
	}

	// NGAC double-check: only for role-based grants where role could be revoked.
	if assignment.GrantSource != "direct" {
		if err := s.verifyApproveAccess(ctx, in.UserNodeID, req.ScopeOAID); err != nil {
			return err
		}
	}

	// Update assignment
	if err := s.store.UpdateAssignmentStatus(ctx, assignment.ID, "rejected", in.Comment); err != nil {
		return fmt.Errorf("update assignment: %w", err)
	}

	// Skip ALL remaining pending assignments across all steps
	if err := s.store.SkipAllPendingAssignments(ctx, in.RequestID); err != nil {
		return fmt.Errorf("skip remaining: %w", err)
	}

	// Mark request as rejected (terminal)
	if err := s.store.CompleteRequest(ctx, in.RequestID, "rejected"); err != nil {
		return fmt.Errorf("complete request: %w", err)
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

// BatchApprove processes multiple approvals atomically.
func (s *Service) BatchApprove(ctx context.Context, in BatchApproveInput) ([]string, error) {
	if len(in.RequestIDs) == 0 || in.UserNodeID == "" {
		return nil, ErrInvalidInput
	}

	approved, err := s.store.BatchApproveAssignments(ctx, in.UserNodeID, in.RequestIDs, in.Comment)
	if err != nil {
		return nil, fmt.Errorf("batch approve: %w", err)
	}

	// Check step completion for each approved request
	for _, reqID := range approved {
		req, err := s.store.GetRequest(ctx, reqID)
		if err != nil {
			continue
		}
		s.logAudit(ctx, reqID, "approved", in.UserNodeID, req.CurrentStep, map[string]string{
			"comment": in.Comment,
			"batch":   "true",
		})
		s.checkStepCompletion(ctx, req)
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
		// No more steps — request fully approved
		if err := s.store.CompleteRequest(ctx, req.ID, "approved"); err != nil {
			return fmt.Errorf("complete request: %w", err)
		}
		s.logAudit(ctx, req.ID, "completed", "", 0, map[string]string{
			"final_status": "approved",
		})
		return nil
	}

	// Advance to next step
	if err := s.store.AdvanceStep(ctx, req.ID, nextStep); err != nil {
		return fmt.Errorf("advance step: %w", err)
	}

	// Resolve approvers for next step
	return s.assignStep(ctx, req.ID, nextStepDef, req.DepartmentID)
}

// assignStep resolves approvers for a step and inserts assignments.
func (s *Service) assignStep(ctx context.Context, requestID string, step *Step, deptID string) error {
	approverValue := ResolvePlaceholder(step.ApproverValue, deptID)

	var assignments []*AssignmentRecord

	switch step.ApproverType {
	case "specific_user":
		assignments = append(assignments, &AssignmentRecord{
			ID:          uuid.New().String(),
			RequestID:   requestID,
			StepOrder:   step.StepOrder,
			UserNodeID:  approverValue,
			GrantSource: "direct",
			Status:      "pending",
		})

	case "role_in_dept":
		// Resolve: get descendants of the role UA → find users
		scopes, err := s.policy.ResolveAccessibleScopes(ctx, approverValue, "approve")
		if err != nil {
			// Fallback: assign to the role itself
			assignments = append(assignments, &AssignmentRecord{
				ID:          uuid.New().String(),
				RequestID:   requestID,
				StepOrder:   step.StepOrder,
				UserNodeID:  approverValue,
				GrantSource: fmt.Sprintf("role:%s", approverValue),
				Status:      "pending",
			})
		} else {
			for _, scopeID := range scopes {
				assignments = append(assignments, &AssignmentRecord{
					ID:          uuid.New().String(),
					RequestID:   requestID,
					StepOrder:   step.StepOrder,
					UserNodeID:  scopeID,
					GrantSource: fmt.Sprintf("role:%s", approverValue),
					Status:      "pending",
				})
			}
		}

	case "department":
		// Assign to all members of the department UA
		assignments = append(assignments, &AssignmentRecord{
			ID:          uuid.New().String(),
			RequestID:   requestID,
			StepOrder:   step.StepOrder,
			UserNodeID:  approverValue,
			GrantSource: fmt.Sprintf("department:%s", approverValue),
			Status:      "pending",
		})

	default:
		return fmt.Errorf("unknown approver_type %q: %w", step.ApproverType, ErrInvalidInput)
	}

	if len(assignments) == 0 {
		return fmt.Errorf("no approvers resolved for step %d: %w", step.StepOrder, ErrInvalidInput)
	}

	if err := s.store.InsertAssignments(ctx, assignments); err != nil {
		return fmt.Errorf("insert assignments: %w", err)
	}

	// Audit each assignment
	for _, a := range assignments {
		s.logAudit(ctx, requestID, "assigned", a.UserNodeID, step.StepOrder, map[string]string{
			"grant_source": a.GrantSource,
		})
	}

	return nil
}

// verifyApproveAccess performs a real-time NGAC permission check before
// allowing an approve/reject action. This prevents stale role exploitation
// where a user's role was revoked after their assignment was created.
func (s *Service) verifyApproveAccess(ctx context.Context, userNodeID, scopeOAID string) error {
	allowed, err := s.policy.CheckAccess(ctx, userNodeID, scopeOAID, "approve")
	if err != nil {
		return fmt.Errorf("ngac check access: %w", err)
	}
	if !allowed {
		return fmt.Errorf("user no longer has approve permission on scope %s: %w", scopeOAID, ErrAccessDenied)
	}
	return nil
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
