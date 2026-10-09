package rest

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/labstack/echo/v4"

	"ngac-platform/services/auth/internal/domain"
	"ngac-platform/services/auth/internal/googleauth"
)

const testAppBase = "http://app.test"

type fakeProvider struct {
	identity *googleauth.Identity
	err      error

	exchanged                         bool
	gotCode, gotVerifier, gotNonce    string
	authState, authNonce, authVerifer string
	authHint                          string
}

func (f *fakeProvider) AuthCodeURL(state, nonce, verifier, loginHint string) string {
	f.authState, f.authNonce, f.authVerifer, f.authHint = state, nonce, verifier, loginHint
	return "https://accounts.example/auth?state=" + url.QueryEscape(state)
}

func (f *fakeProvider) Exchange(_ context.Context, code, verifier, nonce string) (*googleauth.Identity, error) {
	f.exchanged = true
	f.gotCode, f.gotVerifier, f.gotNonce = code, verifier, nonce
	return f.identity, f.err
}

type fakeFlows struct{ m map[string]googleauth.Flow }

func (f *fakeFlows) Save(_ context.Context, state string, fl googleauth.Flow) error {
	f.m[state] = fl
	return nil
}

func (f *fakeFlows) Take(_ context.Context, state string) (googleauth.Flow, error) {
	fl, ok := f.m[state]
	if !ok {
		return googleauth.Flow{}, googleauth.ErrFlowNotFound
	}
	delete(f.m, state)
	return fl, nil
}

type fakeSessions struct {
	result   *domain.SigninResult
	err      error
	gotIdent *domain.ExternalIdentity
	attached *domain.RefreshIdentity
}

func (f *fakeSessions) SignInWithGoogle(_ context.Context, id domain.ExternalIdentity) (*domain.SigninResult, error) {
	f.gotIdent = &id
	return f.result, f.err
}

func (f *fakeSessions) AttachRefreshToken(_ context.Context, id domain.RefreshIdentity) (string, error) {
	f.attached = &id
	return "refresh-token-value", nil
}

type googleFixture struct {
	h        *googleHandler
	provider *fakeProvider
	flows    *fakeFlows
	sessions *fakeSessions
}

func newGoogleFixture() *googleFixture {
	p := &fakeProvider{identity: &googleauth.Identity{
		Subject: "sub-1", Email: "alice@acme.com", EmailVerified: true, HostedDomain: "acme.com", Name: "Alice",
	}}
	fl := &fakeFlows{m: map[string]googleauth.Flow{}}
	s := &fakeSessions{result: &domain.SigninResult{
		Token: "access-token-value", SessionID: "sess-1", UserID: "u-1", Username: "alice.acme",
		NGACNodeID: "n-1", DefaultTenantID: "t-1",
	}}
	return &googleFixture{
		h:        newGoogleHandler(GoogleOptions{Provider: p, Flows: fl, AppBaseURL: testAppBase}, s),
		provider: p, flows: fl, sessions: s,
	}
}

func request(t *testing.T, method, target string, cookies ...*http.Cookie) (echo.Context, *httptest.ResponseRecorder) {
	t.Helper()
	t.Setenv("APP_ENV", "dev")
	req := httptest.NewRequest(method, target, nil)
	for _, c := range cookies {
		req.AddCookie(c)
	}
	rec := httptest.NewRecorder()
	return echo.New().NewContext(req, rec), rec
}

func cookieNamed(rec *httptest.ResponseRecorder, name string) *http.Cookie {
	for _, c := range rec.Result().Cookies() {
		if c.Name == name {
			return c
		}
	}
	return nil
}

func stateCookie(v string) *http.Cookie { return &http.Cookie{Name: googleStateCookieName, Value: v} }

// startedFlow runs /start and returns the state it bound to the browser.
func (fx *googleFixture) startedFlow(t *testing.T) string {
	t.Helper()
	c, rec := request(t, http.MethodGet, "/api/auth/google/start")
	if err := fx.h.Start(c); err != nil {
		t.Fatalf("start: %v", err)
	}
	ck := cookieNamed(rec, googleStateCookieName)
	if ck == nil {
		t.Fatal("start did not set the state cookie")
	}
	return ck.Value
}

func assertLoginError(t *testing.T, rec *httptest.ResponseRecorder, code string) {
	t.Helper()
	if rec.Code != http.StatusFound {
		t.Fatalf("status = %d, want 302", rec.Code)
	}
	if loc := rec.Header().Get("Location"); loc != testAppBase+"/login?error="+code {
		t.Fatalf("Location = %q, want login error %q", loc, code)
	}
	if ck := cookieNamed(rec, refreshCookieName); ck != nil && ck.Value != "" {
		t.Error("a rejected callback must not issue a session")
	}
}

