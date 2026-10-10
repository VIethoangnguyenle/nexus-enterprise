package rest

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/labstack/echo/v4"

	"ngac-platform/pkg/httputil"
	"ngac-platform/services/auth/internal/domain"
	"ngac-platform/services/auth/internal/googleauth"
)

// --- the code route: proves the address of the signed-in account, issues nothing ---

type deliveringSender struct{ last string }

func (d *deliveringSender) SendCode(_ context.Context, _, _, code string) error {
	d.last = code
	return nil
}
func (*deliveringSender) DeliversToOwner() bool { return true }

func verifyApp(t *testing.T) (*acctApp, *deliveringSender) {
	t.Helper()
	sender := &deliveringSender{}
	a := newAcctApp(t, true, &domain.OTPOptions{Sender: sender})
	return a, sender
}

func TestVerifyEmail_RequestThenConfirm_ProvesTheAddressAndSignsNobodyIn(t *testing.T) {
	a, sender := verifyApp(t)
	id := uniq("ve")
	a.st.add(id, id+"@example.test")
	tok := a.token(id)

	rec := a.do(http.MethodPost, "/api/me/email/verify", tok, `{}`)
	if rec.Code != http.StatusAccepted || decode(t, rec)["expires_in"] != float64(300) {
		t.Fatalf("request: %d %s", rec.Code, rec.Body)
	}
	if sender.last == "" {
		t.Fatal("no code was sent")
	}

	rec = a.do(http.MethodPost, "/api/me/email/verify", tok, `{"code":"`+sender.last+`"}`)
	if rec.Code != http.StatusOK || decode(t, rec)["email_verified"] != true {
		t.Fatalf("confirm: %d %s", rec.Code, rec.Body)
	}
	if !a.st.verified[id] {
		t.Error("the address was not marked verified")
	}
	for _, rr := range []*httptest.ResponseRecorder{rec} {
		body := decode(t, rr)
		if _, has := body["access_token"]; has {
			t.Error("proving an address must not hand out a token")
		}
		if len(rr.Result().Cookies()) != 0 {
			t.Errorf("proving an address must not set a session cookie: %v", rr.Result().Cookies())
		}
	}
}

func TestVerifyEmail_NeedsASessionAndOnlyActsOnIt(t *testing.T) {
	a, sender := verifyApp(t)
	victim, attacker := uniq("victim"), uniq("attacker")
	a.st.add(victim, victim+"@example.test")
	a.st.add(attacker, attacker+"@example.test")

	if rec := a.do(http.MethodPost, "/api/me/email/verify", "", `{}`); rec.Code != http.StatusUnauthorized {
		t.Errorf("no session: %d", rec.Code)
	}
	// The body cannot name another account.
	a.do(http.MethodPost, "/api/me/email/verify", a.token(victim), `{"user_id":"`+attacker+`","email":"x@y.vn"}`)
	code := sender.last
	rec := a.do(http.MethodPost, "/api/me/email/verify", a.token(attacker), `{"code":"`+code+`"}`)
	if rec.Code == http.StatusOK || a.st.verified[attacker] || a.st.verified[victim] {
		t.Fatalf("the victim's code verified something: %d %s", rec.Code, rec.Body)
	}
}

func TestVerifyEmail_Answers(t *testing.T) {
	a, sender := verifyApp(t)
	id := uniq("ans")
	a.st.add(id, id+"@example.test")
	tok := a.token(id)

	rec := a.do(http.MethodPost, "/api/me/email/verify", tok, `{"code":"123456"}`)
	if body := decode(t, rec); rec.Code != http.StatusUnauthorized || body["code"] != "otp_expired" {
		t.Errorf("no code was asked for: %d %v", rec.Code, body)
	}
	a.do(http.MethodPost, "/api/me/email/verify", tok, `{}`)
	wrong := "000000"
	if sender.last == wrong {
		wrong = "111111"
	}
	rec = a.do(http.MethodPost, "/api/me/email/verify", tok, `{"code":"`+wrong+`"}`)
	if body := decode(t, rec); rec.Code != http.StatusUnauthorized || body["code"] != "otp_invalid" || body["attempts_left"] != float64(4) {
		t.Errorf("wrong code: %d %v", rec.Code, body)
	}
	if rec := a.do(http.MethodPost, "/api/me/email/verify", tok, `{"code":""}`); rec.Code != http.StatusUnauthorized {
		t.Errorf("an empty code is a wrong code, not a request for a new one: %d", rec.Code)
	}
	if rec := a.do(http.MethodPost, "/api/me/email/verify", tok, `code=1`); rec.Code != http.StatusBadRequest {
		t.Errorf("not JSON: %d", rec.Code)
	}
}

