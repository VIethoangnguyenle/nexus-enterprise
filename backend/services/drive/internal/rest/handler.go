// Package rest provides Echo REST handlers for the drive service.
// Delegates to the gRPC server (which contains the business logic) as a
// transitional adapter. Future: extract domain layer from gRPC handler.
package rest

import (
	"context"
	"net/http"

	"github.com/labstack/echo/v4"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"ngac-platform/pkg/httputil"
	pb "ngac-platform/proto/drive"
)

// DriveService defines the operations the REST handler needs.
// Currently implemented by the gRPC DriveServer directly.
type DriveService interface {
	CreateFolder(ctx context.Context, req *pb.CreateFolderRequest) (*pb.DriveItem, error)
	ListFolder(ctx context.Context, req *pb.ListFolderRequest) (*pb.DriveItemList, error)
	GetItem(ctx context.Context, req *pb.GetItemRequest) (*pb.DriveItem, error)
	CreateFile(ctx context.Context, req *pb.CreateFileRequest) (*pb.CreateFileResponse, error)
	ConfirmFile(ctx context.Context, req *pb.ConfirmFileRequest) (*pb.DriveItem, error)
	GetDownloadURL(ctx context.Context, req *pb.GetDownloadURLRequest) (*pb.GetDownloadURLResponse, error)
	RenameItem(ctx context.Context, req *pb.RenameItemRequest) (*pb.DriveItem, error)
	MoveItem(ctx context.Context, req *pb.MoveItemRequest) (*pb.DriveItem, error)
	CopyItem(ctx context.Context, req *pb.CopyItemRequest) (*pb.DriveItem, error)
	TrashItem(ctx context.Context, req *pb.TrashItemRequest) (*pb.Empty, error)
	RestoreItem(ctx context.Context, req *pb.RestoreItemRequest) (*pb.DriveItem, error)
	DeleteItem(ctx context.Context, req *pb.DeleteItemRequest) (*pb.Empty, error)
	CreateShare(ctx context.Context, req *pb.CreateShareRequest) (*pb.ShareInfo, error)
	RevokeShare(ctx context.Context, req *pb.RevokeShareRequest) (*pb.Empty, error)
	ListShares(ctx context.Context, req *pb.ListSharesRequest) (*pb.ShareList, error)
	GetSharedWithMe(ctx context.Context, req *pb.GetSharedWithMeRequest) (*pb.DriveItemList, error)
	GetChannelDrive(ctx context.Context, req *pb.GetChannelDriveRequest) (*pb.DriveItem, error)
	GetQuota(ctx context.Context, req *pb.GetQuotaRequest) (*pb.Quota, error)
}

// Handler serves drive REST endpoints.
type Handler struct {
	svc DriveService
}

// NewHandler creates a drive REST handler.
func NewHandler(svc DriveService) *Handler {
	return &Handler{svc: svc}
}

// RegisterRoutes mounts drive endpoints on the Echo instance.
func (h *Handler) RegisterRoutes(e *echo.Echo, jwtSecret string) {
	api := e.Group("/api", httputil.JWTMiddleware(jwtSecret))

	// Folders
	api.POST("/workspaces/:id/drive/folders", h.CreateFolder)
	api.GET("/workspaces/:id/drive", h.ListRoot)
	api.GET("/drive/folders/:folderId", h.ListFolder)

	// Items
	api.GET("/drive/items/:itemId", h.GetItem)
	api.POST("/drive/items/:itemId/move", h.MoveItem)
	api.POST("/drive/items/:itemId/copy", h.CopyItem)
	api.PUT("/drive/items/:itemId/rename", h.RenameItem)
	api.DELETE("/drive/items/:itemId", h.TrashItem)
	api.POST("/drive/items/:itemId/restore", h.RestoreItem)
	api.DELETE("/drive/items/:itemId/permanent", h.DeleteItem)

	// Files
	api.POST("/workspaces/:id/drive/files", h.CreateFile)
	api.POST("/drive/files/:fileId/confirm", h.ConfirmFile)
	api.GET("/drive/files/:fileId/download", h.GetDownloadURL)

	// Sharing
	api.POST("/drive/items/:itemId/share", h.CreateShare)
	api.DELETE("/drive/shares/:shareId", h.RevokeShare)
	api.GET("/drive/items/:itemId/shares", h.ListShares)
	api.GET("/drive/shared-with-me", h.SharedWithMe)

	// Quota
	api.GET("/workspaces/:id/drive/quota", h.GetQuota)
}

// CreateFolder handles POST /api/workspaces/:id/drive/folders.
func (h *Handler) CreateFolder(c echo.Context) error {
	claims := httputil.GetClaims(c)
	var body struct {
		Name           string `json:"name"`
		ParentID       string `json:"parent_id"`
		DriveContext   string `json:"drive_context"`
		DriveContextID string `json:"drive_context_id"`
	}
	if err := c.Bind(&body); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid request body")
	}

	resp, err := h.svc.CreateFolder(c.Request().Context(), &pb.CreateFolderRequest{
		WorkspaceId:    c.Param("id"),
		Name:           body.Name,
		ParentId:       body.ParentID,
		UserNgacNodeId: claims.NGACNodeID,
		DriveContext:   body.DriveContext,
		DriveContextId: body.DriveContextID,
	})
	if err != nil {
		return mapGRPCError(err)
	}
	return c.JSON(http.StatusCreated, resp)
}

