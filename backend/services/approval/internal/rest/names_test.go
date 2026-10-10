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

	"ngac-platform/pkg/httputil"
	"ngac-platform/services/approval/internal/domain"
)

const (
	reqID  = "11111111-1111-4111-8111-111111111111"
	hoaN   = "node-hoa"
	yenN   = "node-yen"
	ducN   = "node-duc"
	roleN  = "node-role"
	deptID = "dept-1"
)

// fakeNames answers from a map and records how often it was asked.
type fakeNames struct {
	names map[string]string
	err   error
	calls int
	asked []string
}

func (f *fakeNames) DisplayNames(_ context.Context, _ string, keys []string) (map[string]string, error) {
	f.calls++
	f.asked = append(f.asked, keys...)
	if f.err != nil {
		return nil, f.err
	}
	out := map[string]string{}
	for _, k := range keys {
		if n, ok := f.names[k]; ok {
			out[k] = n
		}
	}
	return out, nil
}

var people = map[string]string{
	hoaN: "Lê Thị Hoa", yenN: "Phạm Hải Yến", ducN: "Trần Minh Đức", roleN: "Kế toán trưởng", deptID: "Vận hành thanh toán",
}

// fakeStore serves one request; any other store method a handler should not
// reach panics through the nil embedded interface.
type fakeStore struct {
	domain.Store
	req         *domain.Request
	assignments []*domain.AssignmentRecord
	audit       []*domain.AuditEntry
	mine        []*domain.Request
}

func (f *fakeStore) GetRequest(_ context.Context, id string) (*domain.Request, error) {
	if f.req == nil || f.req.ID != id {
		return nil, domain.ErrNotFound
	}
	cp := *f.req
	return &cp, nil
}
func (f *fakeStore) HasAssignment(_ context.Context, _ string, users []string) (bool, error) {
	for _, a := range f.assignments {
		for _, user := range users {
			if a.UserNodeID == user {
				return true, nil
			}
		}
	}
	return false, nil
}
func (f *fakeStore) ListAssignments(context.Context, string) ([]*domain.AssignmentRecord, error) {
	return f.assignments, nil
}
func (f *fakeStore) ListAuditEntries(context.Context, string) ([]*domain.AuditEntry, error) {
	return f.audit, nil
}
func (f *fakeStore) ListMyRequests(context.Context, string, string, int) ([]*domain.Request, string, error) {
	return f.mine, "", nil
}

type noScopes struct{}

func (noScopes) ResolveAccessibleScopes(context.Context, string, string) ([]string, error) {
	return nil, nil
}
func (noScopes) CheckAccess(context.Context, string, string, string) (bool, error) { return true, nil }
func (noScopes) GetAncestors(context.Context, string) ([]string, error)            { return nil, nil }
func (noScopes) GetMembers(context.Context, string) ([]string, error)              { return nil, nil }

func newHandler(st *fakeStore, names NameResolver) *Handler {
	h := &Handler{svc: domain.NewService(st, noScopes{})}
	if names != nil {
		h.WithNames(names)
	}
	return h
}

func call(t *testing.T, fn func(echo.Context) error, caller, id string) (int, map[string]any) {
	t.Helper()
	e := echo.New()
	rec := httptest.NewRecorder()
	c := e.NewContext(httptest.NewRequest(http.MethodGet, "/", nil), rec)
	c.SetParamNames("id")
	c.SetParamValues(id)
	httputil.SetClaims(c, &httputil.Claims{UserID: "u", TenantID: "t", NGACNodeID: caller})
	if err := fn(c); err != nil {
		he, ok := err.(*echo.HTTPError)
		if !ok {
			t.Fatalf("err = %v", err)
		}
		return he.Code, nil
	}
	var body map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("body %q: %v", rec.Body.String(), err)
	}
	return rec.Code, body
}

func fixture() *fakeStore {
	return &fakeStore{
		req: &domain.Request{
			ID: reqID, CreatedBy: yenN, DepartmentID: deptID, TemplateName: "Tạm ứng", Status: "pending", CurrentStep: 2,
			TemplateSnapshot: `{"steps":[{"step_order":1,"name":"Trưởng phòng","approver_type":"specific_user","approver_value":"node-duc"},` +
				`{"step_order":2,"name":"Kế toán","approver_type":"role_in_dept","approver_value":"node-role"}],"form_fields":[]}`,
		},
		assignments: []*domain.AssignmentRecord{
			{ID: "a1", RequestID: reqID, StepOrder: 1, UserNodeID: ducN, GrantSource: "direct", Status: "approved"},
			{ID: "a2", RequestID: reqID, StepOrder: 2, UserNodeID: hoaN, GrantSource: "role:Kế toán trưởng", Status: "pending"},
		},
		audit: []*domain.AuditEntry{
			{ID: "e1", RequestID: reqID, Action: "created", ActorNodeID: yenN},
			{ID: "e2", RequestID: reqID, Action: "step_advanced"},
		},
	}
}

