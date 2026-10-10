package rest

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/labstack/echo/v4"

	"ngac-platform/ngac"
	"ngac-platform/pkg/httputil"
	"ngac-platform/services/approval/internal/domain"
)

// tmplStore holds one template. InsertTemplate is not defined, so a test that
// reaches it panics through the nil embedded interface: a refused create must
// never get that far.
type tmplStore struct {
	domain.Store
	tmpl    *domain.Template
	updated int
}

func (s *tmplStore) FindNodeID(_ context.Context, name, nodeType string) (string, error) {
	if nodeType == ngac.TypeOA && name == ngac.MgmtOAName("tenant-a") {
		return "mgmt-oa", nil
	}
	return "", domain.ErrNotFound
}
func (s *tmplStore) CanonicalApprover(_ context.Context, _, _, v string) (string, error) {
	return v, nil
}
func (s *tmplStore) GetTemplate(context.Context, string) (*domain.Template, error) {
	cp := *s.tmpl
	return &cp, nil
}
func (s *tmplStore) UpdateTemplate(_ context.Context, t *domain.Template, expected time.Time) (time.Time, error) {
	if !expected.Equal(s.tmpl.UpdatedAt) {
		return time.Time{}, domain.ErrStale
	}
	s.updated++
	return expected.Add(time.Second), nil
}

// adminOnly allows manage on the management attribute to n-admin alone.
type adminOnly struct{ noScopes }

func (adminOnly) CheckAccess(_ context.Context, user, object, op string) (bool, error) {
	return user == "n-admin" && object == "mgmt-oa" && op == ngac.OpManage, nil
}

func tmplHandler() (*Handler, *tmplStore) {
	st := &tmplStore{tmpl: &domain.Template{
		ID: "t1", Name: "Old", EntityType: "expense", IsActive: true, UpdatedAt: time.Date(2026, 10, 1, 8, 0, 0, 0, time.UTC),
		Steps: []*domain.Step{{StepOrder: 1, ApproverType: "specific_user", ApproverValue: "n-boss", RequiredCount: 1}},
	}}
	return &Handler{svc: domain.NewService(st, adminOnly{})}, st
}

func send(t *testing.T, fn echo.HandlerFunc, method, caller, tenant, id, body string) (int, string) {
	t.Helper()
	e := echo.New()
	req := httptest.NewRequest(method, "/", strings.NewReader(body))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetParamNames("id")
	c.SetParamValues(id)
	httputil.SetClaims(c, &httputil.Claims{UserID: "u", TenantID: tenant, NGACNodeID: caller})
	if err := fn(c); err != nil {
		he := err.(*echo.HTTPError)
		return he.Code, ""
	}
	return rec.Code, rec.Body.String()
}

const createBody = `{"name":"Mới","entity_type":"expense","steps":[{"step_order":1,"name":"S","approver_type":"specific_user","approver_value":"n-me"}]}`

func updateBody2(extra string) string {
	return `{"name":"New","is_active":true,"expected_updated_at":"2026-10-01T08:00:00Z"` + extra + `}`
}

func TestPostTemplate_MemberIsDenied(t *testing.T) {
	h, _ := tmplHandler()
	for name, caller := range map[string]string{"member": "n-member", "someone with no node": ""} {
		if code, _ := send(t, h.CreateTemplate, http.MethodPost, caller, "tenant-a", "", createBody); code != http.StatusForbidden {
			t.Errorf("%s: status = %d, want 403", name, code)
		}
	}
	// An administrator of ANOTHER tenant has no management attribute here.
	if code, _ := send(t, h.CreateTemplate, http.MethodPost, "n-admin", "tenant-b", "", createBody); code != http.StatusForbidden {
		t.Errorf("admin signed in to another tenant: status = %d, want 403", code)
	}
}

func TestPutTemplate_MemberIsDeniedAndNothingChanges(t *testing.T) {
	h, st := tmplHandler()
	evil := updateBody2(`,"steps":[{"step_order":1,"name":"Me","approver_type":"specific_user","approver_value":"n-me"}]`)
	if code, _ := send(t, h.UpdateTemplate, http.MethodPut, "n-member", "tenant-a", "t1", evil); code != http.StatusForbidden {
		t.Fatalf("status = %d, want 403", code)
	}
	if st.updated != 0 {
		t.Fatal("a refused update reached the store")
	}
	if code, _ := send(t, h.UpdateTemplate, http.MethodPut, "n-admin", "tenant-a", "t1", evil); code != http.StatusOK {
		t.Fatalf("administrator: status = %d, want 200", code)
	}
}

func TestPutTemplate_ConflictAndBadBodies(t *testing.T) {
	h, _ := tmplHandler()
	stale := `{"name":"New","is_active":true,"expected_updated_at":"2026-09-30T00:00:00Z"}`
	if code, _ := send(t, h.UpdateTemplate, http.MethodPut, "n-admin", "tenant-a", "t1", stale); code != http.StatusConflict {
		t.Errorf("stale precondition: status = %d, want 409", code)
	}
	if code, _ := send(t, h.UpdateTemplate, http.MethodPut, "n-admin", "tenant-a", "t1", `{"name":"New","is_active":true}`); code != http.StatusBadRequest {
		t.Errorf("no precondition: status = %d, want 400", code)
	}
	dup := updateBody2(`,"steps":[{"step_order":1,"name":"a","approver_type":"specific_user","approver_value":"x"},{"step_order":1,"name":"b","approver_type":"specific_user","approver_value":"y"}]`)
	if code, _ := send(t, h.UpdateTemplate, http.MethodPut, "n-admin", "tenant-a", "t1", dup); code != http.StatusBadRequest {
		t.Errorf("duplicate step_order: status = %d, want 400 (not 500)", code)
	}
	kind := updateBody2(`,"steps":[{"step_order":1,"name":"a","approver_type":"creator_manager","approver_value":"x"}]`)
	if code, _ := send(t, h.UpdateTemplate, http.MethodPut, "n-admin", "tenant-a", "t1", kind); code != http.StatusBadRequest {
		t.Errorf("unsupported approver kind: status = %d, want 400", code)
	}
}

func TestGetPermissions_TellsWhoMayManageTemplates(t *testing.T) {
	h, _ := tmplHandler()
	if code, body := send(t, h.GetPermissions, http.MethodGet, "n-admin", "tenant-a", "", ""); code != 200 || !strings.Contains(body, `"can_manage_templates":true`) {
		t.Errorf("admin: %d %s", code, body)
	}
	if code, body := send(t, h.GetPermissions, http.MethodGet, "n-member", "tenant-a", "", ""); code != 200 || !strings.Contains(body, `"can_manage_templates":false`) {
		t.Errorf("member: %d %s", code, body)
	}
}
