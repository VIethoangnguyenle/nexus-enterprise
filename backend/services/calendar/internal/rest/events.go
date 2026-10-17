package rest

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/labstack/echo/v4"

	"ngac-platform/services/calendar/internal/domain"
)

// ListEvents returns the occurrences inside [from, to) of the named calendars.
func (h *Handler) ListEvents(c echo.Context) error {
	q := domain.ListEventsQuery{From: c.QueryParam("from"), To: c.QueryParam("to")}
	for _, id := range strings.Split(c.QueryParam("calendars"), ",") {
		if id = strings.TrimSpace(id); id != "" {
			q.Calendars = append(q.Calendars, id)
		}
	}
	got, err := h.cal.ListEvents(c.Request().Context(), c.Param("wsId"), q)
	if err != nil {
		return mapError(err)
	}
	return c.JSON(http.StatusOK, map[string]any{"occurrences": got})
}

// GetEvent returns one event in full.
func (h *Handler) GetEvent(c echo.Context) error {
	got, err := h.cal.GetEvent(c.Request().Context(), c.Param("wsId"), c.Param("id"), c.QueryParam("occurrence"))
	if err != nil {
		return mapError(err)
	}
	return c.JSON(http.StatusOK, got)
}

// CreateEvent adds an event to a calendar.
func (h *Handler) CreateEvent(c echo.Context) error {
	var in domain.EventInput
	if err := c.Bind(&in); err != nil {
		return badBody(c, err)
	}
	got, err := h.cal.CreateEvent(c.Request().Context(), c.Param("wsId"), in)
	if err != nil {
		return mapError(err)
	}
	return c.JSON(http.StatusCreated, got)
}

func scopeOf(c echo.Context) domain.ScopeQuery {
	return domain.ScopeQuery{Scope: c.QueryParam("scope"), Occurrence: c.QueryParam("occurrence")}
}

// UpdateEvent changes an event, an occurrence or the tail of a series.
func (h *Handler) UpdateEvent(c echo.Context) error {
	var p domain.EventPatch
	if err := c.Bind(&p); err != nil {
		return badBody(c, err)
	}
	got, err := h.cal.UpdateEvent(c.Request().Context(), c.Param("wsId"), c.Param("id"), scopeOf(c), p)
	if err != nil {
		return mapError(err)
	}
	return c.JSON(http.StatusOK, got)
}

// DeleteEvent removes an event, an occurrence or the tail of a series.
func (h *Handler) DeleteEvent(c echo.Context) error {
	version := 0
	if v := c.QueryParam("version"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil || n < 0 {
			return echo.NewHTTPError(http.StatusUnprocessableEntity, "version must be a number")
		}
		version = n
	}
	if err := h.cal.DeleteEvent(c.Request().Context(), c.Param("wsId"), c.Param("id"), scopeOf(c), version); err != nil {
		return mapError(err)
	}
	return c.NoContent(http.StatusNoContent)
}
