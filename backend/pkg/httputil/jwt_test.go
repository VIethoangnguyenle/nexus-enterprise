package httputil_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/labstack/echo/v4"

	"ngac-platform/pkg/httputil"
)

const testSecret = "test-secret"

func sign(t *testing.T, method jwt.SigningMethod, key any, claims jwt.Claims) string {
	t.Helper()
	tok := jwt.NewWithClaims(method, claims)
	s, err := tok.SignedString(key)
	if err != nil {
		t.Fatalf("sign: %v", err)
	}
	return s
}

// run pushes a request carrying the given Authorization header through the
// middleware and reports whether the handler behind it was reached.
func run(t *testing.T, authHeader string) (reached bool, status int) {
	t.Helper()
	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/protected", nil)
	if authHeader != "" {
		req.Header.Set("Authorization", authHeader)
	}
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	h := httputil.JWTMiddleware(testSecret)(func(echo.Context) error {
		reached = true
		return nil
	})

	err := h(c)
	if err == nil {
		return reached, http.StatusOK
	}
	if he, ok := err.(*echo.HTTPError); ok {
		return reached, he.Code
	}
	return reached, http.StatusInternalServerError
}

func TestJWTMiddleware_AcceptsValidToken(t *testing.T) {
	tok := sign(t, jwt.SigningMethodHS256, []byte(testSecret), &httputil.Claims{
		UserID: "u1",
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Minute)),
		},
	})

	reached, status := run(t, "Bearer "+tok)
	if !reached || status != http.StatusOK {
		t.Errorf("valid token was rejected: reached=%v status=%d", reached, status)
	}
}

func TestJWTMiddleware_RejectsExpiredToken(t *testing.T) {
	tok := sign(t, jwt.SigningMethodHS256, []byte(testSecret), &httputil.Claims{
		UserID: "u1",
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(-time.Minute)),
		},
	})

	if reached, status := run(t, "Bearer "+tok); reached || status != http.StatusUnauthorized {
		t.Errorf("expired token was accepted: reached=%v status=%d", reached, status)
	}
}

// Access tokens are the revocation window, so one without an expiry would never
// close. A token that omits exp must be refused outright.
func TestJWTMiddleware_RejectsTokenWithoutExpiry(t *testing.T) {
	tok := sign(t, jwt.SigningMethodHS256, []byte(testSecret), &httputil.Claims{
		UserID: "u1",
	})

	if reached, status := run(t, "Bearer "+tok); reached || status != http.StatusUnauthorized {
		t.Errorf("token without exp was accepted: reached=%v status=%d", reached, status)
	}
}

// The keyfunc hands back an HMAC secret regardless of what the token's header
// asks for. Pinning the method is what stops that from ever being interpreted
// as anything else.
func TestJWTMiddleware_RejectsAlgNone(t *testing.T) {
	tok := sign(t, jwt.SigningMethodNone, jwt.UnsafeAllowNoneSignatureType, &httputil.Claims{
		UserID: "u1",
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Minute)),
		},
	})

	if reached, status := run(t, "Bearer "+tok); reached || status != http.StatusUnauthorized {
		t.Errorf("alg=none token was accepted: reached=%v status=%d", reached, status)
	}
}

func TestJWTMiddleware_RejectsWrongSecret(t *testing.T) {
	tok := sign(t, jwt.SigningMethodHS256, []byte("some-other-secret"), &httputil.Claims{
		UserID: "u1",
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Minute)),
		},
	})

	if reached, status := run(t, "Bearer "+tok); reached || status != http.StatusUnauthorized {
		t.Errorf("token signed with the wrong secret was accepted: reached=%v status=%d", reached, status)
	}
}

func TestJWTMiddleware_RejectsMissingHeader(t *testing.T) {
	if reached, status := run(t, ""); reached || status != http.StatusUnauthorized {
		t.Errorf("missing header was accepted: reached=%v status=%d", reached, status)
	}
}

// Every 401 a client sees from an authenticated route says why in a code it can
// act on (sign in again, or wait for the refresh), not in English prose.
func TestJWTMiddleware_UnauthorizedAnswersCarryACode(t *testing.T) {
	cases := map[string]struct{ header, code string }{
		"no header":      {"", "session_required"},
		"not a bearer":   {"Basic abc", "session_required"},
		"garbage token":  {"Bearer not-a-jwt", "session_invalid"},
		"expired or bad": {"Bearer " + sign(t, jwt.SigningMethodHS256, []byte("another-secret"), &httputil.Claims{UserID: "u"}), "session_invalid"},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			e := echo.New()
			g := e.Group("", httputil.JWTMiddleware(testSecret))
			g.GET("/x", func(c echo.Context) error { return c.NoContent(http.StatusOK) })
			req := httptest.NewRequest(http.MethodGet, "/x", nil)
			if tc.header != "" {
				req.Header.Set("Authorization", tc.header)
			}
			rec := httptest.NewRecorder()
			e.ServeHTTP(rec, req)

			var body map[string]any
			if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
				t.Fatalf("body %q: %v", rec.Body.String(), err)
			}
			if rec.Code != http.StatusUnauthorized || body["code"] != tc.code || body["message"] == "" {
				t.Errorf("got %d %v, want 401 with code %q and a message", rec.Code, body, tc.code)
			}
		})
	}
}

func TestRequireClaimsAndTenantMiddleware_AnswerWithACode(t *testing.T) {
	e := echo.New()
	e.GET("/me", func(c echo.Context) error {
		_, err := httputil.RequireClaims(c)
		return err
	})
	e.GET("/t", func(c echo.Context) error { return c.NoContent(200) }, httputil.TenantMiddleware())
	e.GET("/t2", func(c echo.Context) error { return c.NoContent(200) }, func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			httputil.SetClaims(c, &httputil.Claims{UserID: "u"})
			return next(c)
		}
	}, httputil.TenantMiddleware())

	for path, want := range map[string]struct {
		status int
		code   string
	}{
		"/me": {401, "session_required"},
		"/t":  {401, "session_required"},
		"/t2": {403, "tenant_required"},
	} {
		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))
		var body map[string]any
		_ = json.Unmarshal(rec.Body.Bytes(), &body)
		if rec.Code != want.status || body["code"] != want.code {
			t.Errorf("%s: %d %v, want %d %q", path, rec.Code, body, want.status, want.code)
		}
	}
}
