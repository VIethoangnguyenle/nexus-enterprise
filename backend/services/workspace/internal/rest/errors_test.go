package rest

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/labstack/echo/v4"

	"ngac-platform/pkg/httputil"
)

// A failure of ours never shows its text: the client gets a generic message and
// a request ID, the log gets the cause. The routes are those that used to reach
// the domain through the gRPC server.
func TestInternalFailuresAreSanitised(t *testing.T) {
	f := &fakeWorkspaceService{err: errors.New(`ERROR: relation "workspaces" does not exist (SQLSTATE 42P01)`)}
	h := NewHandler(f)
	e := httputil.NewEcho("workspace")
	signedIn := func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			httputil.SetClaims(c, &httputil.Claims{UserID: "u", NGACNodeID: callerNode})
			return next(c)
		}
	}
	e.GET("/api/workspaces", h.ListWorkspaces, signedIn)
	e.GET("/api/workspaces/:id", h.GetWorkspace, signedIn)
	e.GET("/api/workspaces/:id/members", h.ListMembers, signedIn)
	e.DELETE("/api/workspaces/:id/members/:nodeId", h.RemoveMember, signedIn)
	e.POST("/api/workspaces/:id/folders", h.CreateFolder, signedIn)

	for _, tc := range []struct{ method, path, body string }{
		{http.MethodGet, "/api/workspaces", ""},
		{http.MethodGet, "/api/workspaces/ws-1", ""},
		{http.MethodGet, "/api/workspaces/ws-1/members", ""},
		{http.MethodDelete, "/api/workspaces/ws-1/members/u-x", ""},
		{http.MethodPost, "/api/workspaces/ws-1/folders", `{"name":"Legal"}`},
	} {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(tc.method, tc.path, strings.NewReader(tc.body))
		req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
		e.ServeHTTP(rec, req)

		body := rec.Body.String()
		if rec.Code != http.StatusInternalServerError {
			t.Errorf("%s %s: status = %d", tc.method, tc.path, rec.Code)
		}
		if strings.Contains(body, "SQLSTATE") || strings.Contains(body, "workspaces\"") {
			t.Errorf("%s %s: body leaks: %s", tc.method, tc.path, body)
		}
		if !strings.Contains(body, `"request_id"`) || !strings.Contains(body, `"internal error"`) {
			t.Errorf("%s %s: body = %s", tc.method, tc.path, body)
		}
	}
}