func TestVerifyEmail_UnavailableWithoutARealSender_AndAlreadyVerified(t *testing.T) {
	a := newAcctApp(t, true, &domain.OTPOptions{FixedCode: "999999"})
	id := uniq("fx")
	a.st.add(id, id+"@example.test")
	rec := a.do(http.MethodPost, "/api/me/email/verify", a.token(id), `{}`)
	if body := decode(t, rec); rec.Code != http.StatusServiceUnavailable || body["code"] != "verification_unavailable" {
		t.Errorf("fixed test code: %d %v", rec.Code, body)
	}

	b, _ := verifyApp(t)
	v := uniq("done")
	b.st.add(v, v+"@example.test")
	b.st.verified[v] = true
	rec = b.do(http.MethodPost, "/api/me/email/verify", b.token(v), `{}`)
	if body := decode(t, rec); rec.Code != http.StatusConflict || body["code"] != "already_verified" {
		t.Errorf("already verified: %d %v", rec.Code, body)
	}
}

func TestVerifyEmail_RefusedRequestsSayWhenToComeBack(t *testing.T) {
	a, _ := verifyApp(t)
	id := uniq("rl")
	a.st.add(id, id+"@example.test")
	for i := 0; i < 5; i++ {
		if rec := a.do(http.MethodPost, "/api/me/email/verify", a.token(id), `{}`); rec.Code != http.StatusAccepted {
			t.Fatalf("request %d: %d", i+1, rec.Code)
		}
	}
	rec := a.do(http.MethodPost, "/api/me/email/verify", a.token(id), `{}`)
	if body := decode(t, rec); rec.Code != http.StatusTooManyRequests || body["code"] != "otp_rate_limited" || rec.Header().Get("Retry-After") == "" {
		t.Errorf("6th: %d %v retry-after=%q", rec.Code, body, rec.Header().Get("Retry-After"))
	}
}

// --- Google: prove the address of this account, never sign in as another ---

type verifySessions struct {
	*fakeSessions
	user        *domain.UserInfo
	verifyErr   error
	verifiedFor *string
	keptSession string
	gotVerify   *domain.ExternalIdentity
}

func (v *verifySessions) GetUserByID(context.Context, string) (*domain.UserInfo, error) {
	return v.user, nil
}

func (v *verifySessions) VerifyEmailWithGoogle(_ context.Context, userID, keep string, id domain.ExternalIdentity) error {
	v.verifiedFor, v.keptSession, v.gotVerify = &userID, keep, &id
	return v.verifyErr
}

func verifyFixture() (*googleFixture, *verifySessions) {
	fx := newGoogleFixture()
	vs := &verifySessions{fakeSessions: fx.sessions, user: &domain.UserInfo{ID: "u-1", Email: "hoa.le@novapay.vn"}}
	fx.h.svc = vs
	return fx, vs
}

func claimsCtx(t *testing.T, method, target string) (echo.Context, *httptest.ResponseRecorder) {
	t.Helper()
	c, rec := request(t, method, target)
	httputil.SetClaims(c, &httputil.Claims{UserID: "u-1", SessionID: "sess-current", NGACNodeID: "n-1"})
	return c, rec
}

