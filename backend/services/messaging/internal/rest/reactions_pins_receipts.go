// Package rest — reactions, pins, read receipts, search REST handlers.
package rest

import (
	"net/http"
	"strconv"

	"github.com/labstack/echo/v4"

	"ngac-platform/pkg/httputil"
)

// --- Reactions ---

// AddReaction handles POST /api/messages/:msgId/reactions.
func (h *Handler) AddReaction(c echo.Context) error {
	claims := httputil.GetClaims(c)
	var body struct {
		Emoji string `json:"emoji"`
	}
	if err := c.Bind(&body); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid request body")
	}
	if err := h.svc.AddReaction(c.Request().Context(), c.Param("msgId"), claims.UserID, body.Emoji); err != nil {
		return httputil.MapDomainError(err)
	}
	return c.JSON(http.StatusOK, map[string]string{"status": "ok"})
}

// RemoveReaction handles DELETE /api/messages/:msgId/reactions/:emoji.
func (h *Handler) RemoveReaction(c echo.Context) error {
	claims := httputil.GetClaims(c)
	if err := h.svc.RemoveReaction(c.Request().Context(), c.Param("msgId"), claims.UserID, c.Param("emoji")); err != nil {
		return httputil.MapDomainError(err)
	}
	return c.JSON(http.StatusOK, map[string]string{"status": "ok"})
}

// ListReactions handles GET /api/messages/:msgId/reactions.
func (h *Handler) ListReactions(c echo.Context) error {
	reactions, err := h.svc.ListReactions(c.Request().Context(), c.Param("msgId"))
	if err != nil {
		return httputil.MapDomainError(err)
	}
	return c.JSON(http.StatusOK, map[string]any{"reactions": reactions})
}

// --- Pins ---

// PinMessage handles POST /api/channels/:chId/pins.
func (h *Handler) PinMessage(c echo.Context) error {
	claims := httputil.GetClaims(c)
	var body struct {
		MessageID string `json:"message_id"`
	}
	if err := c.Bind(&body); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid request body")
	}
	if err := h.svc.PinMessage(c.Request().Context(), c.Param("chId"), body.MessageID, claims.UserID); err != nil {
		return httputil.MapDomainError(err)
	}
	return c.JSON(http.StatusOK, map[string]string{"status": "ok"})
}

// UnpinMessage handles DELETE /api/channels/:chId/pins/:msgId.
func (h *Handler) UnpinMessage(c echo.Context) error {
	if err := h.svc.UnpinMessage(c.Request().Context(), c.Param("chId"), c.Param("msgId")); err != nil {
		return httputil.MapDomainError(err)
	}
	return c.JSON(http.StatusOK, map[string]string{"status": "ok"})
}

// ListPins handles GET /api/channels/:chId/pins.
func (h *Handler) ListPins(c echo.Context) error {
	pins, err := h.svc.ListPins(c.Request().Context(), c.Param("chId"))
	if err != nil {
		return httputil.MapDomainError(err)
	}
	return c.JSON(http.StatusOK, map[string]any{"pins": pins})
}

// --- Read Receipts ---

// MarkChannelRead handles POST /api/channels/:chId/read.
func (h *Handler) MarkChannelRead(c echo.Context) error {
	claims := httputil.GetClaims(c)
	var body struct {
		LastMessageID string `json:"last_message_id"`
	}
	if err := c.Bind(&body); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid request body")
	}
	if err := h.svc.MarkChannelRead(c.Request().Context(), claims.UserID, c.Param("chId"), body.LastMessageID); err != nil {
		return httputil.MapDomainError(err)
	}
	return c.JSON(http.StatusOK, map[string]string{"status": "ok"})
}

// GetUnreadCounts handles GET /api/channels/unread.
func (h *Handler) GetUnreadCounts(c echo.Context) error {
	claims := httputil.GetClaims(c)
	unreads, err := h.svc.GetUnreadCounts(c.Request().Context(), claims.UserID)
	if err != nil {
		return httputil.MapDomainError(err)
	}
	return c.JSON(http.StatusOK, map[string]any{"channels": unreads})
}

// --- Search ---

// SearchMessages handles GET /api/channels/:chId/search.
func (h *Handler) SearchMessages(c echo.Context) error {
	query := c.QueryParam("q")
	limitStr := c.QueryParam("limit")
	limit := 20
	if limitStr != "" {
		if v, err := strconv.Atoi(limitStr); err == nil && v > 0 && v <= 50 {
			limit = v
		}
	}

	result, err := h.svc.SearchMessages(c.Request().Context(), c.Param("chId"), query, limit)
	if err != nil {
		return httputil.MapDomainError(err)
	}
	return c.JSON(http.StatusOK, result)
}
