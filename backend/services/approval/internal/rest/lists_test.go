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

// listStore answers the list queries from canned rows and remembers what it
// was asked, so a test can see which user and scopes reached the store.
type listStore struct {
	domain.Store
	pending     []*domain.RequestWithAssignment
	history     []*domain.RequestWithAssignment
	byScope     []*domain.Request
	templates   []*domain.Template
	err         error
	pendingUser string
	groups      []string
	historyUser string
	scopesAsked []string
	cursor      string
	limit       int
}

func (s *listStore) ListPending(_ context.Context, user string, groups []string) ([]*domain.RequestWithAssignment, error) {
	s.pendingUser, s.groups = user, groups
	return s.pending, s.err
}
func (s *listStore) ListHistory(_ context.Context, user, cursor string, limit int) ([]*domain.RequestWithAssignment, string, error) {
	s.historyUser, s.cursor, s.limit = user, cursor, limit
	return s.history, "next-cursor", s.err
}
func (s *listStore) ListByScopes(_ context.Context, scopes []string, cursor string, limit int) ([]*domain.Request, string, error) {
	s.scopesAsked, s.cursor, s.limit = scopes, cursor, limit
	return s.byScope, "", s.err
}
func (s *listStore) GetTemplate(_ context.Context, id string) (*domain.Template, error) {
	for _, t := range s.templates {
		if t.ID == id {
			return t, nil
		}
	}
	return nil, domain.ErrNotFound
}
func (s *listStore) ListTemplates(context.Context, string, bool) ([]*domain.Template, error) {
	return s.templates, s.err
}

type member struct{ noScopes }

func (member) GetAncestors(context.Context, string) ([]string, error) {
	return []string{"role-node"}, nil
}
func (member) ResolveAccessibleScopes(context.Context, string, string) ([]string, error) {
	return []string{"oa-sales"}, nil
}

func get(t *testing.T, h echo.HandlerFunc, path, caller, paramName, paramValue string) (int, map[string]any) {
	t.Helper()
	e := httputil.NewEcho("approval")
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, path, nil)
	c := e.NewContext(req, rec)
	if paramName != "" {
		c.SetParamNames(paramName)
		c.SetParamValues(paramValue)
	}
	if caller != "" {
		httputil.SetClaims(c, &httputil.Claims{UserID: "u", TenantID: "t", NGACNodeID: caller})
	}
	if err := h(c); err != nil {
		e.HTTPErrorHandler(err, c)
	}
	var body map[string]any
	_ = json.Unmarshal(rec.Body.Bytes(), &body)
	return rec.Code, body
}

func TestGetPending_AsksForTheCallersOwnWorkAndGroups(t *testing.T) {
	st := &listStore{pending: []*domain.RequestWithAssignment{
		{Request: &domain.Request{ID: "r1", CreatedBy: yenN}, Assignment: &domain.AssignmentRecord{ID: "a1", UserNodeID: hoaN}},
	}}
	h := &Handler{svc: domain.NewService(st, member{})}

	code, body := get(t, h.GetPending, "/", hoaN, "", "")
	if code != http.StatusOK || body["total"] != float64(1) {
		t.Fatalf("status %d body %v", code, body)
	}
	if st.pendingUser != hoaN || len(st.groups) != 1 || st.groups[0] != "role-node" {
		t.Errorf("store asked for user %q groups %v", st.pendingUser, st.groups)
	}

	if code, _ := get(t, h.GetPending, "/", "", "", ""); code != http.StatusUnauthorized {
		t.Errorf("no session: status %d", code)
	}
}

func TestGetHistory_PassesTheCursorAndABoundedLimit(t *testing.T) {
	st := &listStore{}
	h := &Handler{svc: domain.NewService(st, member{})}

	code, body := get(t, h.GetHistory, "/?cursor=2026-10-01T00:00:00Z&limit=500", hoaN, "", "")
	if code != http.StatusOK || body["next_cursor"] != "next-cursor" {
		t.Fatalf("status %d body %v", code, body)
	}
	if st.historyUser != hoaN || st.cursor != "2026-10-01T00:00:00Z" {
		t.Errorf("store saw user %q cursor %q", st.historyUser, st.cursor)
	}
	if st.limit != 20 {
		t.Errorf("an out-of-range limit falls back to 20, got %d", st.limit)
	}
	get(t, h.GetHistory, "/?limit=7", hoaN, "", "")
	if st.limit != 7 {
		t.Errorf("limit = %d, want 7", st.limit)
	}
	if code, _ := get(t, h.GetHistory, "/", "", "", ""); code != http.StatusUnauthorized {
		t.Errorf("no session: status %d", code)
	}
}