func TestProviders_ReportsWhetherGoogleIsConfigured(t *testing.T) {
	for _, tc := range []struct {
		name string
		h    *googleHandler
		want bool
	}{
		{"disabled", newGoogleHandler(GoogleOptions{}, nil), false},
		{"enabled", newGoogleFixture().h, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c, rec := request(t, http.MethodGet, "/api/auth/providers")
			if err := tc.h.Providers(c); err != nil {
				t.Fatal(err)
			}
			var body map[string]bool
			if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
				t.Fatal(err)
			}
			if body["google"] != tc.want {
				t.Errorf("google = %v, want %v", body["google"], tc.want)
			}
		})
	}
}

func TestStart_DisabledIs503(t *testing.T) {
	h := newGoogleHandler(GoogleOptions{AppBaseURL: testAppBase}, nil)
	c, _ := request(t, http.MethodGet, "/api/auth/google/start")
	err := h.Start(c)
	var he *echo.HTTPError
	if !errors.As(err, &he) || he.Code != http.StatusServiceUnavailable {
		t.Fatalf("err = %v, want 503", err)
	}
}

func TestStart_RedirectsToGoogleAndBindsStateToBrowser(t *testing.T) {
	fx := newGoogleFixture()
	c, rec := request(t, http.MethodGet, "/api/auth/google/start?login_hint=alice%40acme.com")

	if err := fx.h.Start(c); err != nil {
		t.Fatal(err)
	}
	if rec.Code != http.StatusFound || !strings.HasPrefix(rec.Header().Get("Location"), "https://accounts.example/auth") {
		t.Fatalf("status = %d Location = %q", rec.Code, rec.Header().Get("Location"))
	}
	if cc := rec.Header().Get("Cache-Control"); !strings.Contains(cc, "no-store") {
		t.Errorf("Cache-Control = %q, want no-store", cc)
	}

	ck := cookieNamed(rec, googleStateCookieName)
	if ck == nil {
		t.Fatal("no state cookie")
	}
	if !ck.HttpOnly || ck.SameSite != http.SameSiteLaxMode {
		t.Errorf("state cookie HttpOnly=%v SameSite=%v, want HttpOnly + Lax (Lax so Google's redirect carries it)", ck.HttpOnly, ck.SameSite)
	}
	if ck.Path != googleCookiePath {
		t.Errorf("Path = %q, want %q", ck.Path, googleCookiePath)
	}
	if ck.MaxAge <= 0 || ck.MaxAge > int(googleauth.FlowTTL.Seconds()) {
		t.Errorf("MaxAge = %d, want short-lived (<= flow TTL)", ck.MaxAge)
	}
	if ck.Value != fx.provider.authState {
		t.Error("cookie must carry the same state that is sent to Google")
	}
	flow, ok := fx.flows.m[ck.Value]
	if !ok {
		t.Fatal("flow was not stored under the state")
	}
	if flow.Nonce != fx.provider.authNonce || flow.Verifier != fx.provider.authVerifer {
		t.Error("stored nonce/verifier must be the ones used for the authorization URL")
	}
	if fx.provider.authHint != "alice@acme.com" {
		t.Errorf("login_hint = %q", fx.provider.authHint)
	}
}

func TestStart_DropsMalformedLoginHint(t *testing.T) {
	fx := newGoogleFixture()
	c, _ := request(t, http.MethodGet, "/api/auth/google/start?login_hint="+url.QueryEscape("not an email"))
	if err := fx.h.Start(c); err != nil {
		t.Fatal(err)
	}
	if fx.provider.authHint != "" {
		t.Errorf("login_hint = %q, want dropped", fx.provider.authHint)
	}
}

func TestCallback_StateMismatchIsRejected(t *testing.T) {
	fx := newGoogleFixture()
	state := fx.startedFlow(t)

	c, rec := request(t, http.MethodGet, "/api/auth/google/callback?code=c&state=attacker-state", stateCookie(state))
	if err := fx.h.Callback(c); err != nil {
		t.Fatal(err)
	}
	assertLoginError(t, rec, "google_state")
	if fx.provider.exchanged {
		t.Error("the code must not be exchanged when state does not match")
	}
}

func TestCallback_MissingStateCookieIsRejected(t *testing.T) {
	fx := newGoogleFixture()
	state := fx.startedFlow(t)

	// Login CSRF: the victim's browser never started this flow.
	c, rec := request(t, http.MethodGet, "/api/auth/google/callback?code=c&state="+url.QueryEscape(state))
	if err := fx.h.Callback(c); err != nil {
		t.Fatal(err)
	}
	assertLoginError(t, rec, "google_state")
	if fx.provider.exchanged {
		t.Error("the code must not be exchanged without the browser's state cookie")
	}
}

func TestCallback_ReplayedStateIsRejected(t *testing.T) {
	fx := newGoogleFixture()
	state := fx.startedFlow(t)
	target := "/api/auth/google/callback?code=c&state=" + url.QueryEscape(state)

	c, _ := request(t, http.MethodGet, target, stateCookie(state))
	if err := fx.h.Callback(c); err != nil {
		t.Fatal(err)
	}
	fx.provider.exchanged = false

	c, rec := request(t, http.MethodGet, target, stateCookie(state))
	if err := fx.h.Callback(c); err != nil {
		t.Fatal(err)
	}
	assertLoginError(t, rec, "google_state")
	if fx.provider.exchanged {
		t.Error("a state must work only once")
	}
}

