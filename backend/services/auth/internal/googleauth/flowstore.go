package googleauth

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

// FlowTTL is how long a started sign-in may take before its callback is refused.
const FlowTTL = 10 * time.Minute

const flowKeyPrefix = "google_oauth_flow:"

// ErrFlowNotFound means the callback's state was never issued, has expired,
// or has already been used.
var ErrFlowNotFound = errors.New("google sign-in flow not found")

// Flow is the server-side half of a sign-in attempt, keyed by its state.
// The nonce and PKCE verifier never leave the server.
type Flow struct {
	Nonce    string `json:"nonce"`
	Verifier string `json:"verifier"`
}

// FlowStore keeps in-progress sign-in attempts.
type FlowStore interface {
	Save(ctx context.Context, state string, f Flow) error
	// Take returns and deletes the flow, so each state works exactly once.
	Take(ctx context.Context, state string) (Flow, error)
}

// RedisFlowStore is a FlowStore backed by Redis with FlowTTL expiry.
type RedisFlowStore struct {
	rdb *redis.Client
}

// NewRedisFlowStore creates a Redis-backed flow store.
func NewRedisFlowStore(rdb *redis.Client) *RedisFlowStore {
	return &RedisFlowStore{rdb: rdb}
}

// flowKey hashes the state so the raw value (also the browser's cookie) is
// not stored as a key.
func flowKey(state string) string {
	sum := sha256.Sum256([]byte(state))
	return flowKeyPrefix + hex.EncodeToString(sum[:])
}

// Save stores a flow under its state.
func (s *RedisFlowStore) Save(ctx context.Context, state string, f Flow) error {
	if s == nil || s.rdb == nil {
		return fmt.Errorf("flow store unavailable")
	}
	payload, err := json.Marshal(f)
	if err != nil {
		return fmt.Errorf("encode flow: %w", err)
	}
	if err := s.rdb.Set(ctx, flowKey(state), payload, FlowTTL).Err(); err != nil {
		return fmt.Errorf("store flow: %w", err)
	}
	return nil
}

// Take atomically reads and deletes the flow for state.
func (s *RedisFlowStore) Take(ctx context.Context, state string) (Flow, error) {
	if s == nil || s.rdb == nil || state == "" {
		return Flow{}, ErrFlowNotFound
	}
	payload, err := s.rdb.GetDel(ctx, flowKey(state)).Bytes()
	if errors.Is(err, redis.Nil) {
		return Flow{}, ErrFlowNotFound
	}
	if err != nil {
		return Flow{}, fmt.Errorf("take flow: %w", err)
	}
	var f Flow
	if err := json.Unmarshal(payload, &f); err != nil {
		return Flow{}, ErrFlowNotFound
	}
	return f, nil
}
