// Package rest provides Echo REST handlers for the approval service.
// Each handler: parse → validate → delegate → respond.
package rest

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/labstack/echo/v4"

	"ngac-platform/pkg/httputil"
	"ngac-platform/services/approval/internal/domain"
	"ngac-platform/services/approval/internal/events"
)

// mapDomainError translates domain errors to HTTP errors. Extends
// httputil.MapDomainError with approval-specific sentinel errors.
func mapDomainError(err error) *echo.HTTPError {
	switch {
	case errors.Is(err, domain.ErrStepNotActive):
		return echo.NewHTTPError(http.StatusConflict, err.Error())
	case errors.Is(err, domain.ErrRequestCompleted):
		return echo.NewHTTPError(http.StatusConflict, err.Error())
	case errors.Is(err, domain.ErrNoMatchingTemplate):
		return echo.NewHTTPError(http.StatusNotFound, err.Error())
	default:
		return httputil.MapDomainError(err)
	}
}

// Handler serves approval REST endpoints.
type Handler struct {
	svc      *domain.Service
	resolver *httputil.TenantSchemaResolver
	producer *events.Producer
}

// NewHandler creates an approval REST handler with tenant schema resolution.
func NewHandler(svc *domain.Service, resolver *httputil.TenantSchemaResolver, producer *events.Producer) *Handler {
	return &Handler{svc: svc, resolver: resolver, producer: producer}
}

// RegisterRoutes mounts approval endpoints on the Echo instance.
func (h *Handler) RegisterRoutes(e *echo.Echo, jwtSecret string) {
	// Admin routes — JWT only, no tenant schema required
	admin := e.Group("/api/admin", httputil.JWTMiddleware(jwtSecret))
	admin.POST("/tenants/:id/provision", h.ProvisionTenant)

	// Tenant-scoped routes — JWT + tenant_id + schema resolution
	api := e.Group("/api",
		httputil.JWTMiddleware(jwtSecret),
		httputil.TenantMiddleware(),
		h.tenantSchemaMiddleware(),
	)

	approval := api.Group("/approval")

	// Template management (admin)
	approval.POST("/templates", h.CreateTemplate)
	approval.GET("/templates", h.ListTemplates)
	approval.GET("/templates/:id", h.GetTemplate)
	approval.PUT("/templates/:id", h.UpdateTemplate)

	// Approval lifecycle
	approval.POST("/requests", h.CreateRequest)
	approval.POST("/approve", h.ApproveAction)
	approval.POST("/reject", h.RejectAction)
	approval.POST("/batch-approve", h.BatchApproveAction)

	// Query tabs
	approval.GET("/pending", h.GetPending)
	approval.GET("/history", h.GetHistory)
	approval.GET("/my-requests", h.GetMyRequests)
	approval.GET("/department-requests", h.GetDepartmentRequests)

	// Audit
	approval.GET("/requests/:id/audit", h.GetAuditLog)
}

// tenantSchemaMiddleware resolves the tenant's schema and stores it in context.
func (h *Handler) tenantSchemaMiddleware() echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			claims := httputil.GetClaims(c)
			schema, err := h.resolver.Resolve(c.Request().Context(), claims.TenantID)
			if err != nil {
				return echo.NewHTTPError(http.StatusNotFound, "tenant schema not provisioned")
			}
			ctx := httputil.WithTenantSchema(c.Request().Context(), schema)
			c.SetRequest(c.Request().WithContext(ctx))
			return next(c)
		}
	}
}

// ProvisionTenant handles POST /api/admin/tenants/:id/provision.
func (h *Handler) ProvisionTenant(c echo.Context) error {
	tenantID := c.Param("id")
	if tenantID == "" {
		return echo.NewHTTPError(http.StatusBadRequest, "tenant_id required")
	}

	schema, err := h.svc.ProvisionTenantSchema(c.Request().Context(), tenantID)
	if err != nil {
		return mapDomainError(err)
	}

	h.resolver.Invalidate(tenantID)

	return c.JSON(http.StatusCreated, map[string]string{
		"tenant_id":   tenantID,
		"schema_name": schema,
		"status":      "active",
	})
}

// --- Template endpoints ---

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

	return c.JSON(http.StatusCreated, t)
}

// GetTemplate handles GET /api/approval/templates/:id.
func (h *Handler) GetTemplate(c echo.Context) error {
	t, err := h.svc.GetTemplate(c.Request().Context(), c.Param("id"))
	if err != nil {
		return mapDomainError(err)
	}
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
	return c.JSON(http.StatusOK, map[string]any{"templates": templates})
}

