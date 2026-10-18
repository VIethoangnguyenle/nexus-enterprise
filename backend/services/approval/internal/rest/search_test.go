package rest

import (
	"context"
	"net/http"
	"testing"

	"ngac-platform/services/approval/internal/domain"
)

// searchStore answers search queries from canned rows and remembers what it
// was asked, so a test can see which params reached the store.
type searchStore struct {
	domain.Store
	results []*domain.Request
	total   int
	err     error
	params  domain.SearchParams
}

func (s *searchStore) WorkspaceType(context.Context, string) (string, error) {
	return "organization", nil
}

func (s *searchStore) SearchRequests(_ context.Context, p domain.SearchParams) ([]*domain.Request, int, error) {
	s.params = p
	return s.results, s.total, s.err
}

// searchPolicy provides the ancestors and scopes the search domain method needs.
type searchPolicy struct{ noScopes }

func (searchPolicy) GetAncestors(context.Context, string) ([]string, error) {
	return []string{"role-node"}, nil
}
func (searchPolicy) ResolveAccessibleScopes(context.Context, string, string) ([]string, error) {
	return []string{"oa-sales"}, nil
}

func TestSearch_ShortQueryIs400(t *testing.T) {
	st := &searchStore{}
	h := &Handler{svc: domain.NewService(st, searchPolicy{})}

	code, _ := get(t, h.Search, "/api/approval/search?q=a", hoaN, "", "")
	if code != http.StatusBadRequest {
		t.Fatalf("short query: status %d, want 400", code)
	}
}

func TestSearch_MissingQueryIs400(t *testing.T) {
	st := &searchStore{}
	h := &Handler{svc: domain.NewService(st, searchPolicy{})}

	code, _ := get(t, h.Search, "/api/approval/search", hoaN, "", "")
	if code != http.StatusBadRequest {
		t.Fatalf("missing query: status %d, want 400", code)
	}
}

func TestSearch_BadStatusIs400(t *testing.T) {
	st := &searchStore{}
	h := &Handler{svc: domain.NewService(st, searchPolicy{})}

	code, _ := get(t, h.Search, "/api/approval/search?q=test&status=invalid", hoaN, "", "")
	if code != http.StatusBadRequest {
		t.Fatalf("bad status: status %d, want 400", code)
	}
}

func TestSearch_ValidStatusesAreAccepted(t *testing.T) {
	for _, status := range []string{"pending", "approved", "rejected", "cancelled"} {
		st := &searchStore{results: []*domain.Request{}, total: 0}
		h := &Handler{svc: domain.NewService(st, searchPolicy{})}

		code, _ := get(t, h.Search, "/api/approval/search?q=test&status="+status, hoaN, "", "")
		if code != http.StatusOK {
			t.Errorf("status=%s: got %d, want 200", status, code)
		}
		if st.params.Status != status {
			t.Errorf("status=%s: store saw %q", status, st.params.Status)
		}
	}
}

func TestSearch_NoSession401(t *testing.T) {
	st := &searchStore{}
	h := &Handler{svc: domain.NewService(st, searchPolicy{})}

	code, _ := get(t, h.Search, "/api/approval/search?q=test", "", "", "")
	if code != http.StatusUnauthorized {
		t.Fatalf("no session: status %d, want 401", code)
	}
}

func TestSearch_SuccessPassesParamsToStore(t *testing.T) {
	st := &searchStore{
		results: []*domain.Request{
			{ID: "r1", CreatedBy: yenN, TemplateName: "Tạm ứng", Status: "pending"},
		},
		total: 1,
	}
	h := &Handler{svc: domain.NewService(st, searchPolicy{})}

	code, body := get(t, h.Search, "/api/approval/search?q=tam&limit=10&status=pending&template_id=t1&created_by=node-yen&from=2026-01-01&to=2026-12-31", hoaN, "", "")
	if code != http.StatusOK {
		t.Fatalf("status %d, want 200, body %v", code, body)
	}
	if st.params.Query != "tam" {
		t.Errorf("query = %q, want tam", st.params.Query)
	}
	if st.params.Limit != 10 {
		t.Errorf("limit = %d, want 10", st.params.Limit)
	}
	if st.params.Status != "pending" {
		t.Errorf("status = %q, want pending", st.params.Status)
	}
	if st.params.TemplateID != "t1" {
		t.Errorf("template_id = %q, want t1", st.params.TemplateID)
	}
	if st.params.CreatedBy != "node-yen" {
		t.Errorf("created_by = %q, want node-yen", st.params.CreatedBy)
	}
	if st.params.From != "2026-01-01" {
		t.Errorf("from = %q, want 2026-01-01", st.params.From)
	}
	if st.params.To != "2026-12-31" {
		t.Errorf("to = %q, want 2026-12-31", st.params.To)
	}
	// Access filtering params should be populated from the policy client.
	if st.params.UserNodeID != hoaN {
		t.Errorf("userNodeID = %q, want %q", st.params.UserNodeID, hoaN)
	}
	if len(st.params.GroupNodeIDs) != 1 || st.params.GroupNodeIDs[0] != "role-node" {
		t.Errorf("groups = %v, want [role-node]", st.params.GroupNodeIDs)
	}
	if len(st.params.AccessibleScopes) != 1 || st.params.AccessibleScopes[0] != "oa-sales" {
		t.Errorf("scopes = %v, want [oa-sales]", st.params.AccessibleScopes)
	}

	items := body["items"].([]any)
	if len(items) != 1 {
		t.Errorf("items = %d, want 1", len(items))
	}
	if body["total"] != float64(1) {
		t.Errorf("total = %v, want 1", body["total"])
	}
}

func TestSearch_DefaultLimitIs20_MaxIs50(t *testing.T) {
	st := &searchStore{results: []*domain.Request{}, total: 0}
	h := &Handler{svc: domain.NewService(st, searchPolicy{})}

	// Default limit.
	get(t, h.Search, "/api/approval/search?q=test", hoaN, "", "")
	if st.params.Limit != 20 {
		t.Errorf("default limit = %d, want 20", st.params.Limit)
	}

	// Over-max limit capped.
	get(t, h.Search, "/api/approval/search?q=test&limit=100", hoaN, "", "")
	if st.params.Limit != 50 {
		t.Errorf("over-max limit = %d, want 50", st.params.Limit)
	}

	// Exact max.
	get(t, h.Search, "/api/approval/search?q=test&limit=50", hoaN, "", "")
	if st.params.Limit != 50 {
		t.Errorf("exact max limit = %d, want 50", st.params.Limit)
	}
}
