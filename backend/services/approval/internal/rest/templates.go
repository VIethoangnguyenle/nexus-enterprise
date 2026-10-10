// Package rest provides Echo REST handlers for the approval service.
// Each handler: parse → validate → delegate → respond.
package rest

import (
	"net/http"
	"time"

	"github.com/labstack/echo/v4"

	"ngac-platform/pkg/httputil"
	"ngac-platform/services/approval/internal/domain"
)

// CreateTemplate handles POST /api/approval/templates.
func (h *Handler) CreateTemplate(c echo.Context) error {
	claims, err := httputil.RequireClaims(c)
	if err != nil {
		return err
	}

	var body struct {
		Name       string `json:"name"`
		EntityType string `json:"entity_type"`
		Priority   int    `json:"priority"`
		Conditions []struct {
			Field    string `json:"field"`
			Operator string `json:"operator"`
			Value    string `json:"value"`
		} `json:"conditions"`
		Steps []struct {
			StepOrder     int    `json:"step_order"`
			Name          string `json:"name"`
			ApproverType  string `json:"approver_type"`
			ApproverValue string `json:"approver_value"`
			RequiredCount int    `json:"required_count"`
			TimeoutHours  int    `json:"timeout_hours"`
		} `json:"steps"`
		FormFields []struct {
			Label       string `json:"label"`
			FieldType   string `json:"field_type"`
			Required    bool   `json:"required"`
			Options     string `json:"options"`
			Placeholder string `json:"placeholder"`
		} `json:"form_fields"`
	}
	if err := c.Bind(&body); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid request body")
	}

	in := domain.CreateTemplateInput{
		TenantID:   claims.TenantID,
		Name:       body.Name,
		EntityType: body.EntityType,
		Priority:   body.Priority,
	}
	for _, cond := range body.Conditions {
		in.Conditions = append(in.Conditions, domain.ConditionInput{
			Field: cond.Field, Operator: cond.Operator, Value: cond.Value,
		})
	}
	for _, step := range body.Steps {
		in.Steps = append(in.Steps, domain.StepInput{
			StepOrder: step.StepOrder, Name: step.Name,
			ApproverType: step.ApproverType, ApproverValue: step.ApproverValue,
			RequiredCount: step.RequiredCount, TimeoutHours: step.TimeoutHours,
		})
	}
	for _, ff := range body.FormFields {
		in.FormFields = append(in.FormFields, domain.FormFieldInput{
			Label: ff.Label, FieldType: ff.FieldType,
			Required: ff.Required, Options: ff.Options, Placeholder: ff.Placeholder,
		})
	}

	t, err := h.svc.CreateTemplate(c.Request().Context(), claims.NGACNodeID, in)
	if err != nil {
		return mapDomainError(err)
	}
	h.fillNames(c.Request().Context(), named{templates: []*domain.Template{t}})

	return c.JSON(http.StatusCreated, t)
}

// GetTemplate handles GET /api/approval/templates/:id.
func (h *Handler) GetTemplate(c echo.Context) error {
	t, err := h.svc.GetTemplate(c.Request().Context(), c.Param("id"))
	if err != nil {
		return mapDomainError(err)
	}
	h.fillNames(c.Request().Context(), named{templates: []*domain.Template{t}})
	return c.JSON(http.StatusOK, t)
}

// ListTemplates handles GET /api/approval/templates.
func (h *Handler) ListTemplates(c echo.Context) error {
	entityType := c.QueryParam("entity_type")
	activeOnly := c.QueryParam("active_only") != "false"

	templates, err := h.svc.ListTemplates(c.Request().Context(), entityType, activeOnly)
	if err != nil {
		return mapDomainError(err)
	}
	h.fillNames(c.Request().Context(), named{templates: templates})
	return c.JSON(http.StatusOK, map[string]any{"templates": templates})
}

// updateTemplateBody is the JSON of PUT /api/approval/templates/:id.
type updateTemplateBody struct {
	Name     string `json:"name"`
	IsActive bool   `json:"is_active"`
	Priority int    `json:"priority"`
	// ExpectedUpdatedAt is the template's updated_at as the client read it.
	ExpectedUpdatedAt time.Time `json:"expected_updated_at"`
	FormFields        []struct {
		Label       string `json:"label"`
		FieldType   string `json:"field_type"`
		Required    bool   `json:"required"`
		Options     string `json:"options"`
		Placeholder string `json:"placeholder"`
	} `json:"form_fields"`
	Steps []struct {
		StepOrder     int    `json:"step_order"`
		Name          string `json:"name"`
		ApproverType  string `json:"approver_type"`
		ApproverValue string `json:"approver_value"`
		RequiredCount int    `json:"required_count"`
		TimeoutHours  int    `json:"timeout_hours"`
	} `json:"steps"`
	Conditions []struct {
		Field    string `json:"field"`
		Operator string `json:"operator"`
		Value    string `json:"value"`
	} `json:"conditions"`
}

// formFieldsOf, stepsOf and conditionsOf convert the body's lists. A body that
// leaves a key out gives nil (the template keeps what it has); one that sends an
// empty list gives an empty, non-nil slice (clear it).
func formFieldsOf(b updateTemplateBody) []domain.FormFieldInput {
	if b.FormFields == nil {
		return nil
	}
	out := make([]domain.FormFieldInput, 0, len(b.FormFields))
	for _, ff := range b.FormFields {
		out = append(out, domain.FormFieldInput{
			Label: ff.Label, FieldType: ff.FieldType,
			Required: ff.Required, Options: ff.Options, Placeholder: ff.Placeholder,
		})
	}
	return out
}

func stepsOf(b updateTemplateBody) []domain.StepInput {
	if b.Steps == nil {
		return nil
	}
	out := make([]domain.StepInput, 0, len(b.Steps))
	for _, st := range b.Steps {
		out = append(out, domain.StepInput{
			StepOrder: st.StepOrder, Name: st.Name,
			ApproverType: st.ApproverType, ApproverValue: st.ApproverValue,
			RequiredCount: st.RequiredCount, TimeoutHours: st.TimeoutHours,
		})
	}
	return out
}

func conditionsOf(b updateTemplateBody) []domain.ConditionInput {
	if b.Conditions == nil {
		return nil
	}
	out := make([]domain.ConditionInput, 0, len(b.Conditions))
	for _, c := range b.Conditions {
		out = append(out, domain.ConditionInput{Field: c.Field, Operator: c.Operator, Value: c.Value})
	}
	return out
}

// UpdateTemplate handles PUT /api/approval/templates/:id. Only a caller with
// manage on the tenant may change a template (the domain decides).
func (h *Handler) UpdateTemplate(c echo.Context) error {
	claims, err := httputil.RequireClaims(c)
	if err != nil {
		return err
	}
	var body updateTemplateBody
	if err := c.Bind(&body); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid request body")
	}

	t, err := h.svc.UpdateTemplate(c.Request().Context(), claims.NGACNodeID, c.Param("id"), domain.UpdateTemplateInput{
		TenantID: claims.TenantID, Name: body.Name, IsActive: body.IsActive, Priority: body.Priority,
		FormFields: formFieldsOf(body), Steps: stepsOf(body), Conditions: conditionsOf(body),
		ExpectedUpdatedAt: body.ExpectedUpdatedAt,
	})
	if err != nil {
		return mapDomainError(err)
	}
	h.fillNames(c.Request().Context(), named{templates: []*domain.Template{t}})
	return c.JSON(http.StatusOK, t)
}
