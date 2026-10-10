package rest

import (
	"encoding/json"
	"net"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/labstack/echo/v4"
)

// --- every error has one shape ---

func errBody(t *testing.T, rec *httptest.ResponseRecorder) map[string]any {
	t.Helper()
	var m map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &m); err != nil {
		t.Fatalf("body %q is not JSON: %v", rec.Body.String(), err)
	}
	return m
}

// Sweep the whole route table with an unauthenticated, malformed request: every
// answer that is an error must be {message, code}, whichever handler (or
// middleware) produced it.
func TestEveryErrorAnswerCarriesACode(t *testing.T) {
	a := newAcctApp(t, true, nil)
	a.st.add("u1", "a@example.test")

	routes := a.e.Routes()
	if len(routes) < 10 {
		t.Fatalf("only %d routes registered", len(routes))
	}
	for _, r := range routes {
		path := strings.NewReplacer(":id", "x").Replace(r.Path)
		for _, token := range []string{"", "not-a-token"} {
			req := httptest.NewRequest(r.Method, path, strings.NewReader("{not json"))
			req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
			if token != "" {
				req.Header.Set(echo.HeaderAuthorization, "Bearer "+token)
			}
			rec := httptest.NewRecorder()
			a.e.ServeHTTP(rec, req)
			if rec.Code < 400 {
				continue // a page, a redirect, a public listing
			}
			body := errBody(t, rec)
			if s, _ := body["code"].(string); s == "" {
				t.Errorf("%s %s (token %q): %d %s has no code", r.Method, r.Path, token, rec.Code, rec.Body)
			}
			if s, _ := body["message"].(string); s == "" {
				t.Errorf("%s %s (token %q): %d %s has no message", r.Method, r.Path, token, rec.Code, rec.Body)
			}
		}
	}
}

func TestUnknownRoutesAndMethodsAnswerWithACode(t *testing.T) {
	a := newAcctApp(t, false, nil)
	for _, tc := range []struct {
		method, path string
		status       int
		code         string
	}{
		{http.MethodGet, "/nothing-here", 404, "not_found"},
		// Under /api every path not otherwise served is behind the session
		// check, so an unknown one asks who is asking before it says "no".
		{http.MethodGet, "/api/nothing-here", 401, "session_required"},
		{http.MethodDelete, "/api/auth/providers", 401, "session_required"},
	} {
		rec := httptest.NewRecorder()
		a.e.ServeHTTP(rec, httptest.NewRequest(tc.method, tc.path, nil))
		if body := errBody(t, rec); rec.Code != tc.status || body["code"] != tc.code {
			t.Errorf("%s %s: %d %v, want %d %q", tc.method, tc.path, rec.Code, body, tc.status, tc.code)
		}
	}
}

func TestSessionEndpointsAnswerWithCodes(t *testing.T) {
	a := newAcctApp(t, true, nil)

	rec := a.do(http.MethodPost, "/api/auth/refresh", "", "")
	if body := errBody(t, rec); rec.Code != 401 || body["code"] != "session_required" {
		t.Errorf("refresh without a cookie: %d %v", rec.Code, body)
	}

	req := httptest.NewRequest(http.MethodPost, "/api/auth/refresh", nil)
	req.AddCookie(&http.Cookie{Name: refreshCookieName, Value: "not-a-real-token"})
	rec = httptest.NewRecorder()
	a.e.ServeHTTP(rec, req)
	if body := errBody(t, rec); rec.Code != 401 || body["code"] != "session_invalid" {
		t.Errorf("refresh with a bad cookie: %d %v", rec.Code, body)
	}
}

func TestSwitchTenantRefusalsAnswerWithCodes(t *testing.T) {
	a := newAcctApp(t, false, nil)
	a.st.add("u1", "a@example.test")
	for body, want := range map[string]struct {
		status int
		code   string
	}{
		`{"tenant_id":"not-mine"}`: {403, "access_denied"},
		`{}`:                       {403, "access_denied"},
		`tenant_id=x`:              {400, "invalid_input"},
	} {
		rec := a.do(http.MethodPost, "/api/auth/switch-tenant", a.token("u1"), body)
		if got := errBody(t, rec); rec.Code != want.status || got["code"] != want.code {
			t.Errorf("%s: %d %v, want %d %q", body, rec.Code, got, want.status, want.code)
		}
	}
}

func TestGoogleNotConfiguredAnswersWithACode(t *testing.T) {
	a := newAcctApp(t, false, nil)
	rec := a.do(http.MethodGet, "/api/auth/google/start", "", "")
	if body := errBody(t, rec); rec.Code != 503 || body["code"] != "google_unavailable" {
		t.Errorf("%d %v", rec.Code, body)
	}
}

// --- who is asking ---

