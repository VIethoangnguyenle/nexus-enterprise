// Package rest provides Echo REST handlers for the auth service.
// Handles client-facing HTTP/JSON for signup, signin, tenant switching, and user queries.
// Delegates all logic to the domain layer.
package rest

import (
	"errors"
	"net/http"

	"github.com/labstack/echo/v4"

	"ngac-platform/pkg/httputil"
	"ngac-platform/services/auth/internal/domain"
)

// Handler serves auth REST endpoints.
type Handler struct {
	svc *domain.Service
}

// NewHandler creates an auth REST handler.
func NewHandler(svc *domain.Service) *Handler {
	return &Handler{svc: svc}
}

// RegisterRoutes mounts auth endpoints on the Echo instance.
func (h *Handler) RegisterRoutes(e *echo.Echo, jwtSecret string) {
	// Public — no auth required
	e.POST("/api/auth/register", h.Register)  // legacy
	e.POST("/api/auth/login", h.Login)         // legacy
	e.POST("/api/auth/signup", h.Signup)        // multi-tenant
	e.POST("/api/auth/signin", h.Signin)        // multi-tenant
	e.POST("/api/auth/otp/request", h.RequestOTP)
	e.POST("/api/auth/otp/verify", h.VerifyOTP)

	// Protected — JWT required
	api := e.Group("/api", httputil.JWTMiddleware(jwtSecret))
	api.POST("/auth/switch-tenant", h.SwitchTenant)
	api.GET("/me", h.GetMe)
	api.PATCH("/me/profile", h.UpdateProfile)
	api.GET("/users", h.ListUsers)
	api.GET("/users/lookup", h.LookupUser)
	api.GET("/workspaces/:id/contacts", h.ListContacts)
}

// Signup handles POST /api/auth/signup (multi-tenant flow).
func (h *Handler) Signup(c echo.Context) error {
	var body struct {
		Email       string `json:"email"`
		Password    string `json:"password"`
		DisplayName string `json:"display_name"`
		TenantName  string `json:"tenant_name"`
	}
	if err := c.Bind(&body); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid request body")
	}

	resp, err := h.svc.Signup(c.Request().Context(), body.Email, body.Password, body.DisplayName, body.TenantName)
	if err != nil {
		return mapError(err)
	}

	return c.JSON(http.StatusCreated, map[string]any{
		"token": resp.Token,
		"user": map[string]string{
			"id": resp.UserID, "username": resp.Username,
			"ngac_node_id": resp.NGACNodeID, "email": resp.Email, "union_id": resp.UnionID,
		},
		"tenant": map[string]string{
			"id": resp.TenantID, "name": resp.TenantName,
			"role": resp.TenantRole, "open_id": resp.OpenID,
		},
	})
}

