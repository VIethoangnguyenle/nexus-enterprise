package rest

import (
	"net/http"
	"strings"

	"github.com/labstack/echo/v4"

	"ngac-platform/pkg/httputil"
)

// SharedWithMeBy handles GET /api/workspaces/:id/drive/shared-with-me?owner=<user id>:
// the items of the workspace that person owns and shared with the caller, each
// re-checked for read. A caller outside the workspace is 403.
func (h *Handler) SharedWithMeBy(c echo.Context) error {
	if _, err := httputil.RequireClaims(c); err != nil {
		return err
	}
	owner := strings.TrimSpace(c.QueryParam("owner"))
	if owner == "" {
		return echo.NewHTTPError(http.StatusBadRequest, "owner is required")
	}
	resp, err := h.svc.SharedWithMeBy(c.Request().Context(), c.Param("id"), owner)
	if err != nil {
		return mapError(err)
	}
	return c.JSON(http.StatusOK, resp)
}
