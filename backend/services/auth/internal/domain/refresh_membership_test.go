package domain_test

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"

	"ngac-platform/services/auth/internal/auth"
	"ngac-platform/services/auth/internal/domain"
)

// A refresh asks the policy service whether the person still belongs to the tenant
// their token names. When it cannot answer, the answer is "try again", never "no":
// the token must not be spent and the caller must be told the service is
// unavailable, or a policy outage would sign everybody out.

// seedMember makes the person an active member of the tenant, in the row and in the graph.
func seedMember(t *testing.T, w *fakeWorld, tenant, userID, node string) {
	t.Helper()
	if err := w.InsertTenantUser(context.Background(), tenant, userID, "member", "active", node); err != nil {
		t.Fatal(err)
	}
	w.attach(node, tenant)
}

func TestRefreshSession_ThePolicyBeingDownIsNotAnAnswer(t *testing.T) {
	svc, w, _ := otpService(t, nil)
	ctx := context.Background()
	ws := w.addTenant("Acme", "")
	u := w.addUser(uniqueEmail())
	seedMember(t, w, ws, u.ID, u.NGACNodeID)
	token, err := svc.AttachRefreshToken(ctx, domain.RefreshIdentity{
		UserID: u.ID, Username: u.Username, NGACNodeID: u.NGACNodeID, TenantID: ws, SessionID: "s-" + uuid.NewString(),
	})
	if err != nil {
		t.Fatal(err)
	}

	w.policyDown = errors.New("policy: connection refused")
	_, _, _, err = svc.RefreshSession(ctx, token)
	if !errors.Is(err, domain.ErrUnavailable) {
		t.Fatalf("with the policy down err = %v, want ErrUnavailable", err)
	}
	if errors.Is(err, domain.ErrRefreshRejected) {
		t.Fatal("an outage must not look like a rejected token: the caller would clear the cookie")
	}

	// The same cookie, once the policy is back: the token was not spent.
	w.policyDown = nil
	access, next, tenant, err := svc.RefreshSession(ctx, token)
	if err != nil {
		t.Fatalf("the unspent token must refresh once the policy is back: %v", err)
	}
	if access == "" || next == "" || next == token || tenant != ws {
		t.Errorf("got access=%q next=%q tenant=%q, want fresh tokens for %q", access, next, tenant, ws)
	}
}

// The paired allow for the test above: with the policy up, an active member of the
// tenant keeps it. A refresh that always failed would pass the outage test alone.
func TestRefreshSession_AnActiveMemberKeepsTheirTenant(t *testing.T) {
	svc, w, _ := otpService(t, nil)
	ctx := context.Background()
	ws := w.addTenant("Acme", "")
	u := w.addUser(uniqueEmail())
	seedMember(t, w, ws, u.ID, u.NGACNodeID)
	token, _ := svc.AttachRefreshToken(ctx, domain.RefreshIdentity{
		UserID: u.ID, Username: u.Username, NGACNodeID: u.NGACNodeID, TenantID: ws, SessionID: "s-" + uuid.NewString(),
	})

	_, next, tenant, err := svc.RefreshSession(ctx, token)
	if err != nil || tenant != ws {
		t.Fatalf("tenant = %q, err = %v; want %q", tenant, err, ws)
	}
	if _, _, tenant, err = svc.RefreshSession(ctx, next); err != nil || tenant != ws {
		t.Fatalf("the rotated token must keep working: tenant = %q, err = %v", tenant, err)
	}
}

// A Redis that cannot be reached is an outage too, not a rejected token.
func TestRefreshSession_RedisBeingDownIsNotARejection(t *testing.T) {
	w := newFakeWorld()
	auth.SetJWTSecret("test-secret-key-for-testing-only")
	dead := redis.NewClient(&redis.Options{Addr: "127.0.0.1:1", MaxRetries: -1})
	t.Cleanup(func() { dead.Close() })
	svc := domain.NewService(w, dead, &fakePolicyRead{}, &fakePolicyWrite{w: w}, &fakeWorkspace{w: w}, &fakeMessaging{w: w})

	_, _, _, err := svc.RefreshSession(context.Background(), "some-token")

	if !errors.Is(err, domain.ErrUnavailable) {
		t.Fatalf("err = %v, want ErrUnavailable", err)
	}
	if errors.Is(err, domain.ErrRefreshRejected) {
		t.Fatal("a cookie must not be cleared because Redis is down")
	}
}
