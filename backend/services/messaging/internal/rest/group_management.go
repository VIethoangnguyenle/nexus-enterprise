// Package rest — group management REST handlers.
//
// All routes sit under the existing /api/channels/:chId prefix: the dev proxy
// and nginx already route them. No proxy table change is needed.
package rest

import (
	"net/http"

	"github.com/labstack/echo/v4"

	"ngac-platform/pkg/httputil"
)

// registerGroupManagementRoutes mounts group management routes on the api group.
func (h *Handler) registerGroupManagementRoutes(api *echo.Group) {
	api.GET("/channels/:chId/management", h.GetGroupManagement)
	api.PUT("/channels/:chId/management", h.UpdateGroupSettings)
	api.POST("/channels/:chId/managers/:userId", h.AppointManager)
	api.DELETE("/channels/:chId/managers/:userId", h.RemoveManager)
	api.POST("/channels/:chId/transfer", h.TransferOwnership)
	api.DELETE("/channels/:chId", h.DisbandGroup)
}

// GetGroupManagement handles GET /api/channels/:chId/management.
func (h *Handler) GetGroupManagement(c echo.Context) error {
	claims, err := httputil.RequireClaims(c)
	if err != nil {
		return err
	}
	settings, err := h.svc.GetGroupManagement(c.Request().Context(), c.Param("chId"), claims.NGACNodeID)
	if err != nil {
		return httputil.MapDomainError(err)
	}
	return c.JSON(http.StatusOK, settings)
}

// UpdateGroupSettings handles PUT /api/channels/:chId/management.
func (h *Handler) UpdateGroupSettings(c echo.Context) error {
	claims, err := httputil.RequireClaims(c)
	if err != nil {
		return err
	}
	var body struct {
		PostPolicy       string `json:"post_policy"`
		InvitePolicy     string `json:"invite_policy"`
		MentionAllPolicy string `json:"mention_all_policy"`
	}
	if err := c.Bind(&body); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid request body")
	}
	if err := h.svc.UpdateGroupSettings(c.Request().Context(), c.Param("chId"), claims.NGACNodeID, body.PostPolicy, body.InvitePolicy, body.MentionAllPolicy); err != nil {
		return httputil.MapDomainError(err)
	}
	return c.JSON(http.StatusOK, map[string]string{"status": "ok"})
}

// AppointManager handles POST /api/channels/:chId/managers/:userId. :userId is
// the person's user ID, as the roster and GET management name them.
func (h *Handler) AppointManager(c echo.Context) error {
	claims, err := httputil.RequireClaims(c)
	if err != nil {
		return err
	}
	targetNodeID, err := h.svc.ResolveUserNode(c.Request().Context(), c.Param("userId"))
	if err != nil {
		return httputil.MapDomainError(err)
	}
	if err := h.svc.AppointManager(c.Request().Context(), c.Param("chId"), claims.NGACNodeID, targetNodeID); err != nil {
		return httputil.MapDomainError(err)
	}
	return c.JSON(http.StatusOK, map[string]string{"status": "ok"})
}

// RemoveManager handles DELETE /api/channels/:chId/managers/:userId (a user ID).
func (h *Handler) RemoveManager(c echo.Context) error {
	claims, err := httputil.RequireClaims(c)
	if err != nil {
		return err
	}
	targetNodeID, err := h.svc.ResolveUserNode(c.Request().Context(), c.Param("userId"))
	if err != nil {
		return httputil.MapDomainError(err)
	}
	if err := h.svc.RemoveManager(c.Request().Context(), c.Param("chId"), claims.NGACNodeID, targetNodeID); err != nil {
		return httputil.MapDomainError(err)
	}
	return c.JSON(http.StatusOK, map[string]string{"status": "ok"})
}

// TransferOwnership handles POST /api/channels/:chId/transfer.
func (h *Handler) TransferOwnership(c echo.Context) error {
	claims, err := httputil.RequireClaims(c)
	if err != nil {
		return err
	}
	var body struct {
		UserID string `json:"user_id"`
	}
	if err := c.Bind(&body); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid request body")
	}
	// The client names people by user ID (the roster and GET management do);
	// the domain works on graph nodes, resolved here on the server.
	targetNodeID, err := h.svc.ResolveUserNode(c.Request().Context(), body.UserID)
	if err != nil {
		return httputil.MapDomainError(err)
	}
	if err := h.svc.TransferOwnership(c.Request().Context(), c.Param("chId"), claims.NGACNodeID, targetNodeID); err != nil {
		return httputil.MapDomainError(err)
	}
	return c.JSON(http.StatusOK, map[string]string{"status": "ok"})
}

// DisbandGroup handles DELETE /api/channels/:chId.
func (h *Handler) DisbandGroup(c echo.Context) error {
	claims, err := httputil.RequireClaims(c)
	if err != nil {
		return err
	}
	if err := h.svc.DisbandGroup(c.Request().Context(), c.Param("chId"), claims.NGACNodeID); err != nil {
		return httputil.MapDomainError(err)
	}
	return c.JSON(http.StatusOK, map[string]string{"status": "ok"})
}
