package rest

import (
	"net/http"

	"github.com/labstack/echo/v4"

	"ngac-platform/pkg/httputil"
	"ngac-platform/services/approval/internal/domain"
)

// GetSettings handles GET /api/approval/settings.
func (h *Handler) GetSettings(c echo.Context) error {
	claims, err := httputil.RequireClaims(c)
	if err != nil {
		return err
	}
	s, err := h.svc.GetUserSettings(c.Request().Context(), claims.UserID)
	if err != nil {
		return mapDomainError(err)
	}
	return c.JSON(http.StatusOK, s)
}

// UpdateSettings handles PUT /api/approval/settings. Fields the body leaves
// out keep their stored value.
func (h *Handler) UpdateSettings(c echo.Context) error {
	claims, err := httputil.RequireClaims(c)
	if err != nil {
		return err
	}
	var body domain.ApprovalSettingsPatch
	if err := c.Bind(&body); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid request body")
	}
	s, err := h.svc.UpdateUserSettings(c.Request().Context(), claims.UserID, &body)
	if err != nil {
		return mapDomainError(err)
	}
	return c.JSON(http.StatusOK, s)
}
