package rest

import (
	"net/http"
	"strings"

	"github.com/labstack/echo/v4"

	"ngac-platform/pkg/httputil"
)

// GroupsInCommon handles GET /api/workspaces/:id/channels/in-common?with=<ngac node id>:
// the channels of the workspace that both the caller and that person are in, only
// those the caller is a member of and may read. A caller outside the workspace is 403.
func (h *Handler) GroupsInCommon(c echo.Context) error {
	claims, err := httputil.RequireClaims(c)
	if err != nil {
		return err
	}
	other := strings.TrimSpace(c.QueryParam("with"))
	if other == "" {
		return echo.NewHTTPError(http.StatusBadRequest, "with is required")
	}
	channels, err := h.svc.GroupsInCommon(c.Request().Context(), c.Param("id"), claims.NGACNodeID, other)
	if err != nil {
		return httputil.MapDomainError(err)
	}
	return c.JSON(http.StatusOK, map[string]any{"channels": channels})
}