func ipOf(t *testing.T, extract echo.IPExtractor, remote string, xff ...string) string {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.RemoteAddr = remote
	for _, v := range xff {
		req.Header.Add(echo.HeaderXForwardedFor, v)
	}
	return extract(req)
}

func TestIPExtractor_TrustsNothingByDefault(t *testing.T) {
	extract, err := NewIPExtractor(nil)
	if err != nil {
		t.Fatal(err)
	}
	// A client that writes its own X-Forwarded-For is still who it is.
	if got := ipOf(t, extract, "203.0.113.9:4000", "198.51.100.1"); got != "203.0.113.9" {
		t.Errorf("ip = %q, want the connection's own address", got)
	}
	// Even from a private address: with nothing configured, no proxy is known.
	if got := ipOf(t, extract, "10.0.0.5:4000", "198.51.100.1"); got != "10.0.0.5" {
		t.Errorf("ip = %q, want the connection's own address", got)
	}
}

func TestIPExtractor_HonoursForwardedForOnlyFromAConfiguredProxy(t *testing.T) {
	extract, err := NewIPExtractor([]string{"10.0.0.0/8", "172.16.0.0/12"})
	if err != nil {
		t.Fatal(err)
	}
	// Behind Traefik: the address it saw is the client.
	if got := ipOf(t, extract, "172.18.0.4:5000", "198.51.100.7"); got != "198.51.100.7" {
		t.Errorf("ip = %q, want the client Traefik saw", got)
	}
	// The client wrote a header of its own before Traefik appended the real one:
	// only the right-most entries a trusted proxy added count.
	if got := ipOf(t, extract, "172.18.0.4:5000", "1.2.3.4, 198.51.100.7"); got != "198.51.100.7" {
		t.Errorf("ip = %q, a forged leading entry must not win", got)
	}
	// Not from a proxy: the header is just text a client typed.
	if got := ipOf(t, extract, "203.0.113.9:4000", "1.2.3.4"); got != "203.0.113.9" {
		t.Errorf("ip = %q, want the connection's address when it is not a proxy", got)
	}
	if got := ipOf(t, extract, "172.18.0.4:5000", "not-an-ip"); got != "172.18.0.4" {
		t.Errorf("ip = %q, an unparseable header is not trusted", got)
	}
}

func TestIPExtractor_RefusesABadRange(t *testing.T) {
	if _, err := NewIPExtractor([]string{"10.0.0.0/8", "not-a-cidr"}); err == nil {
		t.Fatal("a typo in the trusted ranges must stop start-up, not silently trust nothing")
	}
}

// --- how often ---

func limitedApp(t *testing.T, max int, trusted []string) *acctApp {
	t.Helper()
	a := newAcctApp(t, true, nil)
	extract, err := NewIPExtractor(trusted)
	if err != nil {
		t.Fatal(err)
	}
	a.e.IPExtractor = extract
	// A fresh route table so the limit applies to it.
	e := echo.New()
	e.IPExtractor = extract
	h := NewHandler(a.svc)
	h.SetPublicLimit(PublicLimit{Max: max, Window: time.Minute})
	h.RegisterRoutes(e, testJWTSecret)
	a.e = e
	return a
}

func (a *acctApp) postFrom(path, remote string, xff ...string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodPost, path, strings.NewReader(`{"session_id":"nope","code":"000000"}`))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	req.RemoteAddr = remote
	for _, v := range xff {
		req.Header.Add(echo.HeaderXForwardedFor, v)
	}
	rec := httptest.NewRecorder()
	a.e.ServeHTTP(rec, req)
	return rec
}

// uniqueRemote keeps one test's counters from another's, and from a previous
// run's: the limiter keeps them in Redis.
func uniqueRemote() string {
	n := time.Now().UnixNano()
	return net.JoinHostPort("198.51."+strconv.Itoa(int(n>>8)%250)+"."+strconv.Itoa(int(n)%250+1), "4000")
}

func TestPublicPosts_AreLimitedPerAddressWithARetryTime(t *testing.T) {
	a := limitedApp(t, 3, nil)
	remote := uniqueRemote()
	for i := 0; i < 3; i++ {
		if rec := a.postFrom("/api/auth/otp/verify", remote); rec.Code == http.StatusTooManyRequests {
			t.Fatalf("request %d was limited too early", i+1)
		}
	}
	rec := a.postFrom("/api/auth/otp/verify", remote)
	body := errBody(t, rec)
	if rec.Code != http.StatusTooManyRequests || body["code"] != "rate_limited" {
		t.Fatalf("4th: %d %v, want 429 rate_limited", rec.Code, body)
	}
	secs, err := strconv.Atoi(rec.Header().Get("Retry-After"))
	if err != nil || secs < 1 || secs > 60 {
		t.Errorf("Retry-After = %q", rec.Header().Get("Retry-After"))
	}
	if body["retry_after_seconds"] != float64(secs) {
		t.Errorf("body %v does not agree with the header %d", body, secs)
	}
	// Someone else is not affected.
	if rec := a.postFrom("/api/auth/otp/verify", uniqueRemote()); rec.Code == http.StatusTooManyRequests {
		t.Error("another address was limited by this one's requests")
	}
}

