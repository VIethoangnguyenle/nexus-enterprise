package domain_test

import (
	"context"
	"os"
	"testing"

	"github.com/redis/go-redis/v9"

	"ngac-platform/services/auth/internal/auth"
	"ngac-platform/services/auth/internal/domain"
)

// Joining a company tenant by domain requires proof that the person controls
// an address at that domain. A password signup proves nothing — anyone can
// type ceo@acme.com — so it must never land in Acme's tenant.

func TestSignup_PasswordNeverAutoJoinsClaimedDomain(t *testing.T) {
	w := newFakeWorld()
	svc := w.service(t)
	acme := w.addTenant("Acme", "acme.com")

	res, err := svc.Signup(context.Background(), "x@acme.com", "pw-123456", "X", "")
	if err != nil {
		t.Fatalf("signup: %v", err)
	}
	if res.TenantID == acme || w.membership(acme, res.UserID) != nil {
		t.Fatal("password signup joined the tenant owning the email domain without proof of ownership")
	}
	if res.TenantRole != "owner" {
		t.Errorf("role = %q, want owner of a personal workspace", res.TenantRole)
	}
	if tw, _ := w.FindTenantByDomain(context.Background(), "acme.com"); tw == nil || tw.ID != acme {
		t.Error("the personal workspace must not claim or take over the domain")
	}
}

func TestSignup_PasswordOnUnclaimedDomainDoesNotClaimIt(t *testing.T) {
	w := newFakeWorld()
	svc := w.service(t)

	if _, err := svc.Signup(context.Background(), "first@newco.io", "pw-123456", "First", ""); err != nil {
		t.Fatalf("signup: %v", err)
	}
	if tw, _ := w.FindTenantByDomain(context.Background(), "newco.io"); tw != nil {
		t.Fatal("an unverified signup must not claim a domain for its workspace")
	}
}

func TestRegister_LegacyNeverAutoJoinsByDomain(t *testing.T) {
	w := newFakeWorld()
	svc := w.service(t)
	acme := w.addTenant("Acme", "acme.com")

	res, err := svc.Register(context.Background(), "eve@acme.com", "pw-123456")
	if err != nil {
		t.Fatalf("register: %v", err)
	}
	if w.membership(acme, res.UserID) != nil {
		t.Fatal("legacy register joined a tenant by domain")
	}
}

// Allow side of the same rule: a Google Workspace account (hd) is proof.
func TestSignInWithGoogle_HostedDomainStillJoinsAfterSignupChange(t *testing.T) {
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

	sessionID, err := svc.RequestOTP(context.Background(), "x@acme.com", "email")
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
