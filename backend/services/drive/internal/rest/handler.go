// Package rest provides Echo REST handlers for the drive service. They adapt
// HTTP to the domain service and hold no business logic.
package rest

import (
	"context"
	"errors"
	"net/http"

	"github.com/labstack/echo/v4"

	"ngac-platform/pkg/httputil"
	pb "ngac-platform/proto/drive"
	policypb "ngac-platform/proto/policy"
	"ngac-platform/services/drive/internal/domain"
	"ngac-platform/services/drive/internal/reason"
)

// DriveService defines the operations the REST handler needs. It is the drive
// domain service; the handler never goes through the gRPC server.
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

// mapError answers a domain error with its HTTP status. A conflict is a 409; a
// full quota is a 413 and an item changed by another request a 409, each with a
// reason the screen can act on. The other refusals the domain classifies map as
// everywhere (httputil.MapDomainError); anything else, including a lock that
// could not be had, is a generic 500.
func mapError(err error) *echo.HTTPError {
	switch {
	case errors.Is(err, domain.ErrConflict):
		return echo.NewHTTPError(http.StatusConflict, err.Error())
	case errors.Is(err, domain.ErrQuotaExceeded):
		return refusalWithReason(http.StatusRequestEntityTooLarge, err, reason.QuotaExceeded)
	case errors.Is(err, domain.ErrAborted):
		return refusalWithReason(http.StatusConflict, err, reason.ItemChanged)
	}
	return httputil.MapDomainError(err)
}

func refusalWithReason(code int, err error, why string) *echo.HTTPError {
	return echo.NewHTTPError(code, map[string]any{"message": err.Error(), "reason": why})
}

// Handler serves drive REST endpoints.
type Handler struct {
	svc        DriveService
	policyRead policypb.PolicyReadServiceClient
}

// NewHandler creates a drive REST handler.
func NewHandler(svc DriveService, policyRead policypb.PolicyReadServiceClient) *Handler {
	return &Handler{svc: svc, policyRead: policyRead}
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

	// Batch permission check
	api.POST("/drive/batch-access", h.BatchAccess)
}

// CreateFolder handles POST /api/workspaces/:id/drive/folders.
func (h *Handler) CreateFolder(c echo.Context) error {
	_, err := httputil.RequireClaims(c)
	if err != nil {
		return err
	}
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
		DriveContext:   body.DriveContext,
		DriveContextId: body.DriveContextID,
	})
	if err != nil {
		return mapError(err)
	}
	return c.JSON(http.StatusCreated, resp)
}

// ListRoot handles GET /api/workspaces/:id/drive.
func (h *Handler) ListRoot(c echo.Context) error {
	_, err := httputil.RequireClaims(c)
	if err != nil {
		return err
	}
	resp, err := h.svc.ListFolder(c.Request().Context(), &pb.ListFolderRequest{
		WorkspaceId:    c.Param("id"),
		DriveContext:   c.QueryParam("drive_context"),
		DriveContextId: c.QueryParam("drive_context_id"),
	})
	if err != nil {
		return mapError(err)
	}
	return c.JSON(http.StatusOK, resp)
}

// ListFolder handles GET /api/drive/folders/:folderId.
func (h *Handler) ListFolder(c echo.Context) error {
	_, err := httputil.RequireClaims(c)
	if err != nil {
		return err
	}
	resp, err := h.svc.ListFolder(c.Request().Context(), &pb.ListFolderRequest{
		FolderId: c.Param("folderId"),
		// Optional: a client that knows the workspace it is browsing names it
		// (?ws=), and the drive then refuses a folder of any other workspace.
		WorkspaceId: c.QueryParam("ws"),
	})
	if err != nil {
		return mapError(err)
	}
	return c.JSON(http.StatusOK, resp)
}

// GetItem handles GET /api/drive/items/:itemId.
func (h *Handler) GetItem(c echo.Context) error {
	_, err := httputil.RequireClaims(c)
	if err != nil {
		return err
	}
	resp, err := h.svc.GetItem(c.Request().Context(), &pb.GetItemRequest{
		ItemId: c.Param("itemId"),
	})
	if err != nil {
		return mapError(err)
	}
	return c.JSON(http.StatusOK, resp)
}

// CreateFile handles POST /api/workspaces/:id/drive/files.
func (h *Handler) CreateFile(c echo.Context) error {
	_, err := httputil.RequireClaims(c)
	if err != nil {
		return err
	}
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
		WorkspaceId: c.Param("id"),
		Name:        body.Name,
		MimeType:    body.MimeType,
		SizeBytes:   body.Size,
		ParentId:    body.ParentID,
	})
	if err != nil {
		return mapError(err)
	}
	return c.JSON(http.StatusCreated, resp)
}

