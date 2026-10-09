package domain_test

import (
	"context"
	"testing"

	"ngac-platform/services/auth/internal/domain"
)

func TestIsPublicEmailDomain(t *testing.T) {
	for _, d := range []string{
		"gmail.com", "googlemail.com", "outlook.com", "hotmail.com", "live.com",
		"yahoo.com", "icloud.com", "proton.me", "protonmail.com", "gmx.com",
		"yandex.com", "mail.ru", "aol.com", "zoho.com",
		"GMAIL.COM", " gmail.com ", "gmail.com.",
	} {
		if !domain.IsPublicEmailDomain(d) {
			t.Errorf("IsPublicEmailDomain(%q) = false, want true", d)
		}
	}
	for _, d := range []string{"acme.com", "newco.io", "gmail.company.com", ""} {
		if domain.IsPublicEmailDomain(d) {
			t.Errorf("IsPublicEmailDomain(%q) = true, want false", d)
		}
	}
}

// The existing password signup path auto-joins by email domain. A tenant that
// sets domain = gmail.com must not capture every Gmail user who signs up.
func TestSignup_PublicDomainNeverAutoJoins(t *testing.T) {
	w := newFakeWorld()
	svc := w.service(t)
	squatter := w.addTenant("Gmail Squatter", "gmail.com")

	res, err := svc.Signup(context.Background(), "victim@gmail.com", "pw-123456", "Victim", "")
	if err != nil {
		t.Fatalf("signup: %v", err)
	}
	if res.TenantID == squatter || w.membership(squatter, res.UserID) != nil {
		t.Fatal("signup with a public email domain auto-joined the tenant claiming that domain")
	}
	if res.TenantRole != "owner" {
		t.Errorf("role = %q, want owner of a fresh tenant", res.TenantRole)
	}
}
