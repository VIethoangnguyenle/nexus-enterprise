package ngac

import "strings"

// Redis key layout for cached access decisions and scope resolutions.
//
// This file is the only place that knows the layout. The writer
// (layeredCache, ReadServer's scope cache) and the invalidator
// (CacheInvalidator) both build keys and match patterns from here, so a pattern
// cannot drift away from the keys it is meant to delete. It already did once:
// decision keys gained a leading workspace segment and the invalidator kept
// deleting "ngac:access:<user>:*", which matched none of them.
//
// Decision key — always exactly four segments after the prefix:
//
//	ngac:access:<workspace>:<user>:<object>:<operation>
//
// <workspace> is the request's workspace ID, or keyNoWorkspace for a request
// without one. Keeping the segment count fixed is what lets the patterns below
// pin an ID to its position (node IDs are UUIDs and never contain ':').
//
// Scope key:
//
//	scopes:<user>:<operation>
const (
	cacheKeyPrefix = "ngac:access:"
	scopeKeyPrefix = "scopes:"

	// keyNoWorkspace fills the workspace segment for requests that carry no
	// workspace. It cannot collide with a workspace ID (those are UUIDs).
	keyNoWorkspace = "_global"
)

// DecisionCacheKey is the Redis key a decision for req is cached under.
func DecisionCacheKey(req AccessRequest) string {
	ws := req.WorkspaceID
	if ws == "" {
		ws = keyNoWorkspace
	}
	return cacheKeyPrefix + ws + ":" + req.UserNodeID + ":" + req.ObjectNodeID + ":" + req.Operation
}

// DecisionKeyPatternForUser matches every cached decision whose user segment
// is userID, in every workspace.
//
// "*:" consumes the workspace segment; ":*:*" requires two more segments after
// the user, which only the user position has — so an ID that happens to equal
// an object or workspace ID elsewhere does not match by accident.
func DecisionKeyPatternForUser(userID string) string {
	return cacheKeyPrefix + "*:" + escapeKeyGlob(userID) + ":*:*"
}

// DecisionKeyPatternForObject matches every cached decision whose object
// segment is objectID, in every workspace and for every user.
func DecisionKeyPatternForObject(objectID string) string {
	return cacheKeyPrefix + "*:*:" + escapeKeyGlob(objectID) + ":*"
}

// DecisionKeyPatternAll matches every cached decision.
func DecisionKeyPatternAll() string { return cacheKeyPrefix + "*" }

// ScopeCacheKey is the Redis key a ResolveAccessibleScopes result is cached under.
func ScopeCacheKey(userID, operation string) string {
	return scopeKeyPrefix + userID + ":" + operation
}

// ScopeKeyPatternForUser matches every cached scope resolution for userID.
func ScopeKeyPatternForUser(userID string) string {
	return scopeKeyPrefix + escapeKeyGlob(userID) + ":*"
}

// ScopeKeyPatternAll matches every cached scope resolution.
func ScopeKeyPatternAll() string { return scopeKeyPrefix + "*" }

// escapeKeyGlob escapes Redis glob metacharacters so an ID is matched literally.
var keyGlobEscaper = strings.NewReplacer(
	`\`, `\\`,
	`*`, `\*`,
	`?`, `\?`,
	`[`, `\[`,
	`]`, `\]`,
)

func escapeKeyGlob(s string) string { return keyGlobEscaper.Replace(s) }
