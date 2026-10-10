package domain

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"time"

	"github.com/google/uuid"

	"ngac-platform/ngac"
)

// FormFieldInput defines a form field for template creation/update.
type FormFieldInput struct {
	Label       string `json:"label"`
	FieldType   string `json:"field_type"`
	Required    bool   `json:"required"`
	Options     string `json:"options"`
	Placeholder string `json:"placeholder"`
}

// CreateTemplateInput contains the fields needed to create a new approval template.
type CreateTemplateInput struct {
	// TenantID is the tenant the template belongs to; the caller must hold
	// manage on that tenant's management attribute.
	TenantID   string
	Name       string
	EntityType string
	Priority   int
	Conditions []ConditionInput
	Steps      []StepInput
	FormFields []FormFieldInput
}

// ConditionInput defines a condition for template creation.
type ConditionInput struct {
	Field    string
	Operator string
	Value    string
}

// StepInput defines a step for template creation.
type StepInput struct {
	StepOrder     int
	Name          string
	ApproverType  string
	ApproverValue string
	RequiredCount int
	TimeoutHours  int
}

// Approver kinds the server can resolve.
const (
	ApproverSpecificUser = "specific_user"
	ApproverRole         = "role_in_dept"
	ApproverDepartment   = "department"
)

var conditionOperators = map[string]bool{"eq": true, "gt": true, "gte": true, "lt": true, "lte": true, "in": true, "between": true}

// authorizeTemplates requires manage on the tenant's management attribute.
// Templates decide who approves what, so writing one is an administrator's act:
// without this any member could name themselves the approver of their own
// request. A tenant with no management attribute denies everyone.
func (s *Service) authorizeTemplates(ctx context.Context, callerNodeID, tenantID string) error {
	if callerNodeID == "" || tenantID == "" {
		return fmt.Errorf("%w: managing templates needs a signed-in member of the tenant", ErrAccessDenied)
	}
	mgmt, err := s.store.FindNodeID(ctx, ngac.MgmtOAName(ngac.WorkspaceID(tenantID)), ngac.TypeOA)
	if errors.Is(err, ErrNotFound) {
		return fmt.Errorf("%w: tenant has no management attribute", ErrAccessDenied)
	}
	if err != nil {
		return fmt.Errorf("find management attribute: %w", err)
	}
	allowed, err := s.policy.CheckAccess(ctx, callerNodeID, mgmt, ngac.OpManage)
	if err != nil {
		return fmt.Errorf("ngac check access: %w", err)
	}
	if !allowed {
		return fmt.Errorf("%w: managing templates needs %s on the tenant", ErrAccessDenied, ngac.OpManage)
	}
	return nil
}

// CanManageTemplates reports whether the caller may create and edit templates.
// A denial is an answer (false), not an error.
func (s *Service) CanManageTemplates(ctx context.Context, callerNodeID, tenantID string) (bool, error) {
	err := s.authorizeTemplates(ctx, callerNodeID, tenantID)
	if errors.Is(err, ErrAccessDenied) {
		return false, nil
	}
	return err == nil, err
}

// buildSteps validates a chain and returns its steps in order: orders are
// exactly 1..n with no repeats, every approver kind is one the server resolves,
// and every approver exists in the tenant (a department id becomes its UA).
func (s *Service) buildSteps(ctx context.Context, tenantID string, in []StepInput) ([]*Step, error) {
	if len(in) == 0 {
		return nil, fmt.Errorf("at least one step required: %w", ErrInvalidInput)
	}
	sorted := append([]StepInput(nil), in...)
	sort.SliceStable(sorted, func(i, j int) bool { return sorted[i].StepOrder < sorted[j].StepOrder })
	steps := make([]*Step, 0, len(sorted))
	for i, st := range sorted {
		if st.StepOrder != i+1 {
			return nil, fmt.Errorf("step_order must be exactly 1..%d without repeats: %w", len(sorted), ErrInvalidInput)
		}
		switch st.ApproverType {
		case ApproverSpecificUser, ApproverRole, ApproverDepartment:
		default:
			return nil, fmt.Errorf("approver_type %q is not supported: %w", st.ApproverType, ErrInvalidInput)
		}
		value := st.ApproverValue
		if value == "" {
			return nil, fmt.Errorf("step %d has no approver: %w", st.StepOrder, ErrInvalidInput)
		}
		if !isPlaceholder(value) {
			v, err := s.store.CanonicalApprover(ctx, tenantID, st.ApproverType, value)
			if err != nil {
				return nil, fmt.Errorf("step %d approver: %w", st.StepOrder, err)
			}
			value = v
		}
		steps = append(steps, &Step{
			ID:            uuid.New().String(),
			StepOrder:     st.StepOrder,
			Name:          st.Name,
			ApproverType:  st.ApproverType,
			ApproverValue: value,
			RequiredCount: max(st.RequiredCount, 1),
			TimeoutHours:  st.TimeoutHours,
		})
	}
	return steps, nil
}

// isPlaceholder reports whether an approver value is the "{creator_dept}"
// placeholder, which is filled in per request and so has nothing to look up yet.
func isPlaceholder(v string) bool { return v == "{creator_dept}" }

