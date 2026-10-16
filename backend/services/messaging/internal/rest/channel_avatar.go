package rest

import (
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"strconv"
	"time"

	"github.com/labstack/echo/v4"

	"ngac-platform/pkg/httputil"
	"ngac-platform/services/messaging/internal/domain"
)

// avatarBodyLimit is the most of a request body an upload reads: the file
// limit plus room for the multipart envelope around it. The file itself is
// checked against domain.AvatarMaxBytes on its own.
const avatarBodyLimit = domain.AvatarMaxBytes + 64<<10

// avatarBusyRetry is the Retry-After of a busy answer, in seconds: one
// picture takes well under a second to process.
const avatarBusyRetry = "2"

// registerAvatarRoutes mounts the picture of a group. The picture itself is
// public (an <img> sends no bearer token) and guarded by an unguessable token
// and a per-address limiter; changing it needs a session.
func (h *Handler) registerAvatarRoutes(e *echo.Echo, api *echo.Group) {
	e.GET("/api/channels/avatars/:token", h.ServeChannelAvatar, h.avatarLimiter())
	e.GET("/api/channels/avatars/:token/:size", h.ServeChannelAvatar, h.avatarLimiter())
	api.PUT("/channels/:chId/avatar", h.UploadChannelAvatar)
	api.DELETE("/channels/:chId/avatar", h.RemoveChannelAvatar)
}

// UploadChannelAvatar handles PUT /api/channels/:chId/avatar: a multipart form
// whose "file" part is a JPEG or PNG of at most 5 MB. Answers the URL the
// channel now points at.
func (h *Handler) UploadChannelAvatar(c echo.Context) error {
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
		return httputil.Internal(fmt.Errorf("open avatar upload: %w", err))
	}
	defer f.Close()
	b, err := io.ReadAll(io.LimitReader(f, domain.AvatarMaxBytes+1))
	if err != nil {
		return httputil.Internal(fmt.Errorf("read avatar upload: %w", err))
	}
	if len(b) > domain.AvatarMaxBytes {
		return avatarTooLarge()
	}

	url, err := h.svc.SetChannelAvatar(req.Context(), c.Param("chId"), claims.UserID, claims.NGACNodeID, b)
	if err != nil {
		return failAvatar(c, err)
	}
	return c.JSON(http.StatusOK, map[string]string{"avatar_url": url})
}

// RemoveChannelAvatar handles DELETE /api/channels/:chId/avatar: the group's
// initials show again.
func (h *Handler) RemoveChannelAvatar(c echo.Context) error {
	claims, err := httputil.RequireClaims(c)
	if err != nil {
		return err
	}
	if err := h.svc.ClearChannelAvatar(c.Request().Context(), c.Param("chId"), claims.NGACNodeID); err != nil {
		return failAvatar(c, err)
	}
	return c.JSON(http.StatusOK, map[string]string{"avatar_url": ""})
}

// ServeChannelAvatar handles GET /api/channels/avatars/:token and
// .../:token/:size, size 64, 192 (the default) or 1024. The token is random per
// upload, so the URL reveals nothing, and a new picture gets a new URL, which
// is what lets a browser keep this one for good.
func (h *Handler) ServeChannelAvatar(c echo.Context) error {
	size := 192
	if s := c.Param("size"); s != "" {
		n, err := strconv.Atoi(s)
		if err != nil {
			return avatarNotFound(c)
		}
		size = n
	}
	b, err := h.svc.ChannelAvatar(c.Request().Context(), c.Param("token"), size)
	if errors.Is(err, domain.ErrNotFound) {
		return avatarNotFound(c)
	}
	if err != nil {
		return failAvatar(c, err)
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

// avatarLimiter guards the public picture route per address. If the counter
// itself fails the request is allowed: a picture is not worth an outage.
func (h *Handler) avatarLimiter() echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			err := h.svc.TakePublicAvatarRequest(c.Request().Context(), clientHost(c))
			if err == nil {
				return next(c)
			}
			if _, limited := domain.RetryAfter(err); limited {
				return failAvatar(c, err)
			}
			slog.Error("avatar rate limit unavailable; request allowed", "error", err)
			return next(c)
		}
	}
}

// avatarNotFound is a 404 that a browser may remember for a minute, so a
// page full of stale addresses does not ask again on every render.
func avatarNotFound(c echo.Context) error {
	c.Response().Header().Set("Cache-Control", "public, max-age=60")
	return httputil.CodedError(http.StatusNotFound, "not_found", "no such picture")
}

func avatarTooLarge() error {
	return httputil.CodedError(http.StatusRequestEntityTooLarge, "avatar_too_large", "the picture is larger than 5 MB")
}

// avatarFormError says what was wrong with the form an upload came in.
// The size check comes first: a body cut off at the limit also fails to parse.
func avatarFormError(err error) error {
	var tooBig *http.MaxBytesError
	switch {
	case errors.As(err, &tooBig):
		return avatarTooLarge()
	case errors.Is(err, http.ErrMissingFile):
		return httputil.CodedError(http.StatusBadRequest, "avatar_missing", "send the picture as the form field \"file\"")
	default:
		return httputil.CodedError(http.StatusBadRequest, "invalid_input", "send the picture as a multipart form")
	}
}

// failAvatar maps what the picture routes can fail with; everything else is a
// plain domain error. Waits are told in Retry-After.
func failAvatar(c echo.Context, err error) error {
	h := c.Response().Header()
	switch {
	case errors.Is(err, domain.ErrAvatarNotImage):
		return httputil.CodedError(http.StatusBadRequest, "avatar_not_image", err.Error())
	case errors.Is(err, domain.ErrAvatarTooManyPixels):
		return httputil.CodedError(http.StatusBadRequest, "avatar_too_many_pixels", err.Error())
	case errors.Is(err, domain.ErrAvatarBusy):
		h.Set(echo.HeaderRetryAfter, avatarBusyRetry)
		return httputil.CodedError(http.StatusServiceUnavailable, "avatar_busy", err.Error())
	case errors.Is(err, domain.ErrRateLimited):
		if d, ok := domain.RetryAfter(err); ok {
			h.Set(echo.HeaderRetryAfter, strconv.Itoa(retrySeconds(d)))
		}
		return httputil.CodedError(http.StatusTooManyRequests, "rate_limited", err.Error())
	}
	return httputil.MapDomainError(err)
}

// retrySeconds rounds a wait up to whole seconds, never below one.
func retrySeconds(d time.Duration) int {
	secs := int((d + time.Second - 1) / time.Second)
	if secs < 1 {
		secs = 1
	}
	return secs
}

// clientHost is who is asking. Behind the reverse proxy it is the client the
// proxy saw, read from X-Forwarded-For only for the networks the server was
// configured to trust (httputil.NewIPExtractor); with none configured it is the
// connection's own address, so a header any client writes cannot choose which
// counter a request lands in.
func clientHost(c echo.Context) string {
	if x := c.Echo().IPExtractor; x != nil {
		if ip := x(c.Request()); ip != "" {
			return ip
		}
	}
	return remoteHost(c.Request())
}

// remoteHost is the address of the connection itself, without its port.
func remoteHost(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}
