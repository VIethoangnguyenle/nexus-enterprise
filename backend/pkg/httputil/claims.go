package httputil

import (
	"github.com/golang-jwt/jwt/v5"
	"github.com/labstack/echo/v4"
)

// Claims holds the JWT payload fields used across all NGAC services.
type Claims struct {
	UserID     string `json:"user_id"`
	Username   string `json:"username"`
	NGACNodeID string `json:"ngac_node_id"`
	jwt.RegisteredClaims
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
