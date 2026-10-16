package rest

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/labstack/echo/v4"

	"ngac-platform/pkg/httputil"
	"ngac-platform/services/drive/internal/domain"
)

const (
	defaultThumbnailSize = 320
	// A thumbnail never changes for a given version of the file (the ETag), so
	// the browser may keep it for a day without asking.
	thumbnailCacheControl = "private, max-age=86400"
	// thumbnailRetryAfter is what a busy answer tells the client, in seconds.
	thumbnailRetryAfter = "2"
)

// GetThumbnail handles GET /api/drive/files/:fileId/thumbnail?size=320|640. It
// answers 200 with a JPEG, 304 when If-None-Match is current, 403 when the
// caller may not read the file, 404 (reason thumbnail_unavailable) when the file
// has no thumbnail, and 503 with Retry-After when drawing is too busy.
func (h *Handler) GetThumbnail(c echo.Context) error {
	if _, err := httputil.RequireClaims(c); err != nil {
		return err
	}
	size := defaultThumbnailSize
	if q := c.QueryParam("size"); q != "" {
		n, err := strconv.Atoi(q)
		if err != nil {
			return echo.NewHTTPError(http.StatusBadRequest, "invalid size")
		}
		size = n
	}
	th, err := h.svc.GetThumbnail(c.Request().Context(), c.Param("fileId"), size, c.Request().Header.Get("If-None-Match"))
	if err != nil {
		if errors.Is(err, domain.ErrThumbnailBusy) {
			c.Response().Header().Set("Retry-After", thumbnailRetryAfter)
		}
		return mapError(err)
	}
	hdr := c.Response().Header()
	hdr.Set("Cache-Control", thumbnailCacheControl)
	hdr.Set("ETag", th.ETag)
	if th.NotModified {
		return c.NoContent(http.StatusNotModified)
	}
	hdr.Set("X-Content-Type-Options", "nosniff")
	return c.Blob(http.StatusOK, "image/jpeg", th.Data)
}
