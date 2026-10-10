// Package rest provides Echo REST handlers for the auth service.
// Handles client-facing HTTP/JSON for signup, signin, tenant switching, and user queries.
// Delegates all logic to the domain layer.
package rest

import (
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/labstack/echo/v4"

	"ngac-platform/pkg/httputil"
	"ngac-platform/services/auth/internal/domain"
)

// Handler serves auth REST endpoints.
type Handler struct {
	svc    *domain.Service
	google *googleHandler // nil until EnableGoogle; routes then report it disabled

	limit    PublicLimit // allowance of one address for one public route
	limitSet bool
}

// NewHandler creates an auth REST handler.
func NewHandler(svc *domain.Service) *Handler {
	return &Handler{svc: svc}
}

// RegisterRoutes mounts auth endpoints on the Echo instance.
func (h *Handler) RegisterRoutes(e *echo.Echo, jwtSecret string) {
	// Every error, from any handler or middleware, is {message, code}.
	e.HTTPErrorHandler = envelopeErrors

	// Public — no auth required, so each is behind a per-address limit.
	e.POST("/api/auth/otp/request", h.RequestOTP, h.publicLimiter("otp-request"))
	e.POST("/api/auth/otp/verify", h.VerifyOTP, h.publicLimiter("otp-verify"))
	// Refresh is public: it authenticates with the httpOnly cookie, and by the
	// time a client needs it the access token has usually already expired.
	e.POST("/api/auth/refresh", h.Refresh, h.publicLimiter("refresh"))
	// Logout accepts an expired access token for the same reason, so it is
	// mounted outside the JWT group and reads the claims opportunistically.
	e.POST("/api/auth/logout", h.Logout, h.publicLimiter("logout"))
	// Sign in with Google. All three sit under /api/auth, so the existing
	// /api/auth entries in the Vite dev proxy and the Traefik router cover them.
	google := h.google
	if google == nil {
		google = newGoogleHandler(GoogleOptions{}, nil)
	}
	e.GET("/api/auth/providers", h.Providers)
	e.GET("/api/auth/google/start", google.Start, h.publicLimiter("google-start"))
	e.GET("/api/auth/google/callback", google.Callback)

	// Protected — JWT required
	api := e.Group("/api", httputil.JWTMiddleware(jwtSecret))
	api.POST("/auth/switch-tenant", h.SwitchTenant)
	api.GET("/me", h.GetMe)
	api.PATCH("/me/profile", h.UpdateProfile)
	api.POST("/me/email/verify", h.VerifyEmail)
	api.POST("/me/email/verify/google", func(c echo.Context) error { return h.googleHandler().StartVerification(c) })
	api.GET("/me/workspaces", h.ListMyWorkspaces)
	api.POST("/me/workspaces", h.CreateMyWorkspace)
	api.GET("/workspaces/:id/contacts", h.ListContacts)
}

// Providers handles GET /api/auth/providers — which sign-in methods the login
// page should offer. otp_fixed_code is true while the documented test-only
// fixed OTP code is in force; it is for operators and the API, and no screen
// may print the code. otp_proves_email is true when a code delivered here
// reaches only the owner of the address, so entering it verifies the address:
// the screen offers "send me a code" as a way to verify only when it is true.
func (h *Handler) Providers(c echo.Context) error {
	return c.JSON(http.StatusOK, map[string]bool{
		"google":           h.google.enabled(),
		"otp":              h.svc.OTPEnabled(),
		"otp_fixed_code":   h.svc.OTPFixedCodeActive(),
		"otp_proves_email": h.svc.OTPProvesOwnership(),
	})
}

// SwitchTenant handles POST /api/auth/switch-tenant.
func (h *Handler) SwitchTenant(c echo.Context) error {
	claims, err := httputil.RequireClaims(c)
	if err != nil {
		return err
	}

	var body struct {
		TenantID string `json:"tenant_id"`
	}
	if err := c.Bind(&body); err != nil {
		return badBody()
	}

	token, sessionID, tenant, err := h.svc.SwitchTenant(c.Request().Context(), claims.UserID, claims.NGACNodeID, claims.Username, body.TenantID)
	if err != nil {
		return fail(c, err)
	}

	// Retire the family scoped to the tenant being left, then bind a new one to
	// the new session — otherwise a later refresh would hand back a token for
	// the previous tenant.
	if err := h.svc.EndSession(c.Request().Context(), claims.SessionID); err != nil {
		return internalFailure(fmt.Errorf("end previous session: %w", err))
	}
	if err := h.issueSession(c, domain.RefreshIdentity{
		UserID: claims.UserID, Username: claims.Username, NGACNodeID: claims.NGACNodeID,
		TenantID: body.TenantID, SessionID: sessionID,
	}); err != nil {
		return err
	}

	return c.JSON(http.StatusOK, map[string]any{
		"access_token": token,
		"tenant": map[string]string{
			"id": tenant.ID, "name": tenant.Name,
			"role": tenant.Role, "open_id": tenant.OpenID,
		},
	})
}

