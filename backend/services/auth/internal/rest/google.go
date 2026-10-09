// Package rest — "Sign in with Google" endpoints.
package rest

import (
	"context"
	"crypto/subtle"
	"errors"
	"log/slog"
	"net/http"
	"net/url"
	"regexp"
	"strings"

	"github.com/labstack/echo/v4"

	"ngac-platform/services/auth/internal/domain"
	"ngac-platform/services/auth/internal/googleauth"
)

const (
	// googleStateCookieName holds the state of the sign-in this browser
	// started. The callback must present the same value in its query, which is
	// what stops an attacker from completing *their* Google login in a victim's
	// browser (login CSRF).
	googleStateCookieName = "google_oauth_state"
	// googleCookiePath limits the state cookie to the Google endpoints.
	googleCookiePath = "/api/auth/google"

	// googleDonePath is the SPA route that finishes sign-in by calling
	// /api/auth/refresh. Tokens are never put in this URL.
	googleDonePath = "/auth/google/done"
)

// Error codes appended to /login?error=. The SPA maps them to sentences and
// never shows the code itself.
const (
	googleErrUnavailable = "google_unavailable"
	googleErrCancelled   = "google_cancelled"
	googleErrState       = "google_state"
	googleErrFailed      = "google_failed"
	googleErrUnverified  = "google_unverified"
	googleErrConflict    = "google_conflict"
	googleErrInternal    = "google_error"
)

// GoogleProvider is the Google side of the flow (googleauth.Client).
type GoogleProvider interface {
	AuthCodeURL(state, nonce, verifier, loginHint string) string
	Exchange(ctx context.Context, code, verifier, nonce string) (*googleauth.Identity, error)
}

// googleSessionService is the part of the domain service the callback needs.
type googleSessionService interface {
	refreshAttacher
	SignInWithGoogle(ctx context.Context, id domain.ExternalIdentity) (*domain.SigninResult, error)
}

// GoogleOptions configures Google sign-in. A nil Provider disables it.
type GoogleOptions struct {
	Provider GoogleProvider
	Flows    googleauth.FlowStore
	// AppBaseURL is the SPA origin the callback redirects back to.
	AppBaseURL string
}

type googleHandler struct {
	provider GoogleProvider
	flows    googleauth.FlowStore
	appBase  string
	svc      googleSessionService
}

func newGoogleHandler(opts GoogleOptions, svc googleSessionService) *googleHandler {
	return &googleHandler{
		provider: opts.Provider,
		flows:    opts.Flows,
		appBase:  strings.TrimRight(opts.AppBaseURL, "/"),
		svc:      svc,
	}
}

// EnableGoogle turns on Google sign-in. Call before RegisterRoutes.
func (h *Handler) EnableGoogle(opts GoogleOptions) {
	h.google = newGoogleHandler(opts, h.svc)
}

func (g *googleHandler) enabled() bool {
	return g != nil && g.provider != nil && g.flows != nil
}

// loginHintPattern accepts what could plausibly be an email; anything else is
// dropped rather than forwarded to Google.
var loginHintPattern = regexp.MustCompile(`^[^\s@]{1,64}@[^\s@]{1,190}$`)

// Start handles GET /api/auth/google/start: it records a new flow and sends
// the browser to Google.
func (g *googleHandler) Start(c echo.Context) error {
	if !g.enabled() {
		return echo.NewHTTPError(http.StatusServiceUnavailable, "google sign-in is not configured")
	}

	hint := strings.TrimSpace(c.QueryParam("login_hint"))
	if !loginHintPattern.MatchString(hint) {
		hint = ""
	}

	secrets, err := googleauth.NewFlowSecrets()
	if err != nil {
		slog.Error("google sign-in: generate flow secrets", "error", err)
		return g.fail(c, googleErrInternal)
	}
	if err := g.flows.Save(c.Request().Context(), secrets.State, googleauth.Flow{
		Nonce: secrets.Nonce, Verifier: secrets.Verifier,
	}); err != nil {
		slog.Error("google sign-in: store flow", "error", err)
		return g.fail(c, googleErrInternal)
	}

	setGoogleStateCookie(c, secrets.State)
	c.Response().Header().Set("Cache-Control", "no-store")
	return c.Redirect(http.StatusFound, g.provider.AuthCodeURL(secrets.State, secrets.Nonce, secrets.Verifier, hint))
}

