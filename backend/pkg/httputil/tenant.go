package httputil

import (
	"github.com/labstack/echo/v4"
)

// TenantMiddleware returns Echo middleware that enforces tenant context in JWT claims.
// Requests with an empty tenant_id are rejected with 403 Forbidden.
// Place this AFTER JWTMiddleware on routes that require tenant scoping.
func TenantMiddleware() echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			claims := GetClaims(c)
			if claims == nil {
				return echo.NewHTTPError(401, "authentication required")
			}
			if claims.TenantID == "" {
				return echo.NewHTTPError(403, "tenant context required: use /api/auth/switch-tenant to select a tenant")
			}
			return next(c)
		}
	}
}