// UpdateTemplate handles PUT /api/approval/templates/:id.
func (h *Handler) UpdateTemplate(c echo.Context) error {
	var body struct {
		Name     string `json:"name"`
		IsActive bool   `json:"is_active"`
		Priority int    `json:"priority"`
		FormFields []struct {
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
	if err := c.Bind(&body); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid request body")
	}

	in := domain.UpdateTemplateInput{
		Name: body.Name, IsActive: body.IsActive, Priority: body.Priority,
	}
	for _, ff := range body.FormFields {
		in.FormFields = append(in.FormFields, domain.FormFieldInput{
			Label: ff.Label, FieldType: ff.FieldType,
			Required: ff.Required, Options: ff.Options, Placeholder: ff.Placeholder,
		})
	}
	for _, step := range body.Steps {
		in.Steps = append(in.Steps, domain.StepInput{
			StepOrder: step.StepOrder, Name: step.Name,
			ApproverType: step.ApproverType, ApproverValue: step.ApproverValue,
			RequiredCount: step.RequiredCount, TimeoutHours: step.TimeoutHours,
		})
	}
	for _, c2 := range body.Conditions {
		in.Conditions = append(in.Conditions, domain.ConditionInput{
			Field: c2.Field, Operator: c2.Operator, Value: c2.Value,
		})
	}

	t, err := h.svc.UpdateTemplate(c.Request().Context(), c.Param("id"), in)
	if err != nil {
		return mapDomainError(err)
	}
	return c.JSON(http.StatusOK, t)
}

// --- Approval lifecycle endpoints ---

// CreateRequest handles POST /api/approval/requests.
func (h *Handler) CreateRequest(c echo.Context) error {
	claims, err := httputil.RequireClaims(c)
	if err != nil {
		return err
	}

	var body struct {
		EntityType   string            `json:"entity_type"`
		EntityID     string            `json:"entity_id"`
		EntityFields map[string]string `json:"entity_fields"`
		FormDataJSON string            `json:"form_data_json"`
		ScopeOAID    string            `json:"scope_oa_id"`
		DepartmentID string            `json:"department_id"`
	}
	if err := c.Bind(&body); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid request body")
	}

	req, err := h.svc.CreateApprovalRequest(c.Request().Context(), domain.CreateRequestInput{
		EntityType:   body.EntityType,
		EntityID:     body.EntityID,
		EntityFields: body.EntityFields,
		FormDataJSON: body.FormDataJSON,
		ScopeOAID:    body.ScopeOAID,
		DepartmentID: body.DepartmentID,
		CreatedBy:    claims.NGACNodeID,
	})
	if err != nil {
		return mapDomainError(err)
	}

	// Publish event after DB commit (fire-and-forget)
	h.producer.Publish(c.Request().Context(), events.ApprovalEventPayload{
		RequestID:    req.ID,
		TemplateName: req.TemplateName,
		EntityType:   req.EntityType,
		Status:       req.Status,
		Action:       "created",
		ActorNodeID:  claims.NGACNodeID,
		CreatedBy:    claims.NGACNodeID,
		ScopeOaID:    req.ScopeOAID,
	})

	return c.JSON(http.StatusCreated, req)
}

// ApproveAction handles POST /api/approval/approve.
func (h *Handler) ApproveAction(c echo.Context) error {
	claims, err := httputil.RequireClaims(c)
	if err != nil {
		return err
	}

	var body struct {
		RequestID string `json:"request_id"`
		Comment   string `json:"comment"`
	}
	if err := c.Bind(&body); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid request body")
	}

	if err := h.svc.Approve(c.Request().Context(), domain.ApproveInput{
		RequestID: body.RequestID, UserNodeID: claims.NGACNodeID, Comment: body.Comment,
	}); err != nil {
		return mapDomainError(err)
	}

	// Publish event after DB commit (fire-and-forget)
	h.producer.Publish(c.Request().Context(), events.ApprovalEventPayload{
		RequestID:   body.RequestID,
		Action:      "approved",
		ActorNodeID: claims.NGACNodeID,
	})

	return c.JSON(http.StatusOK, map[string]string{"status": "approved"})
}

