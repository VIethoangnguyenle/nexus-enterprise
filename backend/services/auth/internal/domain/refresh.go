package domain

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"

	"ngac-platform/services/auth/internal/auth"
)

const (
	// RefreshTokenTTL is how long a session can be kept alive by refreshing.
	// Beyond it the user signs in again.
	RefreshTokenTTL = 7 * 24 * time.Hour

	refreshKeyPrefix  = "refresh:"
	sessionKeyPrefix  = "refresh_session:"
	refreshTokenBytes = 32
)

// ErrRefreshRejected is returned for any refresh token that is unknown,
// expired, already spent, or belongs to a revoked session. The reasons are
// deliberately not distinguished in the error: telling a caller *why* their
// token failed tells an attacker which of their guesses was closer.
var ErrRefreshRejected = errors.New("refresh token rejected")

// RefreshIdentity is the subject a refresh token stands for. It is everything
// needed to mint a new access token without touching the database.
type RefreshIdentity struct {
	UserID     string `json:"user_id"`
	Username   string `json:"username"`
	NGACNodeID string `json:"ngac_node_id"`
	TenantID   string `json:"tenant_id"`
	SessionID  string `json:"session_id"`
}

// refreshRecord is what a refresh token resolves to in Redis.
type refreshRecord struct {
	RefreshIdentity
	// Spent marks a token that has already been exchanged. The record is kept
	// rather than deleted so that a replay is *detectable* instead of merely
	// looking like an expiry.
	Spent bool `json:"spent"`
}

// RefreshStore issues, rotates and revokes refresh tokens.
//
// Tokens are opaque random strings, not JWTs: the whole point is that the
// server decides whether one is still valid, which a self-contained token
// cannot express. Each token belongs to a session family, and rotation moves
// the family forward one token at a time.
type RefreshStore struct {
	rdb *redis.Client
	ttl time.Duration
}

// NewRefreshStore creates a refresh token store backed by Redis.
func NewRefreshStore(rdb *redis.Client) *RefreshStore {
	return &RefreshStore{rdb: rdb, ttl: RefreshTokenTTL}
}

func refreshKey(token string) string { return refreshKeyPrefix + token }
func sessionKey(sid string) string   { return sessionKeyPrefix + sid }

// Issue starts a new session and returns its first refresh token.
func (s *RefreshStore) Issue(ctx context.Context, id RefreshIdentity) (token, sessionID string, err error) {
	if s == nil || s.rdb == nil {
		return "", "", fmt.Errorf("refresh store unavailable")
	}
	if id.SessionID == "" {
		id.SessionID = uuid.New().String()
	}
	token, err = s.mint(ctx, id)
	if err != nil {
		return "", "", err
	}
	return token, id.SessionID, nil
}

// mint stores a fresh token for an existing identity and files it under the
// session family so the family can be revoked wholesale.
func (s *RefreshStore) mint(ctx context.Context, id RefreshIdentity) (string, error) {
	raw := make([]byte, refreshTokenBytes)
	if _, err := rand.Read(raw); err != nil {
		return "", fmt.Errorf("generate refresh token: %w", err)
	}
	token := base64.RawURLEncoding.EncodeToString(raw)

	payload, err := json.Marshal(refreshRecord{RefreshIdentity: id})
	if err != nil {
		return "", fmt.Errorf("encode refresh record: %w", err)
	}

	pipe := s.rdb.TxPipeline()
	pipe.Set(ctx, refreshKey(token), payload, s.ttl)
	pipe.SAdd(ctx, sessionKey(id.SessionID), token)
	pipe.Expire(ctx, sessionKey(id.SessionID), s.ttl)
	if _, err := pipe.Exec(ctx); err != nil {
		return "", fmt.Errorf("store refresh token: %w", err)
	}
	return token, nil
}

