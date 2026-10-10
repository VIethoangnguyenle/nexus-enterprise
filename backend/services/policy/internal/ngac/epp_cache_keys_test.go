package ngac_test

import (
	"context"
	"errors"
	"path"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"ngac-platform/services/policy/internal/ngac"
)

// globMatch approximates Redis MATCH for keys whose segments contain no '/'
// (path.Match's '*' stops at '/'; Redis's does not). Node IDs are UUIDs, so the
// approximation is exact here. The Redis-backed test below checks the real thing.
func globMatch(t *testing.T, pattern, key string) bool {
	t.Helper()
	ok, err := path.Match(pattern, key)
	require.NoError(t, err)
	return ok
}

func TestDecisionKeyPatterns_MatchKeysTheCacheWrites(t *testing.T) {
	requests := []ngac.AccessRequest{
		{UserNodeID: "u1", ObjectNodeID: "o1", Operation: "read", WorkspaceID: "ws-1"},
		{UserNodeID: "u1", ObjectNodeID: "o1", Operation: "read"}, // no workspace
	}
	for _, req := range requests {
		key := ngac.DecisionCacheKey(req)
		assert.Truef(t, globMatch(t, ngac.DecisionKeyPatternForUser("u1"), key), "user pattern must match %q", key)
		assert.Truef(t, globMatch(t, ngac.DecisionKeyPatternForObject("o1"), key), "object pattern must match %q", key)
		assert.Truef(t, globMatch(t, ngac.DecisionKeyPatternAll(), key), "all pattern must match %q", key)
	}
}

// The old invalidator pattern, kept here as the regression it fixes.
func TestDecisionKeyPatterns_OldUserPatternMissedWorkspaceKeys(t *testing.T) {
	key := ngac.DecisionCacheKey(ngac.AccessRequest{UserNodeID: "u1", ObjectNodeID: "o1", Operation: "read", WorkspaceID: "ws-1"})
	assert.False(t, globMatch(t, "ngac:access:u1:*", key), "documents the bug: the old pattern matched no workspace key")
	assert.True(t, globMatch(t, ngac.DecisionKeyPatternForUser("u1"), key))
}

// Patterns are anchored to their segment: an ID that appears in another
// position must not match (no over- or cross-invalidation by coincidence).
func TestDecisionKeyPatterns_AreAnchoredToTheirSegment(t *testing.T) {
	// "x" is the workspace, the object, and the operation here — never the user.
	key := ngac.DecisionCacheKey(ngac.AccessRequest{UserNodeID: "u1", ObjectNodeID: "x", Operation: "x", WorkspaceID: "x"})
	assert.False(t, globMatch(t, ngac.DecisionKeyPatternForUser("x"), key))

	// "y" is the workspace, the user, and the operation — never the object.
	key = ngac.DecisionCacheKey(ngac.AccessRequest{UserNodeID: "y", ObjectNodeID: "o1", Operation: "y", WorkspaceID: "y"})
	assert.False(t, globMatch(t, ngac.DecisionKeyPatternForObject("y"), key))

	// A different user is untouched.
	key = ngac.DecisionCacheKey(ngac.AccessRequest{UserNodeID: "u2", ObjectNodeID: "o1", Operation: "read", WorkspaceID: "ws-1"})
	assert.False(t, globMatch(t, ngac.DecisionKeyPatternForUser("u1"), key))
}

func TestScopeKeyPattern_MatchesScopeKey(t *testing.T) {
	assert.True(t, globMatch(t, ngac.ScopeKeyPatternForUser("u1"), ngac.ScopeCacheKey("u1", "read")))
	assert.False(t, globMatch(t, ngac.ScopeKeyPatternForUser("u1"), ngac.ScopeCacheKey("u2", "read")))
}

