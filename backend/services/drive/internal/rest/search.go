package rest

import (
	"net/http"
	"strconv"

	"github.com/labstack/echo/v4"

	"ngac-platform/pkg/httputil"
)

// Search handles GET /api/drive/search?workspace_id=…&q=…&limit=N.
// It returns drive items matching the query that the caller may read.
func (h *Handler) Search(c echo.Context) error {
	_, err := httputil.RequireClaims(c)
	if err != nil {
		return err
	}

	workspaceID := c.QueryParam("workspace_id")
	if workspaceID == "" {
		return echo.NewHTTPError(http.StatusBadRequest, "workspace_id is required")
	}

	query := c.QueryParam("q")
	if query == "" {
		return echo.NewHTTPError(http.StatusBadRequest, "q is required")
	}

	limit := 20
	if raw := c.QueryParam("limit"); raw != "" {
		n, err := strconv.Atoi(raw)
		if err != nil || n < 1 {
			return echo.NewHTTPError(http.StatusBadRequest, "limit must be a positive number")
		}
		if n > 50 {
			n = 50
		}
		limit = n
	}

	resp, err := h.svc.SearchItems(c.Request().Context(), workspaceID, query, limit)
	if err != nil {
		return mapError(err)
	}
	return c.JSON(http.StatusOK, resp)
}
