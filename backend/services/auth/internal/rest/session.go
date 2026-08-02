// Package rest — refresh-token cookie handling and session endpoints.
package rest

import (
	"net/http"
	"os"
	"time"

	"github.com/labstack/echo/v4"

	"ngac-platform/pkg/httputil"
	"ngac-platform/services/auth/internal/domain"
)

const (
	// refreshCookieName is the only place the refresh token is ever written or
	// read. It is never returned in a response body — a body is readable by
	// JavaScript, which is the exact thing this cookie exists to prevent.
	refreshCookieName = "refresh_token"

	// refreshCookiePath scopes the cookie to the auth endpoints. The browser
	// then withholds it from every other request in the app, so the token is
	// only ever in flight where it is actually needed.
	refreshCookiePath = "/api/auth"
)

// secureCookies reports whether cookies should carry the Secure attribute.
//
// Secure cookies are dropped by the browser over plain HTTP, which would break
// local development on http://localhost. It is enabled unless the service is
// explicitly running in a dev environment, so the default for any unrecognised
// deployment is the safe one.
func secureCookies() bool {
	switch os.Getenv("APP_ENV") {
	case "dev", "development", "local", "test":
		return false
	default:
		return true
	}
}

// setRefreshCookie writes the refresh token as an httpOnly cookie.
func setRefreshCookie(c echo.Context, token string) {
	c.SetCookie(&http.Cookie{
		Name:     refreshCookieName,
		Value:    token,
		Path:     refreshCookiePath,
		MaxAge:   int(domain.RefreshTokenTTL / time.Second),
		HttpOnly: true,
		Secure:   secureCookies(),
		// Strict rather than Lax: nothing in this app expects a cross-site
		// navigation to arrive already authenticated, and Strict is what makes
		// the refresh endpoint unreachable from another origin — which is the
		// CSRF defence for an endpoint that takes no body.
		SameSite: http.SameSiteStrictMode,
	})
}

// clearRefreshCookie expires the refresh cookie on the client.
func clearRefreshCookie(c echo.Context) {
	c.SetCookie(&http.Cookie{
		Name:     refreshCookieName,
		Value:    "",
		Path:     refreshCookiePath,
		MaxAge:   -1,
		HttpOnly: true,
		Secure:   secureCookies(),
		SameSite: http.SameSiteStrictMode,
	})
}

// issueSession binds a refresh token to the session that the access token was
// just minted for, and hands it to the client as a cookie.
//
// A failure here is fatal to the request rather than merely logged: returning
// an access token without a refresh token would look like a successful sign-in
// and then log the user out fifteen minutes later with no explanation.
func (h *Handler) issueSession(c echo.Context, id domain.RefreshIdentity) error {
	refreshToken, err := h.svc.AttachRefreshToken(c.Request().Context(), id)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "could not establish session")
	}
	setRefreshCookie(c, refreshToken)
	return nil
}

// Refresh handles POST /api/auth/refresh.
//
// It reads the refresh token from the cookie, rotates it, and returns a new
// access token. The client never sees either refresh token.
func (h *Handler) Refresh(c echo.Context) error {
	cookie, err := c.Cookie(refreshCookieName)
	if err != nil || cookie.Value == "" {
		return echo.NewHTTPError(http.StatusUnauthorized, "no refresh token")
	}

	access, next, err := h.svc.RefreshSession(c.Request().Context(), cookie.Value)
	if err != nil {
		// The token is spent, unknown, or its session was revoked — possibly
		// because this very call tripped reuse detection. Either way the client
		// must stop presenting it.
		clearRefreshCookie(c)
		return echo.NewHTTPError(http.StatusUnauthorized, "refresh rejected")
	}

	setRefreshCookie(c, next)
	return c.JSON(http.StatusOK, map[string]string{"access_token": access})
}

// Logout handles POST /api/auth/logout.
//
// It revokes the refresh family so no further access token can be minted. The
// access token the caller is holding is self-contained and stays valid until it
// expires — at most AccessTokenTTL.
func (h *Handler) Logout(c echo.Context) error {
	// Prefer the session named by the access token; fall back to the cookie so
	// that logout still works once the access token has expired.
	if claims := httputil.GetClaims(c); claims != nil && claims.SessionID != "" {
		if err := h.svc.EndSession(c.Request().Context(), claims.SessionID); err != nil {
			return echo.NewHTTPError(http.StatusInternalServerError, "logout failed")
		}
	} else if cookie, err := c.Cookie(refreshCookieName); err == nil && cookie.Value != "" {
		if err := h.svc.EndSessionByRefreshToken(c.Request().Context(), cookie.Value); err != nil {
			return echo.NewHTTPError(http.StatusInternalServerError, "logout failed")
		}
	}

	clearRefreshCookie(c)
	return c.JSON(http.StatusOK, map[string]string{"status": "logged out"})
}
