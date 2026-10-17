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

// The rule endpoints of Quản trị › Danh bạ. The caller is the verified claim, the ids are the
// route's, a refusal is a refusal and an outage is an outage; the domain decides the rest.

type fakeRules struct {
	fakeDeptService
	rules *domain.DirectoryRules
	// last is the operation the handler asked for and the id it passed.
	last, lastID string
}

func (f *fakeRules) DirectoryRules(_ context.Context, caller, wsID string) (*domain.DirectoryRules, error) {
	f.last = "rules"
	if err := f.rec(caller, wsID); err != nil {
		return nil, err
	}
	return f.rules, nil
}
func (f *fakeRules) HideDepartment(_ context.Context, caller, wsID, id string) error {
	f.last, f.lastID = "hide", id
	return f.rec(caller, wsID)
}
func (f *fakeRules) UnhideDepartment(_ context.Context, caller, wsID, id string) error {
	f.last, f.lastID = "unhide", id
	return f.rec(caller, wsID)
}
func (f *fakeRules) LimitRole(_ context.Context, caller, wsID, id string) error {
	f.last, f.lastID = "limit-role", id
	return f.rec(caller, wsID)
}
func (f *fakeRules) UnlimitRole(_ context.Context, caller, wsID, id string) error {
	f.last, f.lastID = "unlimit-role", id
	return f.rec(caller, wsID)
}
func (f *fakeRules) LimitDepartment(_ context.Context, caller, wsID, id string) error {
	f.last, f.lastID = "limit-dept", id
	return f.rec(caller, wsID)
}
func (f *fakeRules) UnlimitDepartment(_ context.Context, caller, wsID, id string) error {
	f.last, f.lastID = "unlimit-dept", id
	return f.rec(caller, wsID)
}

type ruleRoute struct {
	name, method, path, op, id string
}

func ruleRoutes() []ruleRoute {
	const base = "/api/workspaces/ws-1/admin/directory-scope"
	return []ruleRoute{
		{"rules", http.MethodGet, base, "rules", ""},
		{"hide", http.MethodPut, base + "/hidden/dept-1", "hide", "dept-1"},
		{"unhide", http.MethodDelete, base + "/hidden/dept-1", "unhide", "dept-1"},
		{"limit role", http.MethodPut, base + "/limited/roles/ua-role", "limit-role", "ua-role"},
		{"unlimit role", http.MethodDelete, base + "/limited/roles/ua-role", "unlimit-role", "ua-role"},
		{"limit department", http.MethodPut, base + "/limited/departments/dept-2", "limit-dept", "dept-2"},
		{"unlimit department", http.MethodDelete, base + "/limited/departments/dept-2", "unlimit-dept", "dept-2"},
	}
}

// serve mounts the real admin routes behind a claims stub (or none) and sends one request.
func serveRule(t *testing.T, f *fakeRules, r ruleRoute, withClaims bool) *httptest.ResponseRecorder {
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
	NewAdminHandler(f).RegisterAdminRoutes(api)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(r.method, r.path, strings.NewReader(""))
	e.ServeHTTP(rec, req)
	return rec
}

func TestDirectoryScopeRoutes_CallerAndIdsComeFromClaimsAndRoute(t *testing.T) {
	for _, r := range ruleRoutes() {
		t.Run(r.name, func(t *testing.T) {
			f := &fakeRules{rules: &domain.DirectoryRules{Ready: true}}
			rec := serveRule(t, f, r, true)
			assert.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
			assert.Equal(t, r.op, f.last)
			assert.Equal(t, r.id, f.lastID)
			assert.Equal(t, callerNode, f.caller)
			assert.Equal(t, "ws-1", f.wsID)
		})
	}
}

func TestDirectoryScopeRoutes_ErrorsKeepTheirStatus(t *testing.T) {
	for name, tc := range map[string]struct {
		err  error
		code int
	}{
		"not an administrator": {domain.ErrAccessDenied, http.StatusForbidden},
		"foreign id":           {domain.ErrNotFound, http.StatusNotFound},
		"personal workspace":   {domain.ErrInvalidInput, http.StatusBadRequest},
		"directory not built":  {domain.ErrUnavailable, http.StatusServiceUnavailable},
		"anything else":        {errors.New("boom: SQLSTATE 42"), http.StatusInternalServerError},
	} {
		for _, r := range ruleRoutes() {
			f := &fakeRules{fakeDeptService: fakeDeptService{err: tc.err}}
			rec := serveRule(t, f, r, true)
			assert.Equal(t, tc.code, rec.Code, "%s %s", name, r.name)
			assert.NotContains(t, rec.Body.String(), "SQLSTATE", "%s %s: a failure of ours never shows its text", name, r.name)
		}
	}
}

func TestDirectoryScopeRoutes_NoClaimsIs401AndNeverReachesTheService(t *testing.T) {
	for _, r := range ruleRoutes() {
		f := &fakeRules{}
		rec := serveRule(t, f, r, false)
		assert.Equal(t, http.StatusUnauthorized, rec.Code, r.name)
		assert.Zero(t, f.calls, r.name)
	}
}

