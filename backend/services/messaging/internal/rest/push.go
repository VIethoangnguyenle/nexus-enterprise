// Package rest — the caller's push preferences.
package rest

import (
	"fmt"
	"net/http"
	"time"

	"github.com/labstack/echo/v4"

	"ngac-platform/pkg/httputil"
	"ngac-platform/services/messaging/internal/store"
)

// pushPreferencesJSON is the settings screen's view of push preferences.
// Quiet hours are "HH:MM" in the user's time zone.
type pushPreferencesJSON struct {
	Kinds struct {
		Direct      bool `json:"direct"`
		Mention     bool `json:"mention"`
		ThreadReply bool `json:"thread_reply"`
	} `json:"kinds"`
	QuietHours struct {
		Enabled  bool   `json:"enabled"`
		Start    string `json:"start"`
		End      string `json:"end"`
		TimeZone string `json:"time_zone"`
	} `json:"quiet_hours"`
}

func clock(minutes int) string { return fmt.Sprintf("%02d:%02d", minutes/60, minutes%60) }

func minutesOf(hhmm string) (int, error) {
	t, err := time.Parse("15:04", hhmm)
	if err != nil {
		return 0, err
	}
	return t.Hour()*60 + t.Minute(), nil
}

// GetPushPreferences handles GET /api/me/push-preferences.
func (h *Handler) GetPushPreferences(c echo.Context) error {
	claims, err := httputil.RequireClaims(c)
	if err != nil {
		return err
	}
	p, err := h.svc.GetPushPreferences(c.Request().Context(), claims.UserID)
	if err != nil {
		return httputil.MapDomainError(err)
	}
	var out pushPreferencesJSON
	out.Kinds.Direct, out.Kinds.Mention, out.Kinds.ThreadReply = p.Direct, p.Mention, p.ThreadReply
	out.QuietHours.Enabled, out.QuietHours.TimeZone = p.QuietEnabled, p.TimeZone
	out.QuietHours.Start, out.QuietHours.End = clock(p.QuietStart), clock(p.QuietEnd)
	return c.JSON(http.StatusOK, out)
}

// PutPushPreferences handles PUT /api/me/push-preferences.
func (h *Handler) PutPushPreferences(c echo.Context) error {
	claims, err := httputil.RequireClaims(c)
	if err != nil {
		return err
	}
	var in pushPreferencesJSON
	if err := c.Bind(&in); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid request body")
	}
	start, err := minutesOf(in.QuietHours.Start)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "quiet_hours.start must be HH:MM")
	}
	end, err := minutesOf(in.QuietHours.End)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "quiet_hours.end must be HH:MM")
	}
	p := store.PushPreferences{
		Direct: in.Kinds.Direct, Mention: in.Kinds.Mention, ThreadReply: in.Kinds.ThreadReply,
		QuietEnabled: in.QuietHours.Enabled, QuietStart: start, QuietEnd: end, TimeZone: in.QuietHours.TimeZone,
	}
	if err := h.svc.SavePushPreferences(c.Request().Context(), claims.UserID, p); err != nil {
		return httputil.MapDomainError(err)
	}
	return c.JSON(http.StatusOK, in)
}
