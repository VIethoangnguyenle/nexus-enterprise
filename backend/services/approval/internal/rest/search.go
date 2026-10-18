package rest

import (
	"net/http"
	"strconv"

	"github.com/labstack/echo/v4"

	"ngac-platform/pkg/httputil"
	"ngac-platform/services/approval/internal/domain"
)

// Search handles GET /api/approval/search.
func (h *Handler) Search(c echo.Context) error {
	claims, err := httputil.RequireClaims(c)
	if err != nil {
		return err
	}

	// Parse all parameters first; validation that would produce a 400 comes
	// after the domain's organization check so that a personal workspace
	// always sees 403, never a misleading 400.
	p := domain.SearchParams{
		Query:      c.QueryParam("q"),
		Status:     c.QueryParam("status"),
		TemplateID: c.QueryParam("template_id"),
		CreatedBy:  c.QueryParam("created_by"),
		From:       c.QueryParam("from"),
		To:         c.QueryParam("to"),
		Limit:      parseSearchLimit(c.QueryParam("limit")),
	}

	items, total, err := h.svc.SearchRequests(c.Request().Context(), claims.NGACNodeID, p)
	if err != nil {
		return mapDomainError(err)
	}
	h.fillNames(c.Request().Context(), named{requests: items})
	return c.JSON(http.StatusOK, map[string]any{"items": items, "total": total})
}

// parseSearchLimit extracts a limit from a query param with search-specific
// defaults: 20 default, 50 max.
func parseSearchLimit(s string) int {
	if s == "" {
		return 20
	}
	n, err := strconv.Atoi(s)
	if err != nil || n <= 0 {
		return 20
	}
	if n > 50 {
		return 50
	}
	return n
}