// Names and counts, never an identifier a person would have to read; "ready" tells the
// screen whether to show the tab at all, which an outage (503) must not be confused with.
func TestDirectoryScopeRoutes_ListCarriesNamesAndReady(t *testing.T) {
	list := ruleRoutes()[0]
	f := &fakeRules{rules: &domain.DirectoryRules{
		Ready:              true,
		HiddenDepartments:  []domain.RuleSubject{{ID: "d1", Name: "Pháp chế", MemberCount: 4}},
		LimitedRoles:       []domain.RuleSubject{{ID: "r1", Name: "Cộng tác viên", MemberCount: 2}},
		LimitedDepartments: []domain.RuleSubject{{ID: "d2", Name: "Đối soát", ParentID: "d0", MemberCount: 7}},
	}}
	rec := serveRule(t, f, list, true)
	require.Equal(t, http.StatusOK, rec.Code)
	var got struct {
		Ready  bool `json:"ready"`
		Hidden []struct {
			ID, Name    string
			MemberCount int    `json:"member_count"`
			ParentID    string `json:"parent_id"`
		} `json:"hidden_departments"`
		Limited []struct {
			Kind, ID, Name string
			MemberCount    int    `json:"member_count"`
			ParentID       string `json:"parent_id"`
		} `json:"limited"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &got))
	assert.True(t, got.Ready)
	require.Len(t, got.Hidden, 1)
	assert.Equal(t, "Pháp chế", got.Hidden[0].Name)
	assert.Equal(t, 4, got.Hidden[0].MemberCount)
	require.Len(t, got.Limited, 2)
	assert.Equal(t, "role", got.Limited[0].Kind)
	assert.Equal(t, "Cộng tác viên", got.Limited[0].Name)
	assert.Equal(t, "department", got.Limited[1].Kind)
	assert.Equal(t, "d0", got.Limited[1].ParentID)

	// A workspace without directory nodes: 200 with ready=false and empty lists (never null).
	f = &fakeRules{rules: &domain.DirectoryRules{}}
	rec = serveRule(t, f, list, true)
	require.Equal(t, http.StatusOK, rec.Code)
	assert.JSONEq(t, `{"ready":false,"hidden_departments":[],"limited":[]}`, rec.Body.String())
}

// Only an administrator is told which departments are hidden: the department list carries
// the flag when the domain sets it and not otherwise.
func TestListDepartments_HiddenFlagIsOnlyWhereTheDomainSetsIt(t *testing.T) {
	f := &fakeDeptListing{results: []*domain.DepartmentResult{{ID: "d1", Name: "Pháp chế", Hidden: true}, {ID: "d2", Name: "Đối soát"}}}
	e := httputil.NewEcho("workspace")
	api := e.Group("/api", func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			httputil.SetClaims(c, &httputil.Claims{UserID: "user-1", NGACNodeID: callerNode})
			return next(c)
		}
	})
	NewAdminHandler(f).RegisterAdminRoutes(api)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/workspaces/ws-1/departments", nil))
	require.Equal(t, http.StatusOK, rec.Code)
	var got struct {
		Departments []map[string]any `json:"departments"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &got))
	require.Len(t, got.Departments, 2)
	assert.Equal(t, true, got.Departments[0]["hidden"])
	assert.NotContains(t, got.Departments[1], "hidden", "a department that is not hidden carries no flag")
}

type fakeDeptListing struct {
	fakeRules
	results []*domain.DepartmentResult
}

func (f *fakeDeptListing) ListDepartments(context.Context, string, string) ([]*domain.DepartmentResult, error) {
	return f.results, nil
}

// fakeDeptService answers the rule calls too, so it stays an AdminService; fakeRules records them.
func (f *fakeDeptService) DirectoryRules(_ context.Context, caller, wsID string) (*domain.DirectoryRules, error) {
	return &domain.DirectoryRules{}, f.rec(caller, wsID)
}
func (f *fakeDeptService) HideDepartment(_ context.Context, caller, wsID, _ string) error {
	return f.rec(caller, wsID)
}
func (f *fakeDeptService) UnhideDepartment(_ context.Context, caller, wsID, _ string) error {
	return f.rec(caller, wsID)
}
func (f *fakeDeptService) LimitRole(_ context.Context, caller, wsID, _ string) error {
	return f.rec(caller, wsID)
}
func (f *fakeDeptService) UnlimitRole(_ context.Context, caller, wsID, _ string) error {
	return f.rec(caller, wsID)
}
func (f *fakeDeptService) LimitDepartment(_ context.Context, caller, wsID, _ string) error {
	return f.rec(caller, wsID)
}
func (f *fakeDeptService) UnlimitDepartment(_ context.Context, caller, wsID, _ string) error {
	return f.rec(caller, wsID)
}