// Rotate exchanges a refresh token for a new one, returning the identity it
// stands for.
//
// Presenting a token that has already been spent means two parties hold the
// same token — the legitimate client and whoever copied it. There is no way to
// tell which one is calling, so the entire session family is revoked and both
// are forced to sign in again.
func (s *RefreshStore) Rotate(ctx context.Context, token string) (string, RefreshIdentity, error) {
	if s == nil || s.rdb == nil {
		return "", RefreshIdentity{}, fmt.Errorf("refresh store unavailable")
	}
	if token == "" {
		return "", RefreshIdentity{}, ErrRefreshRejected
	}

	payload, err := s.rdb.Get(ctx, refreshKey(token)).Bytes()
	if err != nil {
		return "", RefreshIdentity{}, ErrRefreshRejected
	}

	var rec refreshRecord
	if err := json.Unmarshal(payload, &rec); err != nil {
		return "", RefreshIdentity{}, ErrRefreshRejected
	}

	if rec.Spent {
		// Replay detected — burn the family, including the token the honest
		// client is holding right now.
		_ = s.RevokeSession(ctx, rec.SessionID)
		return "", RefreshIdentity{}, ErrRefreshRejected
	}

	// Mark spent before minting the successor, so a crash in between costs the
	// user a re-login rather than leaving a token that can be replayed.
	rec.Spent = true
	spentPayload, err := json.Marshal(rec)
	if err != nil {
		return "", RefreshIdentity{}, fmt.Errorf("encode spent record: %w", err)
	}
	if err := s.rdb.Set(ctx, refreshKey(token), spentPayload, redis.KeepTTL).Err(); err != nil {
		return "", RefreshIdentity{}, fmt.Errorf("mark refresh token spent: %w", err)
	}

	next, err := s.mint(ctx, rec.RefreshIdentity)
	if err != nil {
		return "", RefreshIdentity{}, err
	}
	return next, rec.RefreshIdentity, nil
}

// AttachRefreshToken issues the refresh token for a session that already has
// an access token, binding both to the same session ID so that ending the
// session invalidates the refresh side.
func (s *Service) AttachRefreshToken(ctx context.Context, id RefreshIdentity) (string, error) {
	if id.SessionID == "" {
		return "", fmt.Errorf("refresh token needs the session of its access token")
	}
	token, _, err := s.refresh.Issue(ctx, id)
	if err != nil {
		return "", fmt.Errorf("issue refresh token: %w", err)
	}
	return token, nil
}

// RefreshSession exchanges a refresh token for a new access/refresh pair.
func (s *Service) RefreshSession(ctx context.Context, refreshToken string) (newAccess, newRefresh string, err error) {
	next, id, err := s.refresh.Rotate(ctx, refreshToken)
	if err != nil {
		return "", "", err
	}

	access, err := auth.GenerateTokenForSession(id.UserID, id.Username, id.NGACNodeID, id.TenantID, id.SessionID)
	if err != nil {
		return "", "", fmt.Errorf("generate access token: %w", err)
	}
	return access, next, nil
}

// EndSession revokes the refresh family behind a session. The access token
// already in the caller's hands stays valid for the rest of its short TTL.
func (s *Service) EndSession(ctx context.Context, sessionID string) error {
	return s.refresh.RevokeSession(ctx, sessionID)
}

// EndSessionByRefreshToken revokes a session identified only by one of its
// refresh tokens. Logging out after the access token has already expired takes
// this path, since the session ID is no longer available from the claims.
func (s *Service) EndSessionByRefreshToken(ctx context.Context, refreshToken string) error {
	id, err := s.refresh.Lookup(ctx, refreshToken)
	if err != nil {
		// An unusable token means there is nothing left to revoke. Reporting
		// success keeps logout idempotent.
		return nil
	}
	return s.refresh.RevokeSession(ctx, id.SessionID)
}

// Lookup resolves a refresh token to its identity without spending it.
// It is used by logout, which needs the session ID and nothing else.
func (s *RefreshStore) Lookup(ctx context.Context, token string) (RefreshIdentity, error) {
	if s == nil || s.rdb == nil || token == "" {
		return RefreshIdentity{}, ErrRefreshRejected
	}
	payload, err := s.rdb.Get(ctx, refreshKey(token)).Bytes()
	if err != nil {
		return RefreshIdentity{}, ErrRefreshRejected
	}
	var rec refreshRecord
	if err := json.Unmarshal(payload, &rec); err != nil {
		return RefreshIdentity{}, ErrRefreshRejected
	}
	return rec.RefreshIdentity, nil
}

// RevokeSession invalidates every refresh token in a session family. This is
// what logout does, and what reuse detection falls back on.
//
// The access token already issued for the session is not affected — it is
// self-contained and stays valid for the remainder of its short TTL. What this
// guarantees is that no *further* access token can be minted.
func (s *RefreshStore) RevokeSession(ctx context.Context, sessionID string) error {
	if s == nil || s.rdb == nil || sessionID == "" {
		return nil
	}
	tokens, err := s.rdb.SMembers(ctx, sessionKey(sessionID)).Result()
	if err != nil && !errors.Is(err, redis.Nil) {
		return fmt.Errorf("read session family: %w", err)
	}

	pipe := s.rdb.TxPipeline()
	for _, t := range tokens {
		pipe.Del(ctx, refreshKey(t))
	}
	pipe.Del(ctx, sessionKey(sessionID))
	if _, err := pipe.Exec(ctx); err != nil && !errors.Is(err, redis.Nil) {
		return fmt.Errorf("revoke session: %w", err)
	}
	return nil
}
