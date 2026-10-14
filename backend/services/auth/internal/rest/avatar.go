package rest

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"

	"github.com/labstack/echo/v4"

	"ngac-platform/pkg/httputil"
	"ngac-platform/services/auth/internal/domain"
)

// avatarBodyLimit is the most of a request body an upload reads: the file
// limit plus room for the multipart envelope around it. The file itself is
// checked against domain.AvatarMaxBytes on its own.
const avatarBodyLimit = domain.AvatarMaxBytes + 64<<10

// avatarBusyRetry is the Retry-After of a busy answer, in seconds: one
// picture takes well under a second to process.
const avatarBusyRetry = "2"

// UploadAvatar handles PUT /api/me/avatar: a multipart form whose "file" part
// is a JPEG or PNG of at most 5 MB. The picture is always the caller's own.
// Answers the URL the profile now points at.
func (h *Handler) UploadAvatar(c echo.Context) error {
	claims, err := httputil.RequireClaims(c)
	if err != nil {
		return err
	}
	req := c.Request()
	// Cut the body off at the limit before anything parses it, so an
	// oversized upload is refused without being read.
	req.Body = http.MaxBytesReader(c.Response(), req.Body, avatarBodyLimit)
	fh, err := c.FormFile("file")
	if err != nil {
		return avatarFormError(err)
	}
	if fh.Size > domain.AvatarMaxBytes {
		return avatarTooLarge()
	}
	f, err := fh.Open()
	if err != nil {
		return internalFailure(fmt.Errorf("open avatar upload: %w", err))
	}
	defer f.Close()
	b, err := io.ReadAll(io.LimitReader(f, domain.AvatarMaxBytes+1))
	if err != nil {
		return internalFailure(fmt.Errorf("read avatar upload: %w", err))
	}
	if len(b) > domain.AvatarMaxBytes {
		return avatarTooLarge()
	}

	url, err := h.svc.SetAvatar(req.Context(), claims.UserID, b)
	if err != nil {
		return failAvatar(c, err)
	}
	return c.JSON(http.StatusOK, map[string]string{"avatar_url": url})
}

// RemoveAvatar handles DELETE /api/me/avatar: the caller's initials show again.
func (h *Handler) RemoveAvatar(c echo.Context) error {
	claims, err := httputil.RequireClaims(c)
	if err != nil {
		return err
	}
	if err := h.svc.ClearAvatar(c.Request().Context(), claims.UserID); err != nil {
		return fail(c, err)
	}
	return c.JSON(http.StatusOK, map[string]string{"avatar_url": ""})
}

// ServeAvatar handles GET /api/auth/avatars/:token and .../:token/:size, size
// 64, 192 (the default) or 1024. It is public: an <img> sends no bearer token.
// The token is random per upload, so the URL reveals nothing, and a new
// picture gets a new URL, which is what lets a browser keep this one for good.
func (h *Handler) ServeAvatar(c echo.Context) error {
	size := 192
	if s := c.Param("size"); s != "" {
		n, err := strconv.Atoi(s)
		if err != nil {
			return avatarNotFound(c)
		}
		size = n
	}
	b, err := h.svc.Avatar(c.Request().Context(), c.Param("token"), size)
	if errors.Is(err, domain.ErrNotFound) {
		return avatarNotFound(c)
	}
	if err != nil {
		return fail(c, err)
	}
	hd := c.Response().Header()
	hd.Set("Cache-Control", "public, max-age=31536000, immutable")
	hd.Set("X-Content-Type-Options", "nosniff")
	// The bytes are only ever a JPEG we encoded, but nothing served from here
	// may run as a page.
	hd.Set("Content-Security-Policy", "default-src 'none'")
	hd.Set("Cross-Origin-Resource-Policy", "same-origin")
	hd.Set("Content-Disposition", `inline; filename="avatar.jpg"`)
	return c.Blob(http.StatusOK, "image/jpeg", b)
}

// avatarNotFound is a 404 that a browser may remember for a minute, so a
// page full of stale addresses does not ask again on every render.
func avatarNotFound(c echo.Context) error {
	c.Response().Header().Set("Cache-Control", "public, max-age=60")
	return apiError(http.StatusNotFound, "not_found", "no such picture")
}

func avatarTooLarge() error {
	return apiError(http.StatusRequestEntityTooLarge, "avatar_too_large", "the picture is larger than 5 MB")
}

// avatarFormError says what was wrong with the form an upload came in.
// The size check comes first: a body cut off at the limit also fails to parse.
func avatarFormError(err error) error {
	var tooBig *http.MaxBytesError
	switch {
	case errors.As(err, &tooBig):
		return avatarTooLarge()
	case errors.Is(err, http.ErrMissingFile):
		return apiError(http.StatusBadRequest, "avatar_missing", "send the picture as the form field \"file\"")
	default:
		return apiError(http.StatusBadRequest, "invalid_input", "send the picture as a multipart form")
	}
}

// failAvatar is fail, plus when to come back after a busy answer.
func failAvatar(c echo.Context, err error) error {
	if errors.Is(err, domain.ErrAvatarBusy) {
		c.Response().Header().Set(echo.HeaderRetryAfter, avatarBusyRetry)
	}
	return fail(c, err)
}