// Callback handles GET /api/auth/google/callback.
func (g *googleHandler) Callback(c echo.Context) error {
	c.Response().Header().Set("Cache-Control", "no-store")
	if !g.enabled() {
		return g.fail(c, googleErrUnavailable)
	}

	// The state cookie is single-use whatever happens next.
	cookie, cookieErr := c.Cookie(googleStateCookieName)
	clearGoogleStateCookie(c)

	state := c.QueryParam("state")
	if cookieErr != nil || cookie.Value == "" || state == "" ||
		subtle.ConstantTimeCompare([]byte(cookie.Value), []byte(state)) != 1 {
		slog.Warn("google sign-in rejected", "reason", googleErrState)
		return g.fail(c, googleErrState)
	}
	ctx := c.Request().Context()
	flow, err := g.flows.Take(ctx, state)
	if err != nil {
		slog.Warn("google sign-in rejected", "reason", googleErrState, "error", err)
		return g.fail(c, googleErrState)
	}

	if e := c.QueryParam("error"); e != "" {
		if e == "access_denied" {
			return g.fail(c, googleErrCancelled)
		}
		slog.Warn("google sign-in: provider returned an error", "error_code", truncate(e, 64))
		return g.fail(c, googleErrFailed)
	}
	code := c.QueryParam("code")
	if code == "" {
		return g.fail(c, googleErrFailed)
	}

	ident, err := g.provider.Exchange(ctx, code, flow.Verifier, flow.Nonce)
	if err != nil {
		reason := googleErrFailed
		if errors.Is(err, googleauth.ErrEmailNotVerified) {
			reason = googleErrUnverified
		}
		slog.Warn("google sign-in rejected", "reason", reason, "error", err)
		return g.fail(c, reason)
	}

	res, err := g.svc.SignInWithGoogle(ctx, domain.ExternalIdentity{
		Provider:      domain.ProviderGoogle,
		Subject:       ident.Subject,
		Email:         ident.Email,
		EmailVerified: ident.EmailVerified,
		HostedDomain:  ident.HostedDomain,
		DisplayName:   ident.Name,
	})
	if err != nil {
		reason := googleErrInternal
		switch {
		case errors.Is(err, domain.ErrIdentityConflict):
			reason = googleErrConflict
		case errors.Is(err, domain.ErrEmailNotVerified):
			reason = googleErrUnverified
		}
		slog.Warn("google sign-in rejected", "reason", reason, "error", err)
		return g.fail(c, reason)
	}

	// Exactly the session password/OTP sign-in establishes: a refresh cookie
	// bound to the access token's session. The access token itself is not
	// handed over here; the SPA gets one from /api/auth/refresh.
	if err := issueSessionWith(c, g.svc, domain.RefreshIdentity{
		UserID: res.UserID, Username: res.Username, NGACNodeID: res.NGACNodeID,
		TenantID: res.DefaultTenantID, SessionID: res.SessionID,
	}); err != nil {
		slog.Error("google sign-in: issue session", "user_id", res.UserID)
		return g.fail(c, googleErrInternal)
	}

	slog.Info("google sign-in succeeded", "user_id", res.UserID, "tenant_id", res.DefaultTenantID,
		"workspace_account", ident.HostedDomain != "")
	return c.Redirect(http.StatusFound, g.appBase+googleDonePath)
}

// fail sends the browser back to the login page with a short error code.
func (g *googleHandler) fail(c echo.Context, code string) error {
	return c.Redirect(http.StatusFound, g.appBase+"/login?error="+url.QueryEscape(code))
}

func setGoogleStateCookie(c echo.Context, state string) {
	c.SetCookie(&http.Cookie{
		Name:     googleStateCookieName,
		Value:    state,
		Path:     googleCookiePath,
		MaxAge:   int(googleauth.FlowTTL.Seconds()),
		HttpOnly: true,
		Secure:   secureCookies(),
		// Lax, not Strict: the callback arrives as a top-level navigation
		// from accounts.google.com, and Strict would withhold the cookie.
		SameSite: http.SameSiteLaxMode,
	})
}

func clearGoogleStateCookie(c echo.Context) {
	c.SetCookie(&http.Cookie{
		Name:     googleStateCookieName,
		Value:    "",
		Path:     googleCookiePath,
		MaxAge:   -1,
		HttpOnly: true,
		Secure:   secureCookies(),
		SameSite: http.SameSiteLaxMode,
	})
}

func truncate(s string, n int) string {
	if len(s) > n {
		return s[:n]
	}
	return s
}
