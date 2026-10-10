package grpcauth

import (
	"bytes"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync/atomic"
	"time"
)

// KeyIdentity is the one metadata key that carries the signed identity token.
// gRPC lower-cases keys on the wire.
const KeyIdentity = "x-nexus-identity"

const (
	// tokenTTL is how long a minted token is valid. A call is made the moment
	// its token is minted, so this only has to cover clock drift and queueing.
	tokenTTL = 60 * time.Second
	// clockSkew is the leeway the verifier gives for unsynchronised clocks, in
	// both directions (a token issued slightly in the future, or just expired).
	clockSkew = 5 * time.Second
	// maxLifetime bounds exp-iat, so a token cannot claim to live for hours.
	maxLifetime = 2 * time.Minute
	// minSecretLen is the shortest secret Configure accepts, in bytes. The
	// deployment generates 96 hex characters.
	minSecretLen = 32
	// maxTokenLen bounds a token before any decoding. A real one is about 400
	// bytes (three UUIDs, a method name, a nonce and a 43-character MAC).
	maxTokenLen = 1024
	// macLen is the length of a base64url-encoded HMAC-SHA256 (32 bytes, no
	// padding).
	macLen = 43

	// macDomain separates these signatures from any other use of the secret.
	macDomain = "nexus-identity-v1."
)

// Keys are the secrets identity tokens are signed and verified with.
type Keys struct {
	// Current signs every token and verifies incoming ones.
	Current string
	// Previous only verifies, so a secret can be rotated without a window in
	// which a service holding the old one is refused. Empty when not rotating.
	Previous string
}

type keyring struct{ current, previous []byte }

var active atomic.Pointer[keyring]

// Configure installs the secrets for this process. Call it once at start-up,
// before dialling or serving. Until it succeeds every outgoing call fails and
// every incoming call that needs an identity is refused, so a service that
// forgets it is closed rather than open.
func Configure(k Keys) error {
	if len(k.Current) < minSecretLen {
		return fmt.Errorf("internal identity secret must be at least %d bytes", minSecretLen)
	}
	if k.Previous != "" && len(k.Previous) < minSecretLen {
		return fmt.Errorf("previous internal identity secret must be at least %d bytes", minSecretLen)
	}
	kr := &keyring{current: []byte(k.Current)}
	if k.Previous != "" && k.Previous != k.Current {
		kr.previous = []byte(k.Previous)
	}
	active.Store(kr)
	return nil
}

// claims is the signed payload. Exactly one identity is set: a user (UID and
// NID, optionally TID) or a service (SVC).
type claims struct {
	UID string `json:"uid,omitempty"`
	NID string `json:"nid,omitempty"`
	TID string `json:"tid,omitempty"`
	SVC string `json:"svc,omitempty"`
	AUD string `json:"aud"`
	IAT int64  `json:"iat"`
	EXP int64  `json:"exp"`
	// JTI is a random nonce. It makes every token unique and is available for
	// logging; no replay cache keys on it (see the package comment).
	JTI string `json:"jti"`
}

var errNoKeys = errors.New("internal identity secret is not configured")

var now = time.Now

func sign(key []byte, payload string) []byte {
	m := hmac.New(sha256.New, key)
	m.Write([]byte(macDomain))
	m.Write([]byte(payload))
	return m.Sum(nil)
}

var enc = base64.RawURLEncoding

// mint returns a token for c, valid for method only.
func mint(kr *keyring, c claims, method string) (string, error) {
	if kr == nil {
		return "", errNoKeys
	}
	nonce := make([]byte, 12)
	if _, err := rand.Read(nonce); err != nil {
		return "", fmt.Errorf("generating nonce: %w", err)
	}
	t := now()
	c.AUD = method
	c.IAT = t.Unix()
	c.EXP = t.Add(tokenTTL).Unix()
	c.JTI = enc.EncodeToString(nonce)
	body, err := json.Marshal(c)
	if err != nil {
		return "", fmt.Errorf("encoding identity token: %w", err)
	}
	payload := enc.EncodeToString(body)
	return payload + "." + enc.EncodeToString(sign(kr.current, payload)), nil
}

// verify checks token against kr and method and returns its claims. Every
// failure is an error; callers answer Unauthenticated without saying which
// check failed.
func verify(kr *keyring, token, method string) (claims, error) {
	var c claims
	if kr == nil {
		return c, errNoKeys
	}
	if len(token) > maxTokenLen {
		return c, errors.New("token too long")
	}
	payload, macPart, ok := strings.Cut(token, ".")
	if !ok || payload == "" || len(macPart) != macLen {
		return c, errors.New("malformed token")
	}
	got, err := enc.DecodeString(macPart)
	if err != nil {
		return c, errors.New("malformed signature")
	}
	// Check both keys without short-circuiting on the first, so timing does not
	// say which one matched.
	okCurrent := hmac.Equal(got, sign(kr.current, payload))
	okPrevious := kr.previous != nil && hmac.Equal(got, sign(kr.previous, payload))
	if !okCurrent && !okPrevious {
		return c, errors.New("bad signature")
	}
	body, err := enc.DecodeString(payload)
	if err != nil {
		return c, errors.New("malformed payload")
	}
	dec := json.NewDecoder(bytes.NewReader(body))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&c); err != nil {
		return claims{}, errors.New("malformed payload")
	}
	if c.AUD != method {
		return claims{}, errors.New("token is for another method")
	}
	t := now()
	if c.IAT > t.Add(clockSkew).Unix() {
		return claims{}, errors.New("token issued in the future")
	}
	if c.EXP < t.Add(-clockSkew).Unix() {
		return claims{}, errors.New("token expired")
	}
	if c.EXP-c.IAT > int64(maxLifetime/time.Second) {
		return claims{}, errors.New("token lifetime too long")
	}
	user := c.UID != "" || c.NID != ""
	switch {
	case user && c.SVC != "":
		return claims{}, errors.New("token names both a user and a service")
	case user && !(Caller{UserID: c.UID, NGACNodeID: c.NID}).Authenticated():
		return claims{}, errors.New("incomplete user identity")
	case !user && c.SVC == "":
		return claims{}, errors.New("token names no identity")
	}
	return c, nil
}
