//go:build integration
// +build integration

package domain_test

import (
	"context"
	"os"
	"testing"

	"github.com/redis/go-redis/v9"

	"ngac-platform/services/auth/internal/domain"
)

func testRedis(t *testing.T) *redis.Client {
	t.Helper()
	addr := os.Getenv("REDIS_ADDR")
	if addr == "" {
		addr = "localhost:6379"
	}
	rdb := redis.NewClient(&redis.Options{Addr: addr})
	if err := rdb.Ping(context.Background()).Err(); err != nil {
		t.Skipf("redis not available: %v", err)
	}
	t.Cleanup(func() { rdb.Close() })
	return rdb
}

func newRefreshStore(t *testing.T) *domain.RefreshStore {
	t.Helper()
	return domain.NewRefreshStore(testRedis(t))
}

var identity = domain.RefreshIdentity{
	UserID: "user-1", Username: "alice", NGACNodeID: "ngac-1", TenantID: "tenant-1",
}

func TestRefresh_IssueThenRotate(t *testing.T) {
	rs := newRefreshStore(t)
	ctx := context.Background()

	first, sessionID, err := rs.Issue(ctx, identity)
	if err != nil {
		t.Fatalf("issue: %v", err)
	}
	t.Cleanup(func() { rs.RevokeSession(ctx, sessionID) })
	if first == "" || sessionID == "" {
		t.Fatal("issue returned an empty token or session")
	}

	second, gotIdentity, err := rs.Rotate(ctx, first)
	if err != nil {
		t.Fatalf("rotate: %v", err)
	}
	if second == first {
		t.Error("rotation must mint a new token, got the same one back")
	}
	if gotIdentity.UserID != identity.UserID || gotIdentity.TenantID != identity.TenantID {
		t.Errorf("identity = %+v, want %+v", gotIdentity, identity)
	}
	if gotIdentity.SessionID != sessionID {
		t.Errorf("session = %q, want the family to be preserved across rotation (%q)", gotIdentity.SessionID, sessionID)
	}
}

// The point of rotation: a token that has already been exchanged must not work
// a second time. If it is presented again, the copy is in someone else's hands
// and the whole family has to die — including the token the legitimate client
// is currently holding.
func TestRefresh_ReuseKillsTheWholeSession(t *testing.T) {
	rs := newRefreshStore(t)
	ctx := context.Background()

	first, sessionID, err := rs.Issue(ctx, identity)
	if err != nil {
		t.Fatalf("issue: %v", err)
	}
	t.Cleanup(func() { rs.RevokeSession(ctx, sessionID) })

	second, _, err := rs.Rotate(ctx, first)
	if err != nil {
		t.Fatalf("first rotate: %v", err)
	}

	// Replay the already-spent token.
	if _, _, err := rs.Rotate(ctx, first); err == nil {
		t.Fatal("replaying a spent refresh token must fail")
	}

	// And the live token from the same family must now be dead too.
	if _, _, err := rs.Rotate(ctx, second); err == nil {
		t.Error("reuse detection must revoke the whole session, but the live token still worked")
	}
}

func TestRefresh_UnknownTokenRejected(t *testing.T) {
	rs := newRefreshStore(t)
	ctx := context.Background()

	if _, _, err := rs.Rotate(ctx, "not-a-real-token"); err == nil {
		t.Error("an unknown refresh token must be rejected")
	}
}

// Logout has to invalidate the refresh token immediately — the access token
// still lives out its short TTL, but no new one can be minted.
func TestRefresh_RevokeSession(t *testing.T) {
	rs := newRefreshStore(t)
	ctx := context.Background()

	token, sessionID, err := rs.Issue(ctx, identity)
	if err != nil {
		t.Fatalf("issue: %v", err)
	}

	if err := rs.RevokeSession(ctx, sessionID); err != nil {
		t.Fatalf("revoke: %v", err)
	}
	if _, _, err := rs.Rotate(ctx, token); err == nil {
		t.Error("a revoked session must not be refreshable")
	}
}

// Two sessions for the same user are independent: signing out on one device
// must not sign the user out everywhere.
func TestRefresh_SessionsAreIndependent(t *testing.T) {
	rs := newRefreshStore(t)
	ctx := context.Background()

	tokenA, sessionA, err := rs.Issue(ctx, identity)
	if err != nil {
		t.Fatalf("issue A: %v", err)
	}
	tokenB, sessionB, err := rs.Issue(ctx, identity)
	if err != nil {
		t.Fatalf("issue B: %v", err)
	}
	t.Cleanup(func() { rs.RevokeSession(ctx, sessionB) })

	if sessionA == sessionB {
		t.Fatal("two sign-ins must produce distinct sessions")
	}
	if err := rs.RevokeSession(ctx, sessionA); err != nil {
		t.Fatalf("revoke A: %v", err)
	}
	if _, _, err := rs.Rotate(ctx, tokenA); err == nil {
		t.Error("session A should be dead")
	}
	if _, _, err := rs.Rotate(ctx, tokenB); err != nil {
		t.Errorf("session B must survive a logout on session A: %v", err)
	}
}
