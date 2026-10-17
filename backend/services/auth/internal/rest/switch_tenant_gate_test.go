package rest

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"ngac-platform/services/auth/internal/auth"
	"ngac-platform/services/auth/internal/store"
)

func (a *refreshApp) switchTo(t *testing.T, id, tenant string) *httptest.ResponseRecorder {
	t.Helper()
	u := a.st.users[id]
	tok, _, err := auth.GenerateToken(u.ID, u.Username, u.NGACNodeID, "ws-1")
	if err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodPost, "/api/auth/switch-tenant", strings.NewReader(`{"tenant_id":"`+tenant+`"}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+tok)
	rec := httptest.NewRecorder()
	a.e.ServeHTTP(rec, req)
	return rec
}

// Switching asks the graph. A tenant the graph does not hold the person in is 403 (the
// policy here holds people in ws-1 only, though their rows also list ws-2); an
// outage is 503, which says nothing about their right and issues no token.
func TestSwitchTenant_FollowsTheGraphAndAnOutageIs503(t *testing.T) {
	a := newRefreshApp(t)
	id := uniq("sw")
	a.sessionIn(t, id)
	a.st.members[id] = append(a.st.members[id], store.WorkspaceSummary{ID: "ws-2", Name: "Drifted", Role: "member"})

	rec := a.switchTo(t, id, "ws-1")
	if rec.Code != http.StatusOK || decode(t, rec)["access_token"] == "" {
		t.Fatalf("a tenant the graph holds: status %d (%s), want 200 with a token", rec.Code, rec.Body)
	}

	if rec = a.switchTo(t, id, "ws-2"); rec.Code != http.StatusForbidden {
		t.Errorf("a tenant only the row lists: status %d (%s), want 403", rec.Code, rec.Body)
	}

	a.policy.down.Store(true)
	rec = a.switchTo(t, id, "ws-1")
	if rec.Code != http.StatusServiceUnavailable || decode(t, rec)["code"] != "unavailable" {
		t.Errorf("policy down: status %d (%s), want 503 unavailable", rec.Code, rec.Body)
	}
	if strings.Contains(rec.Body.String(), "access_token") {
		t.Error("a token was issued without an answer from the graph")
	}
}

func (a *refreshApp) listAs(t *testing.T, id, tenant string) *httptest.ResponseRecorder {
	t.Helper()
	u := a.st.users[id]
	tok, _, err := auth.GenerateToken(u.ID, u.Username, u.NGACNodeID, tenant)
	if err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodGet, "/api/me/workspaces", nil)
	req.Header.Set("Authorization", "Bearer "+tok)
	rec := httptest.NewRecorder()
	a.e.ServeHTTP(rec, req)
	return rec
}

// With the policy down the switcher shows the workspace in the caller's token; a caller
// whose token names none gets 503, so the screen offers a retry rather than an empty picker.
func TestMyWorkspaces_AnOutageShowsTheCurrentWorkspaceOr503(t *testing.T) {
	a := newRefreshApp(t)
	id := uniq("ls")
	a.sessionIn(t, id)
	a.st.members[id] = append(a.st.members[id], store.WorkspaceSummary{ID: "ws-2", Name: "Other", Role: "member"})

	rec := a.listAs(t, id, "ws-1")
	if rec.Code != http.StatusOK || strings.Count(rec.Body.String(), `"id"`) != 1 {
		t.Fatalf("policy up: status %d (%s), want only the workspace the graph holds", rec.Code, rec.Body)
	}

	a.policy.down.Store(true)
	rec = a.listAs(t, id, "ws-1")
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"ws-1"`) || strings.Contains(rec.Body.String(), `"ws-2"`) {
		t.Errorf("policy down, token in ws-1: status %d (%s), want only ws-1", rec.Code, rec.Body)
	}
	rec = a.listAs(t, id, "")
	if rec.Code != http.StatusServiceUnavailable || decode(t, rec)["code"] != "unavailable" {
		t.Errorf("policy down, tenant-less token: status %d (%s), want 503 unavailable", rec.Code, rec.Body)
	}
}
