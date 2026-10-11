package domain_test

import (
	"context"
	"errors"
	"testing"

	"ngac-platform/services/auth/internal/domain"
)

func googleIdentity(sub, email, hd string) domain.ExternalIdentity {
	return domain.ExternalIdentity{
		Provider:      domain.ProviderGoogle,
		Subject:       sub,
		Email:         email,
		EmailVerified: true,
		HostedDomain:  hd,
		DisplayName:   "Alice Example",
	}
}

func TestSignInWithGoogle_RejectsUnverifiedEmail(t *testing.T) {
	w := newFakeWorld()
	svc := w.service(t)

	id := googleIdentity("sub-1", "alice@acme.com", "acme.com")
	id.EmailVerified = false

	_, err := svc.SignInWithGoogle(context.Background(), id)
	if !errors.Is(err, domain.ErrEmailNotVerified) {
		t.Fatalf("err = %v, want ErrEmailNotVerified", err)
	}
	if w.userCount() != 0 {
		t.Errorf("an unverified email must not create an account, got %d users", w.userCount())
	}
}

// An unverified email must never be used to attach the Google login to an
// existing account — that is an account takeover.
func TestSignInWithGoogle_UnverifiedEmailNeverLinksExistingAccount(t *testing.T) {
	w := newFakeWorld()
	svc := w.service(t)
	existing := w.addUser("victim@acme.com")

	id := googleIdentity("attacker-sub", "victim@acme.com", "")
	id.EmailVerified = false

	if _, err := svc.SignInWithGoogle(context.Background(), id); err == nil {
		t.Fatal("expected rejection")
	}
	if subj, _ := w.GetIdentitySubject(context.Background(), domain.ProviderGoogle, existing.ID); subj != "" {
		t.Errorf("unverified email linked identity %q to the existing account", subj)
	}
}

func TestSignInWithGoogle_RequiresSubjectAndEmail(t *testing.T) {
	svc := newFakeWorld().service(t)
	for _, id := range []domain.ExternalIdentity{
		googleIdentity("", "a@acme.com", ""),
		googleIdentity("sub", "", ""),
	} {
		if _, err := svc.SignInWithGoogle(context.Background(), id); !errors.Is(err, domain.ErrInvalidInput) {
			t.Errorf("identity %+v: err = %v, want ErrInvalidInput", id, err)
		}
	}
}

func TestSignInWithGoogle_HostedDomainMatchesTenant_JoinsAsMember(t *testing.T) {
	w := newFakeWorld()
	svc := w.service(t)
	acme := w.addTenant("Acme", "acme.com")
	before := w.workspaceCount()

	res, err := svc.SignInWithGoogle(context.Background(), googleIdentity("sub-1", "alice@acme.com", "acme.com"))
	if err != nil {
		t.Fatalf("sign in: %v", err)
	}

	m := w.membership(acme, res.UserID)
	if m == nil {
		t.Fatal("user was not added to the tenant owning the hosted domain")
	}
	if m.Role != "member" {
		t.Errorf("role = %q, want member", m.Role)
	}
	if res.DefaultTenantID != acme {
		t.Errorf("default tenant = %q, want the domain tenant %q", res.DefaultTenantID, acme)
	}
	// Every new account has a workspace of its own, a company one included.
	if w.workspaceCount() != before+1 {
		t.Errorf("workspaces = %d, want the domain tenant plus one personal workspace", w.workspaceCount())
	}
	tenants, _ := w.ListTenantsByUser(context.Background(), res.UserID)
	var personal int
	for _, tm := range tenants {
		if tm.TenantID != acme && tm.Role == "owner" && w.workspaceType(tm.TenantID) == domain.WorkspacePersonal {
			personal++
		}
	}
	if personal != 1 {
		t.Errorf("tenants = %+v, want one personal workspace owned by the user besides the domain tenant", tenants)
	}
	if res.Token == "" || res.SessionID == "" {
		t.Error("sign-in must mint an access token bound to a session")
	}
}

func TestSignInWithGoogle_HostedDomainMatchIsCaseInsensitive(t *testing.T) {
	w := newFakeWorld()
	svc := w.service(t)
	acme := w.addTenant("Acme", "acme.com")

	res, err := svc.SignInWithGoogle(context.Background(), googleIdentity("sub-1", "Alice@ACME.com", "ACME.com"))
	if err != nil {
		t.Fatalf("sign in: %v", err)
	}
	if w.membership(acme, res.UserID) == nil {
		t.Fatal("hd differing only in case must still match the tenant")
	}
}

