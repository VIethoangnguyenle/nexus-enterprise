package rest

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"ngac-platform/pkg/httputil"
	"ngac-platform/services/workspace/internal/domain"
)

type fakeSettings struct {
	err                error
	calls              int
	caller, user, wsID string
	put                map[string]json.RawMessage
	setRule            string
	stored             map[string]json.RawMessage
	rule               string
}

func (f *fakeSettings) seen(caller, user, ws string) error {
	f.calls++
	f.caller, f.user, f.wsID = caller, user, ws
	return f.err
}

func (f *fakeSettings) GetUserSettings(_ context.Context, caller, user, ws string) (map[string]json.RawMessage, error) {
	return f.stored, f.seen(caller, user, ws)
}
func (f *fakeSettings) PutUserSettings(_ context.Context, caller, user, ws string, kv map[string]json.RawMessage) (map[string]json.RawMessage, error) {
	f.put = kv
	return kv, f.seen(caller, user, ws)
}
func (f *fakeSettings) GroupCreationRule(_ context.Context, caller, ws string) (string, error) {
	return f.rule, f.seen(caller, "", ws)
}
func (f *fakeSettings) SetGroupCreationRule(_ context.Context, caller, ws, rule string) (string, error) {
	f.setRule = rule
	return rule, f.seen(caller, "", ws)
}

func serveSettings(t *testing.T, f *fakeSettings, method, path, body string, withClaims bool) *httptest.ResponseRecorder {
	t.Helper()
	e := httputil.NewEcho("workspace")
	api := e.Group("/api", func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			if withClaims {
				httputil.SetClaims(c, &httputil.Claims{UserID: "user-1", NGACNodeID: callerNode})
			}
			return next(c)
		}
	})
	NewSettingsHandler(f).RegisterRoutes(api)
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	return rec
}

func settingsRoutes() []struct{ name, method, path, body string } {
	return []struct{ name, method, path, body string }{
		{"get settings", http.MethodGet, "/api/workspaces/ws-1/settings", ""},
		{"put settings", http.MethodPut, "/api/workspaces/ws-1/settings", `{"settings":{"mobile.bar":["chat","drive","approval","assets"]}}`},
		{"get locks", http.MethodGet, "/api/workspaces/ws-1/locks", ""},
		{"put locks", http.MethodPut, "/api/workspaces/ws-1/locks", `{"group_creation":"admins"}`},
	}
}

func TestSettingsRoutes_NoClaimsIs401AndNeverReachesTheService(t *testing.T) {
	for _, r := range settingsRoutes() {
		f := &fakeSettings{}
		rec := serveSettings(t, f, r.method, r.path, r.body, false)
		assert.Equal(t, http.StatusUnauthorized, rec.Code, r.name)
		assert.Zero(t, f.calls, r.name)
	}
}

func TestSettingsRoutes_CallerAndWorkspaceComeFromClaimsAndRoute(t *testing.T) {
	for _, r := range settingsRoutes() {
		f := &fakeSettings{rule: "everyone"}
		rec := serveSettings(t, f, r.method, r.path, r.body, true)
		assert.Equal(t, http.StatusOK, rec.Code, "%s: %s", r.name, rec.Body.String())
		assert.Equal(t, callerNode, f.caller, r.name)
		assert.Equal(t, "ws-1", f.wsID, r.name)
		if strings.Contains(r.name, "settings") {
			assert.Equal(t, "user-1", f.user, r.name)
		}
	}
}

func TestSettingsRoutes_ErrorsKeepTheirStatusAndHideInternals(t *testing.T) {
	for name, tc := range map[string]struct {
		err  error
		code int
	}{
		"not a member / not an administrator": {domain.ErrAccessDenied, http.StatusForbidden},
		"unknown workspace":                   {domain.ErrNotFound, http.StatusNotFound},
		"invalid value / personal lock":       {domain.ErrInvalidInput, http.StatusBadRequest},
		"anything else":                       {errors.New("boom: SQLSTATE 42"), http.StatusInternalServerError},
	} {
		for _, r := range settingsRoutes() {
			rec := serveSettings(t, &fakeSettings{err: tc.err}, r.method, r.path, r.body, true)
			assert.Equal(t, tc.code, rec.Code, "%s %s", name, r.name)
			assert.NotContains(t, rec.Body.String(), "SQLSTATE", "%s %s", name, r.name)
		}
	}
}

func TestSettings_ABodyNamingAnotherUserIsIgnored(t *testing.T) {
	f := &fakeSettings{}
	rec := serveSettings(t, f, http.MethodPut, "/api/workspaces/ws-1/settings",
		`{"user_id":"someone-else","settings":{"mobile.bar":["chat","drive","approval","assets"]}}`, true)
	require.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, "user-1", f.user, "the person is the verified claim")
}

func TestSettings_UnsetIsAnEmptyObjectNotNull(t *testing.T) {
	rec := serveSettings(t, &fakeSettings{}, http.MethodGet, "/api/workspaces/ws-1/settings", "", true)
	assert.JSONEq(t, `{"settings":{}}`, rec.Body.String())
}

func TestSettings_MalformedBodyIs400(t *testing.T) {
	for _, path := range []string{"/api/workspaces/ws-1/settings", "/api/workspaces/ws-1/locks"} {
		f := &fakeSettings{}
		rec := serveSettings(t, f, http.MethodPut, path, `{not json`, true)
		assert.Equal(t, http.StatusBadRequest, rec.Code, path)
		assert.Zero(t, f.calls, path)
	}
}

func TestLocks_AnswerTheRuleAndPassTheRequestedOne(t *testing.T) {
	f := &fakeSettings{rule: "admins"}
	rec := serveSettings(t, f, http.MethodGet, "/api/workspaces/ws-1/locks", "", true)
	assert.JSONEq(t, `{"group_creation":"admins"}`, rec.Body.String())

	rec = serveSettings(t, f, http.MethodPut, "/api/workspaces/ws-1/locks", `{"group_creation":"everyone"}`, true)
	assert.JSONEq(t, `{"group_creation":"everyone"}`, rec.Body.String())
	assert.Equal(t, "everyone", f.setRule)
}

// A crafted grant of create_channel on Channels while locked is a conflict the screen can name.
func TestAdminHandler_SetRolePermissionsWhileLockedIs409WithAReason(t *testing.T) {
	f := &fakeDeptService{}
	f.err = domain.ErrGroupCreationLocked
	code, body := call(t, f, "set", find("SetRolePermissions"))
	assert.Equal(t, http.StatusConflict, code)
	assert.Contains(t, body, `"reason":"`+ReasonGroupCreationLocked+`"`)
}
