package rest

import (
	"net/http"

	"github.com/labstack/echo/v4"

	"ngac-platform/pkg/httputil"
	"ngac-platform/services/asset/internal/domain"
)

// GetAssetSettings handles GET /api/workspaces/:id/asset-settings.
func (h *Handler) GetAssetSettings(c echo.Context) error {
	claims, err := httputil.RequireClaims(c)
	if err != nil {
		return err
	}
	wsID := c.Param("id")
	if wsID == "" {
		return echo.NewHTTPError(http.StatusBadRequest, "workspace_id required")
	}
	s, err := h.settingsSvc.GetAssetSettings(c.Request().Context(), claims.UserID, wsID)
	if err != nil {
		return mapError(err)
	}
	return c.JSON(http.StatusOK, s)
}

// UpdateAssetSettings handles PUT /api/workspaces/:id/asset-settings. Fields
// the body leaves out keep their stored value.
func (h *Handler) UpdateAssetSettings(c echo.Context) error {
	claims, err := httputil.RequireClaims(c)
	if err != nil {
		return err
	}
	wsID := c.Param("id")
	if wsID == "" {
		return echo.NewHTTPError(http.StatusBadRequest, "workspace_id required")
	}
	var body domain.AssetSettingsPatch
	if err := c.Bind(&body); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid request body")
	}
	s, err := h.settingsSvc.UpdateAssetSettings(c.Request().Context(), claims.UserID, wsID, &body)
	if err != nil {
		return mapError(err)
	}
	return c.JSON(http.StatusOK, s)
}