func TestGoogleVerify_StartNamesTheAccountAndHintsItsAddress(t *testing.T) {
	fx, _ := verifyFixture()
	c, rec := claimsCtx(t, http.MethodPost, "/api/me/email/verify/google")

	if err := fx.h.StartVerification(c); err != nil {
		t.Fatal(err)
	}
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "accounts.example") {
		t.Fatalf("%d %s", rec.Code, rec.Body)
	}
	ck := cookieNamed(rec, googleStateCookieName)
	if ck == nil || ck.Value != fx.provider.authState {
		t.Fatal("the browser is bound to the flow by the state cookie")
	}
	flow := fx.flows.m[ck.Value]
	if flow.VerifyUserID != "u-1" || flow.VerifySessionID != "sess-current" {
		t.Errorf("flow = %+v: it must remember which account and session asked", flow)
	}
	if fx.provider.authHint != "hoa.le@novapay.vn" {
		t.Errorf("hint = %q, want the account's address", fx.provider.authHint)
	}
}

func TestGoogleVerify_StartRefusals(t *testing.T) {
	t.Run("no session", func(t *testing.T) {
		fx, _ := verifyFixture()
		c, _ := request(t, http.MethodPost, "/api/me/email/verify/google")
		if err := fx.h.StartVerification(c); err == nil {
			t.Fatal("expected the session check to refuse")
		}
		if len(fx.flows.m) != 0 {
			t.Error("a flow was stored for nobody")
		}
	})
	t.Run("already verified", func(t *testing.T) {
		fx, vs := verifyFixture()
		vs.user.EmailVerified = true
		c, _ := claimsCtx(t, http.MethodPost, "/")
		err := fx.h.StartVerification(c)
		var he *echo.HTTPError
		if !errors.As(err, &he) || he.Code != http.StatusConflict {
			t.Fatalf("err = %v, want 409", err)
		}
	})
	t.Run("no address on the account", func(t *testing.T) {
		fx, vs := verifyFixture()
		vs.user.Email = ""
		c, _ := claimsCtx(t, http.MethodPost, "/")
		err := fx.h.StartVerification(c)
		var he *echo.HTTPError
		if !errors.As(err, &he) || he.Code != http.StatusBadRequest {
			t.Fatalf("err = %v, want 400", err)
		}
	})
	t.Run("google not configured", func(t *testing.T) {
		h := newGoogleHandler(GoogleOptions{AppBaseURL: testAppBase}, nil)
		c, _ := claimsCtx(t, http.MethodPost, "/")
		err := h.StartVerification(c)
		var he *echo.HTTPError
		if !errors.As(err, &he) || he.Code != http.StatusServiceUnavailable {
			t.Fatalf("err = %v, want 503", err)
		}
	})
}

// callbackFor starts a verification flow and returns the callback's outcome.
func callbackFor(t *testing.T, fx *googleFixture, query string) *httptest.ResponseRecorder {
	t.Helper()
	c, _ := claimsCtx(t, http.MethodPost, "/")
	rec0 := httptest.NewRecorder()
	c.Response().Writer = rec0
	if err := fx.h.StartVerification(c); err != nil {
		t.Fatal(err)
	}
	state := fx.provider.authState
	c2, rec := request(t, http.MethodGet, "/api/auth/google/callback?state="+state+query, stateCookie(state))
	if err := fx.h.Callback(c2); err != nil {
		t.Fatal(err)
	}
	return rec
}

func TestGoogleVerify_CallbackProvesTheAddressAndSignsNobodyIn(t *testing.T) {
	fx, vs := verifyFixture()
	fx.provider.identity = &googleauth.Identity{Subject: "sub-1", Email: "hoa.le@novapay.vn", EmailVerified: true}

	rec := callbackFor(t, fx, "&code=abc")

	if loc := rec.Header().Get("Location"); loc != testAppBase+"/workspace-select?verified=1" {
		t.Fatalf("Location = %q", loc)
	}
	if vs.verifiedFor == nil || *vs.verifiedFor != "u-1" || vs.keptSession != "sess-current" {
		t.Errorf("proved for %v sparing session %q", vs.verifiedFor, vs.keptSession)
	}
	if fx.sessions.gotIdent != nil {
		t.Error("the Google account was signed in: proving an address is not signing in")
	}
	if fx.sessions.attached != nil {
		t.Error("a session was issued")
	}
	if len(rec.Result().Cookies()) > 1 { // only the cleared state cookie
		t.Errorf("cookies = %v", rec.Result().Cookies())
	}
	if ck := cookieNamed(rec, refreshCookieName); ck != nil {
		t.Error("a refresh cookie was set")
	}
}