// GetMe handles GET /api/me.
func (h *Handler) GetMe(c echo.Context) error {
	claims, err := httputil.RequireClaims(c)
	if err != nil {
		return err
	}

	// current_tenant is the token's workspace unless ?workspace= names another;
	// it is present only for a workspace the caller is an active member of, so
	// asking about someone else's workspace learns nothing.
	tenantID := claims.TenantID
	if w := c.QueryParam("workspace"); w != "" {
		tenantID = w
	}
	user, tenant, err := h.svc.GetMe(c.Request().Context(), claims.UserID, tenantID)
	if err != nil {
		return fail(c, err)
	}

	result := map[string]any{
		"user": map[string]any{
			"id": user.ID, "username": user.Username,
			"ngac_node_id": user.NGACNodeID, "email": user.Email,
			"union_id": user.UnionID, "display_name": user.DisplayName,
			"title": user.Title, "location": user.Location, "avatar_url": user.AvatarURL,
			"email_verified": user.EmailVerified, "needs_profile": user.NeedsProfile,
		},
	}
	if tenant != nil {
		result["current_tenant"] = map[string]string{
			"id": tenant.ID, "name": tenant.Name,
			"role": tenant.Role, "open_id": tenant.OpenID,
			// Assigned by an administrator in this workspace; read-only here.
			"department": tenant.Department,
		}
	}
	return c.JSON(http.StatusOK, result)
}

// RequestOTP handles POST /api/auth/otp/request.
func (h *Handler) RequestOTP(c echo.Context) error {
	var body struct {
		Identifier string `json:"identifier"`
		Type       string `json:"type"`
	}
	if err := c.Bind(&body); err != nil {
		return badBody()
	}

	sessionID, err := h.svc.RequestOTP(c.Request().Context(), body.Identifier, body.Type)
	if err != nil {
		return fail(c, err)
	}

	return c.JSON(http.StatusOK, map[string]any{
		"session_id": sessionID,
		"expires_in": 300,
	})
}

// VerifyOTP handles POST /api/auth/otp/verify.
func (h *Handler) VerifyOTP(c echo.Context) error {
	var body struct {
		SessionID string `json:"session_id"`
		Code      string `json:"code"`
	}
	if err := c.Bind(&body); err != nil {
		return badBody()
	}

	result, err := h.svc.VerifyOTP(c.Request().Context(), body.SessionID, body.Code)
	if err != nil {
		return fail(c, err)
	}

	if err := h.issueSession(c, domain.RefreshIdentity{
		UserID: result.UserID, Username: result.Username, NGACNodeID: result.NGACNodeID,
		TenantID: result.TenantID, SessionID: result.SessionID,
	}); err != nil {
		return err
	}

	return c.JSON(http.StatusOK, map[string]any{
		"access_token": result.Token,
		"user": map[string]any{
			"id": result.UserID, "username": result.Username,
			"ngac_node_id": result.NGACNodeID, "email": result.Email,
			"phone": result.Phone, "union_id": result.UnionID,
			"email_verified": result.EmailVerified,
		},
		"is_new_user":   result.IsNewUser,
		"needs_profile": result.NeedsProfile,
	})
}

// googleHandler is the Google side, or a disabled one when Google is not set up.
func (h *Handler) googleHandler() *googleHandler {
	if h.google != nil {
		return h.google
	}
	return newGoogleHandler(GoogleOptions{}, h.svc)
}

// VerifyEmail handles POST /api/me/email/verify, proving the address on the
// signed-in account without signing anyone in. Without a "code" it sends a code
// to the address (202); with one it checks it (200). Neither returns a token or
// sets a cookie, and the session that asks keeps going while every other session
// on the account ends when the address is first proved.
func (h *Handler) VerifyEmail(c echo.Context) error {
	claims, err := httputil.RequireClaims(c)
	if err != nil {
		return err
	}
	var body struct {
		Code *string `json:"code"`
	}
	if err := c.Bind(&body); err != nil {
		return badBody()
	}
	if body.Code == nil {
		ttl, err := h.svc.RequestEmailVerification(c.Request().Context(), claims.UserID)
		if err != nil {
			return fail(c, err)
		}
		return c.JSON(http.StatusAccepted, map[string]any{"expires_in": int(ttl.Seconds())})
	}
	if err := h.svc.ConfirmEmailVerification(c.Request().Context(), claims.UserID, claims.SessionID, *body.Code); err != nil {
		return fail(c, err)
	}
	return c.JSON(http.StatusOK, map[string]any{"email_verified": true})
}