// ListRoot handles GET /api/workspaces/:id/drive.
func (h *Handler) ListRoot(c echo.Context) error {
	claims := httputil.GetClaims(c)
	resp, err := h.svc.ListFolder(c.Request().Context(), &pb.ListFolderRequest{
		WorkspaceId:    c.Param("id"),
		UserNgacNodeId: claims.NGACNodeID,
		DriveContext:   c.QueryParam("drive_context"),
		DriveContextId: c.QueryParam("drive_context_id"),
	})
	if err != nil {
		return mapGRPCError(err)
	}
	return c.JSON(http.StatusOK, resp)
}

// ListFolder handles GET /api/drive/folders/:folderId.
func (h *Handler) ListFolder(c echo.Context) error {
	claims := httputil.GetClaims(c)
	resp, err := h.svc.ListFolder(c.Request().Context(), &pb.ListFolderRequest{
		FolderId:       c.Param("folderId"),
		UserNgacNodeId: claims.NGACNodeID,
	})
	if err != nil {
		return mapGRPCError(err)
	}
	return c.JSON(http.StatusOK, resp)
}

// GetItem handles GET /api/drive/items/:itemId.
func (h *Handler) GetItem(c echo.Context) error {
	resp, err := h.svc.GetItem(c.Request().Context(), &pb.GetItemRequest{
		ItemId: c.Param("itemId"),
	})
	if err != nil {
		return mapGRPCError(err)
	}
	return c.JSON(http.StatusOK, resp)
}

// CreateFile handles POST /api/workspaces/:id/drive/files.
func (h *Handler) CreateFile(c echo.Context) error {
	claims := httputil.GetClaims(c)
	var body struct {
		Name     string `json:"name"`
		MimeType string `json:"mime_type"`
		Size     int64  `json:"size_bytes"`
		ParentID string `json:"parent_id"`
	}
	if err := c.Bind(&body); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid request body")
	}

	resp, err := h.svc.CreateFile(c.Request().Context(), &pb.CreateFileRequest{
		WorkspaceId:    c.Param("id"),
		Name:           body.Name,
		MimeType:       body.MimeType,
		SizeBytes:      body.Size,
		ParentId:       body.ParentID,
		UserId:         claims.UserID,
		UserNgacNodeId: claims.NGACNodeID,
	})
	if err != nil {
		return mapGRPCError(err)
	}
	return c.JSON(http.StatusCreated, resp)
}

// ConfirmFile handles POST /api/drive/files/:fileId/confirm.
func (h *Handler) ConfirmFile(c echo.Context) error {
	resp, err := h.svc.ConfirmFile(c.Request().Context(), &pb.ConfirmFileRequest{
		FileId: c.Param("fileId"),
	})
	if err != nil {
		return mapGRPCError(err)
	}
	return c.JSON(http.StatusOK, resp)
}

// GetDownloadURL handles GET /api/drive/files/:fileId/download.
func (h *Handler) GetDownloadURL(c echo.Context) error {
	claims := httputil.GetClaims(c)
	resp, err := h.svc.GetDownloadURL(c.Request().Context(), &pb.GetDownloadURLRequest{
		FileId:         c.Param("fileId"),
		UserNgacNodeId: claims.NGACNodeID,
	})
	if err != nil {
		return mapGRPCError(err)
	}
	return c.JSON(http.StatusOK, resp)
}

// RenameItem handles PUT /api/drive/items/:itemId/rename.
func (h *Handler) RenameItem(c echo.Context) error {
	claims := httputil.GetClaims(c)
	var body struct {
		Name string `json:"name"`
	}
	if err := c.Bind(&body); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid request body")
	}

	resp, err := h.svc.RenameItem(c.Request().Context(), &pb.RenameItemRequest{
		ItemId:         c.Param("itemId"),
		NewName:        body.Name,
		UserNgacNodeId: claims.NGACNodeID,
	})
	if err != nil {
		return mapGRPCError(err)
	}
	return c.JSON(http.StatusOK, resp)
}

// MoveItem handles POST /api/drive/items/:itemId/move.
func (h *Handler) MoveItem(c echo.Context) error {
	claims := httputil.GetClaims(c)
	var body struct {
		TargetFolderID string `json:"target_folder_id"`
	}
	if err := c.Bind(&body); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid request body")
	}

	resp, err := h.svc.MoveItem(c.Request().Context(), &pb.MoveItemRequest{
		ItemId:         c.Param("itemId"),
		NewParentId:    body.TargetFolderID,
		UserNgacNodeId: claims.NGACNodeID,
	})
	if err != nil {
		return mapGRPCError(err)
	}
	return c.JSON(http.StatusOK, resp)
}

