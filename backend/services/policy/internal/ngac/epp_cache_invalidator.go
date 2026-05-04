package ngac

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/redis/go-redis/v9"
)

// CacheInvalidator handles targeted Redis cache invalidation for NGAC access decisions.
// Instead of flushing all keys on every graph mutation, it resolves the affected
// user and object node IDs and deletes only their cached entries.
type CacheInvalidator struct {
	rdb   *redis.Client
	graph func() *Graph // lazy graph accessor (graph may be reloaded)
}

// NewCacheInvalidator creates a cache invalidator with targeted deletion strategy.
func NewCacheInvalidator(rdb *redis.Client, graphFn func() *Graph) *CacheInvalidator {
	return &CacheInvalidator{rdb: rdb, graph: graphFn}
}

// InvalidateForNodes resolves affected users and objects from the given nodeIDs,
// then deletes only their cached access decisions from Redis.
//
// Node type resolution:
//   - U (User): delete keys for that user
//   - UA (User Attribute): BFS descendants → collect all U nodes → delete their keys
//   - OA (Object Attribute): delete keys referencing that OA + descendant OAs
//   - PC (Policy Class): full flush (PC changes affect everything)
//
// Returns true if a full flush was performed (PC case), false for targeted deletion.
func (c *CacheInvalidator) InvalidateForNodes(ctx context.Context, nodeIDs ...string) bool {
	if c.rdb == nil {
		return false
	}

	graph := c.graph()
	if graph == nil {
		c.flushAll(ctx)
		return true
	}

	affectedUsers := make(map[string]bool)
	affectedObjects := make(map[string]bool)
	needsFullFlush := false

	for _, nodeID := range nodeIDs {
		node := graph.GetNode(nodeID)
		if node == nil {
			// Node not in graph — could be O type (not loaded) or deleted.
			// Invalidate by both user and object prefix as safety measure.
			affectedUsers[nodeID] = true
			affectedObjects[nodeID] = true
			slog.Debug("cache_invalidator: node not found in graph, invalidating both sides", "node_id", nodeID)
			continue
		}

		switch node.NodeType {
		case NodeTypeUser:
			affectedUsers[nodeID] = true

		case NodeTypeUserAttribute:
			// UA change affects all descendant users
			affectedUsers[nodeID] = true // UA itself may have cached scope entries
			descendants := graph.GetDescendants(nodeID)
			for id, n := range descendants {
				if n.NodeType == NodeTypeUser {
					affectedUsers[id] = true
				}
			}

		case NodeTypeObjectAttr:
			affectedObjects[nodeID] = true
			// OA change affects descendant OAs too (nested containers)
			descendants := graph.GetDescendants(nodeID)
			for id, n := range descendants {
				if n.NodeType == NodeTypeObjectAttr {
					affectedObjects[id] = true
				}
			}

		case NodeTypePolicyClass:
			// PC change affects the entire permission model → full flush
			needsFullFlush = true

		case NodeTypeObject:
			// O nodes are not cached in graph, but may have Redis keys
			affectedObjects[nodeID] = true
		}
	}

	if needsFullFlush {
		c.flushAll(ctx)
		slog.Info("cache_invalidator: full flush triggered by PC change", "node_ids", nodeIDs)
		return true
	}

	deleted := c.deleteTargetedKeys(ctx, affectedUsers, affectedObjects)
	slog.Info("cache_invalidator: targeted invalidation",
		"affected_users", len(affectedUsers),
		"affected_objects", len(affectedObjects),
		"keys_deleted", deleted,
	)
	return false
}

// FlushAll removes all cached access decisions and scope resolutions.
// Used only for LoadGraph (full graph reload).
func (c *CacheInvalidator) FlushAll(ctx context.Context) {
	if c.rdb == nil {
		return
	}
	c.flushAll(ctx)
}

// deleteTargetedKeys constructs Redis key patterns for affected users/objects,
// collects matching keys via SCAN, and deletes them using pipelined DEL commands.
func (c *CacheInvalidator) deleteTargetedKeys(ctx context.Context, users, objects map[string]bool) int {
	if len(users) == 0 && len(objects) == 0 {
		return 0
	}

	totalDeleted := 0
	pipe := c.rdb.Pipeline()

	// For each affected user: delete access keys and scope keys
	for userID := range users {
		totalDeleted += c.collectAndDelete(ctx, pipe,
			fmt.Sprintf("%s%s:*", cacheKeyPrefix, userID),
			fmt.Sprintf("%s%s:*", scopeKeyPrefix, userID),
		)
	}

	// For each affected object: delete keys containing this object ID
	for objectID := range objects {
		totalDeleted += c.collectAndDelete(ctx, pipe,
			fmt.Sprintf("%s*:%s:*", cacheKeyPrefix, objectID),
		)
	}

	if totalDeleted > 0 {
		if _, err := pipe.Exec(ctx); err != nil {
			slog.Warn("cache_invalidator: pipeline exec failed", "error", err)
		}
	}

	return totalDeleted
}

// collectAndDelete collects keys matching the given patterns and queues DEL commands.
// Returns the total number of keys queued for deletion.
func (c *CacheInvalidator) collectAndDelete(ctx context.Context, pipe redis.Pipeliner, patterns ...string) int {
	total := 0
	for _, pattern := range patterns {
		keys := c.collectKeys(ctx, pattern)
		for _, k := range keys {
			pipe.Del(ctx, k)
		}
		total += len(keys)
	}
	return total
}

// collectKeys returns keys matching a pattern. Uses SCAN with small COUNT
// scoped to the specific prefix pattern (not wildcard over entire keyspace).
func (c *CacheInvalidator) collectKeys(ctx context.Context, pattern string) []string {
	var keys []string
	iter := c.rdb.Scan(ctx, 0, pattern, 100).Iterator()
	for iter.Next(ctx) {
		keys = append(keys, iter.Val())
	}
	return keys
}

// flushAll performs the legacy full-flush of all access and scope cache entries.
func (c *CacheInvalidator) flushAll(ctx context.Context) {
	patterns := []string{cacheKeyPrefix + "*", scopeKeyPrefix + "*"}
	for _, pattern := range patterns {
		iter := c.rdb.Scan(ctx, 0, pattern, 1000).Iterator()
		var keys []string
		for iter.Next(ctx) {
			keys = append(keys, iter.Val())
		}
		if len(keys) > 0 {
			if err := c.rdb.Del(ctx, keys...).Err(); err != nil {
				slog.Warn("cache_invalidator: full flush failed", "pattern", pattern, "error", err)
			} else {
				slog.Debug("cache_invalidator: full flush complete", "pattern", pattern, "keys_deleted", len(keys))
			}
		}
	}
}
