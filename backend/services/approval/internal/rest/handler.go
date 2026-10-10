// Package rest provides Echo REST handlers for the approval service.
// Each handler: parse → validate → delegate → respond.
package rest

import (
	"context"
	"errors"
	"net/http"

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
	case errors.Is(err, domain.ErrStale):
		return echo.NewHTTPError(http.StatusConflict, "template changed since it was read")
	default:
		return httputil.MapDomainError(err)
	}
}

// EventPublisher publishes approval lifecycle events. *events.Producer
// implements it (and tolerates a nil receiver).
type EventPublisher interface {
	Publish(ctx context.Context, evt events.ApprovalEventPayload)
}

// Handler serves approval REST endpoints.
type Handler struct {
	svc      *domain.Service
	resolver *httputil.TenantSchemaResolver
	producer EventPublisher
	names    NameResolver // optional; see WithNames
}

// NewHandler creates an approval REST handler with tenant schema resolution.
func NewHandler(svc *domain.Service, resolver *httputil.TenantSchemaResolver, producer EventPublisher) *Handler {
	return &Handler{svc: svc, resolver: resolver, producer: producer}
}

// RegisterRoutes mounts approval endpoints on the Echo instance.
func (h *Handler) RegisterRoutes(e *echo.Echo, jwtSecret string) {
	// Admin routes — JWT + a tenant context, but no schema resolution: the
	// schema is what this endpoint creates, so it cannot require one already.
	admin := e.Group("/api/admin",
		httputil.JWTMiddleware(jwtSecret),
		httputil.TenantMiddleware(),
	)
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
	approval.GET("/permissions", h.GetPermissions)

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
	approval.GET("/requests/:id", h.GetRequest)
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
	claims, err := httputil.RequireClaims(c)
	if err != nil {
		return err
	}

	tenantID := c.Param("id")
	if tenantID == "" {
		return echo.NewHTTPError(http.StatusBadRequest, "tenant_id required")
	}

	// Provisioning creates a whole Postgres schema. A caller may only do that
	// for the tenant they are currently signed in to — otherwise any
	// authenticated user of any tenant can create schemas for arbitrary IDs.
	if claims.TenantID != tenantID {
		return echo.NewHTTPError(http.StatusForbidden, "cannot provision another tenant")
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

// GetPermissions handles GET /api/approval/permissions: what the caller may do
// here, so the screen can leave out what the server would refuse.
func (h *Handler) GetPermissions(c echo.Context) error {
	claims, err := httputil.RequireClaims(c)
	if err != nil {
		return err
	}
	can, err := h.svc.CanManageTemplates(c.Request().Context(), claims.NGACNodeID, claims.TenantID)
	if err != nil {
		return mapDomainError(err)
	}
	return c.JSON(http.StatusOK, map[string]bool{"can_manage_templates": can})
}

// --- Approval lifecycle endpoints ---

// --- Query tab endpoints ---

// GetRequest handles GET /api/approval/requests/:id: one request with its
// chain of steps and every approver's assignment, for a caller who may see it.
func (h *Handler) GetRequest(c echo.Context) error {
	claims, err := httputil.RequireClaims(c)
	if err != nil {
		return err
	}
	d, err := h.svc.GetRequestDetail(c.Request().Context(), claims.NGACNodeID, c.Param("id"))
	if err != nil {
		return mapDomainError(err)
	}
	h.fillNames(c.Request().Context(), named{
		requests: []*domain.Request{d.Request}, assignments: d.Assignments, steps: d.Steps,
	})
	return c.JSON(http.StatusOK, d)
}
