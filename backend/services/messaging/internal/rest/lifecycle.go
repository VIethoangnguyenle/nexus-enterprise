// Package rest — editing, deleting and un-reading messages.
package rest

import (
	"net/http"

	"github.com/labstack/echo/v4"

	"ngac-platform/pkg/httputil"
)

// EditMessage handles PATCH /api/messages/:msgId.
func (h *Handler) EditMessage(c echo.Context) error {
	claims, err := httputil.RequireClaims(c)
	if err != nil {
		return err
	}
	var body struct {
		Content       string `json:"content"`
		ContentFormat string `json:"content_format"`
		// Mentions is the mention list after the edit; absent leaves it as it was.
		Mentions []string `json:"mentions"`
	}
	if err := c.Bind(&body); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid request body")
	}
	msg, err := h.svc.EditMessage(c.Request().Context(), c.Param("msgId"), claims.UserID, claims.NGACNodeID,
		body.Content, body.ContentFormat, body.Mentions)
	if err != nil {
		return httputil.MapDomainError(err)
	}
	if h.hub != nil {
		h.hub.BroadcastMessageEdited(msg.ChannelId, msg)
	}
	return c.JSON(http.StatusOK, msg)
}

// DeleteMessage handles DELETE /api/messages/:msgId.
func (h *Handler) DeleteMessage(c echo.Context) error {
	claims, err := httputil.RequireClaims(c)
	if err != nil {
		return err
	}
	out, err := h.svc.DeleteMessage(c.Request().Context(), c.Param("msgId"), claims.UserID, claims.NGACNodeID)
	if err != nil {
		return httputil.MapDomainError(err)
	}
	if h.hub != nil {
		h.hub.BroadcastMessageDeleted(out.Event.ChannelId, out.Event)
		if out.Notice != nil {
			h.hub.BroadcastToChannel(out.Notice.ChannelId, out.Notice)
		}
	}
	return c.JSON(http.StatusOK, map[string]string{"status": "ok"})
}

// MarkMessageUnread handles POST /api/messages/:msgId/unread.
func (h *Handler) MarkMessageUnread(c echo.Context) error {
	claims, err := httputil.RequireClaims(c)
	if err != nil {
		return err
	}
	if err := h.svc.MarkMessageUnread(c.Request().Context(), c.Param("msgId"), claims.UserID, claims.NGACNodeID); err != nil {
		return httputil.MapDomainError(err)
	}
	return c.JSON(http.StatusOK, map[string]string{"status": "ok"})
}