func TestCallback_UserCancelledAtGoogle(t *testing.T) {
	fx := newGoogleFixture()
	state := fx.startedFlow(t)
	c, rec := request(t, http.MethodGet, "/api/auth/google/callback?error=access_denied&state="+url.QueryEscape(state), stateCookie(state))
	if err := fx.h.Callback(c); err != nil {
		t.Fatal(err)
	}
	assertLoginError(t, rec, "google_cancelled")
}

func TestCallback_TokenRejections(t *testing.T) {
	for _, tc := range []struct {
		name string
		err  error
		code string
	}{
		{"invalid id token", googleauth.ErrInvalidIDToken, "google_failed"},
		{"exchange failed", googleauth.ErrExchange, "google_failed"},
		{"email not verified", googleauth.ErrEmailNotVerified, "google_unverified"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			fx := newGoogleFixture()
			fx.provider.identity, fx.provider.err = nil, tc.err
			state := fx.startedFlow(t)

			c, rec := request(t, http.MethodGet, "/api/auth/google/callback?code=c&state="+url.QueryEscape(state), stateCookie(state))
			if err := fx.h.Callback(c); err != nil {
				t.Fatal(err)
			}
			assertLoginError(t, rec, tc.code)
			if fx.sessions.gotIdent != nil {
				t.Error("no account work may happen for a rejected token")
			}
		})
	}
}

func TestCallback_DomainRejections(t *testing.T) {
	for _, tc := range []struct {
		err  error
		code string
	}{
		{domain.ErrIdentityConflict, "google_conflict"},
		{domain.ErrEmailNotVerified, "google_unverified"},
		{errors.New("workspace service down"), "google_error"},
	} {
		fx := newGoogleFixture()
		fx.sessions.result, fx.sessions.err = nil, tc.err
		state := fx.startedFlow(t)

		c, rec := request(t, http.MethodGet, "/api/auth/google/callback?code=c&state="+url.QueryEscape(state), stateCookie(state))
		if err := fx.h.Callback(c); err != nil {
			t.Fatal(err)
		}
		assertLoginError(t, rec, tc.code)
	}
}

func TestCallback_DisabledRedirectsWithError(t *testing.T) {
	h := newGoogleHandler(GoogleOptions{AppBaseURL: testAppBase}, nil)
	c, rec := request(t, http.MethodGet, "/api/auth/google/callback?code=c&state=s", stateCookie("s"))
	if err := h.Callback(c); err != nil {
		t.Fatal(err)
	}
	assertLoginError(t, rec, "google_unavailable")
}

func TestCallback_SuccessIssuesSessionAndRedirectsWithoutTokens(t *testing.T) {
	fx := newGoogleFixture()
	state := fx.startedFlow(t)
	flow := fx.flows.m[state]

	c, rec := request(t, http.MethodGet, "/api/auth/google/callback?code=the-code&state="+url.QueryEscape(state), stateCookie(state))
	if err := fx.h.Callback(c); err != nil {
		t.Fatal(err)
	}

	if rec.Code != http.StatusFound {
		t.Fatalf("status = %d", rec.Code)
	}
	loc := rec.Header().Get("Location")
	if loc != testAppBase+"/auth/google/done" {
		t.Errorf("Location = %q", loc)
	}
	if strings.Contains(loc, "access-token-value") || strings.Contains(loc, "refresh-token-value") {
		t.Fatal("tokens must never appear in the redirect URL")
	}

	if fx.provider.gotCode != "the-code" || fx.provider.gotVerifier != flow.Verifier || fx.provider.gotNonce != flow.Nonce {
		t.Error("exchange must use the code with the flow's own PKCE verifier and nonce")
	}
	id := fx.sessions.gotIdent
	if id == nil || id.Subject != "sub-1" || id.HostedDomain != "acme.com" || !id.EmailVerified || id.Provider != domain.ProviderGoogle {
		t.Errorf("identity passed to domain = %+v", id)
	}

	// Same session as the OTP/signin flows: an httpOnly Strict refresh cookie
	// bound to the access token's session and default tenant.
	ref := cookieNamed(rec, refreshCookieName)
	if ref == nil || ref.Value != "refresh-token-value" || !ref.HttpOnly || ref.SameSite != http.SameSiteStrictMode || ref.Path != refreshCookiePath {
		t.Fatalf("refresh cookie = %+v", ref)
	}
	if a := fx.sessions.attached; a == nil || a.SessionID != "sess-1" || a.TenantID != "t-1" || a.UserID != "u-1" {
		t.Errorf("refresh identity = %+v", a)
	}

	if ck := cookieNamed(rec, googleStateCookieName); ck == nil || ck.MaxAge >= 0 {
		t.Error("the state cookie must be cleared once used")
	}
	if strings.Contains(rec.Body.String(), "access-token-value") {
		t.Error("the access token must not be in the response; the SPA obtains it via /refresh")
	}
}
