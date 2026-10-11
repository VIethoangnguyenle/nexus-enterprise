// Package rest — the caller's stars and the feed of messages that mention them.
package rest

import (
	"net/http"

	"github.com/labstack/echo/v4"

	"ngac-platform/pkg/httputil"
)

// starAction answers a star or unstar with 204 once the domain accepted it.
func (h *Handler) starAction(c echo.Context, do func(claims *httputil.Claims) error) error {
	claims, err := httputil.RequireClaims(c)
	if err != nil {
		return err
	}
	if err := do(claims); err != nil {
		return httputil.MapDomainError(err)
	}
	return c.NoContent(http.StatusNoContent)
}

// StarConversation handles PUT /api/channels/:chId/star.
func (h *Handler) StarConversation(c echo.Context) error {
	return h.starAction(c, func(cl *httputil.Claims) error {
		return h.svc.StarConversation(c.Request().Context(), c.Param("chId"), cl.UserID, cl.NGACNodeID)
	})
}

// UnstarConversation handles DELETE /api/channels/:chId/star.
func (h *Handler) UnstarConversation(c echo.Context) error {
	return h.starAction(c, func(cl *httputil.Claims) error {
		return h.svc.UnstarConversation(c.Request().Context(), c.Param("chId"), cl.UserID, cl.NGACNodeID)
	})
}

// StarMessage handles PUT /api/messages/:msgId/star.
func (h *Handler) StarMessage(c echo.Context) error {
	return h.starAction(c, func(cl *httputil.Claims) error {
		return h.svc.StarMessage(c.Request().Context(), c.Param("msgId"), cl.UserID, cl.NGACNodeID)
	})
}

// UnstarMessage handles DELETE /api/messages/:msgId/star.
func (h *Handler) UnstarMessage(c echo.Context) error {
	return h.starAction(c, func(cl *httputil.Claims) error {
		return h.svc.UnstarMessage(c.Request().Context(), c.Param("msgId"), cl.UserID, cl.NGACNodeID)
	})
}

// StarredMessages handles GET /api/messages/starred?before=&before_id=&limit=:
// the messages the caller starred and still reads in the workspace they are
// signed in to, newest star first.
func (h *Handler) StarredMessages(c echo.Context) error {
	claims, err := httputil.RequireClaims(c)
	if err != nil {
		return err
	}
	limit, err := intQuery(c, "limit")
	if err != nil {
		return err
	}
	list, err := h.svc.StarredMessages(c.Request().Context(), claims.UserID, claims.NGACNodeID, claims.TenantID,
		c.QueryParam("before"), c.QueryParam("before_id"), limit)
	if err != nil {
		return httputil.MapDomainError(err)
	}
	return c.JSON(http.StatusOK, list)
}

// Mentions handles GET /api/messages/mentions?before=&before_id=&limit=: every
// message by someone else that mentions the caller, in the conversations they
// read in the workspace they are signed in to, newest first.
func (h *Handler) Mentions(c echo.Context) error {
	claims, err := httputil.RequireClaims(c)
	if err != nil {
		return err
	}
	limit, err := intQuery(c, "limit")
	if err != nil {
		return err
	}
	list, err := h.svc.Mentions(c.Request().Context(), claims.UserID, claims.NGACNodeID, claims.TenantID,
		c.QueryParam("before"), c.QueryParam("before_id"), limit)
	if err != nil {
		return httputil.MapDomainError(err)
	}
	return c.JSON(http.StatusOK, list)
}