// Signin handles POST /api/auth/signin (multi-tenant flow).
func (h *Handler) Signin(c echo.Context) error {
	var body struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	if err := c.Bind(&body); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid request body")
	}

	resp, err := h.svc.Signin(c.Request().Context(), body.Email, body.Password)
	if err != nil {
		return mapError(err)
	}

	tenants := make([]map[string]string, len(resp.Tenants))
	for i, t := range resp.Tenants {
		tenants[i] = map[string]string{
			"id": t.ID, "name": t.Name, "role": t.Role, "open_id": t.OpenID,
		}
	}

	return c.JSON(http.StatusOK, map[string]any{
		"token": resp.Token,
		"user": map[string]string{
			"id": resp.UserID, "username": resp.Username,
			"ngac_node_id": resp.NGACNodeID, "email": resp.Email,
			"union_id": resp.UnionID, "display_name": resp.DisplayName,
		},
		"tenants":           tenants,
		"default_tenant_id": resp.DefaultTenantID,
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
		return echo.NewHTTPError(http.StatusBadRequest, "invalid request body")
	}

	token, tenant, err := h.svc.SwitchTenant(c.Request().Context(), claims.UserID, claims.NGACNodeID, claims.Username, body.TenantID)
	if err != nil {
		return mapError(err)
	}

	return c.JSON(http.StatusOK, map[string]any{
		"token": token,
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

	user, tenant, err := h.svc.GetMe(c.Request().Context(), claims.UserID, claims.TenantID)
	if err != nil {
		return mapError(err)
	}

	result := map[string]any{
		"user": map[string]string{
			"id": user.ID, "username": user.Username,
			"ngac_node_id": user.NGACNodeID, "email": user.Email,
			"union_id": user.UnionID, "display_name": user.DisplayName,
		},
	}
	if tenant != nil {
		result["current_tenant"] = map[string]string{
			"id": tenant.ID, "name": tenant.Name,
			"role": tenant.Role, "open_id": tenant.OpenID,
		}
	}
	return c.JSON(http.StatusOK, result)
}

// Register handles POST /api/auth/register (legacy).
func (h *Handler) Register(c echo.Context) error {
	var body struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if err := c.Bind(&body); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid request body")
	}

	resp, err := h.svc.Register(c.Request().Context(), body.Username, body.Password)
	if err != nil {
		return mapError(err)
	}

	return c.JSON(http.StatusCreated, map[string]any{
		"token": resp.Token,
		"user": map[string]string{
			"id": resp.UserID, "username": resp.Username, "ngac_node_id": resp.NGACNodeID,
		},
	})
}

// Login handles POST /api/auth/login (legacy).
func (h *Handler) Login(c echo.Context) error {
	var body struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if err := c.Bind(&body); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid request body")
	}

	resp, err := h.svc.Login(c.Request().Context(), body.Username, body.Password)
	if err != nil {
		return mapError(err)
	}

	return c.JSON(http.StatusOK, map[string]any{
		"token": resp.Token,
		"user": map[string]string{
			"id": resp.UserID, "username": resp.Username, "ngac_node_id": resp.NGACNodeID,
		},
	})
}

// ListUsers handles GET /api/users.
func (h *Handler) ListUsers(c echo.Context) error {
	users, err := h.svc.ListUsers(c.Request().Context())
	if err != nil {
		return mapError(err)
	}

	type userJSON struct {
		ID         string `json:"id"`
		Username   string `json:"username"`
		NGACNodeID string `json:"ngac_node_id"`
	}
	result := make([]userJSON, len(users))
	for i, u := range users {
		result[i] = userJSON{ID: u.ID, Username: u.Username, NGACNodeID: u.NGACNodeID}
	}
	return c.JSON(http.StatusOK, map[string]any{"users": result})
}

// LookupUser handles GET /api/users/lookup?username=X.
func (h *Handler) LookupUser(c echo.Context) error {
	username := c.QueryParam("username")
	if username == "" {
		return echo.NewHTTPError(http.StatusBadRequest, "username query param required")
	}

	user, err := h.svc.GetUserByUsername(c.Request().Context(), username)
	if err != nil {
		return mapError(err)
	}

	return c.JSON(http.StatusOK, map[string]string{
		"id": user.ID, "username": user.Username, "ngac_node_id": user.NGACNodeID,
	})
}

// RequestOTP handles POST /api/auth/otp/request.
func (h *Handler) RequestOTP(c echo.Context) error {
	var body struct {
		Identifier string `json:"identifier"`
		Type       string `json:"type"`
	}
	if err := c.Bind(&body); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid request body")
	}

	sessionID, err := h.svc.RequestOTP(c.Request().Context(), body.Identifier, body.Type)
	if err != nil {
		return mapError(err)
	}

	return c.JSON(http.StatusOK, map[string]any{
		"session_id":  sessionID,
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
		return echo.NewHTTPError(http.StatusBadRequest, "invalid request body")
	}

	result, err := h.svc.VerifyOTP(c.Request().Context(), body.SessionID, body.Code)
	if err != nil {
		return mapError(err)
	}

	return c.JSON(http.StatusOK, map[string]any{
		"token": result.Token,
		"user": map[string]string{
			"id": result.UserID, "username": result.Username,
			"ngac_node_id": result.NGACNodeID, "email": result.Email,
			"phone": result.Phone, "union_id": result.UnionID,
		},
		"is_new_user": result.IsNewUser,
	})
}

