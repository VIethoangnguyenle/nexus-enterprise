package rest

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/labstack/echo/v4"

	"ngac-platform/services/calendar/internal/domain"
)

// people reads the comma-separated node ids of the "users" query parameter.
func people(c echo.Context) []string {
	var out []string
	for _, u := range strings.Split(c.QueryParam("users"), ",") {
		if u = strings.TrimSpace(u); u != "" {
			out = append(out, u)
		}
	}
	return out
}

// FreeBusy answers when some people are busy and what their working hours are.
func (h *Handler) FreeBusy(c echo.Context) error {
	got, err := h.cal.FreeBusy(c.Request().Context(), c.Param("wsId"), people(c), c.QueryParam("from"), c.QueryParam("to"))
	if err != nil {
		return mapError(err)
	}
	return c.JSON(http.StatusOK, map[string]any{"people": got})
}

// SuggestSlots offers the earliest times a day when everyone is free.
func (h *Handler) SuggestSlots(c echo.Context) error {
	minutes, err := strconv.Atoi(c.QueryParam("durationMin"))
	if err != nil {
		return echo.NewHTTPError(http.StatusUnprocessableEntity, "durationMin must be a number")
	}
	got, err := h.cal.SuggestSlots(c.Request().Context(), c.Param("wsId"), people(c), minutes, c.QueryParam("day"))
	if err != nil {
		return mapError(err)
	}
	return c.JSON(http.StatusOK, map[string]any{"slots": got})
}

// SearchEvents finds events by their text.
func (h *Handler) SearchEvents(c echo.Context) error {
	got, err := h.cal.SearchEvents(c.Request().Context(), c.Param("wsId"), c.QueryParam("q"))
	if err != nil {
		return mapError(err)
	}
	return c.JSON(http.StatusOK, map[string]any{"occurrences": got})
}

// GetSettings returns the caller's calendar settings.
func (h *Handler) GetSettings(c echo.Context) error {
	got, err := h.cal.GetSettings(c.Request().Context(), c.Param("wsId"))
	if err != nil {
		return mapError(err)
	}
	return c.JSON(http.StatusOK, got)
}

// UpdateSettings saves the caller's calendar settings.
func (h *Handler) UpdateSettings(c echo.Context) error {
	var in domain.SettingsView
	if err := c.Bind(&in); err != nil {
		return badBody(c, err)
	}
	got, err := h.cal.UpdateSettings(c.Request().Context(), c.Param("wsId"), in)
	if err != nil {
		return mapError(err)
	}
	return c.JSON(http.StatusOK, got)
}
