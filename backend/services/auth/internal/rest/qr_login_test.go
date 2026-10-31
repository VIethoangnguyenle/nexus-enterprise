package rest

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/golang-jwt/jwt/v5"
	"github.com/labstack/echo/v4"

	"ngac-platform/services/auth/internal/auth"
	"ngac-platform/services/auth/internal/domain"
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

// phoneToken is an access token of a live phone session of u in tenant.
func (q qrApp) phoneToken(t *testing.T, u *store.User, tenant string) string {
	tok, _ := q.phoneSession(t, u, tenant)
	return tok
}

func (q qrApp) phoneSession(t *testing.T, u *store.User, tenant string) (token, sessionID string) {
	t.Helper()
	tok, sid, err := auth.GenerateToken(u.ID, u.Username, u.NGACNodeID, tenant)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := q.svc.AttachRefreshToken(context.Background(), domain.RefreshIdentity{
		UserID: u.ID, Username: u.Username, NGACNodeID: u.NGACNodeID, TenantID: tenant, SessionID: sid,
	}); err != nil {
		t.Fatal(err)
	}
	return tok, sid
}

// call is a request with the given bearer token and JSON body.
func (q qrApp) call(path, bearer, body string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodPost, path, strings.NewReader(body))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	req.Header.Set("User-Agent", qrUA)
	if bearer != "" {
		req.Header.Set(echo.HeaderAuthorization, "Bearer "+bearer)
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
// the poll secret the page keeps.
func (q qrApp) create(t *testing.T) (string, string) {
	t.Helper()
	rec := q.call("/api/auth/qr", "", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("create: %d %s", rec.Code, rec.Body)
	}
	body := decode(t, rec)
	token, _ := body["token"].(string)
	secret, _ := body["poll_secret"].(string)
	if token == "" || secret == "" || token == secret || body["expires_in"] != float64(120) {
		t.Fatalf("create body %v", body)
	}
	if len(rec.Result().Cookies()) != 0 {
		t.Error("create sets no cookie: each tab keeps its own code's secret")
	}
	return token, secret
}

func statusBody(token, secret string) string {
	return `{"token":"` + token + `","poll_secret":"` + secret + `"}`
}

func TestQRLoginREST_PhoneApprovesAndTheComputerGetsASession(t *testing.T) {
	q := newQRApp(t)
	token, poll := q.create(t)
	phone := q.phoneToken(t, q.phone, q.tenant)

	rec := q.call("/api/auth/qr/status", "", statusBody(token, poll))
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

	rec = q.call("/api/auth/qr/status", "", statusBody(token, poll))
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

	rec = q.call("/api/auth/qr/status", "", statusBody(token, poll))
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

	// Collected once only.
	if rec := q.call("/api/auth/qr/status", "", statusBody(token, poll)); rec.Code != http.StatusGone || decode(t, rec)["code"] != "qr_unavailable" {
		t.Errorf("second collect: %d %s", rec.Code, rec.Body)
	}
}

func TestQRLoginREST_AnotherBrowserWithTheTokenGetsNothing(t *testing.T) {
	q := newQRApp(t)
	token, secret := q.create(t)
	phone := q.phoneToken(t, q.phone, q.tenant)
	q.call("/api/auth/qr/scan", phone, `{"token":"`+token+`"}`)
	q.call("/api/auth/qr/approve", phone, `{"token":"`+token+`"}`)

	for name, body := range map[string]string{
		"no secret": `{"token":"` + token + `"}`, "wrong secret": statusBody(token, "guess"), "the token as secret": statusBody(token, token),
	} {
		rec := q.call("/api/auth/qr/status", "", body)
		if rec.Code != http.StatusGone {
			t.Errorf("%s: %d %s", name, rec.Code, rec.Body)
		}
		if strings.Contains(rec.Body.String(), "access_token") || cookieNamed(rec, refreshCookieName) != nil {
			t.Errorf("%s: a session leaked", name)
		}
	}
	if rec := q.call("/api/auth/qr/status", "", statusBody(token, secret)); decode(t, rec)["status"] != "approved" {
		t.Errorf("the rightful page must still collect: %s", rec.Body)
	}
}

func TestQRLoginREST_AnEndedPhoneSessionIsRefused(t *testing.T) {
	q := newQRApp(t)
	token, _ := q.create(t)
	phone, sid := q.phoneSession(t, q.phone, q.tenant)
	// Scanned while signed in, then signed out before approving.
	if rec := q.call("/api/auth/qr/scan", phone, `{"token":"`+token+`"}`); rec.Code != http.StatusOK {
		t.Fatalf("scan: %d %s", rec.Code, rec.Body)
	}
	if err := q.svc.EndSession(context.Background(), sid); err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{"/api/auth/qr/approve", "/api/auth/qr/scan"} {
		rec := q.call(path, phone, `{"token":"`+token+`"}`)
		if rec.Code != http.StatusUnauthorized || decode(t, rec)["code"] != "session_invalid" {
			t.Errorf("%s from a signed-out phone: %d %s", path, rec.Code, rec.Body)
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
	if rec := q.call("/api/auth/qr/status", "", statusBody(token, poll)); decode(t, rec)["status"] != "declined" {
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