// badBody is the answer to a request body that is not the JSON the route takes.
func badBody() *echo.HTTPError {
	return apiError(http.StatusBadRequest, "invalid_input", "invalid request body")
}

// apiError is an error response whose body is {"message", "code", ...extra}.
// The code is the stable machine-readable part the screens branch on; the
// message is an English fallback for API users and is never shown as is.
func apiError(status int, code, message string, extra ...any) *echo.HTTPError {
	body := map[string]any{"message": message, "code": code}
	for i := 0; i+1 < len(extra); i += 2 {
		body[extra[i].(string)] = extra[i+1]
	}
	return echo.NewHTTPError(status, body)
}

// internalFailure is a 500 in the auth envelope with a generic message. cause
// stays on the error for envelopeErrors to log under the request ID.
func internalFailure(cause error) *echo.HTTPError {
	return apiError(http.StatusInternalServerError, "internal", httputil.InternalMessage).SetInternal(cause)
}

// fail turns a domain error into its HTTP answer. A rate-limited caller is told
// when to come back, in the Retry-After header as well as the body.
func fail(c echo.Context, err error) error {
	if d, ok := domain.RetryAfter(err); ok {
		c.Response().Header().Set(echo.HeaderRetryAfter, strconv.Itoa(retrySeconds(d)))
	}
	return mapError(err)
}

// retrySeconds rounds a wait up to whole seconds, never below one.
func retrySeconds(d time.Duration) int {
	secs := int((d + time.Second - 1) / time.Second)
	if secs < 1 {
		secs = 1
	}
	return secs
}

func mapError(err error) *echo.HTTPError {
	switch {
	case errors.Is(err, domain.ErrInvalidCredentials):
		return apiError(http.StatusUnauthorized, "invalid_credentials", err.Error())
	case errors.Is(err, domain.ErrUserExists):
		return apiError(http.StatusConflict, "already_exists", err.Error())
	case errors.Is(err, domain.ErrNotFound), errors.Is(err, domain.ErrTenantNotFound):
		return apiError(http.StatusNotFound, "not_found", err.Error())
	case errors.Is(err, domain.ErrInvalidInput):
		return apiError(http.StatusBadRequest, "invalid_input", err.Error())
	case errors.Is(err, domain.ErrAccessDenied):
		return apiError(http.StatusForbidden, "access_denied", err.Error())
	case errors.Is(err, domain.ErrOTPExpired):
		return apiError(http.StatusUnauthorized, "otp_expired", err.Error())
	case errors.Is(err, domain.ErrOTPInvalid):
		if left, ok := domain.OTPAttemptsLeft(err); ok {
			return apiError(http.StatusUnauthorized, "otp_invalid", err.Error(), "attempts_left", left)
		}
		return apiError(http.StatusUnauthorized, "otp_invalid", err.Error())
	case errors.Is(err, domain.ErrTooManyAttempts):
		return apiError(http.StatusTooManyRequests, "otp_too_many_attempts", err.Error())
	case errors.Is(err, domain.ErrOTPRateLimited):
		return rateLimited("otp_rate_limited", err)
	case errors.Is(err, domain.ErrRateLimited):
		return rateLimited("rate_limited", err)
	case errors.Is(err, domain.ErrVerificationUnavailable):
		return apiError(http.StatusServiceUnavailable, "verification_unavailable", err.Error())
	case errors.Is(err, domain.ErrAlreadyVerified):
		return apiError(http.StatusConflict, "already_verified", err.Error())
	case errors.Is(err, domain.ErrEmailMismatch):
		return apiError(http.StatusConflict, "email_mismatch", err.Error())
	case errors.Is(err, domain.ErrEmailNotVerified):
		return apiError(http.StatusForbidden, "email_not_verified_by_provider", err.Error())
	case errors.Is(err, domain.ErrEmailUnverified):
		return apiError(http.StatusForbidden, "email_unverified", err.Error())
	case errors.Is(err, domain.ErrOTPUnavailable):
		return apiError(http.StatusServiceUnavailable, "otp_unavailable", err.Error())
	default:
		// The text of an unexpected error names hosts, queries and downstream
		// services. It goes to the log, never to the caller.
		return internalFailure(err)
	}
}

func rateLimited(code string, err error) *echo.HTTPError {
	if d, ok := domain.RetryAfter(err); ok {
		return apiError(http.StatusTooManyRequests, code, err.Error(), "retry_after_seconds", retrySeconds(d))
	}
	return apiError(http.StatusTooManyRequests, code, err.Error())
}
