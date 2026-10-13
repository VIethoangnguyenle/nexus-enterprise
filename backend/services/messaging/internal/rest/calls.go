// Package rest — voice and video calls, and the webhook LiveKit reports to.
package rest

import (
	"errors"
	"log/slog"
	"net/http"

	"github.com/labstack/echo/v4"

	"ngac-platform/pkg/httputil"
	"ngac-platform/services/messaging/internal/domain"
	"ngac-platform/services/messaging/internal/livekit"
)

// Codes of the call refusals a screen tells apart.
const (
	codeCallFull         = "call_full"
	codeCallEnded        = "call_ended"
	codeCallsUnavailable = "calls_unavailable"
)

// CallWebhooks verifies and decodes the webhooks LiveKit sends.
// *livekit.Client implements it.
type CallWebhooks interface {
	ReceiveWebhook(r *http.Request) (livekit.Event, error)
}

// SetCallWebhooks wires the webhook verifier. Without one the webhook route
// refuses everything.
func (h *Handler) SetCallWebhooks(w CallWebhooks) { h.webhooks = w }

// registerCallRoutes mounts the call routes under the JWT group, and the
// webhook outside it: LiveKit signs its requests with the API secret, it holds
// no user session. The webhook is never routed from outside the network.
func (h *Handler) registerCallRoutes(e *echo.Echo, api *echo.Group) {
	api.POST("/channels/:chId/calls", h.StartCall)
	api.GET("/calls/:callId", h.GetCall)
	api.POST("/calls/:callId/token", h.CallToken)
	api.POST("/calls/:callId/decline", h.DeclineCall)
	api.POST("/calls/:callId/cancel", h.CancelCall)
	e.POST("/internal/livekit/webhook", h.LiveKitWebhook)
}

// callError maps a call refusal to its status and code.
func callError(err error) error {
	switch {
	case errors.Is(err, domain.ErrCallsUnavailable):
		return httputil.CodedError(http.StatusServiceUnavailable, codeCallsUnavailable, "calls are not available")
	case errors.Is(err, domain.ErrCallFull):
		return httputil.CodedError(http.StatusConflict, codeCallFull, "the call is full")
	case errors.Is(err, domain.ErrCallEnded):
		return httputil.CodedError(http.StatusConflict, codeCallEnded, "the call has ended")
	}
	return httputil.MapDomainError(err)
}

// StartCall handles POST /api/channels/:chId/calls: starts a call, or joins the
// one already open, and answers with a ticket into its room.
func (h *Handler) StartCall(c echo.Context) error {
	claims, err := httputil.RequireClaims(c)
	if err != nil {
		return err
	}
	var body struct {
		Kind string `json:"kind"`
	}
	if err := c.Bind(&body); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid request body")
	}
	join, card, err := h.svc.StartCall(c.Request().Context(), domain.StartCallInput{
		ChannelID: c.Param("chId"), UserID: claims.UserID, UserNodeID: claims.NGACNodeID, Kind: body.Kind,
	})
	if err != nil {
		return callError(err)
	}
	if card == nil {
		return c.JSON(http.StatusOK, join)
	}
	// The card goes to the live stream only. It is not pushed as a message: a
	// direct call rings by its own notification, a group call does not ring.
	if h.hub != nil {
		h.hub.BroadcastToChannel(card.ChannelId, card)
	}
	return c.JSON(http.StatusCreated, join)
}

// GetCall handles GET /api/calls/:callId.
func (h *Handler) GetCall(c echo.Context) error {
	claims, err := httputil.RequireClaims(c)
	if err != nil {
		return err
	}
	call, err := h.svc.GetCall(c.Request().Context(), c.Param("callId"), claims.NGACNodeID)
	if err != nil {
		return callError(err)
	}
	return c.JSON(http.StatusOK, call)
}

// CallToken handles POST /api/calls/:callId/token.
func (h *Handler) CallToken(c echo.Context) error {
	claims, err := httputil.RequireClaims(c)
	if err != nil {
		return err
	}
	join, err := h.svc.CallToken(c.Request().Context(), c.Param("callId"), claims.UserID, claims.NGACNodeID)
	if err != nil {
		return callError(err)
	}
	return c.JSON(http.StatusOK, join)
}

// DeclineCall handles POST /api/calls/:callId/decline.
func (h *Handler) DeclineCall(c echo.Context) error {
	claims, err := httputil.RequireClaims(c)
	if err != nil {
		return err
	}
	call, err := h.svc.DeclineCall(c.Request().Context(), c.Param("callId"), claims.UserID, claims.NGACNodeID)
	if err != nil {
		return callError(err)
	}
	return c.JSON(http.StatusOK, call)
}

// CancelCall handles POST /api/calls/:callId/cancel.
func (h *Handler) CancelCall(c echo.Context) error {
	claims, err := httputil.RequireClaims(c)
	if err != nil {
		return err
	}
	call, err := h.svc.CancelCall(c.Request().Context(), c.Param("callId"), claims.UserID, claims.NGACNodeID)
	if err != nil {
		return callError(err)
	}
	return c.JSON(http.StatusOK, call)
}

// LiveKitWebhook handles POST /internal/livekit/webhook. A request not signed
// with this server's LiveKit secret is refused before anything is read from it.
func (h *Handler) LiveKitWebhook(c echo.Context) error {
	if h.webhooks == nil {
		return echo.NewHTTPError(http.StatusServiceUnavailable, "calls are not available")
	}
	ev, err := h.webhooks.ReceiveWebhook(c.Request())
	if err != nil {
		slog.Warn("livekit webhook refused", "error", err)
		return echo.NewHTTPError(http.StatusUnauthorized, "invalid webhook signature")
	}
	if err := h.svc.HandleCallEvent(c.Request().Context(), domain.CallEvent{
		Kind: ev.Kind, Room: ev.Room, Identity: ev.Identity,
	}); err != nil {
		return httputil.MapDomainError(err)
	}
	return c.NoContent(http.StatusOK)
}