func TestGetRequest_NamesRequesterApproversAndDepartment(t *testing.T) {
	h := newHandler(fixture(), &fakeNames{names: people})
	code, body := call(t, h.GetRequest, hoaN, reqID)
	if code != http.StatusOK {
		t.Fatalf("status = %d", code)
	}
	req := body["request"].(map[string]any)
	if req["created_by_name"] != "Phạm Hải Yến" || req["department_name"] != "Vận hành thanh toán" {
		t.Errorf("request names = %v / %v", req["created_by_name"], req["department_name"])
	}
	steps := body["steps"].([]any)
	if steps[0].(map[string]any)["approver_name"] != "Trần Minh Đức" || steps[1].(map[string]any)["approver_name"] != "Kế toán trưởng" {
		t.Errorf("steps = %v", steps)
	}
	asg := body["assignments"].([]any)
	if asg[0].(map[string]any)["user_name"] != "Trần Minh Đức" || asg[1].(map[string]any)["user_name"] != "Lê Thị Hoa" {
		t.Errorf("assignments = %v", asg)
	}
	if _, leaked := req["template_snapshot"]; leaked {
		t.Error("the snapshot is unpacked into steps and form_fields, not repeated")
	}
}

func TestGetRequest_DeniedAndMalformed(t *testing.T) {
	h := newHandler(fixture(), &fakeNames{names: people})
	if code, _ := call(t, h.GetRequest, "stranger", reqID); code != http.StatusForbidden {
		t.Errorf("stranger: status = %d, want 403", code)
	}
	if code, _ := call(t, h.GetRequest, hoaN, "nope"); code != http.StatusBadRequest {
		t.Errorf("malformed: status = %d, want 400", code)
	}
}

func TestGetAuditLog_NamesTheActorAndLeavesSystemEntriesAnonymous(t *testing.T) {
	h := newHandler(fixture(), &fakeNames{names: people})
	code, body := call(t, h.GetAuditLog, hoaN, reqID)
	if code != http.StatusOK {
		t.Fatalf("status = %d", code)
	}
	entries := body["entries"].([]any)
	if entries[0].(map[string]any)["actor_name"] != "Phạm Hải Yến" {
		t.Errorf("first entry = %v", entries[0])
	}
	if _, has := entries[1].(map[string]any)["actor_name"]; has {
		t.Errorf("a system entry has no actor: %v", entries[1])
	}
}

func TestGetAuditLog_StaysForbiddenForStrangers(t *testing.T) {
	h := newHandler(fixture(), &fakeNames{names: people})
	if code, _ := call(t, h.GetAuditLog, "stranger", reqID); code != http.StatusForbidden {
		t.Errorf("status = %d, want 403", code)
	}
}

func TestGetMyRequests_NamesEveryRowWithOneLookup(t *testing.T) {
	st := fixture()
	st.mine = []*domain.Request{
		{ID: "r1", CreatedBy: yenN}, {ID: "r2", CreatedBy: yenN}, {ID: "r3", CreatedBy: ducN},
	}
	fn := &fakeNames{names: people}
	h := newHandler(st, fn)
	code, body := call(t, h.GetMyRequests, yenN, "")
	if code != http.StatusOK {
		t.Fatalf("status = %d", code)
	}
	items := body["items"].([]any)
	if items[0].(map[string]any)["created_by_name"] != "Phạm Hải Yến" || items[2].(map[string]any)["created_by_name"] != "Trần Minh Đức" {
		t.Errorf("items = %v", items)
	}
	if fn.calls != 1 {
		t.Errorf("name lookups = %d, want one for the whole page", fn.calls)
	}
}

// Names are a courtesy: when they cannot be looked up the data still goes out,
// and no id takes their place.
func TestNames_LookupFailureStillAnswers(t *testing.T) {
	h := newHandler(fixture(), &fakeNames{err: errors.New("db down")})
	code, body := call(t, h.GetRequest, hoaN, reqID)
	if code != http.StatusOK {
		t.Fatalf("status = %d", code)
	}
	req := body["request"].(map[string]any)
	if _, has := req["created_by_name"]; has {
		t.Errorf("a name appeared from nowhere: %v", req["created_by_name"])
	}
}

func TestNames_NoResolverStillAnswers(t *testing.T) {
	h := newHandler(fixture(), nil)
	if code, _ := call(t, h.GetRequest, hoaN, reqID); code != http.StatusOK {
		t.Errorf("status = %d", code)
	}
}

func TestNames_AskForEachKeyOnce(t *testing.T) {
	fn := &fakeNames{names: people}
	h := newHandler(fixture(), fn)
	call(t, h.GetRequest, hoaN, reqID)
	seen := map[string]int{}
	for _, k := range fn.asked {
		seen[k]++
		if k == "" || strings.HasPrefix(k, "{") {
			t.Errorf("asked for a non-key %q", k)
		}
	}
	for k, n := range seen {
		if n > 1 {
			t.Errorf("%q asked %d times", k, n)
		}
	}
}

// The body of an update says "no form fields" two ways: leaving the key out
// (keep them) and sending an empty list (remove them all).
func TestUpdateTemplate_EmptyFieldListIsNotTheSameAsNoFieldList(t *testing.T) {
	for _, tc := range []struct {
		name, body string
		wantNil    bool
	}{
		{"absent", `{"name":"A","is_active":true}`, true},
		{"empty", `{"name":"A","is_active":true,"form_fields":[]}`, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := formFieldsOf(updateBody(t, tc.body))
			if (got == nil) != tc.wantNil {
				t.Errorf("form fields nil = %v, want %v", got == nil, tc.wantNil)
			}
		})
	}
}

func updateBody(t *testing.T, raw string) updateTemplateBody {
	t.Helper()
	var b updateTemplateBody
	if err := json.Unmarshal([]byte(raw), &b); err != nil {
		t.Fatal(err)
	}
	return b
}
