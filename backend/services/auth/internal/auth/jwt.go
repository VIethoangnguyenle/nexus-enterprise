package auth

import (
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
)

var jwtSecret []byte

// SetJWTSecret configures the signing key for JWT generation and validation.
func SetJWTSecret(secret string) {
	jwtSecret = []byte(secret)
}

// Claims holds JWT payload for the auth service.
type Claims struct {
	UserID     string `json:"user_id"`
	Username   string `json:"username"`
	NGACNodeID string `json:"ngac_node_id"`
	TenantID   string `json:"tenant_id,omitempty"`
	SessionID  string `json:"session_id,omitempty"`
	jwt.RegisteredClaims
}

// HashPassword hashes a plaintext password using bcrypt.
func HashPassword(password string) (string, error) {
	bytes, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	return string(bytes), err
}

// CheckPassword verifies a plaintext password against a bcrypt hash.
func CheckPassword(password, hash string) bool {
	err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(password))
	return err == nil
}

// AccessTokenTTL is how long an access token stays valid.
//
// Every service validates the token on its own against the shared secret and
// never calls back to auth, so this value *is* the revocation window: after a
// logout or a lockout, an already-issued token keeps working until it expires.
// Keeping it short is what makes that window acceptable; the refresh token
// carries the long-lived part of the session.
const AccessTokenTTL = 15 * time.Minute

// GenerateToken creates a signed access token for a brand-new session and
// reports the session ID, so the caller can bind a refresh token to the same
// family.
func GenerateToken(userID, username, ngacNodeID, tenantID string) (token, sessionID string, err error) {
	sessionID = uuid.New().String()
	token, err = GenerateTokenForSession(userID, username, ngacNodeID, tenantID, sessionID)
	if err != nil {
		return "", "", err
	}
	return token, sessionID, nil
}

// GenerateTokenForSession creates a signed access token that belongs to an
// existing session.
//
// Refreshing must not start a new session: the session ID is what ties an
// access token back to the refresh-token family, and therefore what logout
// uses to kill it. Minting a fresh session ID on every refresh would leave a
// trail of families that nothing can revoke.
func GenerateTokenForSession(userID, username, ngacNodeID, tenantID, sessionID string) (string, error) {
	now := time.Now()
	claims := &Claims{
		UserID:     userID,
		Username:   username,
		NGACNodeID: ngacNodeID,
		TenantID:   tenantID,
		SessionID:  sessionID,
		RegisteredClaims: jwt.RegisteredClaims{
			ID:        uuid.New().String(),
			ExpiresAt: jwt.NewNumericDate(now.Add(AccessTokenTTL)),
			IssuedAt:  jwt.NewNumericDate(now),
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(jwtSecret)
}
