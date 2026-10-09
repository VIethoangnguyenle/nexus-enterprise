package googleauth

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"math/big"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

const (
	testClientID = "client-123.apps.googleusercontent.com"
	testKID      = "test-key-1"
)

// fakeGoogle is a local stand-in for Google's JWKS and token endpoints. Tests
// never talk to Google.
type fakeGoogle struct {
	t      *testing.T
	key    *rsa.PrivateKey
	server *httptest.Server

	mu sync.Mutex
	// idToken is what the token endpoint returns for the next exchange.
	idToken string
	// lastVerifier is the PKCE code_verifier the token endpoint received.
	lastVerifier string
	lastCode     string
	jwksHits     int
}

func newFakeGoogle(t *testing.T) *fakeGoogle {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}
	f := &fakeGoogle{t: t, key: key}
	mux := http.NewServeMux()
	mux.HandleFunc("/certs", func(w http.ResponseWriter, _ *http.Request) {
		f.mu.Lock()
		f.jwksHits++
		f.mu.Unlock()
		writeJWKS(w, testKID, &key.PublicKey)
	})
	mux.HandleFunc("/token", func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseForm(); err != nil {
			http.Error(w, "bad form", http.StatusBadRequest)
			return
		}
		f.mu.Lock()
		f.lastVerifier = r.PostForm.Get("code_verifier")
		f.lastCode = r.PostForm.Get("code")
		idt := f.idToken
		f.mu.Unlock()
		if r.PostForm.Get("code") == "bad-code" {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusBadRequest)
			_, _ = w.Write([]byte(`{"error":"invalid_grant"}`))
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"access_token": "google-access-token",
			"token_type":   "Bearer",
			"expires_in":   3600,
			"id_token":     idt,
		})
	})
	f.server = httptest.NewServer(mux)
	t.Cleanup(f.server.Close)
	return f
}

func writeJWKS(w http.ResponseWriter, kid string, pub *rsa.PublicKey) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"keys": []map[string]string{{
			"kty": "RSA", "alg": "RS256", "use": "sig", "kid": kid,
			"n": base64.RawURLEncoding.EncodeToString(pub.N.Bytes()),
			"e": base64.RawURLEncoding.EncodeToString(big.NewInt(int64(pub.E)).Bytes()),
		}},
	})
}

func (f *fakeGoogle) client() *Client {
	return New(Config{
		ClientID:     testClientID,
		ClientSecret: "secret",
		RedirectURL:  "http://localhost:5173/api/auth/google/callback",
		AuthURL:      f.server.URL + "/auth",
		TokenURL:     f.server.URL + "/token",
		JWKSURL:      f.server.URL + "/certs",
	})
}

// validClaims is a token Google would issue for a Workspace user.
func validClaims(nonce string) jwt.MapClaims {
	now := time.Now()
	return jwt.MapClaims{
		"iss":            "https://accounts.google.com",
		"aud":            testClientID,
		"sub":            "1100000000001",
		"email":          "alice@acme.com",
		"email_verified": true,
		"hd":             "acme.com",
		"name":           "Alice Example",
		"nonce":          nonce,
		"iat":            now.Unix(),
		"exp":            now.Add(time.Hour).Unix(),
	}
}

func sign(t *testing.T, key *rsa.PrivateKey, kid string, claims jwt.MapClaims) string {
	t.Helper()
	tok := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
	tok.Header["kid"] = kid
	s, err := tok.SignedString(key)
	if err != nil {
		t.Fatalf("sign: %v", err)
	}
	return s
}

func TestVerifyIDToken_AcceptsValidToken(t *testing.T) {
	g := newFakeGoogle(t)
	raw := sign(t, g.key, testKID, validClaims("n-1"))

	id, err := g.client().VerifyIDToken(context.Background(), raw, "n-1")
	if err != nil {
		t.Fatalf("verify: %v", err)
	}
	if id.Subject != "1100000000001" || id.Email != "alice@acme.com" || !id.EmailVerified ||
		id.HostedDomain != "acme.com" || id.Name != "Alice Example" {
		t.Errorf("identity = %+v", id)
	}
}

func TestVerifyIDToken_AcceptsSchemelessGoogleIssuer(t *testing.T) {
	g := newFakeGoogle(t)
	c := validClaims("n-1")
	c["iss"] = "accounts.google.com"

	if _, err := g.client().VerifyIDToken(context.Background(), sign(t, g.key, testKID, c), "n-1"); err != nil {
		t.Fatalf("Google documents both issuer spellings; got %v", err)
	}
}

