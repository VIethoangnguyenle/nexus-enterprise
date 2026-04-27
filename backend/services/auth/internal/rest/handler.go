// Package rest provides Echo REST handlers for the auth service.
// Handles client-facing HTTP/JSON for register, login, and user queries.
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
// Public routes (register, login) are outside JWT middleware.
// Protected routes (users) require a valid token.
func (h *Handler) RegisterRoutes(e *echo.Echo, jwtSecret string) {
	// Public — no auth required
	e.POST("/api/auth/register", h.Register)
	e.POST("/api/auth/login", h.Login)

	// Protected — JWT required
	api := e.Group("/api", httputil.JWTMiddleware(jwtSecret))
	api.GET("/users", h.ListUsers)
}

// Register handles POST /api/auth/register.
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
			"id":           resp.UserID,
			"username":     resp.Username,
			"ngac_node_id": resp.NGACNodeID,
		},
	})
}

// Login handles POST /api/auth/login.
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
			"id":           resp.UserID,
			"username":     resp.Username,
			"ngac_node_id": resp.NGACNodeID,
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
	default:
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}
}