func TestPublicPosts_EachRouteCountsItsOwn(t *testing.T) {
	a := limitedApp(t, 2, nil)
	remote := uniqueRemote()
	for i := 0; i < 2; i++ {
		a.postFrom("/api/auth/otp/request", remote)
	}
	if rec := a.postFrom("/api/auth/otp/request", remote); rec.Code != http.StatusTooManyRequests {
		t.Fatalf("otp/request: %d, want 429", rec.Code)
	}
	if rec := a.postFrom("/api/auth/refresh", remote); rec.Code == http.StatusTooManyRequests {
		t.Error("exhausting one route must not lock the same address out of refresh")
	}
}

func TestPublicPosts_AllOfThemAreCovered(t *testing.T) {
	a := limitedApp(t, 1, nil)
	for _, path := range []string{"/api/auth/otp/request", "/api/auth/otp/verify", "/api/auth/refresh", "/api/auth/logout"} {
		remote := uniqueRemote()
		a.postFrom(path, remote)
		if rec := a.postFrom(path, remote); rec.Code != http.StatusTooManyRequests {
			t.Errorf("%s was not limited: %d", path, rec.Code)
		}
	}
	// And the sweep: no public POST under /api/auth escapes the limiter.
	for _, r := range a.e.Routes() {
		if r.Method != http.MethodPost || !strings.HasPrefix(r.Path, "/api/auth/") || r.Path == "/api/auth/switch-tenant" {
			continue
		}
		remote := uniqueRemote()
		a.postFrom(r.Path, remote)
		if rec := a.postFrom(r.Path, remote); rec.Code != http.StatusTooManyRequests {
			t.Errorf("public POST %s is not behind the limiter (%d)", r.Path, rec.Code)
		}
	}
}

func TestPublicPosts_ASpoofedForwardedForDoesNotBuyANewAllowance(t *testing.T) {
	a := limitedApp(t, 2, nil) // nothing trusted
	remote := uniqueRemote()
	for i := 0; i < 2; i++ {
		a.postFrom("/api/auth/otp/verify", remote, "10.9.9."+strconv.Itoa(i))
	}
	rec := a.postFrom("/api/auth/otp/verify", remote, "10.9.9.99")
	if rec.Code != http.StatusTooManyRequests {
		t.Fatalf("a new X-Forwarded-For bought a fresh allowance: %d", rec.Code)
	}
	if rec := a.postFrom("/api/auth/otp/verify", remote, "1.1.1.1, 2.2.2.2"); rec.Code != http.StatusTooManyRequests {
		t.Fatalf("a forged chain bought a fresh allowance: %d", rec.Code)
	}
}

func TestPublicPosts_BehindATrustedProxyEachClientHasItsOwnAllowance(t *testing.T) {
	a := limitedApp(t, 2, []string{"172.16.0.0/12"})
	proxy := "172.18.0." + strconv.Itoa(int(time.Now().UnixNano()%200)+1) + ":5000"
	clientA := "203.0.113." + strconv.Itoa(int(time.Now().UnixNano()%200)+1)
	clientB := "198.51.100." + strconv.Itoa(int(time.Now().UnixNano()%200)+1)

	for i := 0; i < 2; i++ {
		a.postFrom("/api/auth/otp/verify", proxy, clientA)
	}
	if rec := a.postFrom("/api/auth/otp/verify", proxy, clientA); rec.Code != http.StatusTooManyRequests {
		t.Fatalf("client A: %d, want 429", rec.Code)
	}
	if rec := a.postFrom("/api/auth/otp/verify", proxy, clientB); rec.Code == http.StatusTooManyRequests {
		t.Fatal("client B was limited by client A: every client looks like the proxy")
	}
	// A forged leading entry does not let A escape.
	if rec := a.postFrom("/api/auth/otp/verify", proxy, "9.9.9.9, "+clientA); rec.Code != http.StatusTooManyRequests {
		t.Fatalf("a forged leading X-Forwarded-For entry gave client A a new allowance: %d", rec.Code)
	}
}

func TestPublicLimit_IsOffWithoutRedisAndNeverBlocksOnItsFailure(t *testing.T) {
	a := newAcctApp(t, false, nil) // no Redis: the domain cannot count
	for i := 0; i < 5; i++ {
		if rec := a.postFrom("/api/auth/refresh", "203.0.113.5:1"); rec.Code == http.StatusTooManyRequests {
			t.Fatal("limited without a counter")
		}
	}
}