func TestGetDepartmentRequests_StaysInsideTheScopesTheCallerMayRead(t *testing.T) {
	st := &listStore{byScope: []*domain.Request{{ID: "r1", CreatedBy: yenN}}}
	h := &Handler{svc: domain.NewService(st, member{})}

	code, body := get(t, h.GetDepartmentRequests, "/", hoaN, "", "")
	if code != http.StatusOK || len(body["items"].([]any)) != 1 {
		t.Fatalf("status %d body %v", code, body)
	}
	if len(st.scopesAsked) != 1 || st.scopesAsked[0] != "oa-sales" {
		t.Errorf("store asked for scopes %v", st.scopesAsked)
	}

	// A caller who can read no scope sees nothing, and the store is not asked.
	none := &listStore{byScope: []*domain.Request{{ID: "leak"}}}
	h2 := &Handler{svc: domain.NewService(none, noScopes{})}
	code, body = get(t, h2.GetDepartmentRequests, "/", hoaN, "", "")
	if code != http.StatusOK {
		t.Fatalf("status %d", code)
	}
	if items, _ := body["items"].([]any); len(items) != 0 {
		t.Errorf("a caller with no scopes sees %v", items)
	}
	if none.scopesAsked != nil {
		t.Errorf("store asked for %v", none.scopesAsked)
	}
}

func TestListFailuresAreGenericAndSessionless(t *testing.T) {
	st := &listStore{err: errors.New(`ERROR: relation "t_acme.approval_assignments" does not exist (SQLSTATE 42P01)`)}
	h := &Handler{svc: domain.NewService(st, member{})}
	for name, fn := range map[string]echo.HandlerFunc{
		"pending": h.GetPending, "history": h.GetHistory, "department": h.GetDepartmentRequests, "templates": h.ListTemplates,
	} {
		code, body := get(t, fn, "/", hoaN, "", "")
		if code != http.StatusInternalServerError {
			t.Errorf("%s: status %d", name, code)
		}
		raw, _ := json.Marshal(body)
		for _, leak := range []string{"SQLSTATE", "approval_assignments", "t_acme"} {
			if strings.Contains(string(raw), leak) {
				t.Errorf("%s leaks %q: %s", name, leak, raw)
			}
		}
	}
}

func TestTemplateReads(t *testing.T) {
	st := &listStore{templates: []*domain.Template{{ID: "t1", Name: "Tạm ứng", EntityType: "expense", IsActive: true}}}
	h := &Handler{svc: domain.NewService(st, member{})}

	code, body := get(t, h.GetTemplate, "/", hoaN, "id", "t1")
	if code != http.StatusOK || body["name"] != "Tạm ứng" {
		t.Errorf("get: status %d body %v", code, body)
	}
	if code, _ := get(t, h.GetTemplate, "/", hoaN, "id", "missing"); code != http.StatusNotFound {
		t.Errorf("unknown template: status %d", code)
	}
	code, body = get(t, h.ListTemplates, "/?entity_type=expense", hoaN, "", "")
	if code != http.StatusOK || len(body["templates"].([]any)) != 1 {
		t.Errorf("list: status %d body %v", code, body)
	}
}

// A cursor the store refuses is the client's mistake: 400, not a failure of ours.
func TestListCursor_RefusedByTheStoreIs400(t *testing.T) {
	st := &listStore{err: domain.ErrInvalidInput}
	h := &Handler{svc: domain.NewService(st, member{})}
	if code, _ := get(t, h.GetHistory, "/?cursor=garbage", hoaN, "", ""); code != http.StatusBadRequest {
		t.Errorf("status %d, want 400", code)
	}
}
