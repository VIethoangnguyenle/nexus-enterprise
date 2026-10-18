package rest

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"ngac-platform/pkg/httputil"
)

// A workspace's kind is fixed when it is created: there is no route that turns a
// personal workspace into an organization (or back), even for a signed-in owner.
func TestRoutes_NoWorkspaceKindConversion(t *testing.T) {
	const secret = "kind-fixed-secret"
	e := echo.New()
	NewHandler(&fakeWorkspaceService{}).RegisterRoutes(e, secret)
	api := e.Group("/api", httputil.JWTMiddleware(secret))
	dept := &fakeDeptService{}
	NewAdminHandler(dept).RegisterAdminRoutes(api)

	for _, r := range e.Routes() {
		assert.NotContains(t, r.Path, "convert", "%s %s", r.Method, r.Path)
	}

	signed, err := jwt.NewWithClaims(jwt.SigningMethodHS256, &httputil.Claims{
		UserID: "u1", NGACNodeID: callerNode,
		RegisteredClaims: jwt.RegisteredClaims{ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Minute))},
	}).SignedString([]byte(secret))
	require.NoError(t, err)

	for _, path := range []string{
		"/api/workspaces/ws-1/convert-to-organization",
		"/api/workspaces/ws-1/convert-to-personal",
	} {
		req := httptest.NewRequest(http.MethodPost, path, strings.NewReader(`{}`))
		req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
		req.Header.Set(echo.HeaderAuthorization, "Bearer "+signed)
		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, req)
		assert.Contains(t, []int{http.StatusNotFound, http.StatusMethodNotAllowed}, rec.Code, path)
	}
	assert.Empty(t, dept.caller, "no admin service method was reached")
}
