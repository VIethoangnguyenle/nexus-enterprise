// Package rest provides Echo REST handlers for the workspace service.
// Delegates to the gRPC server (transitional — domain layer extraction is future work).
package rest

import (
	"context"
	"net/http"

	"github.com/labstack/echo/v4"

	"ngac-platform/pkg/httputil"
	pb "ngac-platform/proto/workspace"
)

// The caller is always taken from verified JWT claims — never from the request
// body. httputil.SetClaims puts it on the request context, which the service
// reads with grpcauth.CallerFrom. Authorization itself happens in the domain.

// WorkspaceService defines the operations the REST handler needs.
type WorkspaceService interface {
	CreateWorkspace(ctx context.Context, req *pb.CreateWorkspaceRequest) (*pb.Workspace, error)
	ListWorkspaces(ctx context.Context, req *pb.ListWorkspacesRequest) (*pb.WorkspaceList, error)
	GetWorkspace(ctx context.Context, req *pb.GetWorkspaceRequest) (*pb.Workspace, error)
	RemoveMember(ctx context.Context, req *pb.RemoveMemberRequest) (*pb.Empty, error)
	ListMembers(ctx context.Context, req *pb.ListMembersRequest) (*pb.MemberList, error)
	CreateFolder(ctx context.Context, req *pb.CreateFolderRequest) (*pb.Folder, error)
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

	api.POST("/workspaces", h.CreateWorkspace)
	api.GET("/workspaces", h.ListWorkspaces)
	api.GET("/workspaces/:id", h.GetWorkspace)
	api.DELETE("/workspaces/:id/members/:nodeId", h.RemoveMember)
	api.GET("/workspaces/:id/members", h.ListMembers)
	api.POST("/workspaces/:id/folders", h.CreateFolder)
}

// CreateWorkspace handles POST /api/workspaces.
func (h *Handler) CreateWorkspace(c echo.Context) error {
	_, err := httputil.RequireClaims(c)
	if err != nil {
		return err
	}
	var body struct {
		Name string `json:"name"`
	}
	if err := c.Bind(&body); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid request body")
	}

	resp, err := h.svc.CreateWorkspace(c.Request().Context(), &pb.CreateWorkspaceRequest{
		Name: body.Name,
	})
	if err != nil {
		return httputil.MapGRPCError(err)
	}
	return c.JSON(http.StatusCreated, resp)
}

// ListWorkspaces handles GET /api/workspaces.
func (h *Handler) ListWorkspaces(c echo.Context) error {
	_, err := httputil.RequireClaims(c)
	if err != nil {
		return err
	}
	resp, err := h.svc.ListWorkspaces(c.Request().Context(), &pb.ListWorkspacesRequest{})
	if err != nil {
		return httputil.MapGRPCError(err)
	}
	return c.JSON(http.StatusOK, resp)
}

// GetWorkspace handles GET /api/workspaces/:id.
func (h *Handler) GetWorkspace(c echo.Context) error {
	_, err := httputil.RequireClaims(c)
	if err != nil {
		return err
	}
	ctx := c.Request().Context()
	resp, err := h.svc.GetWorkspace(ctx, &pb.GetWorkspaceRequest{
		WorkspaceId: c.Param("id"),
	})
	if err != nil {
		return httputil.MapGRPCError(err)
	}
	return c.JSON(http.StatusOK, resp)
}

// RemoveMember handles DELETE /api/workspaces/:id/members/:nodeId.
func (h *Handler) RemoveMember(c echo.Context) error {
	_, err := httputil.RequireClaims(c)
	if err != nil {
		return err
	}
	_, err = h.svc.RemoveMember(c.Request().Context(), &pb.RemoveMemberRequest{
		WorkspaceId:      c.Param("id"),
		TargetNgacNodeId: c.Param("nodeId"),
	})
	if err != nil {
		return httputil.MapGRPCError(err)
	}
	return c.JSON(http.StatusOK, map[string]string{"status": "ok"})
}

// ListMembers handles GET /api/workspaces/:id/members.
func (h *Handler) ListMembers(c echo.Context) error {
	_, err := httputil.RequireClaims(c)
	if err != nil {
		return err
	}
	ctx := c.Request().Context()
	resp, err := h.svc.ListMembers(ctx, &pb.ListMembersRequest{
		WorkspaceId: c.Param("id"),
	})
	if err != nil {
		return httputil.MapGRPCError(err)
	}
	return c.JSON(http.StatusOK, resp)
}

// CreateFolder handles POST /api/workspaces/:id/folders.
func (h *Handler) CreateFolder(c echo.Context) error {
	_, err := httputil.RequireClaims(c)
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

	resp, err := h.svc.CreateFolder(c.Request().Context(), &pb.CreateFolderRequest{
		WorkspaceId: c.Param("id"),
		Name:        body.Name,
		ParentOaId:  body.ParentOAID,
	})
	if err != nil {
		return httputil.MapGRPCError(err)
	}
	return c.JSON(http.StatusCreated, resp)
}
