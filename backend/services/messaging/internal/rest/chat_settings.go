// Package rest — the caller's Chat options in a workspace.
package rest

import (
	"net/http"

	"github.com/labstack/echo/v4"

	"ngac-platform/pkg/httputil"
	"ngac-platform/services/messaging/internal/domain"
)

// chatSettingsJSON is how Chat's settings page reads them. default_level is
// "all", "main", "off" or null (the system default); double_tap is an emoji or
// "" for off.
type chatSettingsJSON struct {
	EnterToSend  bool    `json:"enter_to_send"`
	LinkPreview  bool    `json:"link_preview"`
	OpenAt       string  `json:"open_at"`
	Sort         string  `json:"sort"`
	DoubleTap    string  `json:"double_tap"`
	DefaultLevel *string `json:"default_level"`
}

func toChatSettingsJSON(s domain.ChatSettings) chatSettingsJSON {
	return chatSettingsJSON{
		EnterToSend: s.EnterToSend, LinkPreview: s.LinkPreview, OpenAt: s.OpenAt, Sort: s.Sort,
		DoubleTap: s.DoubleTap, DefaultLevel: s.DefaultLevel,
	}
}

// GetChatSettings handles GET /api/workspaces/:id/chat-settings.
func (h *Handler) GetChatSettings(c echo.Context) error {
	who, err := callerOf(c)
	if err != nil {
		return err
	}
	s, err := h.svc.GetChatSettings(c.Request().Context(), c.Param("id"), who)
	if err != nil {
		return httputil.MapDomainError(err)
	}
	return c.JSON(http.StatusOK, toChatSettingsJSON(s))
}

// PutChatSettings handles PUT /api/workspaces/:id/chat-settings. Every field is
// optional: one left out stays as it is, and default_level null clears it.
func (h *Handler) PutChatSettings(c echo.Context) error {
	who, err := callerOf(c)
	if err != nil {
		return err
	}
	var body struct {
		EnterToSend  *bool                   `json:"enter_to_send"`
		LinkPreview  *bool                   `json:"link_preview"`
		OpenAt       *string                 `json:"open_at"`
		Sort         *string                 `json:"sort"`
		DoubleTap    *string                 `json:"double_tap"`
		DefaultLevel domain.Nullable[string] `json:"default_level"`
	}
	if err := c.Bind(&body); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid request body")
	}
	s, err := h.svc.UpdateChatSettings(c.Request().Context(), c.Param("id"), who, domain.ChatSettingsPatch{
		EnterToSend: body.EnterToSend, LinkPreview: body.LinkPreview, OpenAt: body.OpenAt, Sort: body.Sort,
		DoubleTap: body.DoubleTap, DefaultLevel: body.DefaultLevel,
	})
	if err != nil {
		return httputil.MapDomainError(err)
	}
	return c.JSON(http.StatusOK, toChatSettingsJSON(s))
}
