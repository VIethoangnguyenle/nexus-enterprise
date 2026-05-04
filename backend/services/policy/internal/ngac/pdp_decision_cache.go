package ngac

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"github.com/redis/go-redis/v9"
)

const (
	cacheTTL       = 30 * time.Second
	cacheKeyPrefix = "ngac:access:"
	scopeKeyPrefix = "scopes:"
)

// DecisionCache provides layered caching for access decisions.
// Implementation details (Redis, materialized, version) are hidden.
type DecisionCache interface {
	// Get attempts to retrieve a cached decision.
	// Returns nil if cache miss or stale.
	// The layer string indicates which cache level served the result ("L1" or "L2").
	Get(ctx context.Context, req AccessRequest) (*AccessDecision, string)

	// Set stores a computed decision in all cache layers.
	Set(ctx context.Context, req AccessRequest, decision *AccessDecision)
}

// layeredCache implements DecisionCache with L1 (Redis) + L2 (Materialized + version freshness).
type layeredCache struct {
	rdb          *redis.Client
	materialized *MaterializedAccess
	version      *VersionTracker
}

// NewLayeredCache creates a cache that checks L1 Redis then L2 Materialized.
// All parameters are optional — nil values disable that cache layer.
func NewLayeredCache(rdb *redis.Client, materialized *MaterializedAccess, version *VersionTracker) DecisionCache {
	return &layeredCache{
		rdb:          rdb,
		materialized: materialized,
		version:      version,
	}
}

// Get checks L1 (Redis) then L2 (Materialized with version freshness).
// On L2 hit, promotes to L1 for faster next lookup.
func (c *layeredCache) Get(ctx context.Context, req AccessRequest) (*AccessDecision, string) {
	key := cacheKey(req)

	// L1: Redis
	if c.rdb != nil {
		if decision := c.getRedis(ctx, key); decision != nil {
			return decision, "L1"
		}
	}

	// L2: Materialized with version freshness
	if c.materialized != nil && c.version != nil {
		if decision := c.getMaterialized(ctx, req); decision != nil {
			// Promote L2 → L1
			c.setRedis(ctx, key, decision)
			return decision, "L2"
		}
	}

	return nil, ""
}

// Set stores a computed decision in L2 (materialized) and L1 (Redis).
func (c *layeredCache) Set(ctx context.Context, req AccessRequest, decision *AccessDecision) {
	key := cacheKey(req)

	// L2: Materialized
	if c.materialized != nil && c.version != nil {
		scope := versionScope(req)
		currentVersion, err := c.version.GetVersion(ctx, scope)
		if err == nil {
			allowed := decision.Decision == DecisionAllow
			if storeErr := c.materialized.Store(ctx, req.WorkspaceID, req.UserNodeID, req.ObjectNodeID, req.Operation, allowed, currentVersion); storeErr != nil {
				slog.Warn("failed to store materialized decision", "error", storeErr)
			}
		}
	}

	// L1: Redis
	c.setRedis(ctx, key, decision)
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

func (c *layeredCache) getMaterialized(ctx context.Context, req AccessRequest) *AccessDecision {
	scope := versionScope(req)
	currentVersion, err := c.version.GetVersion(ctx, scope)
	if err != nil {
		return nil
	}

	cached, err := c.materialized.Lookup(ctx, req.WorkspaceID, req.UserNodeID, req.ObjectNodeID, req.Operation, currentVersion)
	if err != nil || cached == nil {
		return nil
	}

	decision := DecisionDeny
	if cached.Decision {
		decision = DecisionAllow
	}

	return &AccessDecision{
		Decision:  decision,
		User:      req.UserNodeID,
		Object:    req.ObjectNodeID,
		Operation: req.Operation,
		Explanation: AccessExplanation{
			Reason: "resolved from materialized cache",
		},
	}
}

// cacheKey generates a workspace-isolated cache key for access decisions.
// When workspace_id is present, keys are prefixed to prevent cross-tenant collisions.
func cacheKey(req AccessRequest) string {
	if req.WorkspaceID != "" {
		return fmt.Sprintf("%s%s:%s:%s:%s",
			cacheKeyPrefix, req.WorkspaceID, req.UserNodeID, req.ObjectNodeID, req.Operation)
	}
	return fmt.Sprintf("%s%s:%s:%s",
		cacheKeyPrefix, req.UserNodeID, req.ObjectNodeID, req.Operation)
}

// versionScope returns the version tracking scope for a request.
// Per-workspace scope prevents cross-tenant invalidation storms.
func versionScope(req AccessRequest) string {
	if req.WorkspaceID != "" {
		return WorkspaceScope(req.WorkspaceID)
	}
	return ScopeGlobal
}
