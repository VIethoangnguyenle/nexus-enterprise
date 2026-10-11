// Package rest — in-line thread REST handlers.
package rest

import (
	"net/http"

	"github.com/labstack/echo/v4"

	"ngac-platform/pkg/httputil"
)

// FollowThread handles POST /api/messages/:msgId/follow.
func (h *Handler) FollowThread(c echo.Context) error {
	claims, err := httputil.RequireClaims(c)
	if err != nil {
		return err
	}
	if err := h.svc.FollowThread(c.Request().Context(), c.Param("msgId"), claims.UserID, claims.NGACNodeID); err != nil {
		return httputil.MapDomainError(err)
	}
	return c.JSON(http.StatusOK, map[string]string{"status": "ok"})
}

// UnfollowThread handles DELETE /api/messages/:msgId/follow.
func (h *Handler) UnfollowThread(c echo.Context) error {
	claims, err := httputil.RequireClaims(c)
	if err != nil {
		return err
	}
	if err := h.svc.UnfollowThread(c.Request().Context(), c.Param("msgId"), claims.UserID, claims.NGACNodeID); err != nil {
		return httputil.MapDomainError(err)
	}
	return c.JSON(http.StatusOK, map[string]string{"status": "ok"})
}

// MarkThreadRead handles POST /api/messages/:msgId/thread/read.
func (h *Handler) MarkThreadRead(c echo.Context) error {
	claims, err := httputil.RequireClaims(c)
	if err != nil {
		return err
	}
	if err := h.svc.MarkThreadRead(c.Request().Context(), c.Param("msgId"), claims.UserID, claims.NGACNodeID); err != nil {
		return httputil.MapDomainError(err)
	}
	return c.JSON(http.StatusOK, map[string]string{"status": "ok"})
}

// ListThreads handles GET /api/channels/:chId/threads?filter=all|following|unread&before=&before_id=&limit=.
func (h *Handler) ListThreads(c echo.Context) error {
	claims, err := httputil.RequireClaims(c)
	if err != nil {
		return err
	}
	limit, err := intQuery(c, "limit")
	if err != nil {
		return err
	}
	list, err := h.svc.ListThreads(c.Request().Context(), c.Param("chId"), claims.UserID, claims.NGACNodeID,
		c.QueryParam("filter"), c.QueryParam("before"), c.QueryParam("before_id"), limit)
	if err != nil {
		return httputil.MapDomainError(err)
	}
	return c.JSON(http.StatusOK, list)
}
