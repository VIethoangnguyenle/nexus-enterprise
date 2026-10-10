package ngac

import (
	"context"
	"encoding/json"
	"log/slog"
	"time"

	"github.com/redis/go-redis/v9"
)

// cacheTTL bounds how long an L1 decision lives. Key layout lives in
// pdp_decision_cache_keys.go, shared with the invalidator.
const cacheTTL = 30 * time.Second

// DecisionCache caches access decisions. The only layer is Redis (L1): a
// runtime check never reads the database, so there is no database-backed layer
// behind it.
type DecisionCache interface {
	// Get attempts to retrieve a cached decision.
	// Returns nil if cache miss. The layer string names the level that served
	// the result ("L1").
	Get(ctx context.Context, req AccessRequest) (*AccessDecision, string)

	// Set stores a computed decision.
	// An error-derived decision (decision.ErrorDerived()) is never stored.
	Set(ctx context.Context, req AccessRequest, decision *AccessDecision)
}

// layeredCache implements DecisionCache on Redis.
type layeredCache struct {
	rdb *redis.Client
}

// NewLayeredCache creates the decision cache. A nil rdb disables caching:
// every Get misses and Set does nothing.
func NewLayeredCache(rdb *redis.Client) DecisionCache {
	return &layeredCache{rdb: rdb}
}

// Get returns the L1 (Redis) entry for the request, with its full explanation.
func (c *layeredCache) Get(ctx context.Context, req AccessRequest) (*AccessDecision, string) {
	if c.rdb != nil {
		if decision := c.getRedis(ctx, cacheKey(req)); decision != nil {
			return decision, "L1"
		}
	}
	return nil, ""
}

// Set stores a computed decision in L1 (Redis).
func (c *layeredCache) Set(ctx context.Context, req AccessRequest, decision *AccessDecision) {
	// Second line of defence behind AccessEvaluator: a decision produced by a
	// failure is not a fact about the policy and must not outlive the request.
	if decision == nil || decision.ErrorDerived() {
		return
	}
	c.setRedis(ctx, cacheKey(req), decision)
}

// --- Internal helpers ---

func (c *layeredCache) getRedis(ctx context.Context, key string) *AccessDecision {
	if c.rdb == nil {
		return nil
	}
	data, err := c.rdb.Get(ctx, key).Bytes()
	if err != nil {
		return nil
	}
	var d AccessDecision
	if err := json.Unmarshal(data, &d); err != nil {
		return nil
	}
	return &d
}

func (c *layeredCache) setRedis(ctx context.Context, key string, decision *AccessDecision) {
	if c.rdb == nil {
		return
	}
	data, err := json.Marshal(decision)
	if err != nil {
		return
	}
	if err := c.rdb.Set(ctx, key, data, cacheTTL).Err(); err != nil {
		slog.Warn("failed to cache access decision", "key", key, "error", err)
	}
}

// cacheKey generates a workspace-isolated cache key for access decisions.
// See DecisionCacheKey for the layout.
func cacheKey(req AccessRequest) string {
	return DecisionCacheKey(req)
}
