package rest

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/labstack/echo/v4"
)

func newCtx(t *testing.T) (echo.Context, *httptest.ResponseRecorder) {
	t.Helper()
	e := echo.New()
	req := httptest.NewRequest(http.MethodPost, "/api/auth/refresh", nil)
	rec := httptest.NewRecorder()
	return e.NewContext(req, rec), rec
}

func refreshCookieFrom(t *testing.T, rec *httptest.ResponseRecorder) *http.Cookie {
	t.Helper()
	for _, ck := range rec.Result().Cookies() {
		if ck.Name == refreshCookieName {
			return ck
		}
	}
	t.Fatalf("no %s cookie was set", refreshCookieName)
	return nil
}

// The refresh token is only worth more than a long-lived bearer token if
// JavaScript cannot read it and it is not sent on cross-site requests. These
// attributes are that guarantee, so they are asserted rather than assumed.
func TestRefreshCookie_IsHttpOnlyAndSameSiteStrict(t *testing.T) {
	t.Setenv("APP_ENV", "dev")
	c, rec := newCtx(t)

	setRefreshCookie(c, "the-token")

	ck := refreshCookieFrom(t, rec)
	if !ck.HttpOnly {
		t.Error("refresh cookie must be HttpOnly — an XSS payload could otherwise read it")
	}
	if ck.SameSite != http.SameSiteStrictMode {
		t.Errorf("SameSite = %v, want Strict — this is the CSRF defence for /refresh", ck.SameSite)
	}
	if ck.Path != refreshCookiePath {
		t.Errorf("Path = %q, want %q so the token is not attached to every request", ck.Path, refreshCookiePath)
	}
	if ck.Value != "the-token" {
		t.Errorf("Value = %q, want the token", ck.Value)
	}
}

// Secure must be on everywhere except local development, and the default for an
// unrecognised environment has to be the safe one.
func TestRefreshCookie_SecureOutsideDev(t *testing.T) {
	for _, tc := range []struct {
		env        string
		wantSecure bool
	}{
		{"dev", false},
		{"development", false},
		{"local", false},
		{"test", false},
		{"production", true},
		{"staging", true},
		{"", true}, // unset must not silently downgrade
	} {
		t.Run("APP_ENV="+tc.env, func(t *testing.T) {
			t.Setenv("APP_ENV", tc.env)
			c, rec := newCtx(t)
			setRefreshCookie(c, "x")
			if got := refreshCookieFrom(t, rec).Secure; got != tc.wantSecure {
				t.Errorf("Secure = %v, want %v", got, tc.wantSecure)
			}
		})
	}
}

func TestClearRefreshCookie_ExpiresIt(t *testing.T) {
	t.Setenv("APP_ENV", "dev")
	c, rec := newCtx(t)

	clearRefreshCookie(c)

	ck := refreshCookieFrom(t, rec)
	if ck.MaxAge >= 0 {
		t.Errorf("MaxAge = %d, want negative so the browser drops it", ck.MaxAge)
	}
	if ck.Value != "" {
		t.Errorf("Value = %q, want empty", ck.Value)
	}
	if ck.Path != refreshCookiePath {
		t.Errorf("Path = %q, want %q — a mismatched path leaves the original cookie in place",
			ck.Path, refreshCookiePath)
	}
}

// Without a cookie there is nothing to rotate, and the client must be told to
// sign in rather than handed anything.
func TestRefresh_NoCookieIsUnauthorized(t *testing.T) {
	h := &Handler{}
	c, _ := newCtx(t)

	err := h.Refresh(c)

	he, ok := err.(*echo.HTTPError)
	if !ok {
		t.Fatalf("err = %v (%T), want *echo.HTTPError", err, err)
	}
	if he.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want %d", he.Code, http.StatusUnauthorized)
	}
}

// Logging out with neither claims nor cookie is a no-op that still clears the
// cookie, so a client can always reach a signed-out state.
func TestLogout_WithoutSessionStillClearsCookie(t *testing.T) {
	t.Setenv("APP_ENV", "dev")
	h := &Handler{}
	c, rec := newCtx(t)

	if err := h.Logout(c); err != nil {
		t.Fatalf("logout: %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", rec.Code)
	}
	if ck := refreshCookieFrom(t, rec); ck.MaxAge >= 0 {
		t.Errorf("logout must expire the cookie, MaxAge = %d", ck.MaxAge)
	}
}
