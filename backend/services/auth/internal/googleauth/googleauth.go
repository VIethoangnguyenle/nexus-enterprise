// Package googleauth implements the relying-party side of "Sign in with
// Google": OpenID Connect authorization-code flow with PKCE, state and nonce,
// and full verification of the returned ID token.
//
// Nothing in here decides who the user is in this system — it only turns a
// Google redirect into a verified Identity. Account and tenant resolution live
// in the domain layer.
package googleauth

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/coreos/go-oidc/v3/oidc"
	"golang.org/x/oauth2"
)

// Google's published OIDC endpoints (https://accounts.google.com/.well-known/openid-configuration).
// They are fixed rather than discovered at startup, so the auth service does
// not depend on reaching Google to boot.
const (
	GoogleIssuer   = "https://accounts.google.com"
	googleAuthURL  = "https://accounts.google.com/o/oauth2/v2/auth"
	googleTokenURL = "https://oauth2.googleapis.com/token"
	googleJWKSURL  = "https://www.googleapis.com/oauth2/v3/certs"
)

var (
	// ErrInvalidIDToken covers every reason an ID token is not acceptable:
	// bad signature, wrong issuer or audience, expired, nonce mismatch, or
	// missing required claims. The reasons are not distinguished to callers.
	ErrInvalidIDToken = errors.New("invalid google id token")
	// ErrEmailNotVerified means the token is genuine but Google has not
	// verified the email address, so it cannot be trusted as an identity.
	ErrEmailNotVerified = errors.New("google email not verified")
	// ErrExchange means Google refused the authorization code or could not be reached.
	ErrExchange = errors.New("google code exchange failed")
)

// Config configures the Google client. Only the first three fields are set in
// production; the endpoint overrides exist so tests can point at a local fake.
type Config struct {
	ClientID     string
	ClientSecret string
	RedirectURL  string

	AuthURL  string
	TokenURL string
	JWKSURL  string
	Issuer   string

	HTTPClient *http.Client
	Now        func() time.Time
}

// Identity is what a verified Google ID token says about the user.
type Identity struct {
	Subject       string
	Email         string
	EmailVerified bool
	// HostedDomain is the `hd` claim: present only for Google Workspace
	// accounts, and proof that the account is managed by that domain.
	HostedDomain string
	Name         string
	Picture      string
}

// Client runs the Google sign-in flow.
type Client struct {
	oauth      *oauth2.Config
	verifier   *oidc.IDTokenVerifier
	httpClient *http.Client
}

// New builds a Client. The JWKS is fetched lazily on first verification and
// cached; an unknown key ID (Google rotated keys) triggers a refetch.
func New(cfg Config) *Client {
	if cfg.AuthURL == "" {
		cfg.AuthURL = googleAuthURL
	}
	if cfg.TokenURL == "" {
		cfg.TokenURL = googleTokenURL
	}
	if cfg.JWKSURL == "" {
		cfg.JWKSURL = googleJWKSURL
	}
	if cfg.Issuer == "" {
		cfg.Issuer = GoogleIssuer
	}
	if cfg.HTTPClient == nil {
		cfg.HTTPClient = &http.Client{Timeout: 10 * time.Second}
	}

	keySet := oidc.NewRemoteKeySet(oidc.ClientContext(context.Background(), cfg.HTTPClient), cfg.JWKSURL)
	return &Client{
		oauth: &oauth2.Config{
			ClientID:     cfg.ClientID,
			ClientSecret: cfg.ClientSecret,
			RedirectURL:  cfg.RedirectURL,
			Endpoint: oauth2.Endpoint{
				AuthURL:   cfg.AuthURL,
				TokenURL:  cfg.TokenURL,
				AuthStyle: oauth2.AuthStyleInParams,
			},
			Scopes: []string{oidc.ScopeOpenID, "email", "profile"},
		},
		// go-oidc accepts both "https://accounts.google.com" and the
		// scheme-less "accounts.google.com" when the issuer is Google's.
		verifier: oidc.NewVerifier(cfg.Issuer, keySet, &oidc.Config{
			ClientID:             cfg.ClientID,
			SupportedSigningAlgs: []string{oidc.RS256},
			Now:                  cfg.Now,
		}),
		httpClient: cfg.HTTPClient,
	}
}