func TestSignInWithGoogle_HostedDomainNew_CreatesTenantAsOwner(t *testing.T) {
	// Signing in never creates an organization: a company account whose domain
	// no organization owns gets its personal workspace, like anyone else, and an
	// organization is made with "Tạo workspace".
	w := newFakeWorld()
	svc := w.service(t)

	res, err := svc.SignInWithGoogle(context.Background(), googleIdentity("sub-1", "ceo@newco.io", "newco.io"))
	if err != nil {
		t.Fatalf("sign in: %v", err)
	}
	if tenant, _ := w.FindTenantByDomain(context.Background(), "newco.io"); tenant != nil {
		t.Fatalf("sign-in claimed the domain for %q; it must not create an organization", tenant.ID)
	}
	tenants, _ := w.ListTenantsByUser(context.Background(), res.UserID)
	if len(tenants) != 1 || tenants[0].Role != "owner" || w.workspaceType(tenants[0].TenantID) != domain.WorkspacePersonal {
		t.Fatalf("tenants = %+v, want exactly one personal workspace owned by the user", tenants)
	}
	if res.DefaultTenantID != tenants[0].TenantID {
		t.Errorf("default tenant = %q, want the personal workspace", res.DefaultTenantID)
	}
	if len(w.channels) != 1 || w.channels[0] != tenants[0].TenantID {
		t.Errorf("#general must be provisioned in the personal workspace, got %v", w.channels)
	}

	// A colleague signing in next gets their own workspace, not the first one's.
	res2, err := svc.SignInWithGoogle(context.Background(), googleIdentity("sub-2", "dev@newco.io", "newco.io"))
	if err != nil {
		t.Fatalf("second sign in: %v", err)
	}
	if w.membership(tenants[0].TenantID, res2.UserID) != nil {
		t.Error("a colleague must not be placed in someone's personal workspace")
	}
}

// Consumer accounts carry no hd claim. Their email domain proves nothing about
// any company, so even a tenant that has (wrongly) claimed gmail.com must not
// receive them.
func TestSignInWithGoogle_ConsumerAccountNeverJoinsByDomain(t *testing.T) {
	w := newFakeWorld()
	svc := w.service(t)
	squatter := w.addTenant("Gmail Squatter", "gmail.com")

	res, err := svc.SignInWithGoogle(context.Background(), googleIdentity("sub-1", "someone@gmail.com", ""))
	if err != nil {
		t.Fatalf("sign in: %v", err)
	}
	if w.membership(squatter, res.UserID) != nil {
		t.Fatal("a consumer Gmail account was auto-joined to a tenant by email domain")
	}
	// Like a brand-new OTP/signup user with no company: a personal workspace.
	tenants, _ := w.ListTenantsByUser(context.Background(), res.UserID)
	if len(tenants) != 1 || tenants[0].Role != "owner" {
		t.Fatalf("tenants = %+v, want exactly one personal workspace owned by the user", tenants)
	}
	if tenants[0].TenantID == squatter {
		t.Fatal("personal workspace must not be the squatting tenant")
	}
	if tw, _ := w.FindTenantByDomain(context.Background(), "gmail.com"); tw == nil || tw.ID != squatter {
		t.Error("the personal workspace must not claim a domain")
	}
}

// A Google account registered on a company address without Google Workspace
// has no hd either. Only hd is trusted, so the email domain is ignored.
func TestSignInWithGoogle_CompanyEmailWithoutHostedDomainDoesNotJoin(t *testing.T) {
	w := newFakeWorld()
	svc := w.service(t)
	acme := w.addTenant("Acme", "acme.com")

	res, err := svc.SignInWithGoogle(context.Background(), googleIdentity("sub-1", "bob@acme.com", ""))
	if err != nil {
		t.Fatalf("sign in: %v", err)
	}
	if w.membership(acme, res.UserID) != nil {
		t.Fatal("email domain without hd must not grant tenant membership")
	}
}

// hd is set by Google only for Workspace domains, so a public domain there is
// not expected; if it ever appears it is still treated as a consumer account.
func TestSignInWithGoogle_PublicHostedDomainIsIgnored(t *testing.T) {
	w := newFakeWorld()
	svc := w.service(t)
	squatter := w.addTenant("Gmail Squatter", "gmail.com")

	res, err := svc.SignInWithGoogle(context.Background(), googleIdentity("sub-1", "x@gmail.com", "gmail.com"))
	if err != nil {
		t.Fatalf("sign in: %v", err)
	}
	if w.membership(squatter, res.UserID) != nil {
		t.Fatal("public hd must never auto-join")
	}
	claimed := 0
	for _, ws := range w.workspaces {
		if ws.Domain == "gmail.com" {
			claimed++
		}
	}
	if claimed != 1 {
		t.Errorf("a public domain must never be claimed by a new tenant, found %d tenants with it", claimed)
	}
}