func TestVerifyIDToken_Rejects(t *testing.T) {
	otherKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}

	cases := []struct {
		name    string
		mutate  func(jwt.MapClaims)
		key     *rsa.PrivateKey // nil = the published key
		nonce   string
		wantErr error
	}{
		{name: "bad signature", key: otherKey, nonce: "n-1", wantErr: ErrInvalidIDToken},
		{name: "wrong audience", mutate: func(c jwt.MapClaims) { c["aud"] = "someone-else.apps.googleusercontent.com" }, nonce: "n-1", wantErr: ErrInvalidIDToken},
		{name: "wrong issuer", mutate: func(c jwt.MapClaims) { c["iss"] = "https://evil.example.com" }, nonce: "n-1", wantErr: ErrInvalidIDToken},
		{name: "expired", mutate: func(c jwt.MapClaims) {
			c["iat"] = time.Now().Add(-2 * time.Hour).Unix()
			c["exp"] = time.Now().Add(-time.Hour).Unix()
		}, nonce: "n-1", wantErr: ErrInvalidIDToken},
		{name: "nonce mismatch", nonce: "a-different-nonce", wantErr: ErrInvalidIDToken},
		{name: "nonce missing", mutate: func(c jwt.MapClaims) { delete(c, "nonce") }, nonce: "n-1", wantErr: ErrInvalidIDToken},
		{name: "empty expected nonce", nonce: "", wantErr: ErrInvalidIDToken},
		{name: "email not verified", mutate: func(c jwt.MapClaims) { c["email_verified"] = false }, nonce: "n-1", wantErr: ErrEmailNotVerified},
		{name: "email_verified absent", mutate: func(c jwt.MapClaims) { delete(c, "email_verified") }, nonce: "n-1", wantErr: ErrEmailNotVerified},
		{name: "email missing", mutate: func(c jwt.MapClaims) { delete(c, "email") }, nonce: "n-1", wantErr: ErrInvalidIDToken},
		{name: "subject missing", mutate: func(c jwt.MapClaims) { delete(c, "sub") }, nonce: "n-1", wantErr: ErrInvalidIDToken},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			g := newFakeGoogle(t)
			claims := validClaims("n-1")
			if tc.mutate != nil {
				tc.mutate(claims)
			}
			key := g.key
			if tc.key != nil {
				key = tc.key
			}
			_, err := g.client().VerifyIDToken(context.Background(), sign(t, key, testKID, claims), tc.nonce)
			if !errors.Is(err, tc.wantErr) {
				t.Fatalf("err = %v, want %v", err, tc.wantErr)
			}
		})
	}
}

func TestVerifyIDToken_RejectsUnsignedToken(t *testing.T) {
	g := newFakeGoogle(t)
	tok := jwt.NewWithClaims(jwt.SigningMethodNone, validClaims("n-1"))
	raw, err := tok.SignedString(jwt.UnsafeAllowNoneSignatureType)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := g.client().VerifyIDToken(context.Background(), raw, "n-1"); !errors.Is(err, ErrInvalidIDToken) {
		t.Fatalf("alg=none must be rejected, got %v", err)
	}
}

// Google rotates its signing keys; the key set is fetched once and cached,
// not fetched on every sign-in.
func TestVerifyIDToken_CachesJWKS(t *testing.T) {
	g := newFakeGoogle(t)
	c := g.client()
	for i := 0; i < 3; i++ {
		if _, err := c.VerifyIDToken(context.Background(), sign(t, g.key, testKID, validClaims("n-1")), "n-1"); err != nil {
			t.Fatalf("verify %d: %v", i, err)
		}
	}
	g.mu.Lock()
	defer g.mu.Unlock()
	if g.jwksHits != 1 {
		t.Errorf("JWKS fetched %d times, want 1", g.jwksHits)
	}
}

