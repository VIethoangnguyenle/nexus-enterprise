// Package rest provides Echo REST handlers for the workspace service. They adapt
// HTTP to the workspace domain service and never go through the gRPC server.
package rest

import (
	"context"
	"net/http"

	"github.com/labstack/echo/v4"

	"ngac-platform/pkg/httputil"
	"ngac-platform/services/workspace/internal/domain"
	"ngac-platform/services/workspace/internal/wire"
)

// The caller is always taken from verified JWT claims — never from the request
// body. Authorization itself happens in the domain.

// WorkspaceService defines the operations the REST handler needs: the domain
// service. Each takes the caller's NGAC user node and authorizes in the domain.
type WorkspaceService interface {
	ListAccessibleWorkspaces(ctx context.Context, userNGACNodeID string) ([]*domain.WorkspaceResult, error)
	ViewWorkspace(ctx context.Context, callerNodeID, id string) (*domain.WorkspaceResult, error)
	RemoveMember(ctx context.Context, callerNodeID, wsID, targetNGACNodeID string) error
	ListMembers(ctx context.Context, callerNodeID, wsID string) ([]*domain.Member, error)
	CreateFolder(ctx context.Context, callerNodeID, wsID, name, parentOaID string) (*domain.Folder, error)
}

// Handler serves workspace REST endpoints.
type Handler struct {
	svc WorkspaceService
}

// NewHandler creates a workspace REST handler.
func NewHandler(svc WorkspaceService) *Handler {
	return &Handler{svc: svc}
}

// RegisterRoutes mounts workspace endpoints on the Echo instance.
func (h *Handler) RegisterRoutes(e *echo.Echo, jwtSecret string) {
	api := e.Group("/api", httputil.JWTMiddleware(jwtSecret))

	api.POST("/workspaces", h.WorkspaceCreationMoved)
	api.GET("/workspaces", h.ListWorkspaces)
	api.GET("/workspaces/:id", h.GetWorkspace)
	api.DELETE("/workspaces/:id/members/:nodeId", h.RemoveMember)
	api.GET("/workspaces/:id/members", h.ListMembers)
	api.POST("/workspaces/:id/folders", h.CreateFolder)
}

// WorkspaceCreationMoved answers POST /api/workspaces, which used to create a
// workspace for any signed-in caller. Creation now belongs to the auth service
// (POST /api/me/workspaces, which also records the person's membership); this
// route says so rather than doing it a second, unchecked way.
func (h *Handler) WorkspaceCreationMoved(c echo.Context) error {
	return echo.NewHTTPError(http.StatusGone, "workspace creation moved to POST /api/me/workspaces")
}

// ListWorkspaces handles GET /api/workspaces.
func (h *Handler) ListWorkspaces(c echo.Context) error {
	claims, err := httputil.RequireClaims(c)
	if err != nil {
		return err
	}
	res, err := h.svc.ListAccessibleWorkspaces(c.Request().Context(), claims.NGACNodeID)
	if err != nil {
		return httputil.MapDomainError(err)
	}
	return c.JSON(http.StatusOK, wire.Workspaces(res))
}

// GetWorkspace handles GET /api/workspaces/:id.
func (h *Handler) GetWorkspace(c echo.Context) error {
	claims, err := httputil.RequireClaims(c)
	if err != nil {
		return err
	}
	res, err := h.svc.ViewWorkspace(c.Request().Context(), claims.NGACNodeID, c.Param("id"))
	if err != nil {
		return httputil.MapDomainError(err)
	}
	return c.JSON(http.StatusOK, wire.Workspace(res))
}

// RemoveMember handles DELETE /api/workspaces/:id/members/:nodeId.
func (h *Handler) RemoveMember(c echo.Context) error {
	claims, err := httputil.RequireClaims(c)
	if err != nil {
		return err
	}
	if err := h.svc.RemoveMember(c.Request().Context(), claims.NGACNodeID, c.Param("id"), c.Param("nodeId")); err != nil {
		return httputil.MapDomainError(err)
	}
	return c.JSON(http.StatusOK, map[string]string{"status": "ok"})
}

// ListMembers handles GET /api/workspaces/:id/members.
func (h *Handler) ListMembers(c echo.Context) error {
	claims, err := httputil.RequireClaims(c)
	if err != nil {
		return err
	}
	members, err := h.svc.ListMembers(c.Request().Context(), claims.NGACNodeID, c.Param("id"))
	if err != nil {
		return httputil.MapDomainError(err)
	}
	return c.JSON(http.StatusOK, wire.Members(members))
}

// CreateFolder handles POST /api/workspaces/:id/folders.
func (h *Handler) CreateFolder(c echo.Context) error {
	claims, err := httputil.RequireClaims(c)
	if err != nil {
		return err
	}
	var body struct {
		Name       string `json:"name"`
		ParentOAID string `json:"parent_oa_id"`
	}
	if err := c.Bind(&body); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid request body")
	}
	if body.Name == "" {
		return echo.NewHTTPError(http.StatusBadRequest, "name required")
	}

	f, err := h.svc.CreateFolder(c.Request().Context(), claims.NGACNodeID, c.Param("id"), body.Name, body.ParentOAID)
	if err != nil {
		return httputil.MapDomainError(err)
	}
	return c.JSON(http.StatusCreated, wire.Folder(f))
}