func TestSignInWithGoogle_ExistingUserWithSameEmailIsLinkedNotDuplicated(t *testing.T) {
	w := newFakeWorld()
	svc := w.service(t)
	existing := w.addUser("alice@gmail.com")
	home := w.addTenant("Alice's Workspace", "")
	_ = w.InsertTenantUser(context.Background(), home, existing.ID, "owner", "active", existing.NGACNodeID)
	usersBefore, wsBefore := w.userCount(), w.workspaceCount()

	res, err := svc.SignInWithGoogle(context.Background(), googleIdentity("sub-1", "alice@gmail.com", ""))
	if err != nil {
		t.Fatalf("sign in: %v", err)
	}
	if res.UserID != existing.ID {
		t.Fatalf("user = %q, want the existing account %q", res.UserID, existing.ID)
	}
	if w.userCount() != usersBefore {
		t.Errorf("users = %d, want %d — linking must not duplicate the account", w.userCount(), usersBefore)
	}
	if w.workspaceCount() != wsBefore {
		t.Errorf("an existing user must keep their tenants, not get a new workspace")
	}
	if res.DefaultTenantID != home {
		t.Errorf("default tenant = %q, want the user's existing tenant %q", res.DefaultTenantID, home)
	}
	if subj, _ := w.GetIdentitySubject(context.Background(), domain.ProviderGoogle, existing.ID); subj != "sub-1" {
		t.Errorf("identity subject = %q, want sub-1", subj)
	}
}

func TestSignInWithGoogle_ReturningUserIsFoundBySubjectEvenIfEmailChanged(t *testing.T) {
	w := newFakeWorld()
	svc := w.service(t)

	first, err := svc.SignInWithGoogle(context.Background(), googleIdentity("sub-1", "alice@acme.com", "acme.com"))
	if err != nil {
		t.Fatalf("first sign in: %v", err)
	}
	second, err := svc.SignInWithGoogle(context.Background(), googleIdentity("sub-1", "alice.renamed@acme.com", "acme.com"))
	if err != nil {
		t.Fatalf("second sign in: %v", err)
	}
	if second.UserID != first.UserID {
		t.Errorf("same Google subject resolved to a different user (%q vs %q)", second.UserID, first.UserID)
	}
	if w.userCount() != 1 {
		t.Errorf("users = %d, want 1", w.userCount())
	}
}

// Workspace admins can delete an account and hand its address to someone else,
// who gets a new subject. The new person must not inherit the old account.
func TestSignInWithGoogle_EmailAlreadyLinkedToAnotherSubjectIsAConflict(t *testing.T) {
	w := newFakeWorld()
	svc := w.service(t)

	if _, err := svc.SignInWithGoogle(context.Background(), googleIdentity("old-sub", "alice@acme.com", "acme.com")); err != nil {
		t.Fatalf("first sign in: %v", err)
	}
	_, err := svc.SignInWithGoogle(context.Background(), googleIdentity("new-sub", "alice@acme.com", "acme.com"))
	if !errors.Is(err, domain.ErrIdentityConflict) {
		t.Fatalf("err = %v, want ErrIdentityConflict", err)
	}
}

func TestSignInWithGoogle_ExistingMemberIsNotRejoined(t *testing.T) {
	w := newFakeWorld()
	svc := w.service(t)
	acme := w.addTenant("Acme", "acme.com")
	existing := w.addUser("alice@acme.com")
	_ = w.InsertTenantUser(context.Background(), acme, existing.ID, "admin", "active", existing.NGACNodeID)

	res, err := svc.SignInWithGoogle(context.Background(), googleIdentity("sub-1", "alice@acme.com", "acme.com"))
	if err != nil {
		t.Fatalf("sign in: %v", err)
	}
	if m := w.membership(acme, res.UserID); m == nil || m.Role != "admin" {
		t.Errorf("membership = %+v, want the existing admin role untouched", m)
	}
}

func TestSignInWithGoogle_DisabledMembershipStaysDisabled(t *testing.T) {
	w := newFakeWorld()
	svc := w.service(t)
	acme := w.addTenant("Acme", "acme.com")
	existing := w.addUser("alice@acme.com")
	_ = w.InsertTenantUser(context.Background(), acme, existing.ID, "member", "disabled", existing.NGACNodeID)

	res, err := svc.SignInWithGoogle(context.Background(), googleIdentity("sub-1", "alice@acme.com", "acme.com"))
	if err != nil {
		t.Fatalf("sign in: %v", err)
	}
	if m := w.membership(acme, res.UserID); m == nil || m.Status != "disabled" {
		t.Errorf("membership = %+v, a disabled member must not be re-activated by signing in", m)
	}
	if res.DefaultTenantID == acme {
		t.Error("a disabled membership must not become the session's tenant")
	}
}
