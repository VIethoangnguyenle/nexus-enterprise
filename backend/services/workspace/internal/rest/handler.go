// Package rest provides Echo REST handlers for the workspace service.
// Delegates to the gRPC server (transitional — domain layer extraction is future work).
package rest

import (
	"context"
	"net/http"

	"github.com/labstack/echo/v4"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"ngac-platform/pkg/httputil"
	pb "ngac-platform/proto/workspace"
)

// WorkspaceService defines the operations the REST handler needs.
type WorkspaceService interface {
	CreateWorkspace(ctx context.Context, req *pb.CreateWorkspaceRequest) (*pb.Workspace, error)
	ListWorkspaces(ctx context.Context, req *pb.ListWorkspacesRequest) (*pb.WorkspaceList, error)
	GetWorkspace(ctx context.Context, req *pb.GetWorkspaceRequest) (*pb.Workspace, error)
	InviteMember(ctx context.Context, req *pb.InviteMemberRequest) (*pb.Empty, error)
	RemoveMember(ctx context.Context, req *pb.RemoveMemberRequest) (*pb.Empty, error)
	ListMembers(ctx context.Context, req *pb.ListMembersRequest) (*pb.MemberList, error)
	CreateRole(ctx context.Context, req *pb.CreateRoleRequest) (*pb.Role, error)
	ListRoles(ctx context.Context, req *pb.ListRolesRequest) (*pb.RoleList, error)
	CreateFolder(ctx context.Context, req *pb.CreateFolderRequest) (*pb.Folder, error)
	CreatePermission(ctx context.Context, req *pb.CreatePermissionRequest) (*pb.Permission, error)
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
	api.POST("/workspaces/:id/invite", h.InviteMember)
	api.DELETE("/workspaces/:id/members/:nodeId", h.RemoveMember)
	api.GET("/workspaces/:id/members", h.ListMembers)
	api.POST("/workspaces/:id/roles", h.CreateRole)
	api.GET("/workspaces/:id/roles", h.ListRoles)
	api.POST("/workspaces/:id/folders", h.CreateFolder)
	api.POST("/workspaces/:id/permissions", h.CreatePermission)
}

// CreateWorkspace handles POST /api/workspaces.
func (h *Handler) CreateWorkspace(c echo.Context) error {
	claims := httputil.GetClaims(c)
	var body struct {
		Name string `json:"name"`
	}
	if err := c.Bind(&body); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid request body")
	}

	resp, err := h.svc.CreateWorkspace(c.Request().Context(), &pb.CreateWorkspaceRequest{
		Name:           body.Name,
		UserId:         claims.UserID,
		UserNgacNodeId: claims.NGACNodeID,
	})
	if err != nil {
		return mapGRPCError(err)
	}
	return c.JSON(http.StatusCreated, resp)
}

// ListWorkspaces handles GET /api/workspaces.
func (h *Handler) ListWorkspaces(c echo.Context) error {
	claims := httputil.GetClaims(c)
	resp, err := h.svc.ListWorkspaces(c.Request().Context(), &pb.ListWorkspacesRequest{
		UserNgacNodeId: claims.NGACNodeID,
	})
	if err != nil {
		return mapGRPCError(err)
	}
	return c.JSON(http.StatusOK, resp)
}

// GetWorkspace handles GET /api/workspaces/:id.
func (h *Handler) GetWorkspace(c echo.Context) error {
	resp, err := h.svc.GetWorkspace(c.Request().Context(), &pb.GetWorkspaceRequest{
		WorkspaceId: c.Param("id"),
	})
	if err != nil {
		return mapGRPCError(err)
	}
	return c.JSON(http.StatusOK, resp)
}