// UpdateProfile handles PATCH /api/me/profile.
func (h *Handler) UpdateProfile(c echo.Context) error {
	claims, err := httputil.RequireClaims(c)
	if err != nil {
		return err
	}

	var body struct {
		DisplayName string `json:"display_name"`
		Title       string `json:"title"`
		Department  string `json:"department"`
		Location    string `json:"location"`
		AvatarURL   string `json:"avatar_url"`
	}
	if err := c.Bind(&body); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid request body")
	}

	if err := h.svc.UpdateProfile(c.Request().Context(), claims.UserID, domain.ProfileUpdateInput{
		DisplayName: body.DisplayName,
		Title:       body.Title,
		Department:  body.Department,
		Location:    body.Location,
		AvatarURL:   body.AvatarURL,
	}); err != nil {
		return mapError(err)
	}

	return c.JSON(http.StatusOK, map[string]string{"status": "updated"})
}

// ListContacts handles GET /api/workspaces/:id/contacts.
func (h *Handler) ListContacts(c echo.Context) error {
	wsID := c.Param("id")
	if wsID == "" {
		return echo.NewHTTPError(http.StatusBadRequest, "workspace id required")
	}

	department := c.QueryParam("department")
	location := c.QueryParam("location")

	contacts, err := h.svc.ListContacts(c.Request().Context(), wsID, department, location)
	if err != nil {
		return mapError(err)
	}

	type contactJSON struct {
		UserID      string `json:"user_id"`
		NGACNodeID  string `json:"ngac_node_id"`
		Username    string `json:"username"`
		DisplayName string `json:"display_name"`
		Email       string `json:"email"`
		Title       string `json:"title"`
		Department  string `json:"department"`
		Location    string `json:"location"`
		AvatarURL   string `json:"avatar_url"`
	}

	result := make([]contactJSON, len(contacts))
	for i, c := range contacts {
		result[i] = contactJSON{
			UserID: c.UserID, NGACNodeID: c.NGACNodeID, Username: c.Username,
			DisplayName: c.DisplayName, Email: c.Email,
			Title: c.Title, Department: c.Department,
			Location: c.Location, AvatarURL: c.AvatarURL,
		}
	}

	return c.JSON(http.StatusOK, map[string]any{
		"contacts": result,
		"total":    len(result),
	})
}

func mapError(err error) *echo.HTTPError {
	switch {
	case errors.Is(err, domain.ErrInvalidCredentials):
		return echo.NewHTTPError(http.StatusUnauthorized, err.Error())
	case errors.Is(err, domain.ErrUserExists):
		return echo.NewHTTPError(http.StatusConflict, err.Error())
	case errors.Is(err, domain.ErrNotFound):
		return echo.NewHTTPError(http.StatusNotFound, err.Error())
	case errors.Is(err, domain.ErrInvalidInput):
		return echo.NewHTTPError(http.StatusBadRequest, err.Error())
	case errors.Is(err, domain.ErrAccessDenied):
		return echo.NewHTTPError(http.StatusForbidden, err.Error())
	case errors.Is(err, domain.ErrTenantNotFound):
		return echo.NewHTTPError(http.StatusNotFound, err.Error())
	case errors.Is(err, domain.ErrOTPExpired):
		return echo.NewHTTPError(http.StatusUnauthorized, err.Error())
	case errors.Is(err, domain.ErrOTPInvalid):
		return echo.NewHTTPError(http.StatusUnauthorized, err.Error())
	case errors.Is(err, domain.ErrTooManyAttempts):
		return echo.NewHTTPError(http.StatusTooManyRequests, err.Error())
	default:
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}
}