func TestGoogleVerify_AnotherGoogleAccountIsRefusedAndNobodyIsSignedInAsThem(t *testing.T) {
	fx, vs := verifyFixture()
	vs.verifyErr = domain.ErrEmailMismatch
	fx.provider.identity = &googleauth.Identity{Subject: "sub-other", Email: "someone.else@gmail.com", EmailVerified: true}

	rec := callbackFor(t, fx, "&code=abc")

	if loc := rec.Header().Get("Location"); loc != testAppBase+"/workspace-select?verify_error=google_mismatch" {
		t.Fatalf("Location = %q", loc)
	}
	if fx.sessions.gotIdent != nil || fx.sessions.attached != nil {
		t.Fatal("the other Google account was signed in")
	}
	if ck := cookieNamed(rec, refreshCookieName); ck != nil && ck.Value != "" {
		t.Error("a session cookie was set")
	}
	if vs.gotVerify == nil || vs.gotVerify.Email != "someone.else@gmail.com" {
		t.Errorf("the domain was not asked to judge the address: %+v", vs.gotVerify)
	}
}

func TestGoogleVerify_FailuresReturnToTheWorkspaceListNotTheLoginPage(t *testing.T) {
	for name, tc := range map[string]struct {
		query string
		setup func(fx *googleFixture, vs *verifySessions)
		want  string
	}{
		"cancelled at Google":  {"&error=access_denied", nil, "google_cancelled"},
		"no code":              {"", nil, "google_failed"},
		"token rejected":       {"&code=abc", func(fx *googleFixture, _ *verifySessions) { fx.provider.err = errors.New("bad token") }, "google_failed"},
		"unverified at Google": {"&code=abc", func(_ *googleFixture, vs *verifySessions) { vs.verifyErr = domain.ErrEmailNotVerified }, "google_unverified"},
		"internal":             {"&code=abc", func(_ *googleFixture, vs *verifySessions) { vs.verifyErr = errors.New("db down") }, "google_error"},
	} {
		t.Run(name, func(t *testing.T) {
			fx, vs := verifyFixture()
			if tc.setup != nil {
				tc.setup(fx, vs)
			}
			rec := callbackFor(t, fx, tc.query)
			if loc := rec.Header().Get("Location"); loc != testAppBase+"/workspace-select?verify_error="+tc.want {
				t.Errorf("Location = %q, want verify_error=%s on the workspace list", loc, tc.want)
			}
			if fx.sessions.attached != nil {
				t.Error("a session was issued")
			}
		})
	}
}

func TestGoogleVerify_APlainSignInFlowIsUnchanged(t *testing.T) {
	fx := newGoogleFixture()
	state := fx.startedFlow(t)
	c, rec := request(t, http.MethodGet, "/api/auth/google/callback?state="+state+"&code=abc", stateCookie(state))
	if err := fx.h.Callback(c); err != nil {
		t.Fatal(err)
	}
	if loc := rec.Header().Get("Location"); loc != testAppBase+googleDonePath {
		t.Fatalf("Location = %q", loc)
	}
	if fx.sessions.attached == nil {
		t.Error("a sign-in flow still issues its session")
	}
}

func TestGoogleVerifyRoute_NeedsASession(t *testing.T) {
	a := newAcctApp(t, true, nil)
	if rec := a.do(http.MethodPost, "/api/me/email/verify/google", "", ""); rec.Code != http.StatusUnauthorized {
		t.Errorf("%d", rec.Code)
	}
	a.st.add("u1", "a@example.test")
	rec := a.do(http.MethodPost, "/api/me/email/verify/google", a.token("u1"), "")
	if body := decode(t, rec); rec.Code != http.StatusServiceUnavailable || body["code"] != "google_unavailable" {
		t.Errorf("google off: %d %v", rec.Code, body)
	}
}