// InviteMember handles POST /api/workspaces/:id/invite.
func (h *Handler) InviteMember(c echo.Context) error {
	claims := httputil.GetClaims(c)
	var body struct {
		NGACNodeID string `json:"ngac_node_id"`
	}
	if err := c.Bind(&body); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid request body")
	}

	_, err := h.svc.InviteMember(c.Request().Context(), &pb.InviteMemberRequest{
		WorkspaceId:       c.Param("id"),
		InviterNgacNodeId: claims.NGACNodeID,
		TargetNgacNodeId:  body.NGACNodeID,
	})
	if err != nil {
		return mapGRPCError(err)
	}
	return c.JSON(http.StatusOK, map[string]string{"status": "ok"})
}

// RemoveMember handles DELETE /api/workspaces/:id/members/:nodeId.
func (h *Handler) RemoveMember(c echo.Context) error {
	_, err := h.svc.RemoveMember(c.Request().Context(), &pb.RemoveMemberRequest{
		WorkspaceId:      c.Param("id"),
		TargetNgacNodeId: c.Param("nodeId"),
	})
	if err != nil {
		return mapGRPCError(err)
	}
	return c.JSON(http.StatusOK, map[string]string{"status": "ok"})
}

// ListMembers handles GET /api/workspaces/:id/members.
func (h *Handler) ListMembers(c echo.Context) error {
	resp, err := h.svc.ListMembers(c.Request().Context(), &pb.ListMembersRequest{
		WorkspaceId: c.Param("id"),
	})
	if err != nil {
		return mapGRPCError(err)
	}
	return c.JSON(http.StatusOK, resp)
}

// CreateRole handles POST /api/workspaces/:id/roles.
func (h *Handler) CreateRole(c echo.Context) error {
	var body struct {
		Name string `json:"name"`
	}
	if err := c.Bind(&body); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid request body")
	}

	resp, err := h.svc.CreateRole(c.Request().Context(), &pb.CreateRoleRequest{
		WorkspaceId: c.Param("id"),
		Name:        body.Name,
	})
	if err != nil {
		return mapGRPCError(err)
	}
	return c.JSON(http.StatusCreated, resp)
}

// ListRoles handles GET /api/workspaces/:id/roles.
func (h *Handler) ListRoles(c echo.Context) error {
	resp, err := h.svc.ListRoles(c.Request().Context(), &pb.ListRolesRequest{
		WorkspaceId: c.Param("id"),
	})
	if err != nil {
		return mapGRPCError(err)
	}
	return c.JSON(http.StatusOK, resp)
}

// CreateFolder handles POST /api/workspaces/:id/folders.
func (h *Handler) CreateFolder(c echo.Context) error {
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
		return mapGRPCError(err)
	}
	return c.JSON(http.StatusCreated, resp)
}

// CreatePermission handles POST /api/workspaces/:id/permissions.
func (h *Handler) CreatePermission(c echo.Context) error {
	var body struct {
		UAID       string   `json:"ua_id"`
		OAID       string   `json:"oa_id"`
		Operations []string `json:"operations"`
	}
	if err := c.Bind(&body); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid request body")
	}

	resp, err := h.svc.CreatePermission(c.Request().Context(), &pb.CreatePermissionRequest{
		WorkspaceId: c.Param("id"),
		UaId:        body.UAID,
		OaId:        body.OAID,
		Operations:  body.Operations,
	})
	if err != nil {
		return mapGRPCError(err)
	}
	return c.JSON(http.StatusCreated, resp)
}

// mapGRPCError translates gRPC status codes to Echo HTTP errors.
func mapGRPCError(err error) *echo.HTTPError {
	st, ok := status.FromError(err)
	if !ok {
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}
	switch st.Code() {
	case codes.NotFound:
		return echo.NewHTTPError(http.StatusNotFound, st.Message())
	case codes.PermissionDenied:
		return echo.NewHTTPError(http.StatusForbidden, st.Message())
	case codes.AlreadyExists:
		return echo.NewHTTPError(http.StatusConflict, st.Message())
	case codes.InvalidArgument:
		return echo.NewHTTPError(http.StatusBadRequest, st.Message())
	default:
		return echo.NewHTTPError(http.StatusInternalServerError, st.Message())
	}
}
