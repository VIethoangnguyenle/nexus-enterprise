// Package rest provides Echo REST handlers for the messaging service.
// All handlers delegate to the domain layer. No SQL or business logic here.
package rest

import (
	"net/http"
	"strconv"

	"github.com/labstack/echo/v4"

	"ngac-platform/pkg/httputil"
	"ngac-platform/services/messaging/internal/domain"
)

// Handler serves messaging REST endpoints.
type Handler struct {
	svc     *domain.Service
	notifSt domain.NotificationStore
}

// NewHandler creates a messaging REST handler.
func NewHandler(svc *domain.Service, notifSt domain.NotificationStore) *Handler {
	return &Handler{svc: svc, notifSt: notifSt}
}

// RegisterRoutes mounts messaging endpoints on the Echo instance.
func (h *Handler) RegisterRoutes(e *echo.Echo, jwtSecret string) {
	api := e.Group("/api", httputil.JWTMiddleware(jwtSecret))

	// Channels
	api.POST("/workspaces/:id/channels", h.CreateChannel)
	api.GET("/workspaces/:id/channels", h.ListChannels)
	api.GET("/channels/:chId", h.GetChannel)

	// Messages
	api.POST("/channels/:chId/messages", h.SendMessage)
	api.GET("/channels/:chId/messages", h.GetMessages)

	// Threads
	api.GET("/messages/:msgId/thread", h.GetThread)
	api.GET("/threads/entity/:entityType/:entityId", h.FindThreadsByEntity)

	// Channel Members
	api.POST("/channels/:chId/members", h.AddChannelMember)
	api.GET("/channels/:chId/members", h.ListChannelMembers)

	// Channel Drive
	api.GET("/channels/:chId/drive", h.GetChannelDrive)

	// DMs
	api.POST("/dms", h.CreateDM)
	api.GET("/dms", h.ListDMs)

	// Notifications
	api.GET("/notifications", h.ListNotifications)
	api.POST("/notifications/:notifId/read", h.MarkRead)
	api.POST("/notifications/read-all", h.MarkAllRead)
	api.GET("/notifications/unread-count", h.UnreadCount)
}

// CreateChannel handles POST /api/workspaces/:id/channels.
func (h *Handler) CreateChannel(c echo.Context) error {
	claims := httputil.GetClaims(c)
	var body struct {
		Name        string `json:"name"`
		ChannelType string `json:"channel_type"`
	}
	if err := c.Bind(&body); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid request body")
	}
	if body.ChannelType == "" {
		body.ChannelType = "workspace"
	}

	ch, err := h.svc.CreateChannel(c.Request().Context(), domain.CreateChannelInput{
		Name:        body.Name,
		WorkspaceID: c.Param("id"),
		UserID:      claims.UserID,
		UserNodeID:  claims.NGACNodeID,
		ChannelType: body.ChannelType,
	})
	if err != nil {
		return httputil.MapDomainError(err)
	}
	return c.JSON(http.StatusCreated, ch)
}

// ListChannels handles GET /api/workspaces/:id/channels.
func (h *Handler) ListChannels(c echo.Context) error {
	claims := httputil.GetClaims(c)
	channels, err := h.svc.ListChannels(c.Request().Context(), c.Param("id"), claims.NGACNodeID)
	if err != nil {
		return httputil.MapDomainError(err)
	}
	return c.JSON(http.StatusOK, map[string]any{"channels": channels})
}

// GetChannel handles GET /api/channels/:chId.
func (h *Handler) GetChannel(c echo.Context) error {
	ch, err := h.svc.GetChannel(c.Request().Context(), c.Param("chId"))
	if err != nil {
		return httputil.MapDomainError(err)
	}
	return c.JSON(http.StatusOK, ch)
}

// SendMessage handles POST /api/channels/:chId/messages.
func (h *Handler) SendMessage(c echo.Context) error {
	claims := httputil.GetClaims(c)
	var body struct {
		Content          string `json:"content"`
		LinkedEntityType string `json:"linked_entity_type"`
		LinkedEntityID   string `json:"linked_entity_id"`
	}
	if err := c.Bind(&body); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid request body")
	}

	msg, err := h.svc.SendMessage(c.Request().Context(), domain.SendMessageInput{
		ChannelID:        c.Param("chId"),
		SenderID:         claims.UserID,
		SenderNodeID:     claims.NGACNodeID,
		Content:          body.Content,
		LinkedEntityType: body.LinkedEntityType,
		LinkedEntityID:   body.LinkedEntityID,
	})
	if err != nil {
		return httputil.MapDomainError(err)
	}
	return c.JSON(http.StatusCreated, msg)
}

// GetMessages handles GET /api/channels/:chId/messages.
func (h *Handler) GetMessages(c echo.Context) error {
	claims := httputil.GetClaims(c)
	before := c.QueryParam("before")
	limitStr := c.QueryParam("limit")
	limit := 50
	if limitStr != "" {
		if v, err := strconv.Atoi(limitStr); err == nil && v > 0 && v <= 100 {
			limit = v
		}
	}

	msgs, err := h.svc.GetMessages(c.Request().Context(), c.Param("chId"), claims.NGACNodeID, before, limit)
	if err != nil {
		return httputil.MapDomainError(err)
	}
	return c.JSON(http.StatusOK, msgs)
}

