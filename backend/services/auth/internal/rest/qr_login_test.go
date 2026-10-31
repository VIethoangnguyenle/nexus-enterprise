package rest

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/golang-jwt/jwt/v5"
	"github.com/labstack/echo/v4"

	"ngac-platform/services/auth/internal/auth"
	"ngac-platform/services/auth/internal/store"
)

const qrUA = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/130.0.0.0 Safari/537.36"

// qrApp is the auth routes with Redis and one person who is an active member of NovaPay.
type qrApp struct {
	*acctApp
	phone  *store.User
	tenant string
}

func newQRApp(t *testing.T) qrApp {
	t.Helper()
	a := newAcctApp(t, true, nil)
	id := uniq("qr")
	u := a.st.add(id, id+"@example.test")
	u.DisplayName = "Lê Thị Hoa"
	tenant := uniq("ws")
	a.st.members[id] = []store.WorkspaceSummary{{ID: tenant, Name: "NovaPay", Role: "member"}}
	return qrApp{acctApp: a, phone: u, tenant: tenant}
}

func (q qrApp) phoneToken(t *testing.T, u *store.User, tenant string) string {
	t.Helper()
	tok, _, err := auth.GenerateToken(u.ID, u.Username, u.NGACNodeID, tenant)
	if err != nil {
		t.Fatal(err)
	}
	return tok
}

// call is a request carrying the given cookies (as a browser would) and bearer token.
func (q qrApp) call(path, bearer, body string, cookies ...*http.Cookie) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodPost, path, strings.NewReader(body))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	req.Header.Set("User-Agent", qrUA)
	if bearer != "" {
		req.Header.Set(echo.HeaderAuthorization, "Bearer "+bearer)
	}
	for _, c := range cookies {
		req.AddCookie(c)
	}
	rec := httptest.NewRecorder()
	q.e.ServeHTTP(rec, req)
	return rec
}

// tenantOfToken is the tenant an access token was issued for.
func tenantOfToken(t *testing.T, token string) string {
	t.Helper()
	claims := &auth.Claims{}
	if _, err := jwt.ParseWithClaims(token, claims, func(*jwt.Token) (any, error) { return []byte(testJWTSecret), nil }); err != nil {
		t.Fatalf("parse token: %v", err)
	}
	return claims.TenantID
}

// create asks for a code the way the sign-in page does and returns its token and
// the browser-binding cookie.
func (q qrApp) create(t *testing.T) (string, *http.Cookie) {
	t.Helper()
	rec := q.call("/api/auth/qr", "", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("create: %d %s", rec.Code, rec.Body)
	}
	body := decode(t, rec)
	token, _ := body["token"].(string)
	if token == "" || body["expires_in"] != float64(120) {
		t.Fatalf("create body %v", body)
	}
	c := cookieNamed(rec, qrPollCookieName)
	if c == nil || c.Value == "" {
		t.Fatal("create must bind the code to this browser with a cookie")
	}
	if !c.HttpOnly || c.SameSite != http.SameSiteStrictMode || c.Path != qrCookiePath {
		t.Errorf("poll cookie must be httpOnly, SameSite=Strict, scoped to %s: %+v", qrCookiePath, c)
	}
	if strings.Contains(rec.Body.String(), c.Value) {
		t.Error("the poll secret must never be in a response body")
	}
	return token, c
}