// RejectAction handles POST /api/approval/reject.
func (h *Handler) RejectAction(c echo.Context) error {
	claims, err := httputil.RequireClaims(c)
	if err != nil {
		return err
	}

	var body struct {
		RequestID string `json:"request_id"`
		Comment   string `json:"comment"`
	}
	if err := c.Bind(&body); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid request body")
	}

	if err := h.svc.Reject(c.Request().Context(), domain.RejectInput{
		RequestID: body.RequestID, UserNodeID: claims.NGACNodeID, Comment: body.Comment,
	}); err != nil {
		return mapDomainError(err)
	}

	// Publish event after DB commit (fire-and-forget)
	h.producer.Publish(c.Request().Context(), events.ApprovalEventPayload{
		RequestID:   body.RequestID,
		Status:      "rejected",
		Action:      "rejected",
		ActorNodeID: claims.NGACNodeID,
		Comment:     body.Comment,
	})

	return c.JSON(http.StatusOK, map[string]string{"status": "rejected"})
}

// BatchApproveAction handles POST /api/approval/batch-approve.
func (h *Handler) BatchApproveAction(c echo.Context) error {
	claims, err := httputil.RequireClaims(c)
	if err != nil {
		return err
	}

	var body struct {
		RequestIDs []string `json:"request_ids"`
		Comment    string   `json:"comment"`
	}
	if err := c.Bind(&body); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid request body")
	}

	approved, err := h.svc.BatchApprove(c.Request().Context(), domain.BatchApproveInput{
		RequestIDs: body.RequestIDs, UserNodeID: claims.NGACNodeID, Comment: body.Comment,
	})
	if err != nil {
		return mapDomainError(err)
	}

	// Publish one event per approved request (fire-and-forget)
	for _, reqID := range approved {
		h.producer.Publish(c.Request().Context(), events.ApprovalEventPayload{
			RequestID:   reqID,
			Action:      "approved",
			ActorNodeID: claims.NGACNodeID,
		})
	}

	return c.JSON(http.StatusOK, map[string]any{
		"approved_count": len(approved),
		"approved_ids":   approved,
	})
}

// --- Query tab endpoints ---

// GetPending handles GET /api/approval/pending.
func (h *Handler) GetPending(c echo.Context) error {
	claims, err := httputil.RequireClaims(c)
	if err != nil {
		return err
	}

	items, err := h.svc.GetPending(c.Request().Context(), claims.NGACNodeID)
	if err != nil {
		return mapDomainError(err)
	}
	return c.JSON(http.StatusOK, map[string]any{"items": items, "total": len(items)})
}

// GetHistory handles GET /api/approval/history.
func (h *Handler) GetHistory(c echo.Context) error {
	claims, err := httputil.RequireClaims(c)
	if err != nil {
		return err
	}

	cursor := c.QueryParam("cursor")
	limit := parseLimit(c.QueryParam("limit"))

	items, nextCursor, err := h.svc.GetHistory(c.Request().Context(), claims.NGACNodeID, cursor, limit)
	if err != nil {
		return mapDomainError(err)
	}
	return c.JSON(http.StatusOK, map[string]any{
		"items": items, "next_cursor": nextCursor,
	})
}

// GetMyRequests handles GET /api/approval/my-requests.
func (h *Handler) GetMyRequests(c echo.Context) error {
	claims, err := httputil.RequireClaims(c)
	if err != nil {
		return err
	}

	cursor := c.QueryParam("cursor")
	limit := parseLimit(c.QueryParam("limit"))

	items, nextCursor, err := h.svc.GetMyRequests(c.Request().Context(), claims.NGACNodeID, cursor, limit)
	if err != nil {
		return mapDomainError(err)
	}
	return c.JSON(http.StatusOK, map[string]any{
		"items": items, "next_cursor": nextCursor,
	})
}

// GetDepartmentRequests handles GET /api/approval/department-requests.
func (h *Handler) GetDepartmentRequests(c echo.Context) error {
	claims, err := httputil.RequireClaims(c)
	if err != nil {
		return err
	}

	cursor := c.QueryParam("cursor")
	limit := parseLimit(c.QueryParam("limit"))

	items, nextCursor, err := h.svc.GetDepartmentRequests(c.Request().Context(), claims.NGACNodeID, cursor, limit)
	if err != nil {
		return mapDomainError(err)
	}
	return c.JSON(http.StatusOK, map[string]any{
		"items": items, "next_cursor": nextCursor,
	})
}

// GetAuditLog handles GET /api/approval/requests/:id/audit.
func (h *Handler) GetAuditLog(c echo.Context) error {
	entries, err := h.svc.GetAuditLog(c.Request().Context(), c.Param("id"))
	if err != nil {
		return mapDomainError(err)
	}
	return c.JSON(http.StatusOK, map[string]any{"entries": entries})
}

// parseLimit extracts a limit from a query param with a sensible default.
func parseLimit(s string) int {
	if s == "" {
		return 20
	}
	n, err := strconv.Atoi(s)
	if err != nil || n <= 0 || n > 100 {
		return 20
	}
	return n
}