// GetThread handles GET /api/messages/:msgId/thread.
func (h *Handler) GetThread(c echo.Context) error {
	msgs, err := h.svc.GetThread(c.Request().Context(), c.Param("msgId"))
	if err != nil {
		return httputil.MapDomainError(err)
	}
	return c.JSON(http.StatusOK, msgs)
}

// FindThreadsByEntity handles GET /api/threads/entity/:entityType/:entityId.
func (h *Handler) FindThreadsByEntity(c echo.Context) error {
	msgs, err := h.svc.FindThreadsByEntity(c.Request().Context(), c.Param("entityType"), c.Param("entityId"))
	if err != nil {
		return httputil.MapDomainError(err)
	}
	return c.JSON(http.StatusOK, msgs)
}

// AddChannelMember handles POST /api/channels/:chId/members.
func (h *Handler) AddChannelMember(c echo.Context) error {
	claims := httputil.GetClaims(c)
	var body struct {
		NGACNodeID string `json:"ngac_node_id"`
	}
	if err := c.Bind(&body); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid request body")
	}

	if err := h.svc.AddMember(c.Request().Context(), c.Param("chId"), claims.NGACNodeID, body.NGACNodeID); err != nil {
		return httputil.MapDomainError(err)
	}
	return c.JSON(http.StatusOK, map[string]string{"status": "ok"})
}

// ListChannelMembers handles GET /api/channels/:chId/members.
func (h *Handler) ListChannelMembers(c echo.Context) error {
	members, err := h.svc.ListMembers(c.Request().Context(), c.Param("chId"))
	if err != nil {
		return httputil.MapDomainError(err)
	}
	return c.JSON(http.StatusOK, map[string]any{"members": members})
}

// GetChannelDrive handles GET /api/channels/:chId/drive.
// Returns the channel info which contains drive references.
func (h *Handler) GetChannelDrive(c echo.Context) error {
	ch, err := h.svc.GetChannel(c.Request().Context(), c.Param("chId"))
	if err != nil {
		return httputil.MapDomainError(err)
	}
	return c.JSON(http.StatusOK, ch)
}

// CreateDM handles POST /api/dms.
func (h *Handler) CreateDM(c echo.Context) error {
	claims := httputil.GetClaims(c)
	var body struct {
		TargetUserID     string `json:"target_user_id"`
		TargetNGACNodeID string `json:"target_ngac_node_id"`
	}
	if err := c.Bind(&body); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid request body")
	}

	ch, err := h.svc.FindOrCreateDM(c.Request().Context(), claims.UserID, claims.NGACNodeID, body.TargetUserID, body.TargetNGACNodeID)
	if err != nil {
		return httputil.MapDomainError(err)
	}
	return c.JSON(http.StatusCreated, ch)
}

// ListDMs handles GET /api/dms.
func (h *Handler) ListDMs(c echo.Context) error {
	claims := httputil.GetClaims(c)
	dms, err := h.svc.ListDMs(c.Request().Context(), claims.NGACNodeID)
	if err != nil {
		return httputil.MapDomainError(err)
	}
	return c.JSON(http.StatusOK, map[string]any{"channels": dms})
}

// ListNotifications handles GET /api/notifications.
func (h *Handler) ListNotifications(c echo.Context) error {
	claims := httputil.GetClaims(c)
	notifs, err := h.notifSt.ListByUser(c.Request().Context(), claims.UserID)
	if err != nil {
		return httputil.MapDomainError(err)
	}
	return c.JSON(http.StatusOK, map[string]any{"notifications": notifs})
}

// MarkRead handles POST /api/notifications/:notifId/read.
func (h *Handler) MarkRead(c echo.Context) error {
	if err := h.notifSt.MarkRead(c.Request().Context(), c.Param("notifId")); err != nil {
		return httputil.MapDomainError(err)
	}
	return c.JSON(http.StatusOK, map[string]string{"status": "ok"})
}

// MarkAllRead handles POST /api/notifications/read-all.
func (h *Handler) MarkAllRead(c echo.Context) error {
	claims := httputil.GetClaims(c)
	if err := h.notifSt.MarkAllRead(c.Request().Context(), claims.UserID); err != nil {
		return httputil.MapDomainError(err)
	}
	return c.JSON(http.StatusOK, map[string]string{"status": "ok"})
}

// UnreadCount handles GET /api/notifications/unread-count.
func (h *Handler) UnreadCount(c echo.Context) error {
	claims := httputil.GetClaims(c)
	count, err := h.notifSt.UnreadCount(c.Request().Context(), claims.UserID)
	if err != nil {
		return httputil.MapDomainError(err)
	}
	return c.JSON(http.StatusOK, map[string]any{"count": count})
}
