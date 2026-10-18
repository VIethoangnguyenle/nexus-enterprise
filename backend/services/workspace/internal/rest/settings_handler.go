package rest

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/labstack/echo/v4"

	"ngac-platform/pkg/httputil"
)

// SettingsService is what the settings and locks routes call. The caller is the
// NGAC user node and the user id from verified claims; the workspace comes from
// the route. Membership and the admin check happen in the domain.
type SettingsService interface {
	GetUserSettings(ctx context.Context, callerNodeID, userID, wsID string) (map[string]json.RawMessage, error)
	PutUserSettings(ctx context.Context, callerNodeID, userID, wsID string, kv map[string]json.RawMessage) (map[string]json.RawMessage, error)
	GroupCreationRule(ctx context.Context, callerNodeID, wsID string) (string, error)
	SetGroupCreationRule(ctx context.Context, callerNodeID, wsID, rule string) (string, error)
}

// SettingsHandler serves a person's workspace-scope settings and the workspace's locks.
type SettingsHandler struct {
	svc SettingsService
}

// NewSettingsHandler creates a settings REST handler.
func NewSettingsHandler(svc SettingsService) *SettingsHandler {
	return &SettingsHandler{svc: svc}
}

// RegisterRoutes mounts the settings and locks endpoints on an authenticated group.
func (h *SettingsHandler) RegisterRoutes(api *echo.Group) {
	api.GET("/workspaces/:id/settings", h.GetSettings)
	api.PUT("/workspaces/:id/settings", h.PutSettings)
	api.GET("/workspaces/:id/locks", h.GetLocks)
	api.PUT("/workspaces/:id/locks", h.PutLocks)
}

type settingsBody struct {
	Settings map[string]json.RawMessage `json:"settings"`
}

func settingsOrEmpty(m map[string]json.RawMessage) settingsBody {
	if m == nil {
		m = map[string]json.RawMessage{}
	}
	return settingsBody{Settings: m}
}

// GetSettings handles GET /api/workspaces/:id/settings: the caller's own settings
// in the workspace that have been set. A key never written is absent.
func (h *SettingsHandler) GetSettings(c echo.Context) error {
	claims, err := httputil.RequireClaims(c)
	if err != nil {
		return err
	}
	got, err := h.svc.GetUserSettings(c.Request().Context(), claims.NGACNodeID, claims.UserID, c.Param("id"))
	if err != nil {
		return httputil.MapDomainError(err)
	}
	return c.JSON(http.StatusOK, settingsOrEmpty(got))
}

// PutSettings handles PUT /api/workspaces/:id/settings. The body is
// {"settings": {key: value}}; only the keys sent change, and all are checked
// before any is written.
func (h *SettingsHandler) PutSettings(c echo.Context) error {
	claims, err := httputil.RequireClaims(c)
	if err != nil {
		return err
	}
	var body settingsBody
	if err := c.Bind(&body); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid request body")
	}
	got, err := h.svc.PutUserSettings(c.Request().Context(), claims.NGACNodeID, claims.UserID, c.Param("id"), body.Settings)
	if err != nil {
		return httputil.MapDomainError(err)
	}
	return c.JSON(http.StatusOK, settingsOrEmpty(got))
}

type locksBody struct {
	GroupCreation string `json:"group_creation"`
}

// GetLocks handles GET /api/workspaces/:id/locks: the workspace's rules, which any
// member may read. group_creation is "everyone" or "admins".
func (h *SettingsHandler) GetLocks(c echo.Context) error {
	claims, err := httputil.RequireClaims(c)
	if err != nil {
		return err
	}
	rule, err := h.svc.GroupCreationRule(c.Request().Context(), claims.NGACNodeID, c.Param("id"))
	if err != nil {
		return httputil.MapDomainError(err)
	}
	return c.JSON(http.StatusOK, locksBody{GroupCreation: rule})
}

// PutLocks handles PUT /api/workspaces/:id/locks. Only a workspace administrator
// may change a rule; an organization workspace only.
func (h *SettingsHandler) PutLocks(c echo.Context) error {
	claims, err := httputil.RequireClaims(c)
	if err != nil {
		return err
	}
	var body locksBody
	if err := c.Bind(&body); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid request body")
	}
	rule, err := h.svc.SetGroupCreationRule(c.Request().Context(), claims.NGACNodeID, c.Param("id"), body.GroupCreation)
	if err != nil {
		return httputil.MapDomainError(err)
	}
	return c.JSON(http.StatusOK, locksBody{GroupCreation: rule})
}
