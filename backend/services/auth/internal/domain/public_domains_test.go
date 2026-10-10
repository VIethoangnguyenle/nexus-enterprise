package domain_test

import (
	"context"
	"fmt"
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

// A tenant that sets domain = gmail.com must not capture every Gmail user, on
// any sign-in path.
func TestPublicDomainNeverAutoJoins(t *testing.T) {
	w := newFakeWorld()
	svc := w.service(t)
	squatter := w.addTenant("Gmail Squatter", "gmail.com")

	// A consumer account, and a (forged or mistaken) hosted domain of gmail.com.
	for i, hd := range []string{"", "gmail.com"} {
		res, err := svc.SignInWithGoogle(context.Background(), googleIdentity(fmt.Sprintf("sub-%d", i), fmt.Sprintf("victim%d@gmail.com", i), hd))
		if err != nil {
			t.Fatalf("sign in (hd %q): %v", hd, err)
		}
		if res.DefaultTenantID == squatter || w.membership(squatter, res.UserID) != nil {
			t.Fatalf("hd %q: a public email domain auto-joined the tenant claiming it", hd)
		}
		if m := w.membership(res.DefaultTenantID, res.UserID); m == nil || m.Role != "owner" {
			t.Errorf("hd %q: membership = %+v, want owner of a fresh tenant", hd, m)
		}
	}
}
