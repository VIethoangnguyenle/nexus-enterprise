package httputil

import (
	"net/http"

	"github.com/golang-jwt/jwt/v5"
	"github.com/labstack/echo/v4"

	"ngac-platform/pkg/grpcauth"
)

// Claims holds the JWT payload fields used across all NGAC services.
type Claims struct {
	UserID     string `json:"user_id"`
	Username   string `json:"username"`
	NGACNodeID string `json:"ngac_node_id"`
	TenantID   string `json:"tenant_id,omitempty"`
	SessionID  string `json:"session_id,omitempty"`
	jwt.RegisteredClaims
}

// SetClaims stores parsed JWT claims on the echo.Context. JWTMiddleware calls
// it after validating a token; handler tests call it to stand in for the
// middleware. It is the only writer of the claims key, so the storage format
// stays in one place.
//
// It also puts the verified caller on the request context, which is what the
// gRPC client interceptor forwards to other services (see pkg/grpcauth).
func SetClaims(c echo.Context, claims *Claims) {
	c.Set(claimsKey, claims)
	if claims == nil {
		return
	}
	ctx := grpcauth.WithCaller(c.Request().Context(), grpcauth.Caller{
		UserID:     claims.UserID,
		NGACNodeID: claims.NGACNodeID,
		TenantID:   claims.TenantID,
	})
	c.SetRequest(c.Request().WithContext(ctx))
}

// GetClaims extracts the parsed JWT claims from the echo.Context.
// Returns nil if JWTMiddleware has not run (e.g., public route).
func GetClaims(c echo.Context) *Claims {
	v := c.Get(claimsKey)
	if v == nil {
		return nil
	}
	claims, ok := v.(*Claims)
	if !ok {
		return nil
	}
	return claims
}

// RequireClaims extracts claims and returns an error if not authenticated.
// Use this instead of GetClaims in protected handlers to prevent nil panics.
func RequireClaims(c echo.Context) (*Claims, error) {
	claims := GetClaims(c)
	if claims == nil {
		return nil, echo.NewHTTPError(http.StatusUnauthorized, "authentication required")
	}
	return claims, nil
}
