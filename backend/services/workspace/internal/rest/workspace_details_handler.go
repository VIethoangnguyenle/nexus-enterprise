package rest

import (
	"context"
	"errors"
	"log/slog"
	"net/http"

	"github.com/labstack/echo/v4"

	"ngac-platform/pkg/httputil"
	"ngac-platform/services/workspace/internal/domain"
)

// WorkspaceDetailsService is what the workspace settings routes need.
type WorkspaceDetailsService interface {
	WorkspaceDetails(ctx context.Context, callerNodeID, wsID string) (*domain.WorkspaceDetails, error)
	UpdateWorkspaceDetails(ctx context.Context, callerNodeID, wsID string, name, description *string) (*domain.WorkspaceDetails, error)
	LeaveWorkspace(ctx context.Context, callerNodeID, wsID string) error
}

func (h *AdminHandler) registerWorkspaceDetailsRoutes(api *echo.Group) {
	api.GET("/workspaces/:id/details", h.GetWorkspaceDetails)
	api.PATCH("/workspaces/:id/details", h.UpdateWorkspaceDetails)
	api.POST("/workspaces/:id/leave", h.LeaveWorkspace)
}

// LeaveWorkspace handles POST /api/workspaces/:id/leave. The person leaving is
// always the caller in the verified claims; the body is ignored. A workspace's
// last Owner is refused with 409 and reason "last_owner".
func (h *AdminHandler) LeaveWorkspace(c echo.Context) error {
	claims, err := httputil.RequireClaims(c)
	if err != nil {
		return err
	}
	if err := h.domain.LeaveWorkspace(c.Request().Context(), claims.NGACNodeID, c.Param("id")); err != nil {
		if errors.Is(err, domain.ErrLastOwner) {
			return c.JSON(http.StatusConflict, map[string]string{
				"error":  "Bạn là chủ sở hữu cuối cùng. Chuyển quyền sở hữu cho người khác trước khi rời.",
				"reason": "last_owner",
			})
		}
		return detailsError(c, err)
	}
	return c.JSON(http.StatusOK, map[string]string{"status": "left"})
}

type workspaceDetailsJSON struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	CanManage   bool   `json:"can_manage"`
}

// GetWorkspaceDetails handles GET /api/workspaces/:id/details: a member reads
// the workspace's name and description and learns whether they may change them.
func (h *AdminHandler) GetWorkspaceDetails(c echo.Context) error {
	claims, err := httputil.RequireClaims(c)
	if err != nil {
		return err
	}
	d, err := h.domain.WorkspaceDetails(c.Request().Context(), claims.NGACNodeID, c.Param("id"))
	if err != nil {
		return detailsError(c, err)
	}
	return c.JSON(http.StatusOK, workspaceDetailsJSON{Name: d.Name, Description: d.Description, CanManage: d.CanManage})
}

// UpdateWorkspaceDetails handles PATCH /api/workspaces/:id/details.
func (h *AdminHandler) UpdateWorkspaceDetails(c echo.Context) error {
	claims, err := httputil.RequireClaims(c)
	if err != nil {
		return err
	}
	var body struct {
		Name        *string `json:"name"`
		Description *string `json:"description"`
	}
	if err := c.Bind(&body); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid request body")
	}
	d, err := h.domain.UpdateWorkspaceDetails(c.Request().Context(), claims.NGACNodeID, c.Param("id"), body.Name, body.Description)
	if err != nil {
		return detailsError(c, err)
	}
	return c.JSON(http.StatusOK, workspaceDetailsJSON{Name: d.Name, Description: d.Description, CanManage: d.CanManage})
}

// detailsError answers a domain error with its status. Anything that is not one
// of the domain's own refusals is logged and answered generically, so database
// text never reaches a client.
func detailsError(c echo.Context, err error) error {
	switch {
	case errors.Is(err, httputil.ErrNotFound), errors.Is(err, httputil.ErrAccessDenied), errors.Is(err, httputil.ErrInvalidInput):
		return httputil.MapDomainError(err)
	}
	slog.Error("workspace details request failed", "path", c.Path(), "error", err)
	return echo.NewHTTPError(http.StatusInternalServerError, "internal error")
}
