// Package rest — personal sections of the chat navigator.
package rest

import (
	"net/http"

	"github.com/labstack/echo/v4"

	"ngac-platform/pkg/httputil"
	"ngac-platform/services/messaging/internal/domain"
)

// sectionJSON is a section as the navigator reads it. Channel ids are for
// placing rows only; the navigator shows conversation names.
type sectionJSON struct {
	ID         string   `json:"id"`
	Name       string   `json:"name"`
	Position   int      `json:"position"`
	ChannelIDs []string `json:"channel_ids"`
}

func toSectionJSON(s *domain.Section) sectionJSON {
	ids := s.ChannelIDs
	if ids == nil {
		ids = []string{}
	}
	return sectionJSON{ID: s.ID, Name: s.Name, Position: s.Position, ChannelIDs: ids}
}

func callerOf(c echo.Context) (domain.Caller, error) {
	claims, err := httputil.RequireClaims(c)
	if err != nil {
		return domain.Caller{}, err
	}
	return domain.Caller{UserID: claims.UserID, NodeID: claims.NGACNodeID, TenantID: claims.TenantID}, nil
}

// ListSections handles GET /api/workspaces/:id/chat-sections.
func (h *Handler) ListSections(c echo.Context) error {
	who, err := callerOf(c)
	if err != nil {
		return err
	}
	secs, err := h.svc.ListSections(c.Request().Context(), c.Param("id"), who)
	if err != nil {
		return httputil.MapDomainError(err)
	}
	out := make([]sectionJSON, 0, len(secs))
	for _, s := range secs {
		out = append(out, toSectionJSON(s))
	}
	return c.JSON(http.StatusOK, map[string]any{"sections": out})
}

// CreateSection handles POST /api/workspaces/:id/chat-sections. position and
// channel_ids are optional; Undo of a delete sends them back.
func (h *Handler) CreateSection(c echo.Context) error {
	who, err := callerOf(c)
	if err != nil {
		return err
	}
	var body struct {
		Name       string   `json:"name"`
		Position   *int     `json:"position"`
		ChannelIDs []string `json:"channel_ids"`
	}
	if err := c.Bind(&body); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid request body")
	}
	sec, err := h.svc.CreateSection(c.Request().Context(), c.Param("id"), who,
		domain.NewSection{Name: body.Name, Position: body.Position, ChannelIDs: body.ChannelIDs})
	if err != nil {
		return httputil.MapDomainError(err)
	}
	return c.JSON(http.StatusCreated, toSectionJSON(sec))
}

// UpdateSection handles PATCH /api/workspaces/:id/chat-sections/:sectionId:
// {"name": …} renames, {"move": "up"|"down"} reorders.
func (h *Handler) UpdateSection(c echo.Context) error {
	who, err := callerOf(c)
	if err != nil {
		return err
	}
	var body struct {
		Name *string `json:"name"`
		Move string  `json:"move"`
	}
	if err := c.Bind(&body); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid request body")
	}
	ctx, ws, id := c.Request().Context(), c.Param("id"), c.Param("sectionId")
	switch {
	case body.Name != nil:
		err = h.svc.RenameSection(ctx, ws, who, id, *body.Name)
	case body.Move != "":
		err = h.svc.MoveSection(ctx, ws, who, id, body.Move)
	default:
		return echo.NewHTTPError(http.StatusBadRequest, "name or move required")
	}
	if err != nil {
		return httputil.MapDomainError(err)
	}
	return c.JSON(http.StatusOK, map[string]string{"status": "ok"})
}

// DeleteSection handles DELETE /api/workspaces/:id/chat-sections/:sectionId and
// returns the section as it was, for Undo.
func (h *Handler) DeleteSection(c echo.Context) error {
	who, err := callerOf(c)
	if err != nil {
		return err
	}
	sec, err := h.svc.DeleteSection(c.Request().Context(), c.Param("id"), who, c.Param("sectionId"))
	if err != nil {
		return httputil.MapDomainError(err)
	}
	return c.JSON(http.StatusOK, toSectionJSON(sec))
}

// PlaceChannel handles PUT /api/workspaces/:id/chat-sections/:sectionId/channels/:chId.
func (h *Handler) PlaceChannel(c echo.Context) error {
	who, err := callerOf(c)
	if err != nil {
		return err
	}
	if err := h.svc.PlaceChannel(c.Request().Context(), c.Param("id"), who, c.Param("sectionId"), c.Param("chId")); err != nil {
		return httputil.MapDomainError(err)
	}
	return c.JSON(http.StatusOK, map[string]string{"status": "ok"})
}

// UnplaceChannel handles DELETE /api/workspaces/:id/chat-sections/channels/:chId.
func (h *Handler) UnplaceChannel(c echo.Context) error {
	who, err := callerOf(c)
	if err != nil {
		return err
	}
	if err := h.svc.UnplaceChannel(c.Request().Context(), c.Param("id"), who, c.Param("chId")); err != nil {
		return httputil.MapDomainError(err)
	}
	return c.JSON(http.StatusOK, map[string]string{"status": "ok"})
}
