// Package rest provides Echo REST handlers for the document service.
// Most document endpoints are deprecated — migrated to the Drive service.
// The remaining active endpoints proxy to Drive for backwards compatibility.
package rest

import (
	"net/http"

	"github.com/labstack/echo/v4"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"ngac-platform/pkg/httputil"
	drivepb "ngac-platform/proto/drive"
)

// Handler serves document REST endpoints.
type Handler struct {
	drive drivepb.DriveServiceClient
}

// NewHandler creates a document REST handler.
func NewHandler(drive drivepb.DriveServiceClient) *Handler {
	return &Handler{drive: drive}
}

// RegisterRoutes mounts document endpoints on the Echo instance.
func (h *Handler) RegisterRoutes(e *echo.Echo, jwtSecret string) {
	api := e.Group("/api", httputil.JWTMiddleware(jwtSecret))

	// Active — proxy to Drive
	api.GET("/workspaces/:id/documents", h.ListDocuments)
	api.POST("/workspaces/:id/documents/upload-url", h.GetUploadURL)
	api.POST("/documents/:docId/confirm", h.ConfirmUpload)
	api.GET("/documents/:docId/download-url", h.GetDownloadURL)

	// Deprecated — return 410 Gone
	api.POST("/workspaces/:id/documents", h.UploadDocument)
	api.POST("/documents/:docId/approve", h.ApproveDocument)
	api.POST("/documents/:docId/share", h.ShareDocument)
	api.POST("/documents/:docId/publish", h.PublishDocument)
}

// ListDocuments proxies to Drive ListFolder (legacy endpoint).
func (h *Handler) ListDocuments(c echo.Context) error {
	claims := httputil.GetClaims(c)
	resp, err := h.drive.ListFolder(c.Request().Context(), &drivepb.ListFolderRequest{
		WorkspaceId:    c.Param("id"),
		UserNgacNodeId: claims.NGACNodeID,
	})
	if err != nil {
		return mapGRPCError(err)
	}
	return c.JSON(http.StatusOK, resp)
}

// GetUploadURL proxies to Drive CreateFile (legacy endpoint).
func (h *Handler) GetUploadURL(c echo.Context) error {
	claims := httputil.GetClaims(c)
	var body struct {
		Filename string `json:"filename"`
		MimeType string `json:"mime_type"`
		Size     int64  `json:"size_bytes"`
		ParentID string `json:"parent_id"`
	}
	if err := c.Bind(&body); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid request body")
	}
	resp, err := h.drive.CreateFile(c.Request().Context(), &drivepb.CreateFileRequest{
		WorkspaceId:    c.Param("id"),
		Name:           body.Filename,
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

// ConfirmUpload proxies to Drive ConfirmFile (legacy endpoint).
func (h *Handler) ConfirmUpload(c echo.Context) error {
	resp, err := h.drive.ConfirmFile(c.Request().Context(), &drivepb.ConfirmFileRequest{
		FileId: c.Param("docId"),
	})
	if err != nil {
		return mapGRPCError(err)
	}
	return c.JSON(http.StatusOK, resp)
}

// GetDownloadURL proxies to Drive GetDownloadURL (legacy endpoint).
func (h *Handler) GetDownloadURL(c echo.Context) error {
	claims := httputil.GetClaims(c)
	resp, err := h.drive.GetDownloadURL(c.Request().Context(), &drivepb.GetDownloadURLRequest{
		FileId:         c.Param("docId"),
		UserNgacNodeId: claims.NGACNodeID,
	})
	if err != nil {
		return mapGRPCError(err)
	}
	return c.JSON(http.StatusOK, resp)
}

// UploadDocument returns 410 Gone — deprecated in favor of Drive.
func (h *Handler) UploadDocument(c echo.Context) error {
	return c.JSON(http.StatusGone, map[string]string{"error": "deprecated: use /api/workspaces/{id}/drive/files"})
}

// ApproveDocument returns 410 Gone — deprecated.
func (h *Handler) ApproveDocument(c echo.Context) error {
	return c.JSON(http.StatusGone, map[string]string{"error": "deprecated: approvals moved to Drive"})
}

// ShareDocument returns 410 Gone — deprecated.
func (h *Handler) ShareDocument(c echo.Context) error {
	return c.JSON(http.StatusGone, map[string]string{"error": "deprecated: use /api/drive/items/{id}/share"})
}

// PublishDocument returns 410 Gone — deprecated.
func (h *Handler) PublishDocument(c echo.Context) error {
	return c.JSON(http.StatusGone, map[string]string{"error": "deprecated: use Drive sharing with public type"})
}

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
	case codes.InvalidArgument:
		return echo.NewHTTPError(http.StatusBadRequest, st.Message())
	default:
		return echo.NewHTTPError(http.StatusInternalServerError, st.Message())
	}
}
