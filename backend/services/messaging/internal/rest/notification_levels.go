// Package rest — the caller's notification level for a conversation.
package rest

import (
	"net/http"

	"github.com/labstack/echo/v4"

	"ngac-platform/pkg/httputil"
)

// GetNotificationLevel handles GET /api/channels/:chId/notification-level.
func (h *Handler) GetNotificationLevel(c echo.Context) error {
	claims, err := httputil.RequireClaims(c)
	if err != nil {
		return err
	}
	level, err := h.svc.GetNotificationLevel(c.Request().Context(), c.Param("chId"), claims.UserID, claims.NGACNodeID)
	if err != nil {
		return httputil.MapDomainError(err)
	}
	return c.JSON(http.StatusOK, map[string]string{"level": level})
}

// SetNotificationLevel handles PUT /api/channels/:chId/notification-level.
// Body: {"level": "all" | "main" | "off"}.
func (h *Handler) SetNotificationLevel(c echo.Context) error {
	claims, err := httputil.RequireClaims(c)
	if err != nil {
		return err
	}
	var body struct {
		Level string `json:"level"`
	}
	if err := c.Bind(&body); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid request body")
	}
	if err := h.svc.SetNotificationLevel(c.Request().Context(), c.Param("chId"), claims.UserID, claims.NGACNodeID, body.Level); err != nil {
		return httputil.MapDomainError(err)
	}
	return c.JSON(http.StatusOK, map[string]string{"level": body.Level})
}
