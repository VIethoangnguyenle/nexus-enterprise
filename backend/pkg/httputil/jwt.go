package httputil

import (
	"strings"

	"github.com/golang-jwt/jwt/v5"
	"github.com/labstack/echo/v4"
)

// claimsKey is the context key used to store parsed JWT claims.
const claimsKey = "jwt_claims"

// JWTMiddleware returns Echo middleware that validates a Bearer token from the
// Authorization header and stores the parsed Claims in the echo.Context.
// Public routes should be registered outside this middleware group.
func JWTMiddleware(secret string) echo.MiddlewareFunc {
	secretBytes := []byte(secret)
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			auth := c.Request().Header.Get("Authorization")
			if !strings.HasPrefix(auth, "Bearer ") {
				return echo.NewHTTPError(401, "missing or invalid authorization header")
			}

			tokenStr := strings.TrimPrefix(auth, "Bearer ")
			token, err := jwt.ParseWithClaims(tokenStr, &Claims{}, func(t *jwt.Token) (any, error) {
				return secretBytes, nil
			})
			if err != nil {
				return echo.NewHTTPError(401, "invalid or expired token")
			}

			claims, ok := token.Claims.(*Claims)
			if !ok || !token.Valid {
				return echo.NewHTTPError(401, "invalid token claims")
			}

			c.Set(claimsKey, claims)
			return next(c)
		}
	}
}