// CopyItem handles POST /api/drive/items/:itemId/copy.
func (h *Handler) CopyItem(c echo.Context) error {
	claims := httputil.GetClaims(c)
	var body struct {
		TargetFolderID string `json:"target_folder_id"`
	}
	if err := c.Bind(&body); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid request body")
	}

	resp, err := h.svc.CopyItem(c.Request().Context(), &pb.CopyItemRequest{
		ItemId:       c.Param("itemId"),
		DestParentId: body.TargetFolderID,
		UserNgacNodeId: claims.NGACNodeID,
	})
	if err != nil {
		return mapGRPCError(err)
	}
	return c.JSON(http.StatusOK, resp)
}

// TrashItem handles DELETE /api/drive/items/:itemId.
func (h *Handler) TrashItem(c echo.Context) error {
	claims := httputil.GetClaims(c)
	_, err := h.svc.TrashItem(c.Request().Context(), &pb.TrashItemRequest{
		ItemId:         c.Param("itemId"),
		UserNgacNodeId: claims.NGACNodeID,
	})
	if err != nil {
		return mapGRPCError(err)
	}
	return c.JSON(http.StatusOK, map[string]string{"status": "trashed"})
}

// RestoreItem handles POST /api/drive/items/:itemId/restore.
func (h *Handler) RestoreItem(c echo.Context) error {
	resp, err := h.svc.RestoreItem(c.Request().Context(), &pb.RestoreItemRequest{
		ItemId: c.Param("itemId"),
	})
	if err != nil {
		return mapGRPCError(err)
	}
	return c.JSON(http.StatusOK, resp)
}

// DeleteItem handles DELETE /api/drive/items/:itemId/permanent.
func (h *Handler) DeleteItem(c echo.Context) error {
	claims := httputil.GetClaims(c)
	_, err := h.svc.DeleteItem(c.Request().Context(), &pb.DeleteItemRequest{
		ItemId:         c.Param("itemId"),
		UserNgacNodeId: claims.NGACNodeID,
	})
	if err != nil {
		return mapGRPCError(err)
	}
	return c.JSON(http.StatusOK, map[string]string{"status": "deleted"})
}

// CreateShare handles POST /api/drive/items/:itemId/share.
func (h *Handler) CreateShare(c echo.Context) error {
	claims := httputil.GetClaims(c)
	var body struct {
		TargetNodeID string `json:"target_node_id"`
		ShareType    string `json:"share_type"`
		Permission   string `json:"permission"`
	}
	if err := c.Bind(&body); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid request body")
	}

	resp, err := h.svc.CreateShare(c.Request().Context(), &pb.CreateShareRequest{
		ItemId:           c.Param("itemId"),
		UserNgacNodeId:   claims.NGACNodeID,
		TargetNgacNodeId: body.TargetNodeID,
		ShareType:        body.ShareType,
		Operations:       []string{body.Permission},
	})
	if err != nil {
		return mapGRPCError(err)
	}
	return c.JSON(http.StatusCreated, resp)
}

// RevokeShare handles DELETE /api/drive/shares/:shareId.
func (h *Handler) RevokeShare(c echo.Context) error {
	claims := httputil.GetClaims(c)
	_, err := h.svc.RevokeShare(c.Request().Context(), &pb.RevokeShareRequest{
		ShareId:        c.Param("shareId"),
		UserNgacNodeId: claims.NGACNodeID,
	})
	if err != nil {
		return mapGRPCError(err)
	}
	return c.JSON(http.StatusOK, map[string]string{"status": "revoked"})
}

// ListShares handles GET /api/drive/items/:itemId/shares.
func (h *Handler) ListShares(c echo.Context) error {
	resp, err := h.svc.ListShares(c.Request().Context(), &pb.ListSharesRequest{
		ItemId: c.Param("itemId"),
	})
	if err != nil {
		return mapGRPCError(err)
	}
	return c.JSON(http.StatusOK, resp)
}

// SharedWithMe handles GET /api/drive/shared-with-me.
func (h *Handler) SharedWithMe(c echo.Context) error {
	claims := httputil.GetClaims(c)
	resp, err := h.svc.GetSharedWithMe(c.Request().Context(), &pb.GetSharedWithMeRequest{
		UserNgacNodeId: claims.NGACNodeID,
	})
	if err != nil {
		return mapGRPCError(err)
	}
	return c.JSON(http.StatusOK, resp)
}

// GetQuota handles GET /api/workspaces/:id/drive/quota.
func (h *Handler) GetQuota(c echo.Context) error {
	resp, err := h.svc.GetQuota(c.Request().Context(), &pb.GetQuotaRequest{
		WorkspaceId: c.Param("id"),
	})
	if err != nil {
		return mapGRPCError(err)
	}
	return c.JSON(http.StatusOK, resp)
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
	case codes.Unauthenticated:
		return echo.NewHTTPError(http.StatusUnauthorized, st.Message())
	default:
		return echo.NewHTTPError(http.StatusInternalServerError, st.Message())
	}
}
