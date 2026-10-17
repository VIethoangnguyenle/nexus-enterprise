package rest

import (
	"net/http"

	"github.com/labstack/echo/v4"

	"ngac-platform/services/calendar/internal/domain"
)

// Rsvp records the caller's answer to an invitation.
func (h *Handler) Rsvp(c echo.Context) error {
	var in domain.RsvpInput
	if err := c.Bind(&in); err != nil {
		return badBody(c, err)
	}
	got, err := h.cal.Rsvp(c.Request().Context(), c.Param("wsId"), c.Param("id"), in)
	if err != nil {
		return mapError(err)
	}
	return c.JSON(http.StatusOK, got)
}

// AddGuests invites more people (or a whole group) to an event.
func (h *Handler) AddGuests(c echo.Context) error {
	var in domain.GuestsInput
	if err := c.Bind(&in); err != nil {
		return badBody(c, err)
	}
	got, err := h.cal.AddGuests(c.Request().Context(), c.Param("wsId"), c.Param("id"), in)
	if err != nil {
		return mapError(err)
	}
	return c.JSON(http.StatusOK, got)
}
