package rest

import (
	"net/http"
	"testing"

	"ngac-platform/services/auth/internal/auth"
	"ngac-platform/services/auth/internal/store"
)

// titleApp is a person who is a member of workspaces w1 and w2, and not of w3.
func titleApp(t *testing.T) *acctApp {
	t.Helper()
	a := newAcctApp(t, false, nil)
	a.st.add("u1", "a@example.test").Title = "Mặc định"
	a.st.members["u1"] = []store.WorkspaceSummary{{ID: "w1", Name: "Khối 1", Role: "member"}, {ID: "w2", Name: "Khối 2", Role: "member"}}
	return a
}

func meTitle(t *testing.T, a *acctApp, path string) string {
	t.Helper()
	rec := a.do(http.MethodGet, path, a.token("u1"), "")
	if rec.Code != http.StatusOK {
		t.Fatalf("GET %s: %d %s", path, rec.Code, rec.Body)
	}
	return decode(t, rec)["user"].(map[string]any)["title"].(string)
}

// PATCH /api/me/profile?workspace=<id> writes the title of that workspace; GET
// /api/me?workspace=<id> reads it back, and another workspace keeps the default.
func TestProfile_TitleIsWrittenForTheWorkspaceNamed(t *testing.T) {
	a := titleApp(t)
	rec := a.do(http.MethodPatch, "/api/me/profile?workspace=w1", a.token("u1"), `{"title":"Kế toán"}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("%d %s", rec.Code, rec.Body)
	}
	if got := a.st.changes["u1"].WorkspaceID; got != "w1" {
		t.Fatalf("store got workspace %q, want w1", got)
	}
	if got := meTitle(t, a, "/api/me?workspace=w1"); got != "Kế toán" {
		t.Errorf("w1 title = %q", got)
	}
	if got := meTitle(t, a, "/api/me?workspace=w2"); got != "Mặc định" {
		t.Errorf("w2 title = %q, want the account default", got)
	}
}

// Without ?workspace= nothing is scoped, whatever workspace the token is for:
// the write is the account's default (the profile step of sign-up sends none).
func TestProfile_NoWorkspaceNamedWritesTheAccountDefault(t *testing.T) {
	a := titleApp(t)
	u := a.st.users["u1"]
	tok, _, err := auth.GenerateToken(u.ID, u.Username, u.NGACNodeID, "w1")
	if err != nil {
		t.Fatal(err)
	}
	if rec := a.do(http.MethodPatch, "/api/me/profile", tok, `{"title":"Chuyên viên"}`); rec.Code != http.StatusOK {
		t.Fatalf("%d %s", rec.Code, rec.Body)
	}
	if got := a.st.changes["u1"].WorkspaceID; got != "" {
		t.Errorf("store got workspace %q, want none", got)
	}
}

// A workspace the caller is not an active member of is refused with 403, and
// nothing of the request is written.
func TestProfile_TitleForANonMemberWorkspaceIsForbidden(t *testing.T) {
	t.Run("another workspace", func(t *testing.T) {
		a := titleApp(t)
		rec := a.do(http.MethodPatch, "/api/me/profile?workspace=w3", a.token("u1"), `{"title":"Giám đốc","display_name":"Đổi tên"}`)
		if rec.Code != http.StatusForbidden {
			t.Fatalf("status %d %s, want 403", rec.Code, rec.Body)
		}
		if len(a.st.changes) != 0 || a.st.users["u1"].DisplayName != "u-u1" {
			t.Errorf("a refused request wrote %+v", a.st.changes)
		}
	})
	t.Run("a disabled membership", func(t *testing.T) {
		a := titleApp(t)
		a.st.status["u1"] = "disabled"
		rec := a.do(http.MethodPatch, "/api/me/profile?workspace=w1", a.token("u1"), `{"title":"Giám đốc"}`)
		if rec.Code != http.StatusForbidden {
			t.Fatalf("status %d %s, want 403", rec.Code, rec.Body)
		}
		if len(a.st.changes) != 0 {
			t.Errorf("a refused request wrote %+v", a.st.changes)
		}
	})
}
