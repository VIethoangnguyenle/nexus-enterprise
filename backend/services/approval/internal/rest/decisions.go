// Package rest provides Echo REST handlers for the approval service.
// Each handler: parse → validate → delegate → respond.
package rest

import (
	"context"
	"log/slog"
	"net/http"

	"github.com/labstack/echo/v4"

	"ngac-platform/pkg/httputil"
	"ngac-platform/services/approval/internal/domain"
	"ngac-platform/services/approval/internal/events"
)

// CreateRequest handles POST /api/approval/requests.
func (h *Handler) CreateRequest(c echo.Context) error {
	claims, err := httputil.RequireClaims(c)
	if err != nil {
		return err
	}

	var body struct {
		TemplateID   string `json:"template_id"`
		EntityType   string `json:"entity_type"`
		EntityID     string `json:"entity_id"`
		FormDataJSON string `json:"form_data_json"`
		ScopeOAID    string `json:"scope_oa_id"`
		DepartmentID string `json:"department_id"`
	}
	if err := c.Bind(&body); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid request body")
	}

	req, err := h.svc.CreateApprovalRequest(c.Request().Context(), domain.CreateRequestInput{
		TemplateID:   body.TemplateID,
		EntityType:   body.EntityType,
		EntityID:     body.EntityID,
		FormDataJSON: body.FormDataJSON,
		ScopeOAID:    body.ScopeOAID,
		DepartmentID: body.DepartmentID,
		CreatedBy:    claims.NGACNodeID,
	})
	if err != nil {
		return mapDomainError(err)
	}

	// Publish event after DB commit (fire-and-forget)
	h.publishEvent(c.Request().Context(), claims, req.ID, "created", "")

	// The snapshot is the server's record of the template; the client reads the
	// request's chain from GET /requests/:id.
	req.TemplateSnapshot = ""
	h.fillNames(c.Request().Context(), named{requests: []*domain.Request{req}})
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
	h.publishEvent(c.Request().Context(), claims, body.RequestID, "approved", "")

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
	h.publishEvent(c.Request().Context(), claims, body.RequestID, "rejected", body.Comment)

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
		h.publishEvent(c.Request().Context(), claims, reqID, "approved", "")
	}

	return c.JSON(http.StatusOK, map[string]any{
		"approved_count": len(approved),
		"approved_ids":   approved,
	})
}

// publishEvent announces a lifecycle action that has already been committed.
//
// The event names who it concerns so consumers can deliver it to them alone:
// the actor, the requester (created_by), and the approvers now pending on the
// request's current step — after an approval that advanced the step, that is
// the next approver. tenant_id is the tenant of the acting request's JWT, so a
// consumer can also keep the event inside that tenant.
//
// If the request cannot be re-read the event still goes out with what the
// action itself carries; the consumer then reaches only the actor.
func (h *Handler) publishEvent(ctx context.Context, claims *httputil.Claims, requestID, action, comment string) {
	if h.producer == nil {
		return
	}
	evt := events.ApprovalEventPayload{
		RequestID:   requestID,
		Action:      action,
		ActorNodeID: claims.NGACNodeID,
		TenantID:    claims.TenantID,
		WorkspaceID: claims.TenantID,
		Comment:     comment,
	}
	aud, err := h.svc.EventAudience(ctx, requestID)
	if err != nil {
		slog.Warn("approval event audience unavailable; publishing actor-only event",
			"request_id", requestID, "action", action, "error", err)
	} else {
		evt.TemplateName = aud.Request.TemplateName
		evt.EntityType = aud.Request.EntityType
		evt.Status = aud.Request.Status
		evt.CreatedBy = aud.Request.CreatedBy
		evt.ScopeOaID = aud.Request.ScopeOAID
		evt.AssigneeNodeIDs = aud.AssigneeNodeIDs
	}
	h.producer.Publish(ctx, evt)
}