func TestAuthCodeURL_CarriesStateNonceAndPKCE(t *testing.T) {
	g := newFakeGoogle(t)
	verifier := "a-verifier-that-is-long-enough-for-pkce-0123456789"

	raw := g.client().AuthCodeURL("st-1", "n-1", verifier, "alice@acme.com")
	u, err := url.Parse(raw)
	if err != nil {
		t.Fatal(err)
	}
	q := u.Query()
	sum := sha256.Sum256([]byte(verifier))
	want := map[string]string{
		"client_id":             testClientID,
		"response_type":         "code",
		"state":                 "st-1",
		"nonce":                 "n-1",
		"code_challenge":        base64.RawURLEncoding.EncodeToString(sum[:]),
		"code_challenge_method": "S256",
		"login_hint":            "alice@acme.com",
		"redirect_uri":          "http://localhost:5173/api/auth/google/callback",
	}
	for k, v := range want {
		if got := q.Get(k); got != v {
			t.Errorf("%s = %q, want %q", k, got, v)
		}
	}
	scopes := strings.Fields(q.Get("scope"))
	for _, s := range []string{"openid", "email", "profile"} {
		found := false
		for _, have := range scopes {
			found = found || have == s
		}
		if !found {
			t.Errorf("scope %q missing from %v", s, scopes)
		}
	}
	if q.Has("code_verifier") {
		t.Error("the PKCE verifier must never appear in the authorization URL")
	}
}

func TestAuthCodeURL_OmitsEmptyLoginHint(t *testing.T) {
	g := newFakeGoogle(t)
	u, _ := url.Parse(g.client().AuthCodeURL("s", "n", "verifier-verifier-verifier-verifier-verifier", ""))
	if u.Query().Has("login_hint") {
		t.Error("login_hint must be omitted when not given")
	}
}

func TestExchange_SendsVerifierAndReturnsVerifiedIdentity(t *testing.T) {
	g := newFakeGoogle(t)
	g.idToken = sign(t, g.key, testKID, validClaims("n-1"))
	verifier := "a-verifier-that-is-long-enough-for-pkce-0123456789"

	id, err := g.client().Exchange(context.Background(), "good-code", verifier, "n-1")
	if err != nil {
		t.Fatalf("exchange: %v", err)
	}
	if id.Subject != "1100000000001" {
		t.Errorf("subject = %q", id.Subject)
	}
	g.mu.Lock()
	defer g.mu.Unlock()
	if g.lastVerifier != verifier {
		t.Errorf("token endpoint got code_verifier %q, want %q", g.lastVerifier, verifier)
	}
	if g.lastCode != "good-code" {
		t.Errorf("token endpoint got code %q", g.lastCode)
	}
}

func TestExchange_VerifiesTheReturnedIDToken(t *testing.T) {
	g := newFakeGoogle(t)
	g.idToken = sign(t, g.key, testKID, validClaims("n-1"))

	if _, err := g.client().Exchange(context.Background(), "good-code", "v-v-v-v-v-v-v-v-v-v-v-v-v-v-v-v-v-v-v-v-v-v", "other-nonce"); !errors.Is(err, ErrInvalidIDToken) {
		t.Fatalf("err = %v, want ErrInvalidIDToken for a nonce mismatch after exchange", err)
	}
}

func TestExchange_MissingIDToken(t *testing.T) {
	g := newFakeGoogle(t)
	g.idToken = ""
	if _, err := g.client().Exchange(context.Background(), "good-code", "v-v-v-v-v-v-v-v-v-v-v-v-v-v-v-v-v-v-v-v-v-v", "n-1"); !errors.Is(err, ErrInvalidIDToken) {
		t.Fatalf("err = %v, want ErrInvalidIDToken", err)
	}
}

func TestExchange_GoogleRejectsCode(t *testing.T) {
	g := newFakeGoogle(t)
	_, err := g.client().Exchange(context.Background(), "bad-code", "v-v-v-v-v-v-v-v-v-v-v-v-v-v-v-v-v-v-v-v-v-v", "n-1")
	if !errors.Is(err, ErrExchange) {
		t.Fatalf("err = %v, want ErrExchange", err)
	}
}

func TestNewFlowSecrets_AreRandomAndURLSafe(t *testing.T) {
	a, err := NewFlowSecrets()
	if err != nil {
		t.Fatal(err)
	}
	b, err := NewFlowSecrets()
	if err != nil {
		t.Fatal(err)
	}
	if a.State == b.State || a.Nonce == b.Nonce || a.Verifier == b.Verifier {
		t.Error("flow secrets must differ between flows")
	}
	if a.State == a.Nonce {
		t.Error("state and nonce must be independent values")
	}
	for _, s := range []string{a.State, a.Nonce, a.Verifier} {
		if len(s) < 43 || strings.ContainsAny(s, "+/=") {
			t.Errorf("secret %q is too short or not URL-safe", s)
		}
	}
}