func TestDecisionKeyPatterns_EscapeGlobMetacharacters(t *testing.T) {
	// An ID containing '*' must match only itself, not act as a wildcard.
	other := ngac.DecisionCacheKey(ngac.AccessRequest{UserNodeID: "uABC", ObjectNodeID: "o1", Operation: "read", WorkspaceID: "ws"})
	assert.False(t, globMatch(t, ngac.DecisionKeyPatternForUser("u*"), other))
}

// End to end against real Redis: what the decision cache writes is what the
// invalidator deletes — in every workspace, and nothing else.
func TestCacheInvalidator_DeletesKeysTheDecisionCacheWrote(t *testing.T) {
	rdb := testRedis(t, 11)
	ctx := context.Background()
	g := buildFailClosedGraph()
	cache := ngac.NewLayeredCache(rdb)
	inv := ngac.NewCacheInvalidator(rdb, func() *ngac.Graph { return g })

	allow := func(req ngac.AccessRequest) {
		cache.Set(ctx, req, &ngac.AccessDecision{Decision: ngac.DecisionAllow})
	}
	exists := func(req ngac.AccessRequest) bool {
		n, err := rdb.Exists(ctx, ngac.DecisionCacheKey(req)).Result()
		require.NoError(t, err)
		return n == 1
	}

	aliceWS := ngac.AccessRequest{UserNodeID: "u-alice", ObjectNodeID: "oa-docs", Operation: "read", WorkspaceID: "ws-1"}
	aliceGlobal := ngac.AccessRequest{UserNodeID: "u-alice", ObjectNodeID: "oa-docs", Operation: "read"}
	bobWS := ngac.AccessRequest{UserNodeID: "u-bob", ObjectNodeID: "oa-docs", Operation: "read", WorkspaceID: "ws-1"}
	bobOther := ngac.AccessRequest{UserNodeID: "u-bob", ObjectNodeID: "oa-other", Operation: "read", WorkspaceID: "ws-1"}
	for _, r := range []ngac.AccessRequest{aliceWS, aliceGlobal, bobWS, bobOther} {
		allow(r)
		require.True(t, exists(r))
	}
	require.NoError(t, rdb.Set(ctx, ngac.ScopeCacheKey("u-alice", "read"), "[]", 0).Err())

	// User invalidation: alice's keys go, in both shapes; bob's stay.
	inv.InvalidateForNodes(ctx, "u-alice")
	assert.False(t, exists(aliceWS), "workspace-scoped key must be invalidated")
	assert.False(t, exists(aliceGlobal))
	assert.True(t, exists(bobWS), "another user's decision must survive")
	n, _ := rdb.Exists(ctx, ngac.ScopeCacheKey("u-alice", "read")).Result()
	assert.Zero(t, n, "scope cache for the user is invalidated too")

	// UA invalidation reaches its users.
	allow(aliceWS)
	inv.InvalidateForNodes(ctx, "ua-staff")
	assert.False(t, exists(aliceWS), "UA change must invalidate its users' decisions")
	assert.True(t, exists(bobWS))

	// Object invalidation: every user's decision on that object goes.
	allow(aliceWS)
	inv.InvalidateForNodes(ctx, "oa-docs")
	assert.False(t, exists(aliceWS))
	assert.False(t, exists(bobWS))
	assert.True(t, exists(bobOther), "decision on another object must survive")
}

func TestLayeredCache_RefusesErrorDerivedDecision(t *testing.T) {
	rdb := testRedis(t, 11)
	ctx := context.Background()
	cache := ngac.NewLayeredCache(rdb)
	req := ngac.AccessRequest{UserNodeID: "u-alice", ObjectNodeID: "oa-docs", Operation: "read", WorkspaceID: "ws-1"}

	cache.Set(ctx, req, &ngac.AccessDecision{Decision: ngac.DecisionDeny, EvaluationErr: errors.New("db down")})

	n, err := rdb.Exists(ctx, ngac.DecisionCacheKey(req)).Result()
	require.NoError(t, err)
	assert.Zero(t, n, "an error-derived decision must never reach Redis")
	got, _ := cache.Get(ctx, req)
	assert.Nil(t, got)
}
