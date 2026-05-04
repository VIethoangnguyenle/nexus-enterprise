package domain

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
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

// CreateTemplate validates and persists a new approval template.
func (s *Service) CreateTemplate(ctx context.Context, creatorNodeID string, in CreateTemplateInput) (*Template, error) {
	if in.Name == "" || in.EntityType == "" {
		return nil, fmt.Errorf("name and entity_type: %w", ErrInvalidInput)
	}
	if len(in.Steps) == 0 {
		return nil, fmt.Errorf("at least one step required: %w", ErrInvalidInput)
	}

	now := time.Now()
	t := &Template{
		ID:         uuid.New().String(),
		Name:       in.Name,
		EntityType: in.EntityType,
		IsActive:   true,
		Priority:   in.Priority,
		CreatedBy:  creatorNodeID,
		CreatedAt:  now,
		UpdatedAt:  now,
	}

	for _, c := range in.Conditions {
		t.Conditions = append(t.Conditions, &Condition{
			ID:       uuid.New().String(),
			Field:    c.Field,
			Operator: c.Operator,
			Value:    c.Value,
		})
	}

	for _, st := range in.Steps {
		t.Steps = append(t.Steps, &Step{
			ID:            uuid.New().String(),
			StepOrder:     st.StepOrder,
			Name:          st.Name,
			ApproverType:  st.ApproverType,
			ApproverValue: st.ApproverValue,
			RequiredCount: max(st.RequiredCount, 1),
			TimeoutHours:  st.TimeoutHours,
		})
	}

	for i, ff := range in.FormFields {
		t.FormFields = append(t.FormFields, &FormField{
			Label:       ff.Label,
			FieldType:   ff.FieldType,
			Required:    ff.Required,
			Options:     ff.Options,
			FieldOrder:  i + 1,
			Placeholder: ff.Placeholder,
		})
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
type UpdateTemplateInput struct {
	Name       string
	IsActive   bool
	Priority   int
	FormFields []FormFieldInput
	Steps      []StepInput
	Conditions []ConditionInput
}

// UpdateTemplate updates a template's metadata.
func (s *Service) UpdateTemplate(ctx context.Context, id string, in UpdateTemplateInput) (*Template, error) {
	if id == "" {
		return nil, ErrInvalidInput
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

	// Rebuild form fields if provided.
	if len(in.FormFields) > 0 {
		t.FormFields = nil
		for i, ff := range in.FormFields {
			t.FormFields = append(t.FormFields, &FormField{
				Label:       ff.Label,
				FieldType:   ff.FieldType,
				Required:    ff.Required,
				Options:     ff.Options,
				FieldOrder:  i + 1,
				Placeholder: ff.Placeholder,
			})
		}
	}

	// Rebuild steps if provided.
	if len(in.Steps) > 0 {
		t.Steps = nil
		for _, st := range in.Steps {
			t.Steps = append(t.Steps, &Step{
				ID:            uuid.New().String(),
				StepOrder:     st.StepOrder,
				Name:          st.Name,
				ApproverType:  st.ApproverType,
				ApproverValue: st.ApproverValue,
				RequiredCount: max(st.RequiredCount, 1),
				TimeoutHours:  st.TimeoutHours,
			})
		}
	}

	// Rebuild conditions if provided.
	if len(in.Conditions) > 0 {
		t.Conditions = nil
		for _, c := range in.Conditions {
			t.Conditions = append(t.Conditions, &Condition{
				ID:       uuid.New().String(),
				Field:    c.Field,
				Operator: c.Operator,
				Value:    c.Value,
			})
		}
	}

	if err := s.store.UpdateTemplate(ctx, t); err != nil {
		return nil, fmt.Errorf("update template: %w", err)
	}
	return t, nil
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
