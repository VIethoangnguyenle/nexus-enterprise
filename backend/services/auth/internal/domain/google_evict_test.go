package domain_test

import (
	"context"
	"errors"
	"testing"

	"ngac-platform/services/auth/internal/auth"
	"ngac-platform/services/auth/internal/domain"
)

// Pre-hijacking: an attacker registers victim@acme.com with a password (signup
// never verifies the email) and waits. When the real owner signs in with
// Google, the account is linked to them — so whatever the attacker set up on
// it must stop working at that moment.

func seedPasswordUser(t *testing.T, w *fakeWorld, email, password string) string {
	t.Helper()
	u := w.addUser(email)
	hash, err := auth.HashPassword(password)
	if err != nil {
		t.Fatal(err)
	}
	w.mu.Lock()
	w.users[u.ID].Password = hash
	w.mu.Unlock()
	return u.ID
}

func TestGoogleLinkByEmail_ClearsUnverifiedPassword(t *testing.T) {
	w := newFakeWorld()
	svc := w.service(t)
	userID := seedPasswordUser(t, w, "victim@acme.com", "attacker-pw")

	if _, err := svc.Signin(context.Background(), "victim@acme.com", "attacker-pw"); err != nil {
		t.Fatalf("precondition: password works before linking: %v", err)
	}

	res, err := svc.SignInWithGoogle(context.Background(), googleIdentity("victim-sub", "victim@acme.com", ""))
	if err != nil {
		t.Fatalf("google sign in: %v", err)
	}
	if res.UserID != userID {
		t.Fatalf("linked to %q, want the existing account %q", res.UserID, userID)
	}
	if _, err := svc.Signin(context.Background(), "victim@acme.com", "attacker-pw"); !errors.Is(err, domain.ErrInvalidCredentials) {
		t.Fatalf("the pre-existing password still signs in after Google proved ownership: %v", err)
	}
}

func TestGoogleReturningUserBySubject_KeepsPassword(t *testing.T) {
	w := newFakeWorld()
	svc := w.service(t)
	userID := seedPasswordUser(t, w, "alice@acme.com", "alice-pw")
	_ = w.LinkIdentity(context.Background(), domain.ProviderGoogle, "alice-sub", userID, "alice@acme.com")

	if _, err := svc.SignInWithGoogle(context.Background(), googleIdentity("alice-sub", "alice@acme.com", "")); err != nil {
		t.Fatalf("google sign in: %v", err)
	}
	if _, err := svc.Signin(context.Background(), "alice@acme.com", "alice-pw"); err != nil {
		t.Fatalf("a returning, already-linked user must keep their password: %v", err)
	}
}

func TestGoogleLinkByEmail_RevokesExistingSessions(t *testing.T) {
	svc, w, _ := otpService(t, nil)
	userID := seedPasswordUser(t, w, "victim@acme.com", "attacker-pw")
	ctx := context.Background()

	// The attacker's session from before the real owner showed up.
	oldRefresh, err := svc.AttachRefreshToken(ctx, domain.RefreshIdentity{
		UserID: userID, Username: "victim", NGACNodeID: "n", SessionID: "attacker-session",
	})
	if err != nil {
		t.Fatalf("attach: %v", err)
	}

	res, err := svc.SignInWithGoogle(ctx, googleIdentity("victim-sub", "victim@acme.com", ""))
	if err != nil {
		t.Fatalf("google sign in: %v", err)
	}
	if _, _, err := svc.RefreshSession(ctx, oldRefresh); err == nil {
		t.Fatal("a refresh token issued before Google linking still works")
	}

	// The session the owner just got is unaffected.
	fresh, err := svc.AttachRefreshToken(ctx, domain.RefreshIdentity{
		UserID: res.UserID, Username: res.Username, NGACNodeID: res.NGACNodeID,
		TenantID: res.DefaultTenantID, SessionID: res.SessionID,
	})
	if err != nil {
		t.Fatalf("attach new: %v", err)
	}
	if _, _, err := svc.RefreshSession(ctx, fresh); err != nil {
		t.Fatalf("the new session must refresh: %v", err)
	}
}

func TestGoogleReturningUserBySubject_KeepsSessions(t *testing.T) {
	svc, w, _ := otpService(t, nil)
	userID := seedPasswordUser(t, w, "alice@acme.com", "alice-pw")
	_ = w.LinkIdentity(context.Background(), domain.ProviderGoogle, "alice-sub", userID, "alice@acme.com")
	ctx := context.Background()

	laptop, err := svc.AttachRefreshToken(ctx, domain.RefreshIdentity{
		UserID: userID, Username: "alice", NGACNodeID: "n", SessionID: "laptop-session",
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := svc.SignInWithGoogle(ctx, googleIdentity("alice-sub", "alice@acme.com", "")); err != nil {
		t.Fatal(err)
	}
	if _, _, err := svc.RefreshSession(ctx, laptop); err != nil {
		t.Fatalf("signing in on another device must not end existing sessions: %v", err)
	}
}
