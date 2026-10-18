package rest

import (
	"net/http"

	"github.com/labstack/echo/v4"

	"ngac-platform/pkg/httputil"
	"ngac-platform/services/drive/internal/domain"
)

// StarItem handles POST /api/drive/items/:itemId/star. The item's own
// workspace is the one checked; nothing in the request names it.
func (h *Handler) StarItem(c echo.Context) error {
	if _, err := httputil.RequireClaims(c); err != nil {
		return err
	}
	if err := h.svc.StarItem(c.Request().Context(), c.Param("itemId")); err != nil {
		return mapError(err)
	}
	return c.NoContent(http.StatusNoContent)
}

// UnstarItem handles DELETE /api/drive/items/:itemId/star: the caller's own star only.
func (h *Handler) UnstarItem(c echo.Context) error {
	if _, err := httputil.RequireClaims(c); err != nil {
		return err
	}
	if err := h.svc.UnstarItem(c.Request().Context(), c.Param("itemId")); err != nil {
		return mapError(err)
	}
	return c.NoContent(http.StatusNoContent)
}

// workspaceParam reads the required workspace_id query parameter.
func workspaceParam(c echo.Context) (string, error) {
	ws := c.QueryParam("workspace_id")
	if ws == "" {
		return "", echo.NewHTTPError(http.StatusBadRequest, "workspace_id is required")
	}
	return ws, nil
}

// ListStarred handles GET /api/drive/starred?workspace_id=…
func (h *Handler) ListStarred(c echo.Context) error {
	if _, err := httputil.RequireClaims(c); err != nil {
		return err
	}
	ws, err := workspaceParam(c)
	if err != nil {
		return err
	}
	resp, err := h.svc.ListStarredItems(c.Request().Context(), ws)
	if err != nil {
		return mapError(err)
	}
	return c.JSON(http.StatusOK, resp)
}

// RecentFiles handles GET /api/drive/recent?workspace_id=…
func (h *Handler) RecentFiles(c echo.Context) error {
	if _, err := httputil.RequireClaims(c); err != nil {
		return err
	}
	ws, err := workspaceParam(c)
	if err != nil {
		return err
	}
	resp, err := h.svc.RecentFiles(c.Request().Context(), ws)
	if err != nil {
		return mapError(err)
	}
	return c.JSON(http.StatusOK, resp)
}

// quickAccessResponse is the body of GET /api/drive/quick-access.
type quickAccessResponse struct {
	Entries []*domain.QuickAccessEntry `json:"entries"`
}

// QuickAccess handles GET /api/drive/quick-access?workspace_id=…
func (h *Handler) QuickAccess(c echo.Context) error {
	if _, err := httputil.RequireClaims(c); err != nil {
		return err
	}
	ws, err := workspaceParam(c)
	if err != nil {
		return err
	}
	entries, err := h.svc.QuickAccessItems(c.Request().Context(), ws)
	if err != nil {
		return mapError(err)
	}
	return c.JSON(http.StatusOK, quickAccessResponse{Entries: entries})
}