// ConfirmFile handles POST /api/drive/files/:fileId/confirm.
func (h *Handler) ConfirmFile(c echo.Context) error {
	_, err := httputil.RequireClaims(c)
	if err != nil {
		return err
	}
	resp, err := h.svc.ConfirmFile(c.Request().Context(), &pb.ConfirmFileRequest{
		FileId: c.Param("fileId"),
	})
	if err != nil {
		return mapError(err)
	}
	return c.JSON(http.StatusOK, resp)
}

// GetDownloadURL handles GET /api/drive/files/:fileId/download.
func (h *Handler) GetDownloadURL(c echo.Context) error {
	_, err := httputil.RequireClaims(c)
	if err != nil {
		return err
	}
	resp, err := h.svc.GetDownloadURL(c.Request().Context(), &pb.GetDownloadURLRequest{
		FileId: c.Param("fileId"),
	})
	if err != nil {
		return mapError(err)
	}
	return c.JSON(http.StatusOK, resp)
}

// RenameItem handles PUT /api/drive/items/:itemId/rename.
func (h *Handler) RenameItem(c echo.Context) error {
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

	resp, err := h.svc.RenameItem(c.Request().Context(), &pb.RenameItemRequest{
		ItemId:  c.Param("itemId"),
		NewName: body.Name,
	})
	if err != nil {
		return mapError(err)
	}
	return c.JSON(http.StatusOK, resp)
}

// MoveItem handles POST /api/drive/items/:itemId/move.
func (h *Handler) MoveItem(c echo.Context) error {
	_, err := httputil.RequireClaims(c)
	if err != nil {
		return err
	}
	var body struct {
		TargetFolderID string `json:"target_folder_id"`
	}
	if err := c.Bind(&body); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid request body")
	}

	resp, err := h.svc.MoveItem(c.Request().Context(), &pb.MoveItemRequest{
		ItemId:      c.Param("itemId"),
		NewParentId: body.TargetFolderID,
	})
	if err != nil {
		return mapError(err)
	}
	return c.JSON(http.StatusOK, resp)
}

// CopyItem handles POST /api/drive/items/:itemId/copy.
func (h *Handler) CopyItem(c echo.Context) error {
	_, err := httputil.RequireClaims(c)
	if err != nil {
		return err
	}
	var body struct {
		TargetFolderID string `json:"target_folder_id"`
	}
	if err := c.Bind(&body); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid request body")
	}

	resp, err := h.svc.CopyItem(c.Request().Context(), &pb.CopyItemRequest{
		ItemId:       c.Param("itemId"),
		DestParentId: body.TargetFolderID,
	})
	if err != nil {
		return mapError(err)
	}
	return c.JSON(http.StatusOK, resp)
}

// TrashItem handles DELETE /api/drive/items/:itemId.
func (h *Handler) TrashItem(c echo.Context) error {
	_, err := httputil.RequireClaims(c)
	if err != nil {
		return err
	}
	_, err = h.svc.TrashItem(c.Request().Context(), &pb.TrashItemRequest{
		ItemId: c.Param("itemId"),
	})
	if err != nil {
		return mapError(err)
	}
	return c.JSON(http.StatusOK, map[string]string{"status": "trashed"})
}

// RestoreItem handles POST /api/drive/items/:itemId/restore.
func (h *Handler) RestoreItem(c echo.Context) error {
	_, err := httputil.RequireClaims(c)
	if err != nil {
		return err
	}
	resp, err := h.svc.RestoreItem(c.Request().Context(), &pb.RestoreItemRequest{
		ItemId: c.Param("itemId"),
	})
	if err != nil {
		return mapError(err)
	}
	return c.JSON(http.StatusOK, resp)
}

// DeleteItem handles DELETE /api/drive/items/:itemId/permanent.
func (h *Handler) DeleteItem(c echo.Context) error {
	_, err := httputil.RequireClaims(c)
	if err != nil {
		return err
	}
	_, err = h.svc.DeleteItem(c.Request().Context(), &pb.DeleteItemRequest{
		ItemId: c.Param("itemId"),
	})
	if err != nil {
		// A folder that still holds text documents is refused with 409 and a
		// machine-readable reason; the writing in it is never deleted with it.
		if errors.Is(err, domain.ErrFolderHasDocuments) {
			return c.JSON(http.StatusConflict, map[string]string{
				"error":  "Thư mục này còn văn bản. Chuyển hoặc xoá các văn bản trước, rồi xoá thư mục.",
				"reason": reason.FolderHasDocuments,
			})
		}
		return mapError(err)
	}
	return c.JSON(http.StatusOK, map[string]string{"status": "deleted"})
}