// FlowSecrets are the per-attempt values that tie a callback to the browser
// and the request that started it.
type FlowSecrets struct {
	// State is echoed back by Google and must match the browser's cookie —
	// the CSRF defence for the callback.
	State string
	// Nonce is embedded in the ID token and must match — the replay defence
	// for the token itself.
	Nonce string
	// Verifier is the PKCE code verifier; only its hash goes to the browser.
	Verifier string
}

// NewFlowSecrets generates fresh, independent random values for one attempt.
func NewFlowSecrets() (FlowSecrets, error) {
	state, err := randomToken()
	if err != nil {
		return FlowSecrets{}, err
	}
	nonce, err := randomToken()
	if err != nil {
		return FlowSecrets{}, err
	}
	return FlowSecrets{State: state, Nonce: nonce, Verifier: oauth2.GenerateVerifier()}, nil
}

func randomToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("generate random token: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

// AuthCodeURL is where the browser is sent to sign in with Google.
func (c *Client) AuthCodeURL(state, nonce, verifier, loginHint string) string {
	opts := []oauth2.AuthCodeOption{
		oauth2.S256ChallengeOption(verifier),
		oauth2.SetAuthURLParam("nonce", nonce),
		// Let people with several Google accounts pick the work one.
		oauth2.SetAuthURLParam("prompt", "select_account"),
	}
	if loginHint != "" {
		opts = append(opts, oauth2.SetAuthURLParam("login_hint", loginHint))
	}
	return c.oauth.AuthCodeURL(state, opts...)
}

// Exchange trades the authorization code (plus the PKCE verifier) for tokens
// and returns the identity from the verified ID token. The access token Google
// returns is discarded: nothing here calls Google APIs on the user's behalf.
func (c *Client) Exchange(ctx context.Context, code, verifier, nonce string) (*Identity, error) {
	ctx = context.WithValue(ctx, oauth2.HTTPClient, c.httpClient)
	tok, err := c.oauth.Exchange(ctx, code, oauth2.VerifierOption(verifier))
	if err != nil {
		// Never wrap the raw error: a RetrieveError carries the response body.
		var re *oauth2.RetrieveError
		if errors.As(err, &re) {
			return nil, fmt.Errorf("%w: %s", ErrExchange, re.ErrorCode)
		}
		return nil, ErrExchange
	}
	raw, _ := tok.Extra("id_token").(string)
	if raw == "" {
		return nil, fmt.Errorf("%w: token response has no id_token", ErrInvalidIDToken)
	}
	return c.VerifyIDToken(ctx, raw, nonce)
}

// VerifyIDToken checks signature (against Google's JWKS), issuer, audience,
// expiry and nonce, then requires a subject, an email, and email_verified.
func (c *Client) VerifyIDToken(ctx context.Context, raw, expectedNonce string) (*Identity, error) {
	if expectedNonce == "" {
		return nil, fmt.Errorf("%w: no nonce to check against", ErrInvalidIDToken)
	}
	idt, err := c.verifier.Verify(ctx, raw)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrInvalidIDToken, err)
	}
	if subtle.ConstantTimeCompare([]byte(idt.Nonce), []byte(expectedNonce)) != 1 {
		return nil, fmt.Errorf("%w: nonce mismatch", ErrInvalidIDToken)
	}

	var claims struct {
		Email         string `json:"email"`
		EmailVerified any    `json:"email_verified"`
		HostedDomain  string `json:"hd"`
		Name          string `json:"name"`
		Picture       string `json:"picture"`
	}
	if err := idt.Claims(&claims); err != nil {
		return nil, fmt.Errorf("%w: decode claims", ErrInvalidIDToken)
	}
	if idt.Subject == "" || claims.Email == "" {
		return nil, fmt.Errorf("%w: missing sub or email", ErrInvalidIDToken)
	}
	if !isTrue(claims.EmailVerified) {
		return nil, ErrEmailNotVerified
	}

	return &Identity{
		Subject:       idt.Subject,
		Email:         claims.Email,
		EmailVerified: true,
		HostedDomain:  claims.HostedDomain,
		Name:          claims.Name,
		Picture:       claims.Picture,
	}, nil
}

// isTrue accepts email_verified as a JSON boolean, or as the string "true"
// that some Google token paths have historically produced.
func isTrue(v any) bool {
	switch b := v.(type) {
	case bool:
		return b
	case string:
		return b == "true"
	default:
		return false
	}
}
