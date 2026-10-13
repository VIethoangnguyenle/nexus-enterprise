// Package rest — conversation list previews.
package rest

import (
	"net/http"

	"github.com/labstack/echo/v4"

	"ngac-platform/pkg/httputil"
)

// ConversationActivity handles GET /api/channels/activity: for each
// conversation the caller reads in the workspace they are signed in to (and
// their direct messages), its newest main-stream message and, when newer, its
// newest thread reply.
func (h *Handler) ConversationActivity(c echo.Context) error {
	claims, err := httputil.RequireClaims(c)
	if err != nil {
		return err
	}
	channels, err := h.svc.ConversationActivity(c.Request().Context(), claims.UserID, claims.NGACNodeID, claims.TenantID)
	if err != nil {
		return httputil.MapDomainError(err)
	}
	return c.JSON(http.StatusOK, map[string]any{"channels": channels})
}