func buildConditions(in []ConditionInput) ([]*Condition, error) {
	out := make([]*Condition, 0, len(in))
	for _, c := range in {
		if c.Field == "" || !conditionOperators[c.Operator] || !json.Valid([]byte(c.Value)) {
			return nil, fmt.Errorf("condition %q %q: %w", c.Field, c.Operator, ErrInvalidInput)
		}
		out = append(out, &Condition{ID: uuid.New().String(), Field: c.Field, Operator: c.Operator, Value: c.Value})
	}
	return out, nil
}

func buildFormFields(in []FormFieldInput) []*FormField {
	out := make([]*FormField, 0, len(in))
	for i, ff := range in {
		out = append(out, &FormField{
			Label: ff.Label, FieldType: ff.FieldType, Required: ff.Required,
			Options: ff.Options, FieldOrder: i + 1, Placeholder: ff.Placeholder,
		})
	}
	return out
}

// CreateTemplate validates and persists a new approval template. The caller
// must hold manage on the tenant.
func (s *Service) CreateTemplate(ctx context.Context, creatorNodeID string, in CreateTemplateInput) (*Template, error) {
	if err := s.authorizeTemplates(ctx, creatorNodeID, in.TenantID); err != nil {
		return nil, err
	}
	if in.Name == "" || in.EntityType == "" {
		return nil, fmt.Errorf("name and entity_type: %w", ErrInvalidInput)
	}
	steps, err := s.buildSteps(ctx, in.TenantID, in.Steps)
	if err != nil {
		return nil, err
	}
	conds, err := buildConditions(in.Conditions)
	if err != nil {
		return nil, err
	}

	// Microsecond precision is what the database keeps; a finer stamp would not
	// round-trip as the update precondition.
	now := time.Now().Truncate(time.Microsecond)
	t := &Template{
		ID:         uuid.New().String(),
		Name:       in.Name,
		EntityType: in.EntityType,
		IsActive:   true,
		Priority:   in.Priority,
		Conditions: conds,
		Steps:      steps,
		FormFields: buildFormFields(in.FormFields),
		CreatedBy:  creatorNodeID,
		CreatedAt:  now,
		UpdatedAt:  now,
	}
	if err := s.store.InsertTemplate(ctx, t); err != nil {
		return nil, fmt.Errorf("insert template: %w", err)
	}
	return t, nil
}

// GetTemplate retrieves a template by ID.
func (s *Service) GetTemplate(ctx context.Context, id string) (*Template, error) {
	if id == "" {
		return nil, ErrInvalidInput
	}
	t, err := s.store.GetTemplate(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("get template: %w", err)
	}
	return t, nil
}

// ListTemplates returns templates filtered by entity type and active status.
func (s *Service) ListTemplates(ctx context.Context, entityType string, activeOnly bool) ([]*Template, error) {
	templates, err := s.store.ListTemplates(ctx, entityType, activeOnly)
	if err != nil {
		return nil, fmt.Errorf("list templates: %w", err)
	}
	return templates, nil
}

// UpdateTemplateInput contains the mutable fields for template update.
//
// FormFields, Steps and Conditions are tri-state: nil keeps what the template
// has, an empty list clears it (steps cannot be cleared: a template needs one).
// A template's entity type does not change.
type UpdateTemplateInput struct {
	TenantID   string
	Name       string
	IsActive   bool
	Priority   int
	FormFields []FormFieldInput
	Steps      []StepInput
	Conditions []ConditionInput
	// ExpectedUpdatedAt is the updated_at of the template as the caller read
	// it. If the template has changed since, the update is refused (ErrStale)
	// rather than overwriting someone else's edit.
	ExpectedUpdatedAt time.Time
}

// UpdateTemplate updates a template. The caller must hold manage on the tenant.
func (s *Service) UpdateTemplate(ctx context.Context, callerNodeID, id string, in UpdateTemplateInput) (*Template, error) {
	if err := s.authorizeTemplates(ctx, callerNodeID, in.TenantID); err != nil {
		return nil, err
	}
	if id == "" || in.ExpectedUpdatedAt.IsZero() {
		return nil, fmt.Errorf("template id and expected_updated_at: %w", ErrInvalidInput)
	}

	t, err := s.store.GetTemplate(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("get template for update: %w", err)
	}

	if in.Name != "" {
		t.Name = in.Name
	}
	t.IsActive = in.IsActive
	t.Priority = in.Priority

	if in.FormFields != nil {
		t.FormFields = buildFormFields(in.FormFields)
	}
	if in.Steps != nil {
		if t.Steps, err = s.buildSteps(ctx, in.TenantID, in.Steps); err != nil {
			return nil, err
		}
	}
	if in.Conditions != nil {
		if t.Conditions, err = buildConditions(in.Conditions); err != nil {
			return nil, err
		}
	}

	updatedAt, err := s.store.UpdateTemplate(ctx, t, in.ExpectedUpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("update template: %w", err)
	}
	t.UpdatedAt = updatedAt
	return t, nil
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
