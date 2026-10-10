package domain_test

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"

	"ngac-platform/services/auth/internal/auth"
	"ngac-platform/services/auth/internal/domain"
)

// Joining a company tenant by domain requires proof that the person controls
// an address at that domain. Only a Google Workspace account (hd) is proof; a
// consumer Google account at the same address, or a one-time code, is not.

func TestGoogle_ConsumerAccountNeverAutoJoinsClaimedDomain(t *testing.T) {
	w := newFakeWorld()
	svc := w.service(t)
	acme := w.addTenant("Acme", "acme.com")

	// No hd: Google vouches for the address, not for the company.
	res, err := svc.SignInWithGoogle(context.Background(), googleIdentity("sub-x", "x@acme.com", ""))
	if err != nil {
		t.Fatalf("sign in: %v", err)
	}
	if res.DefaultTenantID == acme || w.membership(acme, res.UserID) != nil {
		t.Fatal("a Google account without a hosted domain joined the tenant owning the email domain")
	}
	if m := w.membership(res.DefaultTenantID, res.UserID); m == nil || m.Role != "owner" {
		t.Errorf("membership = %+v, want owner of a personal workspace", m)
	}
	if tw, _ := w.FindTenantByDomain(context.Background(), "acme.com"); tw == nil || tw.ID != acme {
		t.Error("the personal workspace must not claim or take over the domain")
	}
}

func TestGoogle_ConsumerAccountOnUnclaimedDomainDoesNotClaimIt(t *testing.T) {
	w := newFakeWorld()
	svc := w.service(t)

	if _, err := svc.SignInWithGoogle(context.Background(), googleIdentity("sub-first", "first@newco.io", "")); err != nil {
		t.Fatalf("sign in: %v", err)
	}
	if tw, _ := w.FindTenantByDomain(context.Background(), "newco.io"); tw != nil {
		t.Fatal("an account without a hosted domain must not claim a domain for its workspace")
	}
}

// Allow side of the same rule: a Google Workspace account (hd) is proof.
func TestSignInWithGoogle_HostedDomainJoins(t *testing.T) {
	w := newFakeWorld()
	svc := w.service(t)
	acme := w.addTenant("Acme", "acme.com")

	res, err := svc.SignInWithGoogle(context.Background(), googleIdentity("sub-x", "x@acme.com", "acme.com"))
	if err != nil {
		t.Fatalf("sign in: %v", err)
	}
	if m := w.membership(acme, res.UserID); m == nil || m.Role != "member" {
		t.Fatalf("membership = %+v, want member — hd is proof of ownership", m)
	}
}

// The OTP flow does not prove email ownership: the code is a constant that is
// never sent to the address. So an OTP-registered user must not auto-join.
// Needs a disposable Redis (REDIS_ADDR), since OTP sessions live there.
func TestOTP_NewUserNeverAutoJoinsByDomain(t *testing.T) {
	addr := os.Getenv("REDIS_ADDR")
	if addr == "" {
		t.Skip("REDIS_ADDR not set")
	}
	rdb := redis.NewClient(&redis.Options{Addr: addr})
	if err := rdb.Ping(context.Background()).Err(); err != nil {
		t.Skipf("redis not available: %v", err)
	}
	t.Cleanup(func() { rdb.Close() })

	w := newFakeWorld()
	auth.SetJWTSecret("test-secret-key-for-testing-only")
	svc := domain.NewService(w, rdb, &fakePolicyRead{}, &fakePolicyWrite{w: w}, &fakeWorkspace{w: w}, &fakeMessaging{w: w})
	acme := w.addTenant("Acme", "acme.com")

	// Unique per run: OTP requests are rate-limited per identifier in Redis.
	email := fmt.Sprintf("x-%d@acme.com", time.Now().UnixNano())
	sessionID, err := svc.RequestOTP(context.Background(), email, "email")
	if err != nil {
		t.Fatalf("request otp: %v", err)
	}
	res, err := svc.VerifyOTP(context.Background(), sessionID, "999999")
	if err != nil {
		t.Fatalf("verify otp: %v", err)
	}
	if !res.IsNewUser {
		t.Fatal("expected a new user")
	}
	if w.membership(acme, res.UserID) != nil {
		t.Fatal("an OTP-registered user was auto-joined by domain; OTP does not prove email ownership")
	}
}

// Sign-in runs before any token exists, so the downstream workspace and channel
// RPCs must carry the user auth just created: otherwise their servers see no
// caller and refuse.
func TestSignIn_DownstreamRPCsCarryTheNewUserAsCaller(t *testing.T) {
	w := newFakeWorld()
	svc := w.service(t)

	res, err := svc.SignInWithGoogle(context.Background(), googleIdentity("sub-new", "new@example.org", ""))
	if err != nil {
		t.Fatalf("sign in: %v", err)
	}

	w.mu.Lock()
	defer w.mu.Unlock()
	for _, rpc := range []string{"CreateWorkspace", "CreateChannel"} {
		got := w.callers[rpc]
		if got.UserID != res.UserID || got.NGACNodeID != res.NGACNodeID || !got.Authenticated() {
			t.Errorf("%s caller = %+v, want user %s node %s", rpc, got, res.UserID, res.NGACNodeID)
		}
	}
}
