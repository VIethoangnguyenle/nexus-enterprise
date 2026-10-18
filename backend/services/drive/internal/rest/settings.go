package rest

import (
	"net/http"

	"github.com/labstack/echo/v4"

	"ngac-platform/pkg/httputil"
	"ngac-platform/services/drive/internal/domain"
)

// GetDriveSettings handles GET /api/drive/settings?workspace_id=.
func (h *Handler) GetDriveSettings(c echo.Context) error {
	claims, err := httputil.RequireClaims(c)
	if err != nil {
		return err
	}
	wsID := c.QueryParam("workspace_id")
	if wsID == "" {
		return echo.NewHTTPError(http.StatusBadRequest, "workspace_id required")
	}
	s, err := h.svc.GetDriveSettings(c.Request().Context(), claims.UserID, wsID)
	if err != nil {
		return mapError(err)
	}
	return c.JSON(http.StatusOK, s)
}

// UpdateDriveSettings handles PUT /api/drive/settings?workspace_id=. Fields the
// body leaves out keep their stored value.
func (h *Handler) UpdateDriveSettings(c echo.Context) error {
	claims, err := httputil.RequireClaims(c)
	if err != nil {
		return err
	}
	wsID := c.QueryParam("workspace_id")
	if wsID == "" {
		return echo.NewHTTPError(http.StatusBadRequest, "workspace_id required")
	}
	var body domain.DriveSettingsPatch
	if err := c.Bind(&body); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid request body")
	}
	s, err := h.svc.UpdateDriveSettings(c.Request().Context(), claims.UserID, wsID, &body)
	if err != nil {
		return mapError(err)
	}
	return c.JSON(http.StatusOK, s)
}
