// Package rest provides Echo REST handlers for the approval service.
// Each handler: parse → validate → delegate → respond.
package rest

import (
	"net/http"
	"strconv"

	"github.com/labstack/echo/v4"

	"ngac-platform/pkg/httputil"
)

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
	h.fillNames(c.Request().Context(), withAssignments(items))
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
	h.fillNames(c.Request().Context(), withAssignments(items))
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
	h.fillNames(c.Request().Context(), named{requests: items})
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
	h.fillNames(c.Request().Context(), named{requests: items})
	return c.JSON(http.StatusOK, map[string]any{
		"items": items, "next_cursor": nextCursor,
	})
}

// GetAuditLog handles GET /api/approval/requests/:id/audit.
func (h *Handler) GetAuditLog(c echo.Context) error {
	claims, err := httputil.RequireClaims(c)
	if err != nil {
		return err
	}
	entries, err := h.svc.GetAuditLog(c.Request().Context(), claims.NGACNodeID, c.Param("id"))
	if err != nil {
		return mapDomainError(err)
	}
	h.fillNames(c.Request().Context(), named{audit: entries})
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
