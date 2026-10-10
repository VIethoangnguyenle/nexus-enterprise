package domain_test

import (
	"context"
	"errors"
	"testing"

	"github.com/golang-jwt/jwt/v5"

	"ngac-platform/services/auth/internal/auth"
	"ngac-platform/services/auth/internal/domain"
)

func tenantOf(t *testing.T, token string) string {
	t.Helper()
	claims := &auth.Claims{}
	_, err := jwt.ParseWithClaims(token, claims, func(*jwt.Token) (any, error) {
		return []byte("test-secret-key-for-testing-only"), nil
	})
	if err != nil {
		t.Fatalf("parse token: %v", err)
	}
	return claims.TenantID
}

func TestSwitchTenant_IssuesATokenForTheTenantAnActiveMemberChose(t *testing.T) {
	w := newFakeWorld()
	svc := w.service(t)
	u := w.addUser("sw@example.test")
	home := w.addTenant("Home", "")
	other := w.addTenant("Other", "")
	_ = w.InsertTenantUser(context.Background(), home, u.ID, "owner", "active", u.NGACNodeID)
	_ = w.InsertTenantUser(context.Background(), other, u.ID, "member", "active", u.NGACNodeID)

	token, sessionID, info, err := svc.SwitchTenant(context.Background(), u.ID, u.NGACNodeID, u.Username, other)
	if err != nil {
		t.Fatal(err)
	}
	if got := tenantOf(t, token); got != other {
		t.Errorf("token tenant = %q, want %q", got, other)
	}
	if sessionID == "" || info.ID != other || info.Role != "member" {
		t.Errorf("session %q info %+v", sessionID, info)
	}
}

func TestSwitchTenant_RefusesWhoeverIsNotAnActiveMember(t *testing.T) {
	w := newFakeWorld()
	svc := w.service(t)
	u := w.addUser("sw2@example.test")
	stranger := w.addUser("stranger@example.test")
	ws := w.addTenant("Closed", "")
	_ = w.InsertTenantUser(context.Background(), ws, stranger.ID, "owner", "active", stranger.NGACNodeID)

	suspended := w.addTenant("Suspended", "")
	_ = w.InsertTenantUser(context.Background(), suspended, u.ID, "member", "disabled", u.NGACNodeID)
	pending := w.addTenant("Pending", "")
	_ = w.InsertTenantUser(context.Background(), pending, u.ID, "member", "invited", u.NGACNodeID)

	for name, target := range map[string]string{
		"not a member":          ws,
		"disabled membership":   suspended,
		"invited, not accepted": pending,
		"unknown tenant":        "no-such-tenant",
		"empty tenant":          "",
	} {
		token, _, _, err := svc.SwitchTenant(context.Background(), u.ID, u.NGACNodeID, u.Username, target)
		if !errors.Is(err, domain.ErrAccessDenied) {
			t.Errorf("%s: err = %v, want ErrAccessDenied", name, err)
		}
		if token != "" {
			t.Errorf("%s: a token was issued", name)
		}
	}
}

func TestGetMe_DoesNotPresentASuspendedMembershipAsTheCurrentTenant(t *testing.T) {
	w := newFakeWorld()
	svc := w.service(t)
	u := w.addUser("me3@example.test")
	suspended := w.addTenant("Suspended", "")
	_ = w.InsertTenantUser(context.Background(), suspended, u.ID, "member", "disabled", u.NGACNodeID)

	_, tenant, err := svc.GetMe(context.Background(), u.ID, suspended)
	if err != nil {
		t.Fatal(err)
	}
	if tenant != nil {
		t.Errorf("current tenant = %+v, want none for a disabled member", tenant)
	}
}
