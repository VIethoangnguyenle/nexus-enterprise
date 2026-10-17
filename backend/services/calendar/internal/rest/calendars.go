// Package rest serves the calendar service's HTTP API. The handlers adapt HTTP
// to the domain and hold no business logic and no SQL: a request names its
// workspace in the path, and the domain admits it only when that is the
// workspace the verified token is scoped to.
package rest

import (
	"context"
	"net/http"

	"github.com/labstack/echo/v4"

	"ngac-platform/pkg/httputil"
	"ngac-platform/services/calendar/internal/domain"
)

// Calendars is what the handlers need of the domain.
type Calendars interface {
	ListCalendars(ctx context.Context, wsID string) ([]*domain.CalendarView, error)
	SetSubscription(ctx context.Context, wsID, calID string, in domain.SubscriptionInput) (*domain.CalendarView, error)
	Subscribe(ctx context.Context, wsID, personNodeID string) (*domain.CalendarView, error)
	Unsubscribe(ctx context.Context, wsID, calID string) error
	EnsureChannelCalendar(ctx context.Context, wsID, channelID string) (*domain.CalendarView, error)
	ListShares(ctx context.Context, wsID, calID string) ([]*domain.ShareView, error)
	PutShare(ctx context.Context, wsID, calID, granteeNode, level string) (*domain.ShareView, error)
	DeleteShare(ctx context.Context, wsID, calID, granteeNode string) error
}

// Handler serves the calendar REST endpoints.
type Handler struct{ cal Calendars }

// NewHandler creates the calendar REST handler.
func NewHandler(c Calendars) *Handler { return &Handler{cal: c} }

// RegisterRoutes mounts the endpoints under /api/workspaces/:wsId/calendar.
func (h *Handler) RegisterRoutes(e *echo.Echo, jwtSecret string) {
	api := e.Group("/api", httputil.JWTMiddleware(jwtSecret))
	g := api.Group("/workspaces/:wsId/calendar")

	g.GET("/calendars", h.ListCalendars)
	g.PUT("/calendars/:calId/subscription", h.SetSubscription)
	g.POST("/subscriptions", h.Subscribe)
	g.DELETE("/subscriptions/:calId", h.Unsubscribe)

	g.GET("/calendars/:calId/shares", h.ListShares)
	g.PUT("/calendars/:calId/shares/:granteeNodeId", h.PutShare)
	g.DELETE("/calendars/:calId/shares/:granteeNodeId", h.DeleteShare)

	g.POST("/channels/:channelId/calendar", h.EnsureChannelCalendar)
}

// mapError turns a domain refusal into its HTTP status; anything else is a
// failure of ours and answers a generic 500.
func mapError(err error) error { return httputil.MapDomainError(err) }

func badBody(c echo.Context, err error) error {
	return echo.NewHTTPError(http.StatusBadRequest, "malformed request body")
}

// ListCalendars returns the caller's calendars.
func (h *Handler) ListCalendars(c echo.Context) error {
	cals, err := h.cal.ListCalendars(c.Request().Context(), c.Param("wsId"))
	if err != nil {
		return mapError(err)
	}
	return c.JSON(http.StatusOK, map[string]any{"calendars": cals})
}

type subscriptionBody struct {
	Color   *int  `json:"color"`
	Visible *bool `json:"visible"`
}

// SetSubscription changes the caller's colour and visibility for a calendar.
func (h *Handler) SetSubscription(c echo.Context) error {
	var body subscriptionBody
	if err := c.Bind(&body); err != nil {
		return badBody(c, err)
	}
	cal, err := h.cal.SetSubscription(c.Request().Context(), c.Param("wsId"), c.Param("calId"),
		domain.SubscriptionInput{Color: body.Color, Visible: body.Visible})
	if err != nil {
		return mapError(err)
	}
	return c.JSON(http.StatusOK, cal)
}

type subscribeBody struct {
	UserNodeID string `json:"userNodeId"`
}

// Subscribe adds a colleague's personal calendar to the caller's list.
func (h *Handler) Subscribe(c echo.Context) error {
	var body subscribeBody
	if err := c.Bind(&body); err != nil {
		return badBody(c, err)
	}
	cal, err := h.cal.Subscribe(c.Request().Context(), c.Param("wsId"), body.UserNodeID)
	if err != nil {
		return mapError(err)
	}
	return c.JSON(http.StatusOK, cal)
}

// Unsubscribe takes a colleague's calendar off the caller's list.
func (h *Handler) Unsubscribe(c echo.Context) error {
	if err := h.cal.Unsubscribe(c.Request().Context(), c.Param("wsId"), c.Param("calId")); err != nil {
		return mapError(err)
	}
	return c.NoContent(http.StatusNoContent)
}

// EnsureChannelCalendar returns the calendar of a group conversation, creating it
// on first use.
func (h *Handler) EnsureChannelCalendar(c echo.Context) error {
	cal, err := h.cal.EnsureChannelCalendar(c.Request().Context(), c.Param("wsId"), c.Param("channelId"))
	if err != nil {
		return mapError(err)
	}
	return c.JSON(http.StatusOK, cal)
}

// ListShares returns who a personal calendar is shared with.
func (h *Handler) ListShares(c echo.Context) error {
	shares, err := h.cal.ListShares(c.Request().Context(), c.Param("wsId"), c.Param("calId"))
	if err != nil {
		return mapError(err)
	}
	return c.JSON(http.StatusOK, map[string]any{"shares": shares})
}

type shareBody struct {
	Level string `json:"level"`
}

// PutShare shares a personal calendar with a person, or changes the level.
func (h *Handler) PutShare(c echo.Context) error {
	var body shareBody
	if err := c.Bind(&body); err != nil {
		return badBody(c, err)
	}
	share, err := h.cal.PutShare(c.Request().Context(), c.Param("wsId"), c.Param("calId"), c.Param("granteeNodeId"), body.Level)
	if err != nil {
		return mapError(err)
	}
	return c.JSON(http.StatusOK, share)
}

// DeleteShare ends a share.
func (h *Handler) DeleteShare(c echo.Context) error {
	if err := h.cal.DeleteShare(c.Request().Context(), c.Param("wsId"), c.Param("calId"), c.Param("granteeNodeId")); err != nil {
		return mapError(err)
	}
	return c.NoContent(http.StatusNoContent)
}