func TestQRLoginREST_PhoneApprovesAndTheComputerGetsASession(t *testing.T) {
	q := newQRApp(t)
	token, poll := q.create(t)
	phone := q.phoneToken(t, q.phone, q.tenant)

	rec := q.call("/api/auth/qr/status", "", `{"token":"`+token+`"}`, poll)
	if rec.Code != http.StatusOK || decode(t, rec)["status"] != "pending" {
		t.Fatalf("pending: %d %s", rec.Code, rec.Body)
	}

	rec = q.call("/api/auth/qr/scan", phone, `{"token":"`+token+`"}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("scan: %d %s", rec.Code, rec.Body)
	}
	scan := decode(t, rec)
	if scan["browser"] != "Chrome" || scan["os"] != "Windows" || scan["tenant_name"] != "NovaPay" || scan["created_at"] == nil {
		t.Errorf("scan body %v", scan)
	}

	rec = q.call("/api/auth/qr/status", "", `{"token":"`+token+`"}`, poll)
	st := decode(t, rec)
	scanner, _ := st["scanner"].(map[string]any)
	if st["status"] != "scanned" || scanner["display_name"] != "Lê Thị Hoa" {
		t.Fatalf("scanned: %v", st)
	}
	if _, leaks := scanner["id"]; leaks {
		t.Error("the computer must not learn the scanner's id")
	}

	if rec := q.call("/api/auth/qr/approve", phone, `{"token":"`+token+`"}`); rec.Code != http.StatusNoContent {
		t.Fatalf("approve: %d %s", rec.Code, rec.Body)
	}

	rec = q.call("/api/auth/qr/status", "", `{"token":"`+token+`"}`, poll)
	if rec.Code != http.StatusOK {
		t.Fatalf("collect: %d %s", rec.Code, rec.Body)
	}
	body := decode(t, rec)
	access, _ := body["access_token"].(string)
	user, _ := body["user"].(map[string]any)
	if body["status"] != "approved" || access == "" || user["id"] != q.phone.ID {
		t.Fatalf("approved body %v", body)
	}
	if got := tenantOfToken(t, access); got != q.tenant {
		t.Errorf("computer's token is for %q, want the phone's tenant %q", got, q.tenant)
	}
	if c := cookieNamed(rec, refreshCookieName); c == nil || c.Value == "" {
		t.Error("the computer must get a refresh cookie like every other sign-in")
	}
	if c := cookieNamed(rec, qrPollCookieName); c == nil || c.MaxAge >= 0 {
		t.Error("the poll cookie must be cleared once the session is collected")
	}

	// Collected once only.
	if rec := q.call("/api/auth/qr/status", "", `{"token":"`+token+`"}`, poll); rec.Code != http.StatusGone || decode(t, rec)["code"] != "qr_unavailable" {
		t.Errorf("second collect: %d %s", rec.Code, rec.Body)
	}
}

func TestQRLoginREST_AnotherBrowserWithTheTokenGetsNothing(t *testing.T) {
	q := newQRApp(t)
	token, _ := q.create(t)
	phone := q.phoneToken(t, q.phone, q.tenant)
	q.call("/api/auth/qr/scan", phone, `{"token":"`+token+`"}`)
	q.call("/api/auth/qr/approve", phone, `{"token":"`+token+`"}`)

	forged := &http.Cookie{Name: qrPollCookieName, Value: "guess"}
	for name, cookies := range map[string][]*http.Cookie{"no cookie": nil, "wrong cookie": {forged}} {
		rec := q.call("/api/auth/qr/status", "", `{"token":"`+token+`"}`, cookies...)
		if rec.Code != http.StatusGone {
			t.Errorf("%s: %d %s", name, rec.Code, rec.Body)
		}
		if strings.Contains(rec.Body.String(), "access_token") || cookieNamed(rec, refreshCookieName) != nil {
			t.Errorf("%s: a session leaked", name)
		}
	}
}

func TestQRLoginREST_PhoneRoutesNeedASignedInMember(t *testing.T) {
	q := newQRApp(t)
	token, _ := q.create(t)
	for _, path := range []string{"/api/auth/qr/scan", "/api/auth/qr/approve", "/api/auth/qr/decline"} {
		if rec := q.call(path, "", `{"token":"`+token+`"}`); rec.Code != http.StatusUnauthorized {
			t.Errorf("%s without a session: %d", path, rec.Code)
		}
	}
	strangerID := uniq("stranger")
	stranger := q.st.add(strangerID, strangerID+"@example.test")
	rec := q.call("/api/auth/qr/scan", q.phoneToken(t, stranger, q.tenant), `{"token":"`+token+`"}`)
	if rec.Code != http.StatusForbidden || decode(t, rec)["code"] != "access_denied" {
		t.Errorf("scan by a non-member: %d %s", rec.Code, rec.Body)
	}
}

func TestQRLoginREST_DeclineAndUnknownCodes(t *testing.T) {
	q := newQRApp(t)
	token, poll := q.create(t)
	phone := q.phoneToken(t, q.phone, q.tenant)
	q.call("/api/auth/qr/scan", phone, `{"token":"`+token+`"}`)
	if rec := q.call("/api/auth/qr/decline", phone, `{"token":"`+token+`"}`); rec.Code != http.StatusNoContent {
		t.Fatalf("decline: %d %s", rec.Code, rec.Body)
	}
	if rec := q.call("/api/auth/qr/status", "", `{"token":"`+token+`"}`, poll); decode(t, rec)["status"] != "declined" {
		t.Errorf("after decline: %s", rec.Body)
	}

	for _, path := range []string{"/api/auth/qr/scan", "/api/auth/qr/approve"} {
		rec := q.call(path, phone, `{"token":"never-issued"}`)
		if rec.Code != http.StatusGone || decode(t, rec)["code"] != "qr_unavailable" {
			t.Errorf("%s unknown token: %d %s", path, rec.Code, rec.Body)
		}
	}
	if rec := q.call("/api/auth/qr/scan", phone, `not json`); rec.Code != http.StatusBadRequest {
		t.Errorf("bad body: %d", rec.Code)
	}
}
